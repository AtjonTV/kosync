//
// File:        internal/epub/epub_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package epub_test

import (
	"archive/zip"
	"bytes"
	"errors"
	"strings"
	"testing"

	"git.obth.eu/atjontv/kosync/internal/epub"
)

// entry is one file in a synthetic EPUB.
type entry struct {
	name    string
	content string
}

// build assembles a zip from the given entries.
func build(t testing.TB, entries []entry) *bytes.Reader {
	t.Helper()

	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	for _, item := range entries {
		writer, err := archive.Create(item.name)
		if err != nil {
			t.Fatalf("create %s: %v", item.name, err)
		}
		if _, err := writer.Write([]byte(item.content)); err != nil {
			t.Fatalf("write %s: %v", item.name, err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close archive: %v", err)
	}

	return bytes.NewReader(buffer.Bytes())
}

const container = `<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>`

const packageDocument = `<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Zeit des Sturms</dc:title>
    <dc:creator>Andrzej Sapkowski</dc:creator>
    <dc:language>de</dc:language>
    <dc:identifier scheme="ISBN">9783423426091</dc:identifier>
    <meta name="cover" content="cover-img"/>
  </metadata>
  <manifest>
    <item id="cover-img" href="images/cover.jpg" media-type="image/jpeg"/>
    <item id="one" href="text/one.xhtml" media-type="application/xhtml+xml"/>
    <item id="two" href="text/two.xhtml" media-type="application/xhtml+xml"/>
    <item id="orphan" href="text/orphan.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine>
    <itemref idref="one"/>
    <itemref idref="two"/>
  </spine>
</package>`

// chapter renders a document with the given number of words, plus markup that
// must not be counted.
func chapter(words int) string {
	body := strings.TrimSpace(strings.Repeat("wort ", words))

	return `<?xml version="1.0"?>
<html xmlns="http://www.w3.org/1999/xhtml">
  <head><title>Ignored Title Words</title><style>body { margin: 0 }</style></head>
  <body><p>` + body + `</p><script>var ignored = 1;</script></body>
</html>`
}

func newBook(t testing.TB) *bytes.Reader {
	t.Helper()

	return build(t, []entry{
		{name: "mimetype", content: "application/epub+zip"},
		{name: "META-INF/container.xml", content: container},
		{name: "OEBPS/content.opf", content: packageDocument},
		{name: "OEBPS/text/one.xhtml", content: chapter(120)},
		{name: "OEBPS/text/two.xhtml", content: chapter(80)},
		{name: "OEBPS/text/orphan.xhtml", content: chapter(5000)},
		{name: "OEBPS/images/cover.jpg", content: "not really a jpeg"},
	})
}

func TestOpenReadsMetadata(t *testing.T) {
	reader, err := epub.Open(newBook(t), int64(newBook(t).Len()))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	meta := reader.Metadata()
	if meta.Title != "Zeit des Sturms" {
		t.Errorf("title is %q", meta.Title)
	}
	if len(meta.Authors) != 1 || meta.Authors[0] != "Andrzej Sapkowski" {
		t.Errorf("authors are %v", meta.Authors)
	}
	if meta.Language != "de" {
		t.Errorf("language is %q", meta.Language)
	}
	if meta.Identifiers["ISBN"] != "9783423426091" {
		t.Errorf("identifiers are %v", meta.Identifiers)
	}
	if meta.SpineCount != 2 {
		t.Errorf("spine count is %d, want 2", meta.SpineCount)
	}
}

// The spine is the only thing the reader paginates. Counting every XHTML file
// in the archive instead inflates the count by whatever orphaned documents the
// publisher left behind, which throws off the words-per-page fallback without
// looking wrong.
func TestWordCountCoversTheSpineOnly(t *testing.T) {
	book := newBook(t)
	reader, err := epub.Open(book, int64(book.Len()))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	words, err := reader.WordCount()
	if err != nil {
		t.Fatalf("WordCount: %v", err)
	}

	// 120 + 80 from the spine; the 5000-word orphan, the head title and the
	// script must all be excluded.
	if words != 200 {
		t.Errorf("counted %d words, want 200", words)
	}
}

// Real books wrap the title across indented lines, and EPUB 3 dropped the
// scheme attribute in favour of a URN in the value. Both showed up in the very
// first five real files this was pointed at.
func TestMetadataNormalisesRealWorldShapes(t *testing.T) {
	pkg := strings.Replace(packageDocument,
		`<dc:title>Zeit des Sturms</dc:title>`,
		"<dc:title>Die Witcher-Saga - Das Erbe der Elfen\n\t\t\t\t Die Zeit der Verachtung</dc:title>",
		1)
	pkg = strings.Replace(pkg,
		`<dc:identifier scheme="ISBN">9783423426091</dc:identifier>`,
		`<dc:identifier id="pub-id">urn:isbn:9783423439930</dc:identifier>
    <dc:identifier>urn:uuid:0c2b7a10-5f3e-4a9d-9c1b-2f6d8e4a1b33</dc:identifier>`,
		1)

	book := build(t, []entry{
		{name: "META-INF/container.xml", content: container},
		{name: "OEBPS/content.opf", content: pkg},
		{name: "OEBPS/text/one.xhtml", content: chapter(5)},
		{name: "OEBPS/text/two.xhtml", content: chapter(5)},
	})

	reader, err := epub.Open(book, int64(book.Len()))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	meta := reader.Metadata()
	want := "Die Witcher-Saga - Das Erbe der Elfen Die Zeit der Verachtung"
	if meta.Title != want {
		t.Errorf("title is %q, want %q", meta.Title, want)
	}
	if meta.Identifiers["ISBN"] != "9783423439930" {
		t.Errorf("ISBN is %q", meta.Identifiers["ISBN"])
	}
	if meta.Identifiers["UUID"] != "0c2b7a10-5f3e-4a9d-9c1b-2f6d8e4a1b33" {
		t.Errorf("UUID is %q", meta.Identifiers["UUID"])
	}
}

func TestCoverFromEPUB2Pointer(t *testing.T) {
	book := newBook(t)
	reader, err := epub.Open(book, int64(book.Len()))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	name, data, err := reader.Cover()
	if err != nil {
		t.Fatalf("Cover: %v", err)
	}
	if name != "OEBPS/images/cover.jpg" {
		t.Errorf("cover path is %q", name)
	}
	if string(data) != "not really a jpeg" {
		t.Errorf("cover content is %q", data)
	}
}

func TestCoverFromEPUB3Property(t *testing.T) {
	pkg := strings.Replace(packageDocument,
		`<item id="cover-img" href="images/cover.jpg" media-type="image/jpeg"/>`,
		`<item id="cover-img" href="images/front.png" media-type="image/png" properties="cover-image"/>`,
		1)
	pkg = strings.Replace(pkg, `<meta name="cover" content="cover-img"/>`, "", 1)

	book := build(t, []entry{
		{name: "META-INF/container.xml", content: container},
		{name: "OEBPS/content.opf", content: pkg},
		{name: "OEBPS/images/front.png", content: "png bytes"},
	})

	reader, err := epub.Open(book, int64(book.Len()))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	name, data, err := reader.Cover()
	if err != nil {
		t.Fatalf("Cover: %v", err)
	}
	if name != "OEBPS/images/front.png" || string(data) != "png bytes" {
		t.Errorf("cover is %q / %q", name, data)
	}
}

func TestCoverIsOptional(t *testing.T) {
	pkg := strings.Replace(packageDocument, `<meta name="cover" content="cover-img"/>`, "", 1)
	pkg = strings.Replace(pkg,
		`<item id="cover-img" href="images/cover.jpg" media-type="image/jpeg"/>`, "", 1)

	book := build(t, []entry{
		{name: "META-INF/container.xml", content: container},
		{name: "OEBPS/content.opf", content: pkg},
		{name: "OEBPS/text/one.xhtml", content: chapter(10)},
		{name: "OEBPS/text/two.xhtml", content: chapter(10)},
	})

	reader, err := epub.Open(book, int64(book.Len()))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	name, data, err := reader.Cover()
	if err != nil {
		t.Fatalf("Cover: %v", err)
	}
	if name != "" || data != nil {
		t.Errorf("expected no cover, got %q with %d bytes", name, len(data))
	}
}

func TestOpenRejectsNonEPUB(t *testing.T) {
	book := build(t, []entry{{name: "readme.txt", content: "just a zip"}})

	if _, err := epub.Open(book, int64(book.Len())); !errors.Is(err, epub.ErrNotEPUB) {
		t.Errorf("error is %v, want ErrNotEPUB", err)
	}
}

// Hrefs are URL-escaped in the package document but not in the archive.
func TestResolvesEscapedHrefs(t *testing.T) {
	pkg := strings.Replace(packageDocument,
		`href="text/one.xhtml"`, `href="text/chapter%20one.xhtml"`, 1)

	book := build(t, []entry{
		{name: "META-INF/container.xml", content: container},
		{name: "OEBPS/content.opf", content: pkg},
		{name: "OEBPS/text/chapter one.xhtml", content: chapter(42)},
		{name: "OEBPS/text/two.xhtml", content: chapter(8)},
	})

	reader, err := epub.Open(book, int64(book.Len()))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	words, err := reader.WordCount()
	if err != nil {
		t.Fatalf("WordCount: %v", err)
	}
	if words != 50 {
		t.Errorf("counted %d words, want 50", words)
	}
}

// HTML entities are not defined in XML, and real books are full of them.
func TestWordCountToleratesHTMLEntities(t *testing.T) {
	document := `<?xml version="1.0"?>
<html xmlns="http://www.w3.org/1999/xhtml">
  <body><p>one&nbsp;two three&mdash;four &amp; five</p></body>
</html>`

	book := build(t, []entry{
		{name: "META-INF/container.xml", content: container},
		{name: "OEBPS/content.opf", content: packageDocument},
		{name: "OEBPS/text/one.xhtml", content: document},
		{name: "OEBPS/text/two.xhtml", content: chapter(0)},
	})

	reader, err := epub.Open(book, int64(book.Len()))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	words, err := reader.WordCount()
	if err != nil {
		t.Fatalf("WordCount: %v", err)
	}
	if words == 0 {
		t.Error("entities defeated the word count entirely")
	}
}
