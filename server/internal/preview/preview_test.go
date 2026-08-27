//
// File:        internal/preview/preview_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package preview_test

import (
	"strings"
	"testing"

	"git.obth.eu/atjontv/kosync/internal/preview"
)

// document wraps a body in the shape a spine document arrives in.
func document(body string) []byte {
	return []byte(`<?xml version="1.0" encoding="utf-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml">
  <head><title>Kapitel 1</title><link rel="stylesheet" href="../style.css"/></head>
  <body>` + body + `</body></html>`)
}

// clean is the common case: nothing resolves, so only the markup is under test.
func clean(t testing.TB, body string) string {
	t.Helper()

	markup, truncated := preview.Clean(document(body), nil)
	if truncated {
		t.Fatalf("a short chapter was truncated: %q", markup)
	}

	return markup
}

// mustNotContain fails with the whole output, because what is wrong with a
// sanitiser's answer is never the fragment that was searched for.
func mustNotContain(t testing.TB, markup string, forbidden ...string) {
	t.Helper()

	for _, one := range forbidden {
		if strings.Contains(strings.ToLower(markup), strings.ToLower(one)) {
			t.Errorf("the output still holds %q: %s", one, markup)
		}
	}
}

func TestProseSurvives(t *testing.T) {
	markup := clean(t, `<h1>Kapitel 1</h1><p>Ein <em>Sturm</em> zieht auf über den `+
		`Königreichen.</p><blockquote><p>Und weiter.</p></blockquote>`+
		`<ul><li>eins</li><li>zwei</li></ul>`)

	want := `<h1>Kapitel 1</h1><p>Ein <em>Sturm</em> zieht auf über den Königreichen.</p>` +
		`<blockquote><p>Und weiter.</p></blockquote><ul><li>eins</li><li>zwei</li></ul>`
	if markup != want {
		t.Errorf("the chapter came out as\n%s\nwant\n%s", markup, want)
	}
}

func TestScriptIsGoneAlongWithWhatIsInsideIt(t *testing.T) {
	markup := clean(t, `<p>vorher</p><script>steal(document.cookie)</script><p>nachher</p>`)

	mustNotContain(t, markup, "script", "steal", "cookie")
	if !strings.Contains(markup, "vorher") || !strings.Contains(markup, "nachher") {
		t.Errorf("the prose around the script is missing: %s", markup)
	}
}

// The stylesheet in the head is never walked; one written into the body is,
// and has to go the same way.
func TestStylesheetsAreGone(t *testing.T) {
	markup := clean(t, `<style>body { display: none }</style><p>lesbar</p>`)

	mustNotContain(t, markup, "style", "display", "none")
	if !strings.Contains(markup, "lesbar") {
		t.Errorf("the prose is missing: %s", markup)
	}
}

// Every attribute is dropped unless the table names it, which is what makes an
// event handler nobody thought of go the same way as the ones that were.
func TestHandlersAndOtherAttributesAreGone(t *testing.T) {
	markup := clean(t, `<p onclick="steal()" onmouseover="steal()" style="color:red" `+
		`class="x" id="y" data-thing="z">text</p>`)

	if markup != "<p>text</p>" {
		t.Errorf("the paragraph came out as %s", markup)
	}
}

func TestFormsAndFramesAreGone(t *testing.T) {
	markup := clean(t, `<form action="https://example.invalid/"><input name="password"/>`+
		`<button>go</button></form><iframe src="https://example.invalid/"></iframe>`+
		`<object data="x.swf"></object><embed src="x.swf"/><p>text</p>`)

	mustNotContain(t, markup, "form", "input", "button", "iframe", "object", "embed", "example.invalid")
}

// An unknown element is not necessarily hostile — books are full of them — so
// what it wrapped stays and the element itself does not.
func TestAnUnknownElementIsUnwrapped(t *testing.T) {
	markup := clean(t, `<p><font color="red">rot</font> und <marquee>beweglich</marquee></p>`)

	if markup != "<p>rot und beweglich</p>" {
		t.Errorf("the paragraph came out as %s", markup)
	}
}

// An <svg> can carry script, and an SVG that is markup on the page is the one
// place that matters — inlined into an <img> it is an image and cannot.
func TestInlineSvgIsGone(t *testing.T) {
	markup := clean(t, `<p>vorher</p><svg xmlns="http://www.w3.org/2000/svg">`+
		`<script>steal()</script><image href="x.png"/></svg><p>nachher</p>`)

	mustNotContain(t, markup, "svg", "script", "steal", "image", "x.png")
}

