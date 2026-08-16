//
// File:        internal/books/authors_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package books_test

import (
	"testing"

	"git.obth.eu/atjontv/kosync/internal/books"
)

func TestANameWrittenBackwardsIsTurnedRound(t *testing.T) {
	cases := map[string]string{
		"Child, Lee":            "Lee Child",
		"Gabaldon, Diana":       "Diana Gabaldon",
		"Rowling, J.K.":         "J.K. Rowling",
		"Sigurðardóttir, Lilja": "Lilja Sigurðardóttir",
		"O'Leary, Anabel":       "Anabel O'Leary",
		// The family name may be several words, and only the first comma
		// separates the two halves.
		"de la Cruz, Melissa":   "Melissa de la Cruz",
		"Martin, George, R. R.": "George R. R. Martin",
		// Untidy spacing is not a different name.
		"  Child ,  Lee  ": "Lee Child",
	}

	for written, expected := range cases {
		if actual := books.AuthorName(written); actual != expected {
			t.Errorf("AuthorName(%q) = %q, want %q", written, actual, expected)
		}
	}
}

func TestANameThatIsNotBackwardsIsLeftAlone(t *testing.T) {
	unchanged := []string{
		"Lee Child",
		"J.K. Rowling",
		"村上春樹",
		"",
		// Four people in one field, not one name written backwards.
		"Corinna Mieth, Simon Weber, Rainer Schäfer, Anna Schriefl",
		// The word after the comma is a suffix, not a given name.
		"Penguin Random House, LLC",
		"Smith, Jr.",
		"King, PhD",
		// Half a name is not a name to turn round.
		"Child,",
		", Lee",
	}

	for _, name := range unchanged {
		if actual := books.AuthorName(name); actual != name {
			t.Errorf("AuthorName(%q) = %q, want it unchanged", name, actual)
		}
	}
}

// The five groups this merges are the five the reference library actually holds,
// and the largest of them is its most read author split in two.
func TestTheSpellingsOfOneAuthorShareAKey(t *testing.T) {
	groups := [][]string{
		{"Lee Child", "Child, Lee", "CHILD, LEE", "Lee  Child"},
		{"George R. R. Martin", "George R.R. Martin", "Martin, George, R. R."},
		{"J. K. Rowling", "J.K. Rowling", "Rowling, J.K."},
		{"Diana Gabaldon", "Gabaldon, Diana"},
		{"Pottermore Publishing", "Publishing, Pottermore"},
	}

	for _, spellings := range groups {
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

func TestTwoAuthorsDoNotShareAKey(t *testing.T) {
	apart := [][2]string{
		{"Lee Child", "Lee Childs"},
		{"George R. R. Martin", "George Martin"},
		{"Andrzej Sapkowski", "Andrew Sapkowski"},
		// Nothing is transliterated, so a name in another script keeps itself
		// rather than folding into the group of everything unpronounceable.
		{"村上春樹", "東野圭吾"},
		{"Александр Пушкин", "Лев Толстой"},
	}

	for _, pair := range apart {
		if books.AuthorKey(pair[0]) == books.AuthorKey(pair[1]) {
			t.Errorf("AuthorKey(%q) and AuthorKey(%q) are the same key", pair[0], pair[1])
		}
	}
}

// A name with no letters in it at all has no key, and the catalog drops it rather
// than shelving every such book together under nothing.
func TestANameOfPunctuationHasNoKey(t *testing.T) {
	for _, name := range []string{"", "   ", "---", ".,.", ","} {
		if key := books.AuthorKey(name); key != "" {
			t.Errorf("AuthorKey(%q) = %q, want no key", name, key)
		}
	}
}
