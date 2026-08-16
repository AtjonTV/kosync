//
// File:        internal/books/authors_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package books_test

import (
	"encoding/json"
	"os"
	"testing"

	"git.obth.eu/atjontv/kosync/internal/books"
)

// sharedCases are the cases the rule is held to in both languages it is written
// in: here, and in the browser's copy at webui/src/lib/grouping.ts. The two have
// to agree or the same library reads differently depending on which client
// asked, and a corpus each would let them drift apart quietly.
type sharedCases struct {
	Display          map[string]string `json:"display"`
	SameAuthor       [][]string        `json:"sameAuthor"`
	DifferentAuthors [][2]string       `json:"differentAuthors"`
	NoKey            []string          `json:"noKey"`
}

func loadSharedCases(t testing.TB) sharedCases {
	t.Helper()

	raw, err := os.ReadFile("../../../testdata/author-names.json")
	if err != nil {
		t.Fatalf("failed to read the shared author cases: %v", err)
	}

	var cases sharedCases
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatalf("failed to read the shared author cases: %v", err)
	}
	if len(cases.Display) == 0 || len(cases.SameAuthor) == 0 {
		t.Fatal("the shared author cases are empty")
	}

	return cases
}

// The names turned round here are the ones written backwards in the reference
// library; the ones left alone are what the rule must not touch — four people in
// one field, a company suffix, half a name.
func TestEveryNameIsShownTheWayTheSharedCasesSay(t *testing.T) {
	for written, expected := range loadSharedCases(t).Display {
		if actual := books.AuthorName(written); actual != expected {
			t.Errorf("AuthorName(%q) = %q, want %q", written, actual, expected)
		}
	}
}

// The groups this merges are the ones the reference library actually holds, and
// the largest of them is its most read author split in two.
func TestTheSpellingsOfOneAuthorShareAKey(t *testing.T) {
	for _, spellings := range loadSharedCases(t).SameAuthor {
		key := books.AuthorKey(spellings[0])
		if key == "" {
			t.Fatalf("AuthorKey(%q) is empty", spellings[0])
		}

		for _, spelling := range spellings[1:] {
			if actual := books.AuthorKey(spelling); actual != key {
				t.Errorf("AuthorKey(%q) = %q, want %q like %q", spelling, actual, key, spellings[0])
			}
		}
	}
}

// Nothing is transliterated, so a name in another script keeps itself rather than
// folding into the group of everything unpronounceable.
func TestTwoAuthorsDoNotShareAKey(t *testing.T) {
	for _, pair := range loadSharedCases(t).DifferentAuthors {
		if books.AuthorKey(pair[0]) == books.AuthorKey(pair[1]) {
			t.Errorf("AuthorKey(%q) and AuthorKey(%q) are the same key", pair[0], pair[1])
		}
	}
}

// A name with no letters in it at all has no key, and the catalog drops it rather
// than shelving every such book together under nothing.
func TestANameOfPunctuationHasNoKey(t *testing.T) {
	for _, name := range loadSharedCases(t).NoKey {
		if key := books.AuthorKey(name); key != "" {
			t.Errorf("AuthorKey(%q) = %q, want no key", name, key)
		}
	}
}
