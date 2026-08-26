//
// File:        internal/epub/epub.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package epub

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
)

// containerPath is where every EPUB points at its package document.
const containerPath = "META-INF/container.xml"

// maxDocumentBytes caps a single spine document while counting words, so a
// malformed or hostile archive cannot exhaust memory.
const maxDocumentBytes = 32 << 20

// maxSpineBytes caps everything the word count reads across the whole spine, and
// maxSpineItems caps how many documents it will open at all.
//
// The per-document cap alone bounds memory but not work. Zip compresses
// repetitive markup at something like a thousand to one, and the spine may name
// as many documents as it likes, so an archive well inside the upload limit can
// ask for hundreds of gigabytes of decompression and XML tokenising — in the
// upload handler, on one request, as often as anybody cares to send it.
//
// The budget is far past any real book. A quarter of a gigabyte of text is on
// the order of forty million words, and the largest thing in the reference
// library is under two hundred thousand; a file that runs into either of these
// is not a book that counts wrong, it is a file with something else in mind.
const (
	maxSpineBytes = 256 << 20
	maxSpineItems = 10000
)

// ErrTooLarge is returned when an archive's text runs past what the word count
// will read.
var ErrTooLarge = errors.New("epub: the archive holds more text than this server will read")

// ErrNotEPUB is returned when the archive has no EPUB container.
var ErrNotEPUB = errors.New("epub: not an EPUB archive")

// maxSubjects caps how many subjects are kept from one book.
//
// Not a defence against a hostile file so much as against an enthusiastic one:
// a book in the reference library declares a hundred and one keywords, which is
// a search engine strategy rather than a description of what it is about.
const maxSubjects = 24

// Metadata is what the library shows about a book.
//
// Series and SeriesIndex are what a reader wants to walk down and are the least
// standardised thing in here: EPUB 3 has belongs-to-collection, Calibre wrote a
// name/content meta long before that existed, and the files in the wild are
// split between them.
type Metadata struct {
	Title       string
	Authors     []string
	Language    string
	Identifiers map[string]string
	Series      string
	SeriesIndex float64
	Subjects    []string
	SpineCount  int
}

// Reader gives access to one EPUB. Open it once and ask it for what is needed;
// the archive is parsed a single time.
type Reader struct {
	zip     *zip.Reader
	pkg     opfPackage
	opfDir  string
	byName  map[string]*zip.File
	hrefIDs map[string]string
}

type opfPackage struct {
	Metadata struct {
		Titles      []string `xml:"title"`
		Creators    []string `xml:"creator"`
		Languages   []string `xml:"language"`
		Identifiers []struct {
			Scheme string `xml:"scheme,attr"`
			Value  string `xml:",chardata"`
		} `xml:"identifier"`
		Subjects []string `xml:"subject"`
		Metas    []struct {
			ID       string `xml:"id,attr"`
			Name     string `xml:"name,attr"`
			Content  string `xml:"content,attr"`
			Property string `xml:"property,attr"`
			Refines  string `xml:"refines,attr"`
			Value    string `xml:",chardata"`
		} `xml:"meta"`
	} `xml:"metadata"`
	Manifest struct {
		Items []struct {
			ID         string `xml:"id,attr"`
			Href       string `xml:"href,attr"`
			MediaType  string `xml:"media-type,attr"`
			Properties string `xml:"properties,attr"`
		} `xml:"item"`
	} `xml:"manifest"`
	Spine struct {
		Items []struct {
			IDRef string `xml:"idref,attr"`
		} `xml:"itemref"`
	} `xml:"spine"`
	Guide struct {
		References []struct {
			Type string `xml:"type,attr"`
			Href string `xml:"href,attr"`
		} `xml:"reference"`
	} `xml:"guide"`
}

type containerXML struct {
	Rootfiles []struct {
		FullPath string `xml:"full-path,attr"`
	} `xml:"rootfiles>rootfile"`
}

