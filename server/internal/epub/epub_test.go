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
	"fmt"
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

// The cover tests below are shapes taken from a real shelf. Every one of them
// is a book that shows a cover in every other reader, and showed a blank here.

// coverPackage assembles a package document out of the pieces the cover
// fallbacks care about.
func coverPackage(metadata, manifest, spine, guide string) string {
	return `<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="2.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Zeit des Sturms</dc:title>
` + metadata + `
  </metadata>
  <manifest>
` + manifest + `
  </manifest>
  <spine>
` + spine + `
  </spine>
` + guide + `
</package>`
}

// coverOf builds a book from a package document and whatever files it names,
// and returns what the reader makes of its cover.
func coverOf(t testing.TB, pkg string, files ...entry) (string, string) {
	t.Helper()

	entries := append([]entry{
		{name: "META-INF/container.xml", content: container},
		{name: "OEBPS/content.opf", content: pkg},
	}, files...)

	book := build(t, entries)
	reader, err := epub.Open(book, int64(book.Len()))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	name, data, err := reader.Cover()
	if err != nil {
		t.Fatalf("Cover: %v", err)
	}

	return name, string(data)
}

// The content of <meta name="cover"> is supposed to be a manifest id. A good
// number of books put the href there instead, and every reader takes it.
func TestCoverFromAMetaHref(t *testing.T) {
	pkg := coverPackage(
		`<meta name="cover" content="images/cover.jpg"/>`,
		`<item id="cover.jpg" href="images/cover.jpg" media-type="image/jpeg"/>`,
		`<itemref idref="cover.jpg"/>`, "")

	name, data := coverOf(t, pkg, entry{name: "OEBPS/images/cover.jpg", content: "jpeg bytes"})
	if name != "OEBPS/images/cover.jpg" || data != "jpeg bytes" {
		t.Errorf("cover is %q / %q", name, data)
	}
}

// The pointer names the page that shows the cover rather than the image, which
// is what a reader displays anyway.
func TestCoverThroughACoverPage(t *testing.T) {
	pkg := coverPackage(
		`<meta name="cover" content="titlepage"/>`,
		`<item id="titlepage" href="Text/titlepage.xhtml" media-type="application/xhtml+xml"/>
     <item id="cover0001" href="Images/cover0001.jpeg" media-type="image/jpeg"/>`,
		`<itemref idref="titlepage"/>`, "")

	page := `<?xml version="1.0"?>
<html xmlns="http://www.w3.org/1999/xhtml"><body>
  <p><img src="../Images/cover0001.jpeg" alt="Cover"/></p>
</body></html>`

	name, data := coverOf(t, pkg,
		entry{name: "OEBPS/Text/titlepage.xhtml", content: page},
		entry{name: "OEBPS/Images/cover0001.jpeg", content: "jpeg bytes"})
	if name != "OEBPS/Images/cover0001.jpeg" || data != "jpeg bytes" {
		t.Errorf("cover is %q / %q", name, data)
	}
}

// Calibre draws its cover page as an SVG, where the image is an <image> with an
// xlink:href rather than an <img> with a src.
func TestCoverThroughAnSVGCoverPage(t *testing.T) {
	pkg := coverPackage("",
		`<item id="cover" href="cover.xhtml" media-type="application/xhtml+xml"/>
     <item id="art" href="images/front.png" media-type="image/png"/>`,
		`<itemref idref="cover"/>`,
		`<guide><reference type="cover" title="Cover" href="cover.xhtml"/></guide>`)

	page := `<?xml version="1.0"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:svg="http://www.w3.org/2000/svg"><body>
  <svg:svg viewBox="0 0 600 900"><svg:image width="600" height="900"
    xmlns:xlink="http://www.w3.org/1999/xlink" xlink:href="images/front.png"/></svg:svg>
</body></html>`

	name, data := coverOf(t, pkg,
		entry{name: "OEBPS/cover.xhtml", content: page},
		entry{name: "OEBPS/images/front.png", content: "png bytes"})
	if name != "OEBPS/images/front.png" || data != "png bytes" {
		t.Errorf("cover is %q / %q", name, data)
	}
}