// Nothing may leave for another server. An <img> the resolver refuses keeps
// what the book said it showed, and nothing else.
func TestRemoteImagesAreNotDrawn(t *testing.T) {
	markup := clean(t, `<p><img src="https://example.invalid/pixel.png" alt="Eine Tafel"/></p>`)

	mustNotContain(t, markup, "img", "example.invalid", "src")
	if !strings.Contains(markup, "Eine Tafel") {
		t.Errorf("the alt text is missing: %s", markup)
	}
}

func TestLinksOutOfTheBookBecomeTheirOwnText(t *testing.T) {
	markup := clean(t, `<p>siehe <a href="https://example.invalid/">dort</a> und `+
		`<a href="javascript:steal()">hier</a></p>`)

	mustNotContain(t, markup, "<a", "href", "javascript", "example.invalid")
	if markup != "<p>siehe dort und hier</p>" {
		t.Errorf("the paragraph came out as %s", markup)
	}
}

// A link inside the book goes the same way. The frame it is drawn in is a
// srcdoc, whose base address is the address of the application around it, so an
// href of any shape would take the frame to KOsync itself — sandboxed into a
// blank page. The words are what was wanted; the chapter list is the way about.
func TestALinkInsideTheBookBecomesItsOwnTextToo(t *testing.T) {
	markup := clean(t, `<p><a href="chapter7.xhtml" id="ref7">weiter</a></p>`)

	if markup != `<p>weiter</p>` {
		t.Errorf("the link came out as %s", markup)
	}
}

func TestAnImageTheArchiveHoldsIsDrawn(t *testing.T) {
	markup, _ := preview.Clean(
		document(`<p><img src="plate.png" alt="Tafel" width="600" height="junk"/></p>`),
		func(href string) (string, bool) {
			return "data:image/png;base64,AAAA", href == "plate.png"
		})

	want := `<p><img src="data:image/png;base64,AAAA" alt="Tafel" width="600"></p>`
	if markup != want {
		t.Errorf("the image came out as %s, want %s", markup, want)
	}
}

// A width is a number of pixels or it is nothing. Anything else in it is
// something a book has no business putting into the page's markup.
func TestOnlyNumbersSurviveAsSizes(t *testing.T) {
	markup := clean(t, `<table><tr><td colspan="2">a</td><td colspan="x&quot; onclick=&quot;steal()">b</td></tr></table>`)

	mustNotContain(t, markup, "onclick", "steal")
	if !strings.Contains(markup, `<td colspan="2">a</td>`) {
		t.Errorf("the table came out as %s", markup)
	}
}

// The parser rebuilds what a book left open, and the output is written from the
// rebuilt tree, so it closes whether or not the file did.
func TestUnbalancedMarkupComesOutClosed(t *testing.T) {
	markup := clean(t, `<p>eins<p>zwei<div><b>drei`)

	if markup != "<p>eins</p><p>zwei</p><div><b>drei</b></div>" {
		t.Errorf("the chapter came out as %s", markup)
	}
}

// A payload written as entities is text, and text is escaped on the way out, so
// it stays the words it was rather than becoming the markup it spells.
func TestAnEncodedPayloadStaysText(t *testing.T) {
	markup := clean(t, `<p>&lt;script&gt;steal()&lt;/script&gt;</p>`)

	if markup != `<p>&lt;script&gt;steal()&lt;/script&gt;</p>` {
		t.Errorf("the paragraph came out as %s", markup)
	}
}

// Some books are one enormous file. The chapter stops, says so, and what it
// stopped in the middle of is still closed.
func TestAnEnormousChapterStopsAndSaysSo(t *testing.T) {
	var body strings.Builder
	body.WriteString("<div>")
	for range 20000 {
		body.WriteString("<p>Ein Sturm zieht auf über den Königreichen.</p>")
	}
	body.WriteString("</div>")

	markup, truncated := preview.Clean(document(body.String()), nil)

	if !truncated {
		t.Fatalf("a chapter of %d bytes was not truncated", body.Len())
	}
	if len(markup) > 512<<10 {
		t.Errorf("the truncated chapter is %d bytes", len(markup))
	}
	if !strings.HasSuffix(markup, "</p></div>") {
		t.Errorf("the truncated chapter ends with %q", markup[max(0, len(markup)-40):])
	}
}

// A document with nothing in it is not an error, it is an empty chapter.
func TestAnEmptyDocumentIsEmpty(t *testing.T) {
	markup, truncated := preview.Clean([]byte(""), nil)

	if markup != "" || truncated {
		t.Errorf("an empty document came out as %q (truncated %v)", markup, truncated)
	}
}
