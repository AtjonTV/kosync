//
// File:        internal/mail/summary.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package mail

import (
	"fmt"
	"net/mail"
	"strings"
	"time"

	"git.obth.eu/atjontv/kosync/internal/achievements"
	"git.obth.eu/atjontv/kosync/internal/config"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/summary"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/mailer"
)

// Cron job id and schedule for the summaries.
//
// Hourly, because the hour a summary should arrive at is a local one and this
// server has accounts in whatever zones they say they are in. The job itself
// decides who is owed anything, which on all but one run an hour is nobody.
const (
	JobSummaries      = "kosync.mail.summaries"
	scheduleSummaries = "5 * * * *"
)

// RegisterSummaries adds the job that sends the weekly and monthly reports.
//
// Nothing is added at all when the operator has turned them off, so an instance
// that sends no mail of its own is not running a job every hour to decide that
// again.
func RegisterSummaries(app core.App, conf *config.Config) {
	if !conf.EnableSummaryMail {
		return
	}

	app.Cron().MustAdd(JobSummaries, scheduleSummaries, func() {
		sent, err := Summaries(app, time.Now())
		if err != nil {
			app.Logger().Error("failed to send the reading summaries", "error", err)

			return
		}
		if sent > 0 {
			app.Logger().Info("sent reading summaries", "count", sent)
		}
	})
}

// Summaries sends every summary that is due at the given moment.
//
// The moment is a parameter so that a test can be a Monday morning in Vienna
// without waiting for one.
func Summaries(app core.App, now time.Time) (int, error) {
	wanted, err := summary.Wanted(app)
	if err != nil {
		return 0, err
	}

	sent := 0
	for _, owner := range wanted {
		period, due := summary.Due(owner, now)
		if !due {
			continue
		}

		delivered, err := sendSummary(app, owner, period)
		if err != nil {
			// One account's broken address is not a reason to skip everybody
			// else's summary, and the period is marked either way, so this is
			// not the start of an hourly retry.
			app.Logger().Warn("failed to send a reading summary",
				"owner", owner.Id, "period", period.Key, "error", err)

			continue
		}
		if delivered {
			sent++
		}
	}

	return sent, nil
}

// sendSummary writes one account's report and marks the period as covered.
//
// The mark goes down before the message goes out, and deliberately: a mail
// server that is refusing connections would otherwise be retried on every hourly
// run for the rest of the week. A summary is a courtesy about reading that is
// already recorded and already on the dashboard — missing one is a smaller harm
// than sending a hundred attempts, or than sending the same summary twice
// because the failure happened after the message had been accepted.
func sendSummary(app core.App, owner *core.Record, period summary.Period) (bool, error) {
	stats, err := summary.For(app, owner, period)
	if err != nil {
		return false, err
	}

	owner.Set(schema.FieldSummarySent, period.Key)
	if err := app.Save(owner); err != nil {
		return false, fmt.Errorf("record the summary as sent: %w", err)
	}

	// A week nobody read in is not worth a message. The period is still marked,
	// so this does not become a daily reminder of not having read.
	if stats.IsEmpty() {
		return false, nil
	}

	address, ok := deliverableAddress(owner)
	if !ok {
		return false, nil
	}

	message := &mailer.Message{
		From: mail.Address{
			Name:    app.Settings().Meta.SenderName,
			Address: app.Settings().Meta.SenderAddress,
		},
		To:      []mail.Address{{Address: address}},
		Subject: summarySubject(stats),
		HTML:    summaryBody(app, stats, true),
		Text:    summaryBody(app, stats, false),
	}

	if err := app.NewMailClient().Send(message); err != nil {
		return false, fmt.Errorf("send the summary: %w", err)
	}

	return true, nil
}

// summarySubject leads with the number somebody would want to know.
//
// "Your week in books" says nothing that the sender's name did not already say.
// "312 pages last week" is the summary; opening the mail is for the rest of it.
func summarySubject(stats summary.Stats) string {
	if stats.Period.Kind == schema.SummaryMonthly {
		return fmt.Sprintf("%s pages in %s", plural(stats.Pages), stats.Period.Title)
	}

	return fmt.Sprintf("%s pages last week", plural(stats.Pages))
}

// plural writes a count with a thousands separator, which matters exactly once a
// month and costs nothing the rest of the time.
func plural(count int) string {
	text := fmt.Sprintf("%d", count)
	if len(text) <= 3 {
		return text
	}

	out := new(strings.Builder)
	for index, digit := range text {
		if index > 0 && (len(text)-index)%3 == 0 {
			out.WriteRune(',')
		}
		out.WriteRune(digit)
	}

	return out.String()
}

// summaryBody writes the report in both of the forms it is sent in.
func summaryBody(app core.App, stats summary.Stats, asHTML bool) string {
	out := new(strings.Builder)
	url := strings.TrimRight(app.Settings().Meta.AppURL, "/")

	line := func(text string) {
		if asHTML {
			out.WriteString("<p>" + text + "</p>\n")

			return
		}

		out.WriteString(text + "\n\n")
	}

	item := func(text string) {
		if asHTML {
			out.WriteString("<p><strong>" + text + "</strong></p>\n")

			return
		}

		out.WriteString("  " + text + "\n")
	}

	line(fmt.Sprintf("Here is %s.", stats.Period.Title))

	line(fmt.Sprintf("You read %s pages over %s, on %s.",
		plural(stats.Pages), stats.Hours(), days(stats.DaysRead)))

	if stats.BestPages > 0 {
		line(fmt.Sprintf("Your best day was %s, with %s pages.",
			stats.BestDate, plural(stats.BestPages)))
	}

	if len(stats.Books) > 0 {
		line("What you were reading:")

		for _, book := range stats.Books {
			text := fmt.Sprintf("%s — %s pages", book.Title, plural(book.Pages))
			if book.Finished {
				text += ", finished"
			}
			item(text)
		}

		if stats.MoreBooks > 0 {
			line(fmt.Sprintf("…and %d more.", stats.MoreBooks))
		}
		if !asHTML {
			out.WriteString("\n")
		}
	}

	if len(stats.Achievements) > 0 {
		line("You also earned:")

		for _, award := range stats.Achievements {
			item(awardName(award))
		}

		if !asHTML {
			out.WriteString("\n")
		}
	}

	if url != "" {
		if asHTML {
			line(fmt.Sprintf(`The rest is at <a href="%s">%s</a>.`, url, url))
		} else {
			line("The rest is at " + url)
		}
	}

	line("You can change how often this arrives, or stop it, under Account in the web interface.")

	return out.String()
}

// awardName names one earned tier, from the rules rather than from a second copy
// of their names kept here.
func awardName(award summary.Award) string {
	name := award.Rule
	if rule, ok := achievements.FindRule(award.Rule); ok {
		name = rule.Name
	}

	return fmt.Sprintf("%s — %s", name, tierName(award.Tier))
}

// days writes the number of days read as a phrase, because "on 1 days" is the
// kind of sentence that makes a person trust nothing else in the message.
func days(count int) string {
	if count == 1 {
		return "one day"
	}

	return fmt.Sprintf("%d days", count)
}