// Nothing declares the cover, but the manifest calls an image one. Project
// Gutenberg and a good many Calibre exports look like this.
func TestCoverFromANamedManifestImage(t *testing.T) {
	pkg := coverPackage("",
		`<item id="b1_cover" href="b1/001.xhtml" media-type="application/xhtml+xml"/>
     <item id="b1_cover-image" href="b1/images/cover.jpeg" media-type="image/jpeg"/>
     <item id="b1_logo" href="b1/images/logo.jpeg" media-type="image/jpeg"/>`,
		`<itemref idref="b1_cover"/>`, "")

	name, _ := coverOf(t, pkg,
		entry{name: "OEBPS/b1/001.xhtml", content: chapter(3)},
		entry{name: "OEBPS/b1/images/cover.jpeg", content: "jpeg bytes"},
		entry{name: "OEBPS/b1/images/logo.jpeg", content: "not the cover"})
	if name != "OEBPS/b1/images/cover.jpeg" {
		t.Errorf("cover is %q", name)
	}
}

// Nothing says anything, so the page the book opens on has to answer for it.
func TestCoverFromTheFirstPage(t *testing.T) {
	pkg := coverPackage("",
		`<item id="first" href="content/page1.xhtml" media-type="application/xhtml+xml"/>
     <item id="art" href="content/resources/index-1_1_0.jpg" media-type="image/jpeg"/>`,
		`<itemref idref="first"/>`, "")

	page := `<?xml version="1.0"?>
<html xmlns="http://www.w3.org/1999/xhtml"><body>
  <div><img src="resources/index-1_1_0.jpg"/></div>
</body></html>`

	name, data := coverOf(t, pkg,
		entry{name: "OEBPS/content/page1.xhtml", content: page},
		entry{name: "OEBPS/content/resources/index-1_1_0.jpg", content: "jpeg bytes"})
	if name != "OEBPS/content/resources/index-1_1_0.jpg" || data != "jpeg bytes" {
		t.Errorf("cover is %q / %q", name, data)
	}
}

// Project Gutenberg names a chapter as its cover and the real cover in the
// guide. Following the pointers in order would hang an illustration from the
// middle of the book on the shelf.
func TestCoverPrefersADeclaredImageToAPage(t *testing.T) {
	pkg := coverPackage(
		`<meta name="cover" content="item78"/>`,
		`<item id="item78" href="chapter-2.htm" media-type="application/xhtml+xml"/>
     <item id="item1" href="images/icover.jpg" media-type="image/jpeg"/>
     <item id="item9" href="images/i295.jpg" media-type="image/jpeg"/>`,
		`<itemref idref="item78"/>`,
		`<guide><reference type="cover" title="Cover Image" href="images/icover.jpg"/></guide>`)

	page := `<?xml version="1.0"?>
<html xmlns="http://www.w3.org/1999/xhtml"><body><img src="images/i295.jpg"/></body></html>`

	name, data := coverOf(t, pkg,
		entry{name: "OEBPS/chapter-2.htm", content: page},
		entry{name: "OEBPS/images/icover.jpg", content: "the cover"},
		entry{name: "OEBPS/images/i295.jpg", content: "an illustration"})
	if name != "OEBPS/images/icover.jpg" || data != "the cover" {
		t.Errorf("cover is %q / %q", name, data)
	}
}

// A pointer at something that is not a picture is not a cover. One book in the
// reference library names its stylesheet.
func TestCoverIgnoresAPointerAtSomethingElse(t *testing.T) {
	pkg := coverPackage(
		`<meta name="cover" content="item60"/>`,
		`<item id="item60" href="1.css" media-type="text/css"/>
     <item id="one" href="text/one.xhtml" media-type="application/xhtml+xml"/>`,
		`<itemref idref="one"/>`, "")

	name, _ := coverOf(t, pkg,
		entry{name: "OEBPS/1.css", content: "body { margin: 0 }"},
		entry{name: "OEBPS/text/one.xhtml", content: chapter(10)})
	if name != "" {
		t.Errorf("cover is %q, want none", name)
	}
}