// Open parses the archive's container and package document.
func Open(ra io.ReaderAt, size int64) (*Reader, error) {
	archive, err := zip.NewReader(ra, size)
	if err != nil {
		// An EPUB is a zip. Something that is not a zip is not an EPUB, and
		// callers should not have to tell those two cases apart.
		return nil, fmt.Errorf("%w: %v", ErrNotEPUB, err)
	}

	reader := &Reader{
		zip:     archive,
		byName:  make(map[string]*zip.File, len(archive.File)),
		hrefIDs: make(map[string]string),
	}
	for _, file := range archive.File {
		reader.byName[file.Name] = file
	}

	raw, err := reader.readFile(containerPath)
	if err != nil {
		return nil, ErrNotEPUB
	}

	var container containerXML
	if err := unmarshal(raw, &container); err != nil || len(container.Rootfiles) == 0 {
		return nil, ErrNotEPUB
	}

	opfPath := path.Clean(container.Rootfiles[0].FullPath)
	raw, err = reader.readFile(opfPath)
	if err != nil {
		return nil, fmt.Errorf("epub: read package document: %w", err)
	}
	if err := unmarshal(raw, &reader.pkg); err != nil {
		return nil, fmt.Errorf("epub: parse package document: %w", err)
	}

	reader.opfDir = path.Dir(opfPath)
	for _, item := range reader.pkg.Manifest.Items {
		reader.hrefIDs[item.ID] = item.Href
	}

	return reader, nil
}

// Metadata returns the book's bibliographic details.
func (r *Reader) Metadata() Metadata {
	meta := Metadata{
		Identifiers: make(map[string]string),
		SpineCount:  len(r.pkg.Spine.Items),
	}

	if len(r.pkg.Metadata.Titles) > 0 {
		meta.Title = normalize(r.pkg.Metadata.Titles[0])
	}
	for _, creator := range r.pkg.Metadata.Creators {
		if trimmed := normalize(creator); trimmed != "" {
			meta.Authors = append(meta.Authors, trimmed)
		}
	}
	if len(r.pkg.Metadata.Languages) > 0 {
		meta.Language = normalize(r.pkg.Metadata.Languages[0])
	}
	for _, identifier := range r.pkg.Metadata.Identifiers {
		scheme, value := classifyIdentifier(identifier.Scheme, identifier.Value)
		if value == "" {
			continue
		}
		if _, taken := meta.Identifiers[scheme]; !taken {
			meta.Identifiers[scheme] = value
		}
	}

	meta.Series, meta.SeriesIndex = r.series()
	meta.Subjects = r.subjects()

	return meta
}

// series returns the series the book belongs to and its position in it.
//
// Two spellings, because the files are split between them. EPUB 3 declares a
// collection and refines it with its type and the book's place in it; Calibre
// has written a pair of name/content metas since long before that existed, and
// on the reference library those two never appear in the same file. The EPUB 3
// form is read first only because it is the one with a standard behind it.
func (r *Reader) series() (string, float64) {
	if name, index, found := r.collection(); found {
		return name, index
	}

	var name string
	var index float64

	for _, meta := range r.pkg.Metadata.Metas {
		switch strings.ToLower(normalize(meta.Name)) {
		case "calibre:series":
			name = normalize(meta.Content)
		case "calibre:series_index":
			index = number(meta.Content)
		}
	}

	if name == "" {
		return "", 0
	}

	return name, index
}

// collection reads the EPUB 3 belongs-to-collection form.
//
// A collection that refines another one is a set within a set — an omnibus
// inside a series — and naming the book after the inner one would file the
// three volumes of a trilogy under three different series. Only the outermost
// is taken. A collection typed as anything other than a series is skipped for
// the same reason: a publisher's imprint is not something to browse a shelf by.
func (r *Reader) collection() (string, float64, bool) {
	for _, meta := range r.pkg.Metadata.Metas {
		if strings.ToLower(normalize(meta.Property)) != "belongs-to-collection" {
			continue
		}
		if meta.Refines != "" {
			continue
		}

		name := normalize(meta.Value)
		if name == "" {
			continue
		}

		kind := r.refinement(meta.ID, "collection-type")
		if kind != "" && !strings.EqualFold(kind, "series") {
			continue
		}

		return name, number(r.refinement(meta.ID, "group-position")), true
	}

	return "", 0, false
}

