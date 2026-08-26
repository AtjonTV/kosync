//
// File:        internal/books/authors.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package books

import (
	"strings"
	"unicode"
)

// maxGivenNames caps how much of a comma separated name may be the given part.
//
// "Child, Lee" is a name written backwards; "Corinna Mieth, Simon Weber, Rainer
// Schäfer, Anna Schriefl" is four people crammed into one field, and turning
// that inside out would produce nonsense. Three words is enough for "Martin,
// George, R. R." and short of anything that is really a list.
const maxGivenNames = 3

// nameSuffixes are the words that follow a comma without the name being written
// backwards. Without them "Penguin Random House, LLC" becomes "LLC Penguin
// Random House".
var nameSuffixes = map[string]bool{
	"jr": true, "sr": true, "ii": true, "iii": true, "iv": true,
	"phd": true, "md": true, "esq": true,
	"llc": true, "inc": true, "ltd": true, "gmbh": true, "ag": true, "co": true,
}

// AuthorName is the form of an author's name to show.
//
// Publisher metadata writes a name either way round, and one library holds both:
// "Lee Child" on twenty-six books and "Child, Lee" on three. Sorted order is
// useful in a filing cabinet and not in a list a reader skims, so the name is
// turned back round. Anything this cannot read confidently is left exactly as it
// was found.
func AuthorName(name string) string {
	name = strings.TrimSpace(name)

	comma := strings.IndexByte(name, ',')
	if comma < 0 {
		return name
	}

	family := strings.TrimSpace(name[:comma])
	given := strings.Fields(strings.ReplaceAll(name[comma+1:], ",", " "))
	if family == "" || len(given) == 0 || len(given) > maxGivenNames {
		return name
	}

	if nameSuffixes[strings.ToLower(strings.Trim(given[len(given)-1], "."))] {
		return name
	}

	return strings.Join(given, " ") + " " + family
}

// AuthorKey is what every spelling of one author's name has in common.
//
// Two names belong to the same person here when they are the same letters in the
// same order: "George R. R. Martin", "George R.R. Martin" and "Martin, George,
// R. R." differ only in the spaces and dots between the initials, and shelving
// them apart splits an author's books across three places in the catalog.
//
// Only punctuation and case are dropped, and nothing is transliterated. A name
// in a script with no case and no spaces is its own key rather than an empty
// one, which is what would happen if the key were ASCII letters only — and every
// such author would then fold into a single nameless group.
func AuthorKey(name string) string {
	var key strings.Builder

	for _, letter := range AuthorName(name) {
		if unicode.IsLetter(letter) || unicode.IsDigit(letter) {
			key.WriteRune(unicode.ToLower(letter))
		}
	}

	return key.String()
}