// A cover page may point out of the archive entirely. Whatever that is, it is
// not something this server can store.
func TestCoverIgnoresAnImageFromElsewhere(t *testing.T) {
	pkg := coverPackage("",
		`<item id="cover" href="cover.xhtml" media-type="application/xhtml+xml"/>`,
		`<itemref idref="cover"/>`,
		`<guide><reference type="cover" href="cover.xhtml"/></guide>`)

	page := `<?xml version="1.0"?>
<html xmlns="http://www.w3.org/1999/xhtml"><body>
  <img src="https://example.invalid/cover.jpg"/>
  <img src="data:image/gif;base64,R0lGODlhAQABAAAAACw="/>
</body></html>`

	name, _ := coverOf(t, pkg, entry{name: "OEBPS/cover.xhtml", content: page})
	if name != "" {
		t.Errorf("cover is %q, want none", name)
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

// withMetadata builds a book with extra elements inside <metadata>.
func withMetadata(t testing.TB, extra string) *epub.Reader {
	t.Helper()

	pkg := strings.Replace(packageDocument, "  </metadata>", extra+"\n  </metadata>", 1)
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

	return reader
}

// Twenty-nine of the reference library's books are shaped like this one.
func TestSeriesFromCalibreMetas(t *testing.T) {
	meta := withMetadata(t, `
    <meta name="calibre:series" content="A Song of Ice and Fire"/>
    <meta name="calibre:series_index" content="2.0"/>`).Metadata()

	if meta.Series != "A Song of Ice and Fire" {
		t.Errorf("series is %q", meta.Series)
	}
	if meta.SeriesIndex != 2 {
		t.Errorf("series index is %v, want 2", meta.SeriesIndex)
	}
}

// Three of them are shaped like this one, which is the form with a standard
// behind it.
func TestSeriesFromEPUB3Collection(t *testing.T) {
	meta := withMetadata(t, `
    <meta property="belongs-to-collection" id="id-3">Die Legende von Gold und Jade</meta>
    <meta refines="#id-3" property="collection-type">series</meta>
    <meta refines="#id-3" property="group-position">1</meta>`).Metadata()

	if meta.Series != "Die Legende von Gold und Jade" {
		t.Errorf("series is %q", meta.Series)
	}
	if meta.SeriesIndex != 1 {
		t.Errorf("series index is %v, want 1", meta.SeriesIndex)
	}
}

// A collection with no type declared is still a collection worth shelving by.
func TestSeriesWithoutADeclaredType(t *testing.T) {
	meta := withMetadata(t, `
    <meta property="belongs-to-collection" id="c1">Verborgene Schätze</meta>`).Metadata()

	if meta.Series != "Verborgene Schätze" {
		t.Errorf("series is %q", meta.Series)
	}
	if meta.SeriesIndex != 0 {
		t.Errorf("series index is %v, want 0", meta.SeriesIndex)
	}
}

// An omnibus is a set inside a series. Filing the book under the inner one puts
// each volume of a trilogy on a shelf of its own, which is the opposite of what
// a series shelf is for.
func TestTheOutermostCollectionWins(t *testing.T) {
	meta := withMetadata(t, `
    <meta property="belongs-to-collection" id="outer">The Wheel of Time</meta>
    <meta refines="#outer" property="collection-type">series</meta>
    <meta refines="#outer" property="group-position">3</meta>
    <meta property="belongs-to-collection" id="inner" refines="#outer">Boxed Set</meta>
    <meta refines="#inner" property="collection-type">set</meta>`).Metadata()

	if meta.Series != "The Wheel of Time" {
		t.Errorf("series is %q, want the outer collection", meta.Series)
	}
	if meta.SeriesIndex != 3 {
		t.Errorf("series index is %v, want 3", meta.SeriesIndex)
	}
}

// An imprint is not a series, and a shelf of imprints is not a thing anybody
// wants to browse.
func TestANonSeriesCollectionIsNotASeries(t *testing.T) {
	meta := withMetadata(t, `
    <meta property="belongs-to-collection" id="c1">Penguin Classics</meta>
    <meta refines="#c1" property="collection-type">publication</meta>`).Metadata()

	if meta.Series != "" {
		t.Errorf("series is %q, want none", meta.Series)
	}
}

func TestSubjectsAreReadAndDeduplicated(t *testing.T) {
	meta := withMetadata(t, `
    <dc:subject>Fantasy</dc:subject>
    <dc:subject>  Dark   Fantasy </dc:subject>
    <dc:subject>fantasy</dc:subject>
    <dc:subject></dc:subject>`).Metadata()

	want := []string{"Fantasy", "Dark Fantasy"}
	if len(meta.Subjects) != len(want) {
		t.Fatalf("subjects are %v, want %v", meta.Subjects, want)
	}
	for index, subject := range want {
		if meta.Subjects[index] != subject {
			t.Errorf("subject %d is %q, want %q", index, meta.Subjects[index], subject)
		}
	}
}

// One book in the reference library declares a hundred and one keywords. That
// is a search engine strategy, not a description, and the record should not
// carry all of it.
func TestSubjectsAreCapped(t *testing.T) {
	var declared strings.Builder
	for index := range 101 {
		fmt.Fprintf(&declared, "\n    <dc:subject>Keyword %d</dc:subject>", index)
	}

	meta := withMetadata(t, declared.String()).Metadata()
	if len(meta.Subjects) != 24 {
		t.Errorf("kept %d subjects, want the cap of 24", len(meta.Subjects))
	}
}

// A book with neither says so, rather than saying it belongs to the series "".
func TestABookWithNoSeriesOrSubjects(t *testing.T) {
	meta := withMetadata(t, "")

	if got := meta.Metadata(); got.Series != "" || got.SeriesIndex != 0 || got.Subjects != nil {
		t.Errorf("series %q index %v subjects %v, want all empty", got.Series, got.SeriesIndex, got.Subjects)
	}
}

// A series index that is not a number does not take the series down with it.
func TestAnUnparseableSeriesIndexIsZero(t *testing.T) {
	meta := withMetadata(t, `
    <meta name="calibre:series" content="Discworld"/>
    <meta name="calibre:series_index" content="the first one"/>`).Metadata()

	if meta.Series != "Discworld" {
		t.Errorf("series is %q", meta.Series)
	}
	if meta.SeriesIndex != 0 {
		t.Errorf("series index is %v, want 0", meta.SeriesIndex)
	}
}

// hugeSpine builds an archive whose spine names the given number of documents,
// all of them the same short file.
//
// That is the shape of the problem: the spine costs a line each, so an archive
// small enough to upload can ask for as much reading as it likes. The manifest
// entries all point at one document because what is being counted here is how
// many times the word count is asked to open something, not what is in it.
func hugeSpine(t testing.TB, items int) *bytes.Reader {
	t.Helper()

	var manifest, spine strings.Builder
	for index := range items {
		fmt.Fprintf(&manifest, `<item id="c%d" href="text/one.xhtml" media-type="application/xhtml+xml"/>`, index)
		fmt.Fprintf(&spine, `<itemref idref="c%d"/>`, index)
	}

	opf := `<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>Endless</dc:title></metadata>
  <manifest>` + manifest.String() + `</manifest>
  <spine>` + spine.String() + `</spine>
</package>`

	return build(t, []entry{
		{name: "mimetype", content: "application/epub+zip"},
		{name: "META-INF/container.xml", content: container},
		{name: "OEBPS/content.opf", content: opf},
		{name: "OEBPS/text/one.xhtml", content: chapter(10)},
	})
}

// A spine longer than any book refuses to be counted, instead of decompressing
// and tokenising its way through however much work the file asked for. The
// upload is rejected with a reason rather than the request taking the server
// down with it.
func TestWordCountRefusesAnEndlessSpine(t *testing.T) {
	book := hugeSpine(t, 10001)
	reader, err := epub.Open(book, int64(book.Len()))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	words, err := reader.WordCount()
	if !errors.Is(err, epub.ErrTooLarge) {
		t.Fatalf("WordCount returned %d, %v; want ErrTooLarge", words, err)
	}
}

// And a spine that is merely long is still counted, because the cap is there to
// exclude what is not a book rather than to have an opinion about long ones.
func TestWordCountAcceptsALongSpine(t *testing.T) {
	book := hugeSpine(t, 500)
	reader, err := epub.Open(book, int64(book.Len()))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	words, err := reader.WordCount()
	if err != nil {
		t.Fatalf("WordCount: %v", err)
	}
	if words != 5000 {
		t.Errorf("counted %d words, want 5000", words)
	}
}
