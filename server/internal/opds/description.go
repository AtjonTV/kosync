//
// File:        internal/opds/description.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package opds

import (
	"fmt"
	"strings"

	"git.obth.eu/atjontv/kosync/internal/books"
	"git.obth.eu/atjontv/kosync/internal/devices"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

// dateLayout is how a moment reads in a sentence.
const dateLayout = "2 January 2006"

// readingState is where a book stands, as the newest push for it left things.
type readingState struct {
	Progress   float64
	DeviceId   string
	LastReadAt types.DateTime
}

// details is what a feed needs besides the book records themselves.
//
// Both maps are loaded once per feed rather than per book: a page of fifty
// publications would otherwise be a hundred extra queries to say "63% read".
type details struct {
	at      urls
	reading map[string]readingState
	devices map[string]string
}

// loadDetails gathers the reading state and the device names of one account.
//
// A failure is not fatal and not reported: the catalog's job is to hand over
// books, and losing the sentence that says how far along one is must not lose
// the book with it.
func loadDetails(app core.App, owner string, at urls) details {
	return details{
		at:      at,
		reading: loadReadingStates(app, owner),
		devices: loadDeviceNames(app, owner),
	}
}

// loadReadingStates returns the newest state of each book the account has read.
func loadReadingStates(app core.App, owner string) map[string]readingState {
	states := map[string]readingState{}

	rows := []struct {
		Book       string         `db:"book"`
		Progress   float64        `db:"progress"`
		DeviceId   string         `db:"last_device_id"`
		LastReadAt types.DateTime `db:"last_read_at"`
	}{}

	// Ascending, so that the last row written for a book is the newest one. Two
	// documents can point at one book, and the more recent position wins.
	err := app.ConcurrentDB().
		Select("book", "progress", "last_device_id", "last_read_at").
		From(schema.CollectionDocuments).
		AndWhere(dbx.HashExp{schema.FieldOwner: owner}).
		AndWhere(dbx.NewExp("book != ''")).
		OrderBy("last_read_at ASC").
		All(&rows)
	if err != nil {
		return states
	}

	for _, row := range rows {
		states[row.Book] = readingState{
			Progress:   row.Progress,
			DeviceId:   row.DeviceId,
			LastReadAt: row.LastReadAt,
		}
	}

	return states
}

// loadDeviceNames maps a device identifier to the name its owner recognises.
func loadDeviceNames(app core.App, owner string) map[string]string {
	names := map[string]string{}

	records, err := app.FindAllRecords(schema.CollectionDevices, dbx.HashExp{schema.FieldOwner: owner})
	if err != nil {
		return names
	}

	for _, record := range records {
		names[record.GetString(schema.FieldDeviceId)] = devices.DisplayName(record)
	}

	return names
}

// describe is the prose a reader shows under "book information".
//
// The book's own blurb leads, when it has one. That is the thing somebody
// standing in front of a shelf actually wants — "what is this one about" — and
// on a device the catalog is the only place it can be read.
//
// Most EPUBs carry no description at all, which is why this does not stop there.
// A catalog that only passed the blurb through would grey that button out for
// most of the library; what this server knows instead is where the reading
// stands, which on a shelf you are browsing from a second device is worth
// saying either way.
func (d details) describe(book *core.Record) string {
	var lines []string

	if blurb := strings.TrimSpace(book.GetString(schema.FieldDescription)); blurb != "" {
		lines = append(lines, blurb, "")
	}

	lines = append(lines, d.readingLine(book))

	facts := []string{}
	if pages, source := books.EffectivePages(book); pages > 0 {
		facts = append(facts, fmt.Sprintf("%s pages, %s.", thousands(pages), pageSourceWords(source)))
	}
	if words := book.GetInt(schema.FieldWordCount); words > 0 {
		facts = append(facts, thousands(words)+" words.")
	}
	if identifier := printedIdentifier(book); identifier != "" {
		facts = append(facts, identifier)
	}

	if len(facts) > 0 {
		lines = append(lines, "", strings.Join(facts, "\n"))
	}

	return strings.Join(lines, "\n")
}

// readingLine says where the reading stands, in a sentence.
func (d details) readingLine(book *core.Record) string {
	state, read := d.reading[book.Id]
	if !read || state.LastReadAt.IsZero() {
		return "Not started on any of your devices yet."
	}

	position := fmt.Sprintf("%.0f%% read", state.Progress*100)
	if state.Progress >= 1 {
		position = "Finished"
	}

	where := ""
	if name := d.devices[state.DeviceId]; name != "" {
		where = " on " + name
	}

	return fmt.Sprintf("%s, last opened%s on %s.",
		position, where, state.LastReadAt.Time().Format(dateLayout))
}

// pageSourceWords explains where a page count came from, since a measured count
// and a guess from the word count are worth telling apart.
func pageSourceWords(source string) string {
	if source == books.PageSourceMeasured {
		return "measured from your own reading"
	}

	return "estimated from the word count"
}

// printedIdentifier is the book's identity written the way it appears on a book
// rather than as the URN the feed carries.
func printedIdentifier(book *core.Record) string {
	identifier := identifierOf(book)

	if value, found := strings.CutPrefix(identifier, "urn:isbn:"); found {
		return "ISBN " + value
	}

	return ""
}

// thousands groups a number so that a five figure word count can be read at a
// glance.
func thousands(value int) string {
	digits := fmt.Sprintf("%d", value)

	var grouped strings.Builder
	for index, digit := range digits {
		if index > 0 && (len(digits)-index)%3 == 0 {
			grouped.WriteRune(',')
		}
		grouped.WriteRune(digit)
	}

	return grouped.String()
}
