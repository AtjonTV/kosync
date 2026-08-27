//
// File:        internal/epub/toc_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package epub_test

import (
	"testing"

	"git.obth.eu/atjontv/kosync/internal/epub"
)

// tocPackage is a book of four documents: a cover the contents say nothing
// about, and three chapters they do.
func tocPackage(extra string) string {
	return `<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Metro - Die Trilogie</dc:title>
  </metadata>
  <manifest>
    <item id="cover" href="text/cover.xhtml" media-type="application/xhtml+xml"/>
    <item id="one" href="text/one.xhtml" media-type="application/xhtml+xml"/>
    <item id="two" href="text/two.xhtml" media-type="application/xhtml+xml"/>
    <item id="three" href="text/three.xhtml" media-type="application/xhtml+xml"/>
` + extra + `
  </manifest>
  <spine>
    <itemref idref="cover"/>
    <itemref idref="one"/>
    <itemref idref="two"/>
    <itemref idref="three"/>
  </spine>
</package>`
}

// navItem and ncxItem are how each kind of table of contents is declared.
const (
	navItem = `    <item id="nav" href="text/nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>`
	ncxItem = `    <item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>`
)

// navDocument is an EPUB 3 navigation document, written the way a real one is:
// nested, pointing at fragments, and one directory down from the package, so
// its links have to be resolved against itself and not against the package.
const navDocument = `<?xml version="1.0"?>
<html xmlns="http://www.w3.org/1999/xhtml">
  <head><title>Metro - Die Trilogie</title></head>
  <body>
    <nav xmlns:epub="http://www.idpf.org/2007/ops" epub:type="landmarks">
      <ol><li><a href="../text/cover.xhtml">Anfang des Buches</a></li></ol>
    </nav>
    <nav xmlns:epub="http://www.idpf.org/2007/ops" epub:type="toc">
      <ol>
        <li><a href="../text/one.xhtml#marker-1">METRO 2033</a>
          <ol>
            <li><a href="../text/two.xhtml#marker-1-1">1</a></li>
            <li><a href="../text/three.xhtml#marker-1-2">2</a></li>
          </ol>
        </li>
      </ol>
    </nav>
  </body>
</html>`

// ncxContents names the same three documents differently, so a test can tell
// which of the two a book was read from.
const ncxContents = `<?xml version="1.0"?>
<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1">
  <docTitle><text>Metro - Die Trilogie</text></docTitle>
  <navMap>
    <navPoint id="p1">
      <navLabel><text>ZWEITAUSENDDREIUNDDREISSIG</text></navLabel>
      <content src="text/one.xhtml#marker-1"/>
      <navPoint id="p2">
        <navLabel><text>Eins</text></navLabel>
        <content src="text/two.xhtml"/>
      </navPoint>
      <navPoint id="p3">
        <navLabel><text>Zwei</text></navLabel>
        <content src="text/three.xhtml"/>
      </navPoint>
    </navPoint>
  </navMap>
</ncx>`

// tocBook builds the book with whichever tables of contents it is given.
//
// Every chapter's <title> is the book's own, which is not a contrivance: it is
// what the generators of the big German publishers write into all eighty-four
// documents of a file, and the reason a preview cannot simply believe them.
func tocBook(t testing.TB, files ...entry) *epub.Reader {
	t.Helper()

	head := `<?xml version="1.0"?>
<html xmlns="http://www.w3.org/1999/xhtml">
  <head><title>Metro - Die Trilogie</title></head>
  <body>`

	base := []entry{
		{name: "mimetype", content: "application/epub+zip"},
		{name: "META-INF/container.xml", content: container},
		{name: "OEBPS/text/cover.xhtml", content: head + `<p>Ein Umschlag.</p></body></html>`},
		{name: "OEBPS/text/one.xhtml", content: head + `<p>Ein Teil.</p></body></html>`},
		{name: "OEBPS/text/two.xhtml", content: head + `<h2>Kapitelchen</h2></body></html>`},
		{name: "OEBPS/text/three.xhtml", content: head + `<p>Namenlos.</p></body></html>`},
	}

	book := build(t, append(base, files...))
	reader, err := epub.Open(book, int64(book.Len()))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	return reader
}

