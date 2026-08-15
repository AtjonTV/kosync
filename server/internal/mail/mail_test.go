//
// File:        internal/mail/mail_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package mail_test

import (
	"strings"
	"testing"

	"git.obth.eu/atjontv/kosync/internal/achievements"
	"git.obth.eu/atjontv/kosync/internal/mail"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/testutil"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

// newApp returns a migrated app with one account that has asked for the mail.
func newApp(t testing.TB) (*tests.TestApp, *core.Record) {
	t.Helper()

	app := testutil.NewApp(t)
	user := testutil.CreateUser(t, app, testutil.IdUserA, testutil.EmailUserA, testutil.PasswordUsers)
	user.Set(schema.FieldAchievementMail, true)
	if err := app.Save(user); err != nil {
		t.Fatalf("failed to ask for the mail: %v", err)
	}

	return app, user
}

// awardOf builds one award from a real rule, so the message is written from the
// same names the interface shows.
func awardOf(t testing.TB, slug string, tier, value int) achievements.Awarded {
	t.Helper()

	rule, ok := achievements.FindRule(slug)
	if !ok {
		t.Fatalf("no rule named %q", slug)
	}

	return achievements.Awarded{Rule: rule, Tier: tier, Value: value}
}

func TestOneAchievementIsNamedInTheSubject(t *testing.T) {
	app, user := newApp(t)

	sent, err := mail.Achievements(app, user.Id,
		[]achievements.Awarded{awardOf(t, "night-prowler", 1, 3)})
	if err != nil {
		t.Fatalf("failed to send: %v", err)
	}
	if !sent {
		t.Fatal("expected the mail to be sent")
	}
	if app.TestMailer.TotalSend() != 1 {
		t.Fatalf("expected one message, got %d", app.TestMailer.TotalSend())
	}

	message := app.TestMailer.FirstMessage()
	if !strings.Contains(message.Subject, "Night Prowler") {
		t.Errorf("expected the badge in the subject, got %q", message.Subject)
	}
	if !strings.Contains(message.Subject, "bronze") {
		t.Errorf("expected the tier in the subject, got %q", message.Subject)
	}
	if len(message.To) != 1 || message.To[0].Address != testutil.EmailUserA {
		t.Errorf("expected the mail to go to the account, got %v", message.To)
	}
	// Both forms are sent, because a mail client picks the one it can draw.
	if message.Text == "" || message.HTML == "" {
		t.Error("expected the message to carry both a text and an HTML body")
	}
	if !strings.Contains(message.Text, "Night Prowler") {
		t.Errorf("expected the badge in the body, got %q", message.Text)
	}
}

// A first evaluation of an account that has been reading for years awards a
// dozen tiers at once. That is one mail.
func TestManyAchievementsAreOneMessage(t *testing.T) {
	app, user := newApp(t)

	earned := []achievements.Awarded{
		awardOf(t, "first-pounce", 1, 12),
		awardOf(t, "first-pounce", 2, 12),
		awardOf(t, "lap-warmer", 1, 9),
	}

	if _, err := mail.Achievements(app, user.Id, earned); err != nil {
		t.Fatalf("failed to send: %v", err)
	}
	if app.TestMailer.TotalSend() != 1 {
		t.Fatalf("expected one message for three awards, got %d", app.TestMailer.TotalSend())
	}

	message := app.TestMailer.FirstMessage()
	if !strings.Contains(message.Subject, "3") {
		t.Errorf("expected the count in the subject, got %q", message.Subject)
	}
	for _, want := range []string{"First Pounce", "Lap Warmer", "bronze", "silver"} {
		if !strings.Contains(message.Text, want) {
			t.Errorf("expected %q in the body, got %q", want, message.Text)
		}
	}
}

func TestNothingIsSentWithoutTheAccountAskingFirst(t *testing.T) {
	app, user := newApp(t)
	user.Set(schema.FieldAchievementMail, false)
	if err := app.Save(user); err != nil {
		t.Fatalf("failed to turn the mail off: %v", err)
	}

	sent, err := mail.Achievements(app, user.Id,
		[]achievements.Awarded{awardOf(t, "night-prowler", 1, 3)})
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	if sent || app.TestMailer.TotalSend() != 0 {
		t.Errorf("expected nothing to be sent, got %d", app.TestMailer.TotalSend())
	}
}

// The legacy import parks accounts on a placeholder address, and mail to those
// bounces forever without anybody being told.
func TestNothingIsSentToAnUnverifiedAddress(t *testing.T) {
	app, user := newApp(t)
	user.SetVerified(false)
	if err := app.Save(user); err != nil {
		t.Fatalf("failed to unverify: %v", err)
	}

	sent, err := mail.Achievements(app, user.Id,
		[]achievements.Awarded{awardOf(t, "night-prowler", 1, 3)})
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	if sent || app.TestMailer.TotalSend() != 0 {
		t.Errorf("expected nothing to be sent, got %d", app.TestMailer.TotalSend())
	}
}

func TestNothingIsSentForNothing(t *testing.T) {
	app, user := newApp(t)

	sent, err := mail.Achievements(app, user.Id, nil)
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	if sent || app.TestMailer.TotalSend() != 0 {
		t.Errorf("expected nothing to be sent, got %d", app.TestMailer.TotalSend())
	}
}
