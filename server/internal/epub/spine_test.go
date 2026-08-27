//
// File:        internal/epub/spine_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package epub_test

import (
	"bytes"
	"strings"
	"testing"

	"git.obth.eu/atjontv/kosync/internal/epub"
)

// spinePackage names four documents and reads three of them, one of which is
// not in the archive at all.
const spinePackage = `<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Zeit des Sturms</dc:title>
  </metadata>
  <manifest>
    <item id="one" href="text/one.xhtml" media-type="application/xhtml+xml"/>
    <item id="two" href="text/two.xhtml" media-type="application/xhtml+xml"/>
    <item id="three" href="text/three.xhtml" media-type="application/xhtml+xml"/>
    <item id="gone" href="text/gone.xhtml" media-type="application/xhtml+xml"/>
    <item id="picture" href="images/plate.png" media-type="image/png"/>
    <item id="unnamed" href="images/plate.gif"/>
  </manifest>
  <spine>
    <itemref idref="one"/>
    <itemref idref="gone"/>
    <itemref idref="two"/>
    <itemref idref="three"/>
  </spine>
</package>`

// spineBook is a book whose three readable documents each say what they are
// called in a different way.
func spineBook(t testing.TB) *bytes.Reader {
	t.Helper()

	return build(t, []entry{
		{name: "mimetype", content: "application/epub+zip"},
		{name: "META-INF/container.xml", content: container},
		{name: "OEBPS/content.opf", content: spinePackage},
		{name: "OEBPS/text/one.xhtml", content: `<?xml version="1.0"?>
<html xmlns="http://www.w3.org/1999/xhtml">
  <head><title>Der Anfang</title></head>
  <body><h1>Etwas anderes</h1><p>Ein Sturm zieht auf.</p>
  <img src="../images/plate.png" alt="Eine Tafel"/>
  <a href="three.xhtml">weiter</a></body>
</html>`},
		{name: "OEBPS/text/two.xhtml", content: `<?xml version="1.0"?>
<html xmlns="http://www.w3.org/1999/xhtml">
  <head><title>   </title></head>
  <body><h2>Die
  Fortsetzung</h2><p>Und weiter.</p></body>
</html>`},
		{name: "OEBPS/text/three.xhtml", content: `<?xml version="1.0"?>
<html xmlns="http://www.w3.org/1999/xhtml"><body><p>Namenlos.</p></body></html>`},
		{name: "OEBPS/images/plate.png", content: "not really a png"},
		{name: "OEBPS/images/plate.gif", content: "not really a gif"},
	})
}

func openSpineBook(t testing.TB) *epub.Reader {
	t.Helper()

	book := spineBook(t)
	reader, err := epub.Open(book, int64(book.Len()))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	return reader
}

// A spine entry naming a file the archive does not hold has nothing to show, so
// it is left out — and the numbering closes over it, because an index that
// cannot be read is only a way to ask for an error.
func TestSpineLeavesOutWhatTheArchiveDoesNotHold(t *testing.T) {
	documents := openSpineBook(t).Spine()

	if len(documents) != 3 {
		t.Fatalf("spine has %d documents, want 3", len(documents))
	}
	for index, document := range documents {
		if document.Index != index {
			t.Errorf("document %d says it is %d", index, document.Index)
		}
	}
	if documents[1].Path != "OEBPS/text/two.xhtml" {
		t.Errorf("the second document is %q", documents[1].Path)
	}
}

func TestSpineTitlesComeFromTheDocuments(t *testing.T) {
	documents := openSpineBook(t).Spine()

	want := []string{"Der Anfang", "Die Fortsetzung", "Chapter 3"}
	for index, title := range want {
		if documents[index].Title != title {
			t.Errorf("document %d is called %q, want %q", index, documents[index].Title, title)
		}
	}
}

func TestReadDocumentReturnsTheDocumentAndWhereItIs(t *testing.T) {
	raw, document, err := openSpineBook(t).ReadDocument(0)
	if err != nil {
		t.Fatalf("ReadDocument: %v", err)
	}

	if !strings.Contains(string(raw), "Ein Sturm zieht auf.") {
		t.Errorf("the document is %q", raw)
	}
	if document.Path != "OEBPS/text/one.xhtml" || document.Title != "Der Anfang" {
		t.Errorf("the document is %+v", document)
	}
}

func TestReadDocumentRefusesAnIndexTheBookDoesNotHave(t *testing.T) {
	reader := openSpineBook(t)

	for _, index := range []int{-1, 3, 9999} {
		if _, _, err := reader.ReadDocument(index); err == nil {
			t.Errorf("reading document %d succeeded", index)
		}
	}
}

// The href is relative to the document that wrote it, not to the package.
func TestResourceResolvesRelativeToTheDocument(t *testing.T) {
	raw, kind, err := openSpineBook(t).Resource("OEBPS/text/one.xhtml", "../images/plate.png")
	if err != nil {
		t.Fatalf("Resource: %v", err)
	}

	if string(raw) != "not really a png" {
		t.Errorf("the resource is %q", raw)
	}
	if kind != "image/png" {
		t.Errorf("the media type is %q", kind)
	}
}

// A manifest that forgot to say what a file is does not make it unreadable.
func TestResourceNamesAFileTheManifestDoesNot(t *testing.T) {
	_, kind, err := openSpineBook(t).Resource("OEBPS/text/one.xhtml", "../images/plate.gif")
	if err != nil {
		t.Fatalf("Resource: %v", err)
	}
	if kind != "image/gif" {
		t.Errorf("the media type is %q", kind)
	}
}

// Nothing a book writes may send this server anywhere. A scheme is the whole
// test: an http URL and a data: URI both resolve to nothing rather than to a
// request made on a publisher's behalf.
func TestResourceRefusesToLeaveTheArchive(t *testing.T) {
	reader := openSpineBook(t)

	for _, href := range []string{
		"https://example.invalid/pixel.png",
		"//example.invalid/pixel.png",
		"data:image/png;base64,AAAA",
		"../../../etc/passwd",
		"",
	} {
		if _, _, err := reader.Resource("OEBPS/text/one.xhtml", href); err == nil {
			t.Errorf("resolving %q succeeded", href)
		}
	}
}