// named returns the titles and the sections of a book's spine, for comparing.
func named(documents []epub.Document) []string {
	names := make([]string, 0, len(documents))
	for _, document := range documents {
		name := document.Title
		if document.Section != "" {
			name = document.Section + " / " + name
		}
		names = append(names, name)
	}

	return names
}

func mustName(t *testing.T, reader *epub.Reader, want ...string) {
	t.Helper()

	got := named(reader.Spine())
	if len(got) != len(want) {
		t.Fatalf("the book has %d documents (%v), want %d", len(got), got, len(want))
	}
	for index := range want {
		if got[index] != want[index] {
			t.Errorf("document %d is %q, want %q", index, got[index], want[index])
		}
	}
}

// The names a person wrote, and the nesting that says which part of a trilogy
// a chapter numbered "1" belongs to.
func TestTheBooksOwnContentsNameItsChapters(t *testing.T) {
	reader := tocBook(t,
		entry{name: "OEBPS/content.opf", content: tocPackage(navItem)},
		entry{name: "OEBPS/text/nav.xhtml", content: navDocument},
	)

	mustName(t, reader, "Chapter 1", "METRO 2033", "METRO 2033 / 1", "METRO 2033 / 2")
}

// EPUB 2 books, and there are a great many of them, say the same thing in the
// NCX instead.
func TestTheNCXNamesTheChaptersWhenThereIsNoNavigationDocument(t *testing.T) {
	reader := tocBook(t,
		entry{name: "OEBPS/content.opf", content: tocPackage(ncxItem)},
		entry{name: "OEBPS/toc.ncx", content: ncxContents},
	)

	mustName(t, reader,
		"Chapter 1",
		"ZWEITAUSENDDREIUNDDREISSIG",
		"ZWEITAUSENDDREIUNDDREISSIG / Eins",
		"ZWEITAUSENDDREIUNDDREISSIG / Zwei",
	)
}

// Most files carry both, the NCX only so that older readers can open them at
// all. The navigation document is the one the format says to believe.
func TestTheNavigationDocumentWinsOverTheNCX(t *testing.T) {
	reader := tocBook(t,
		entry{name: "OEBPS/content.opf", content: tocPackage(navItem + "\n" + ncxItem)},
		entry{name: "OEBPS/text/nav.xhtml", content: navDocument},
		entry{name: "OEBPS/toc.ncx", content: ncxContents},
	)

	mustName(t, reader, "Chapter 1", "METRO 2033", "METRO 2033 / 1", "METRO 2033 / 2")
}

// A navigation document holds more than one list. The landmarks are where the
// cover and the first page are named, and naming a chapter after an entry in
// them would be reading the wrong list.
func TestTheLandmarksAreNotTheTableOfContents(t *testing.T) {
	reader := tocBook(t,
		entry{name: "OEBPS/content.opf", content: tocPackage(navItem)},
		entry{name: "OEBPS/text/nav.xhtml", content: navDocument},
	)

	for _, name := range named(reader.Spine()) {
		if name == "Anfang des Buches" {
			t.Errorf("a landmark was used as a chapter name")
		}
	}
}

// A book that marks none of its navs is still a book with a table of contents.
func TestAnUnmarkedNavIsReadAnyway(t *testing.T) {
	reader := tocBook(t,
		entry{name: "OEBPS/content.opf", content: tocPackage(navItem)},
		entry{name: "OEBPS/text/nav.xhtml", content: `<?xml version="1.0"?>
<html xmlns="http://www.w3.org/1999/xhtml"><body>
  <nav><ol><li><a href="../text/one.xhtml">Der Anfang</a></li></ol></nav>
</body></html>`},
	)

	mustName(t, reader, "Chapter 1", "Der Anfang", "Kapitelchen", "Chapter 4")
}

