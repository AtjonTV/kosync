//
// File:        internal/mail/mail.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

// Package mail sends the one message KOsync writes itself.
//
// Everything else that arrives from this server — verification, password reset,
// the confirmation of an address change — is PocketBase's, sent from its own
// templates and configured in the superuser interface. There is nothing to
// reimplement there and no reason to have a second opinion about it.
//
// What is left is the achievement notice, which nothing else could send because
// nothing else knows what an achievement is.
package mail

import (
	"fmt"
	"net/mail"
	"strings"

	"git.obth.eu/atjontv/kosync/internal/achievements"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/mailer"
)

// Achievements tells an account what it has just earned.
//
// One message for the whole batch, never one per badge: a first evaluation of an
// account that has been reading for years awards a dozen tiers at once, and a
// dozen mails about it would be a bug rather than a celebration.
//
// It reports whether anything was sent, so a caller can say so in a log without
// having to know the rules about when it is skipped.
func Achievements(app core.App, ownerId string, earned []achievements.Awarded) (bool, error) {
	if len(earned) == 0 {
		return false, nil
	}

	owner, err := app.FindRecordById(schema.CollectionUsers, ownerId)
	if err != nil {
		return false, fmt.Errorf("find the account: %w", err)
	}

	if !owner.GetBool(schema.FieldAchievementMail) {
		return false, nil
	}

	// An unverified address is either one nobody has proved they can read or one
	// of the placeholders the legacy import parks accounts on, and mail to those
	// bounces forever without anybody being told. The interface says as much
	// beside the setting.
	if !owner.Verified() {
		return false, nil
	}

	address := strings.TrimSpace(owner.Email())
	if address == "" {
		return false, nil
	}

	message := &mailer.Message{
		From: mail.Address{
			Name:    app.Settings().Meta.SenderName,
			Address: app.Settings().Meta.SenderAddress,
		},
		To:      []mail.Address{{Address: address}},
		Subject: subject(earned),
		HTML:    body(app, earned, true),
		Text:    body(app, earned, false),
	}

	if err := app.NewMailClient().Send(message); err != nil {
		return false, fmt.Errorf("send the achievement mail: %w", err)
	}

	return true, nil
}

// subject names the badge when there is one and counts them when there are more.
//
// "You earned Night Prowler, bronze" is worth opening. "You earned 5 new
// achievements" is worth opening. "KOsync notification" is not.
func subject(earned []achievements.Awarded) string {
	if len(earned) == 1 {
		return fmt.Sprintf("You earned %s, %s", earned[0].Rule.Name, tierName(earned[0].Tier))
	}

	return fmt.Sprintf("You earned %d new achievements", len(earned))
}

// body writes the message in both of the forms it is sent in.
//
// The badges are cats and the cats are SVG, which mail clients either strip or
// refuse to draw, so the message says what was earned in words and links to the
// place the drawings live.
func body(app core.App, earned []achievements.Awarded, asHTML bool) string {
	out := new(strings.Builder)
	url := strings.TrimRight(app.Settings().Meta.AppURL, "/")

	line := func(text string) {
		if asHTML {
			out.WriteString("<p>" + text + "</p>\n")

			return
		}

		out.WriteString(text + "\n\n")
	}

	if len(earned) == 1 {
		line("You have earned an achievement:")
	} else {
		line("You have earned some achievements:")
	}

	for _, award := range earned {
		item := fmt.Sprintf("%s — %s (%d %s). %s",
			award.Rule.Name, tierName(award.Tier), award.Value, award.Rule.Unit, award.Rule.Summary)

		if asHTML {
			out.WriteString("<p><strong>" + item + "</strong></p>\n")

			continue
		}

		out.WriteString("  " + item + "\n")
	}

	if !asHTML {
		out.WriteString("\n")
	}

	if url != "" {
		if asHTML {
			line(fmt.Sprintf(`The cats are at <a href="%s">%s</a>.`, url, url))
		} else {
			line("The cats are at " + url)
		}
	}

	line("You can turn these off under Account in the web interface.")

	return out.String()
}

// tierName is the level in words, taken from the rules rather than named again
// here, and falling back to the number so that adding a fourth tier one day
// cannot produce a sentence with a blank in it.
func tierName(tier int) string {
	if tier >= 1 && tier <= len(achievements.TierNames) {
		return achievements.TierNames[tier-1]
	}

	return fmt.Sprintf("tier %d", tier)
}
