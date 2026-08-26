//
// File:        internal/epub/description_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package epub_test

import (
	"strings"
	"testing"

	"git.obth.eu/atjontv/kosync/internal/epub"
)

// describing builds a book whose package document carries the given
// dc:description, written into the metadata exactly as passed.
func describing(t testing.TB, element string) epub.Metadata {
	t.Helper()

	document := strings.Replace(packageDocument,
		"<dc:language>de</dc:language>",
		"<dc:language>de</dc:language>\n    "+element, 1)

	archive := build(t, []entry{
		{name: "mimetype", content: "application/epub+zip"},
		{name: "META-INF/container.xml", content: container},
		{name: "OEBPS/content.opf", content: document},
		{name: "OEBPS/text/one.xhtml", content: chapter(10)},
		{name: "OEBPS/text/two.xhtml", content: chapter(10)},
	})

	reader, err := epub.Open(archive, int64(archive.Len()))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	return reader.Metadata()
}

func TestABookWithNoDescriptionHasNone(t *testing.T) {
	if description := describing(t, "").Description; description != "" {
		t.Errorf("description is %q, want empty", description)
	}
}

func TestAPlainDescriptionIsKeptAsItIs(t *testing.T) {
	meta := describing(t, "<dc:description>Geralt kehrt zurück.</dc:description>")

	if meta.Description != "Geralt kehrt zurück." {
		t.Errorf("description is %q", meta.Description)
	}
}

// The common case in the wild: the publisher wrote HTML and the XML escaped it,
// so what the parser hands back is text that still says "&lt;p&gt;".
func TestEscapedMarkupIsReadAsMarkup(t *testing.T) {
	meta := describing(t,
		"<dc:description>&lt;p&gt;Der Hexer.&lt;/p&gt;&lt;p&gt;Ein Vorspiel.&lt;/p&gt;</dc:description>")

	if meta.Description != "Der Hexer.\n\nEin Vorspiel." {
		t.Errorf("description is %q", meta.Description)
	}
}

// The other way publishers write it: the elements are really there in the XML.
func TestUnescapedMarkupIsReadAsMarkup(t *testing.T) {
	meta := describing(t,
		"<dc:description><p>Der Hexer.</p><p>Ein Vorspiel.</p></dc:description>")

	if meta.Description != "Der Hexer.\n\nEin Vorspiel." {
		t.Errorf("description is %q", meta.Description)
	}
}

// An inline element sits inside a sentence and must not break it in two, but the
// words on either side of it are still two words.
func TestInlineMarkupStaysInsideItsSentence(t *testing.T) {
	meta := describing(t, "<dc:description>&lt;p&gt;Ein &lt;em&gt;sehr&lt;/em&gt; langer Weg.&lt;/p&gt;</dc:description>")

	if meta.Description != "Ein sehr langer Weg." {
		t.Errorf("description is %q", meta.Description)
	}
}

// Hand-written HTML in this element is not XML: <br> is never closed and the
// entities are the HTML ones. Neither is a reason to lose the blurb.
func TestHandWrittenHtmlSurvives(t *testing.T) {
	meta := describing(t,
		"<dc:description>&lt;p&gt;Erste Zeile&lt;br&gt;Zweite Zeile&amp;nbsp;— Ende&lt;/p&gt;</dc:description>")

	want := "Erste Zeile\n\nZweite Zeile — Ende"
	if meta.Description != want {
		t.Errorf("description is %q, want %q", meta.Description, want)
	}
}

func TestTheWhitespaceOfAWrappedDescriptionIsCollapsed(t *testing.T) {
	meta := describing(t, "<dc:description>\n      Eine Zeile\n      und noch eine.\n    </dc:description>")

	if meta.Description != "Eine Zeile und noch eine." {
		t.Errorf("description is %q", meta.Description)
	}
}

// Text that merely mentions an angle bracket is text, not markup, and a second
// parse of it would eat the half that looks like an element.
func TestAngleBracketsInProseAreNotParsedAway(t *testing.T) {
	meta := describing(t, "<dc:description>Für alle Werte a &lt; b gilt das.</dc:description>")

	if meta.Description != "Für alle Werte a < b gilt das." {
		t.Errorf("description is %q", meta.Description)
	}
}

// A description is a blurb. Something that runs to tens of thousands of
// characters is a back catalogue, and the book page is not the place for it.
func TestAnEndlessDescriptionIsCut(t *testing.T) {
	meta := describing(t, "<dc:description>"+strings.TrimSpace(strings.Repeat("wort ", 5000))+"</dc:description>")

	if length := len([]rune(meta.Description)); length > 4001 {
		t.Fatalf("description is %d runes long", length)
	}
	if !strings.HasSuffix(meta.Description, "…") {
		t.Errorf("a shortened description does not say so: %q", meta.Description[len(meta.Description)-20:])
	}
	// Cut between words rather than through one.
	if strings.HasSuffix(meta.Description, "wor…") {
		t.Errorf("the description was cut mid-word: %q", meta.Description[len(meta.Description)-20:])
	}
}

// A book that declares the element twice, which happens when one is empty.
func TestTheFirstDescriptionWithWordsInItWins(t *testing.T) {
	meta := describing(t,
		"<dc:description>  </dc:description><dc:description>Der Hexer.</dc:description>")

	if meta.Description != "Der Hexer." {
		t.Errorf("description is %q", meta.Description)
	}
}

// A description made of nothing but markup describes nothing.
func TestADescriptionOfOnlyMarkupIsEmpty(t *testing.T) {
	meta := describing(t, "<dc:description>&lt;p&gt;&lt;/p&gt;&lt;br/&gt;</dc:description>")

	if meta.Description != "" {
		t.Errorf("description is %q, want empty", meta.Description)
	}
}