// refinement returns the value of a meta refining the element with the given id.
func (r *Reader) refinement(id, property string) string {
	if id == "" {
		return ""
	}

	for _, meta := range r.pkg.Metadata.Metas {
		if strings.TrimPrefix(normalize(meta.Refines), "#") != id {
			continue
		}
		if strings.EqualFold(normalize(meta.Property), property) {
			return normalize(meta.Value)
		}
	}

	return ""
}

// subjects returns what the book says it is about, deduplicated and capped.
//
// The duplicates are real and are not typos: publishers list "Thriller" beside
// "Thrillers", and the same keyword arrives twice in different cases. Folding
// case is as far as this goes — anything cleverer would be guessing at what a
// publisher meant, and the record is supposed to say what the file says.
func (r *Reader) subjects() []string {
	var subjects []string
	seen := make(map[string]bool)

	for _, raw := range r.pkg.Metadata.Subjects {
		subject := normalize(raw)
		if subject == "" {
			continue
		}

		key := strings.ToLower(subject)
		if seen[key] {
			continue
		}
		seen[key] = true

		subjects = append(subjects, subject)
		if len(subjects) == maxSubjects {
			break
		}
	}

	return subjects
}

// number parses a metadata value that is supposed to be one. Calibre writes the
// first volume of a series as "1.0", EPUB 3 group positions are usually "1", and
// a value that is neither is not worth failing an upload over.
func number(value string) float64 {
	parsed, err := strconv.ParseFloat(normalize(value), 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) || parsed < 0 {
		return 0
	}

	return parsed
}