// Several entries pointing into one document is how a book with a chapter per
// file marks its parts. The document is named after the first thing in it.
func TestTheFirstMentionOfADocumentNamesIt(t *testing.T) {
	reader := tocBook(t,
		entry{name: "OEBPS/content.opf", content: tocPackage(navItem)},
		entry{name: "OEBPS/text/nav.xhtml", content: `<?xml version="1.0"?>
<html xmlns="http://www.w3.org/1999/xhtml"><body>
  <nav><ol>
    <li><a href="../text/one.xhtml#a">Zuerst</a></li>
    <li><a href="../text/one.xhtml#b">Danach</a></li>
  </ol></nav>
</body></html>`},
	)

	if got := named(reader.Spine())[1]; got != "Zuerst" {
		t.Errorf("the document is called %q, want %q", got, "Zuerst")
	}
}

// The contents of a book cover its chapters and leave out the cover, the title
// page and the imprint, which still have to be nameable — and are named by
// their own heading where they have one, and by their number where they do not.
func TestADocumentTheContentsSkipIsNamedByItself(t *testing.T) {
	reader := tocBook(t,
		entry{name: "OEBPS/content.opf", content: tocPackage(navItem)},
		entry{name: "OEBPS/text/nav.xhtml", content: `<?xml version="1.0"?>
<html xmlns="http://www.w3.org/1999/xhtml"><body>
  <nav><ol><li><a href="../text/one.xhtml">Der Anfang</a></li></ol></nav>
</body></html>`},
	)

	mustName(t, reader, "Chapter 1", "Der Anfang", "Kapitelchen", "Chapter 4")
}

// The one that started this. Every document of the file says it is called
// "Metro - Die Trilogie", and a list of eighty-four of those is not a list.
func TestAChapterIsNotNamedAfterTheWholeBook(t *testing.T) {
	reader := tocBook(t, entry{name: "OEBPS/content.opf", content: tocPackage("")})

	mustName(t, reader, "Chapter 1", "Chapter 2", "Kapitelchen", "Chapter 4")
}

// A table of contents that names a file the archive does not hold, or one the
// spine never reads, is not something to fall over.
func TestContentsThatPointNowhereAreIgnored(t *testing.T) {
	reader := tocBook(t,
		entry{name: "OEBPS/content.opf", content: tocPackage(navItem)},
		entry{name: "OEBPS/text/nav.xhtml", content: `<?xml version="1.0"?>
<html xmlns="http://www.w3.org/1999/xhtml"><body>
  <nav><ol>
    <li><a href="../text/gone.xhtml">Verschwunden</a></li>
    <li><a href="https://example.invalid/">Woanders</a></li>
    <li><a href="../text/three.xhtml">Das Ende</a></li>
  </ol></nav>
</body></html>`},
	)

	mustName(t, reader, "Chapter 1", "Chapter 2", "Kapitelchen", "Das Ende")
}

// ReadDocument answers about one document what Spine answers about all of them,
// and the two must not disagree: the header of the preview is drawn from one
// and its chapter list from the other.
func TestOneDocumentIsNamedTheSameWayTheListNamesIt(t *testing.T) {
	reader := tocBook(t,
		entry{name: "OEBPS/content.opf", content: tocPackage(navItem)},
		entry{name: "OEBPS/text/nav.xhtml", content: navDocument},
	)

	listed := reader.Spine()[2]
	_, alone, err := reader.ReadDocument(2)
	if err != nil {
		t.Fatalf("ReadDocument: %v", err)
	}

	if alone.Title != listed.Title || alone.Section != listed.Section {
		t.Errorf("read as %q/%q, listed as %q/%q",
			alone.Section, alone.Title, listed.Section, listed.Title)
	}
}
