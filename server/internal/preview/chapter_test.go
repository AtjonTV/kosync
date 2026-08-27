//
// File:        internal/preview/chapter_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package preview_test

import (
	"archive/zip"
	"bytes"
	"errors"
	"strings"
	"testing"

	"git.obth.eu/atjontv/kosync/internal/epub"
	"git.obth.eu/atjontv/kosync/internal/preview"
)

const illustratedContainer = `<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>`

const illustratedPackage = `<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Zeit des Sturms</dc:title>
  </metadata>
  <manifest>
    <item id="one" href="text/one.xhtml" media-type="application/xhtml+xml"/>
    <item id="two" href="text/two.xhtml" media-type="application/xhtml+xml"/>
    <item id="small" href="images/small.png" media-type="image/png"/>
    <item id="huge" href="images/huge.png" media-type="image/png"/>
    <item id="font" href="fonts/serif.otf" media-type="application/font-sfnt"/>
  </manifest>
  <spine>
    <itemref idref="one"/>
    <itemref idref="two"/>
  </spine>
</package>`

// bookWith assembles an illustrated book around the two given chapter bodies.
func bookWith(t testing.TB, first, second string, extra map[string][]byte) *epub.Reader {
	t.Helper()

	files := map[string][]byte{
		"mimetype":               []byte("application/epub+zip"),
		"META-INF/container.xml": []byte(illustratedContainer),
		"OEBPS/content.opf":      []byte(illustratedPackage),
		"OEBPS/text/one.xhtml": []byte(`<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml">` +
			`<head><title>Der Anfang</title></head><body>` + first + `</body></html>`),
		"OEBPS/text/two.xhtml": []byte(`<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml">` +
			`<head><title>Die Fortsetzung</title></head><body>` + second + `</body></html>`),
		"OEBPS/images/small.png": []byte("a small picture"),
		"OEBPS/fonts/serif.otf":  []byte("not a picture at all"),
	}
	for name, content := range extra {
		files[name] = content
	}

	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	for name, content := range files {
		writer, err := archive.Create(name)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if _, err := writer.Write(content); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close archive: %v", err)
	}

	reader, err := epub.Open(bytes.NewReader(buffer.Bytes()), int64(buffer.Len()))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	return reader
}

func TestReadReturnsTheChapterAndWhatItIsCalled(t *testing.T) {
	book := bookWith(t, `<h1>Der Anfang</h1><p>Ein Sturm zieht auf.</p>`, `<p>Und weiter.</p>`, nil)

	chapter, err := preview.Read(book, 1)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if chapter.Index != 1 || chapter.Title != "Die Fortsetzung" {
		t.Errorf("the chapter is %d %q", chapter.Index, chapter.Title)
	}
	if chapter.HTML != "<p>Und weiter.</p>" || chapter.Truncated {
		t.Errorf("the chapter came out as %q (truncated %v)", chapter.HTML, chapter.Truncated)
	}
}

func TestReadRefusesAChapterTheBookDoesNotHave(t *testing.T) {
	book := bookWith(t, `<p>eins</p>`, `<p>zwei</p>`, nil)

	if _, err := preview.Read(book, 2); !errors.Is(err, preview.ErrNoChapter) {
		t.Errorf("reading past the end returned %v", err)
	}
}

func TestAnImageInTheArchiveTravelsWithTheChapter(t *testing.T) {
	book := bookWith(t, `<p><img src="../images/small.png" alt="Tafel"/></p>`, `<p>zwei</p>`, nil)

	chapter, err := preview.Read(book, 0)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	// "a small picture" as base64, which is what makes it an image the browser
	// can draw without asking anybody for it.
	want := `<p><img src="data:image/png;base64,YSBzbWFsbCBwaWN0dXJl" alt="Tafel"></p>`
	if chapter.HTML != want {
		t.Errorf("the chapter came out as %s", chapter.HTML)
	}
}

// The reader draws pictures, not whatever else an archive holds.
func TestSomethingThatIsNotAnImageIsNotDrawn(t *testing.T) {
	book := bookWith(t, `<p><img src="../fonts/serif.otf" alt="keine Tafel"/></p>`, `<p>zwei</p>`, nil)

	chapter, err := preview.Read(book, 0)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if chapter.HTML != "<p>keine Tafel</p>" {
		t.Errorf("the chapter came out as %s", chapter.HTML)
	}
}

// The images ride inside the answer, so one enormous plate would be paid for by
// the whole chapter's arrival. It is dropped and what the book called it stays.
func TestAnImageTooLargeToSendIsLeftBehind(t *testing.T) {
	book := bookWith(t, `<p><img src="../images/huge.png" alt="Riesige Tafel"/></p>`, `<p>zwei</p>`,
		map[string][]byte{"OEBPS/images/huge.png": bytes.Repeat([]byte("x"), 3<<20)})

	chapter, err := preview.Read(book, 0)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if chapter.HTML != "<p>Riesige Tafel</p>" {
		t.Errorf("the chapter came out as %.80s", chapter.HTML)
	}
}

// A book of full-page scans stops when the chapter's budget is spent, and the
// pictures before that one are still there.
func TestAChapterStopsDrawingWhenItsBudgetIsSpent(t *testing.T) {
	const plates = 6
	body := strings.Builder{}
	extra := map[string][]byte{}
	for plate := range plates {
		name := string(rune('a'+plate)) + ".png"
		extra["OEBPS/images/"+name] = bytes.Repeat([]byte("x"), 3<<19) // 1.5 MiB each
		body.WriteString(`<p><img src="../images/` + name + `" alt="Tafel"/></p>`)
	}

	book := bookWith(t, body.String(), `<p>zwei</p>`, extra)

	chapter, err := preview.Read(book, 0)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	drawn := strings.Count(chapter.HTML, "<img ")
	// Five fit into eight mebibytes and the sixth does not.
	if drawn != 5 {
		t.Errorf("%d of %d plates were drawn", drawn, plates)
	}
	if !strings.HasSuffix(chapter.HTML, "<p>Tafel</p>") {
		t.Errorf("the plate that did not fit did not leave its alt text behind")
	}
}

// A decorative rule drawn forty times is one picture. Charging the budget for
// each of them would stop a chapter that costs almost nothing to send.
func TestTheSamePictureIsPaidForOnce(t *testing.T) {
	body := strings.Repeat(`<p><img src="../images/small.png" alt="Tafel"/></p>`, 40)
	book := bookWith(t, body, `<p>zwei</p>`, nil)

	chapter, err := preview.Read(book, 0)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if drawn := strings.Count(chapter.HTML, "<img "); drawn != 40 {
		t.Errorf("%d of 40 repeats were drawn", drawn)
	}
}

// Every link is left as its own words, wherever it pointed. A footnote marker,
// a link to the next chapter and a link off the internet all come out the same,
// because the frame a chapter is drawn in cannot follow any of them.
func TestNoLinkSurvivesTheRebuild(t *testing.T) {
	book := bookWith(t, `<p>siehe <a href="#note">Anmerkung</a>, <a href="two.xhtml">weiter</a> `+
		`und <a href="https://example.invalid/">dort</a></p>`, `<p>zwei</p>`, nil)

	chapter, err := preview.Read(book, 0)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if chapter.HTML != "<p>siehe Anmerkung, weiter und dort</p>" {
		t.Errorf("the chapter came out as %s", chapter.HTML)
	}
}