// normalize collapses the whitespace inside a metadata value. Real books put
// the title across several indented lines, which arrives with the newlines and
// tabs still in it.
func normalize(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

// classifyIdentifier names an identifier's scheme and strips the URN prefix.
//
// EPUB 2 carries the scheme in an attribute; EPUB 3 dropped it and puts a URN
// in the value instead, so every real EPUB 3 identifier would otherwise be
// filed as unknown.
func classifyIdentifier(scheme, value string) (string, string) {
	value = normalize(value)
	scheme = strings.ToUpper(normalize(scheme))

	for prefix, named := range map[string]string{
		"urn:isbn:": "ISBN",
		"isbn:":     "ISBN",
		"urn:uuid:": "UUID",
		"uuid:":     "UUID",
	} {
		if strings.HasPrefix(strings.ToLower(value), prefix) {
			trimmed := value[len(prefix):]
			if scheme == "" {
				scheme = named
			}

			return scheme, trimmed
		}
	}

	if scheme == "" {
		scheme = "UNKNOWN"
	}

	return scheme, value
}

// Cover returns the archive path and bytes of the cover image, if the book
// declares one. A book without a cover is not an error.
func (r *Reader) Cover() (string, []byte, error) {
	name := r.coverPath()
	if name == "" {
		return "", nil, nil
	}

	data, err := r.readFile(name)
	if err != nil {
		return "", nil, fmt.Errorf("epub: read cover: %w", err)
	}

	return name, data, nil
}

// coverPath resolves the cover image the way readers do: each place a book
// might say where its cover is, in the order in which that saying is worth
// believing, and the first one that lands on an image in the archive wins.
//
// Only the first two of those places are standards. The rest are here because
// on a real shelf the standards miss a tenth of the books: a <meta name="cover">
// whose content is the href rather than the id it is supposed to be, or an id
// that names a stylesheet, or nothing at all where the guide has said it all
// along. Every one of those files shows a cover in every other reader, which
// is what makes a blank in the library look like our bug rather than theirs.
func (r *Reader) coverPath() string {
	hrefs := r.coverHrefs()

	// Twice over the same list, because a pointer that lands on an image is
	// better evidence than one that lands on a page. Project Gutenberg's books
	// name a chapter as their cover and then name the real cover in the guide;
	// taking the pages in the first pass would file an illustration from the
	// middle of the book as the cover of it.
	for _, href := range hrefs {
		if name := r.imageFile(r.opfDir, href); name != "" {
			return name
		}
	}
	for _, href := range hrefs {
		if name := r.imageInDocument(r.opfDir, href); name != "" {
			return name
		}
	}

	return ""
}

// coverHrefs lists the hrefs that might name the cover, best first.
//
// Nothing here checks whether an href leads anywhere; a pointer that names a
// missing file, or a stylesheet, simply loses to the next one. That is what
// keeps the guesses at the end safe to make.
func (r *Reader) coverHrefs() []string {
	var hrefs []string

	// EPUB 3 says it in the manifest.
	for _, item := range r.pkg.Manifest.Items {
		if strings.Contains(item.Properties, "cover-image") {
			hrefs = append(hrefs, item.Href)
		}
	}

	// EPUB 2 says it in a meta whose content is a manifest id — or, often
	// enough to be worth trying second, the href itself.
	for _, meta := range r.pkg.Metadata.Metas {
		if !strings.EqualFold(meta.Name, "cover") || meta.Content == "" {
			continue
		}
		if href, found := r.hrefIDs[meta.Content]; found {
			hrefs = append(hrefs, href)
		}
		hrefs = append(hrefs, meta.Content)
	}

	// The guide, which names either the image or the page showing it.
	for _, reference := range r.pkg.Guide.References {
		if strings.EqualFold(reference.Type, "cover") && reference.Href != "" {
			hrefs = append(hrefs, reference.Href)
		}
	}

	// Nothing declared it. An image the book itself calls a cover is a guess,
	// but it is the guess the file is asking for.
	for _, item := range r.pkg.Manifest.Items {
		if !strings.HasPrefix(item.MediaType, "image/") {
			continue
		}
		if mentionsCover(item.ID) || mentionsCover(path.Base(item.Href)) {
			hrefs = append(hrefs, item.Href)
		}
	}

	// Last, the page the book opens on: what a reader shows first is what a
	// person recognises as the cover, whether or not the file ever labelled it.
	if len(r.pkg.Spine.Items) > 0 {
		if href, found := r.hrefIDs[r.pkg.Spine.Items[0].IDRef]; found {
			hrefs = append(hrefs, href)
		}
	}

	return hrefs
}

// imageFile returns the archive path an href names, when that is an image the
// archive actually holds.
func (r *Reader) imageFile(dir, href string) string {
	name := r.resolveFrom(dir, href)
	if name == "" || !isImage(name) {
		return ""
	}
	if _, found := r.byName[name]; !found {
		return ""
	}

	return name
}

// imageInDocument returns the first image drawn by the document an href names.
//
// A cover pointer very often does not point at an image. It points at the page
// that shows it: an XHTML wrapper holding a single <img>, or the <svg><image>
// form Calibre writes. Following that one hop is most of the difference between
// a shelf of covers and a shelf of placeholders. Only one hop, because a page
// that leads to another page is not a cover, it is a book.
func (r *Reader) imageInDocument(dir, href string) string {
	name := r.resolveFrom(dir, href)
	if name == "" || !isDocument(name) {
		return ""
	}

	raw, err := r.readFile(name)
	if err != nil {
		return ""
	}

	for _, source := range documentImages(raw) {
		// Relative to the document, which is not always where the package is.
		if found := r.imageFile(path.Dir(name), source); found != "" {
			return found
		}
	}

	return ""
}

// documentImages lists what an XHTML document draws, in the order it draws it.
func documentImages(raw []byte) []string {
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	decoder.Strict = false
	decoder.Entity = xml.HTMLEntity

	var sources []string

	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}

		element, ok := token.(xml.StartElement)
		if !ok {
			continue
		}

		// <img src> is the HTML form and <image href> the SVG one; the latter
		// arrives as either href or xlink:href, and the decoder gives the local
		// name of both as "href".
		wanted := ""
		switch strings.ToLower(element.Name.Local) {
		case "img":
			wanted = "src"
		case "image":
			wanted = "href"
		default:
			continue
		}

		for _, attribute := range element.Attr {
			if strings.EqualFold(attribute.Name.Local, wanted) && attribute.Value != "" {
				sources = append(sources, attribute.Value)

				break
			}
		}
	}

	return sources
}

// coverImageExtensions are the images a cover may be stored as.
//
// SVG is missing on purpose: the field it ends up in only takes bitmaps, and an
// SVG candidate winning here would mean discarding a perfectly good JPEG later
// in the list. A cover drawn as an <svg><image> still works — what that names
// is a bitmap.
var coverImageExtensions = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true,
}

// documentExtensions are the documents a cover pointer may lead through.
var documentExtensions = map[string]bool{
	".xhtml": true, ".html": true, ".htm": true, ".xml": true, ".svg": true,
}

func isImage(name string) bool {
	return coverImageExtensions[strings.ToLower(path.Ext(name))]
}

func isDocument(name string) bool {
	return documentExtensions[strings.ToLower(path.Ext(name))]
}

// mentionsCover reports whether a name says, in the only way a file name can,
// that it is the cover.
func mentionsCover(name string) bool {
	return strings.Contains(strings.ToLower(name), "cover")
}

// WordCount counts the words in the spine documents, in spine order.
//
// It deliberately does not count every XHTML file in the archive: alternate
// renditions and orphaned files are not paginated by the reader, so counting
// them makes the words-per-page estimate wrong in a way that is very hard to
// notice.
func (r *Reader) WordCount() (int, error) {
	if len(r.pkg.Spine.Items) > maxSpineItems {
		return 0, ErrTooLarge
	}

	total := 0
	read := 0
	for _, ref := range r.pkg.Spine.Items {
		href, found := r.hrefIDs[ref.IDRef]
		if !found {
			continue
		}

		raw, err := r.readFile(r.resolve(href))
		if err != nil {
			// A spine entry pointing at a missing file is a broken book, not a
			// reason to refuse the upload.
			continue
		}

		// Counted after the read rather than before it, because what a zip entry
		// costs is only known once it has been decompressed — and the per
		// document cap is what keeps that one read affordable.
		read += len(raw)
		if read > maxSpineBytes {
			return 0, ErrTooLarge
		}

		total += countWords(raw)
	}

	return total, nil
}

// resolve turns a manifest href into an archive path.
func (r *Reader) resolve(href string) string {
	return r.resolveFrom(r.opfDir, href)
}

// remoteHref matches an href that names something outside the archive: an http
// URL, or the data: URI a generator inlines a placeholder image as.
var remoteHref = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*:`)

// resolveFrom turns an href into an archive path, relative to the directory of
// whatever document it was written in — the package for a manifest href, the
// page itself for an image inside one.
func (r *Reader) resolveFrom(dir, href string) string {
	if index := strings.IndexAny(href, "#?"); index >= 0 {
		href = href[:index]
	}
	if decoded, err := url.PathUnescape(href); err == nil {
		href = decoded
	}

	href = strings.TrimSpace(href)
	if href == "" || remoteHref.MatchString(href) {
		return ""
	}
	if dir == "." || dir == "/" || dir == "" {
		return path.Clean(href)
	}

	return path.Join(dir, href)
}

func (r *Reader) readFile(name string) ([]byte, error) {
	file, found := r.byName[name]
	if !found {
		return nil, fmt.Errorf("epub: %s not in archive", name)
	}

	handle, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer handle.Close()

	return io.ReadAll(io.LimitReader(handle, maxDocumentBytes))
}

// unmarshal parses XML leniently. EPUBs in the wild carry HTML entities and
// occasional namespace sloppiness that a strict parser rejects.
func unmarshal(raw []byte, into any) error {
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	decoder.Strict = false
	decoder.Entity = xml.HTMLEntity

	return decoder.Decode(into)
}

// countWords extracts the text of an XHTML document and counts whitespace-
// separated tokens, skipping the parts a reader never renders.
func countWords(raw []byte) int {
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	decoder.Strict = false
	decoder.Entity = xml.HTMLEntity

	total := 0
	skipDepth := 0

	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}

		switch element := token.(type) {
		case xml.StartElement:
			if skipDepth > 0 {
				skipDepth++
				continue
			}
			switch strings.ToLower(element.Name.Local) {
			case "script", "style", "head":
				skipDepth = 1
			}
		case xml.EndElement:
			if skipDepth > 0 {
				skipDepth--
			}
		case xml.CharData:
			if skipDepth == 0 {
				total += len(strings.Fields(string(element)))
			}
		}
	}

	return total
}
