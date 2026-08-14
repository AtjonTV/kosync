//
// File:        internal/epub/epub.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package epub

import (
	"archive/zip"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"
)

// containerPath is where every EPUB points at its package document.
const containerPath = "META-INF/container.xml"

// maxDocumentBytes caps a single spine document while counting words, so a
// malformed or hostile archive cannot exhaust memory.
const maxDocumentBytes = 32 << 20

// ErrNotEPUB is returned when the archive has no EPUB container.
var ErrNotEPUB = errors.New("epub: not an EPUB archive")

// Metadata is what the library shows about a book.
type Metadata struct {
	Title       string
	Authors     []string
	Language    string
	Identifiers map[string]string
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
		Metas []struct {
			Name    string `xml:"name,attr"`
			Content string `xml:"content,attr"`
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

	return meta
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

// coverPath resolves the cover image the way readers do: the EPUB 3 manifest
// property first, then the EPUB 2 <meta name="cover"> pointer.
func (r *Reader) coverPath() string {
	for _, item := range r.pkg.Manifest.Items {
		if strings.Contains(item.Properties, "cover-image") {
			return r.resolve(item.Href)
		}
	}

	for _, meta := range r.pkg.Metadata.Metas {
		if !strings.EqualFold(meta.Name, "cover") || meta.Content == "" {
			continue
		}
		if href, found := r.hrefIDs[meta.Content]; found {
			return r.resolve(href)
		}
	}

	return ""
}

// WordCount counts the words in the spine documents, in spine order.
//
// It deliberately does not count every XHTML file in the archive: alternate
// renditions and orphaned files are not paginated by the reader, so counting
// them makes the words-per-page estimate wrong in a way that is very hard to
// notice.
func (r *Reader) WordCount() (int, error) {
	total := 0
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
		total += countWords(raw)
	}

	return total, nil
}

// resolve turns a manifest href into an archive path.
func (r *Reader) resolve(href string) string {
	if index := strings.IndexAny(href, "#?"); index >= 0 {
		href = href[:index]
	}
	if decoded, err := url.PathUnescape(href); err == nil {
		href = decoded
	}
	if r.opfDir == "." || r.opfDir == "/" {
		return path.Clean(href)
	}

	return path.Join(r.opfDir, href)
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
	decoder := xml.NewDecoder(strings.NewReader(string(raw)))
	decoder.Strict = false
	decoder.Entity = xml.HTMLEntity

	return decoder.Decode(into)
}

// countWords extracts the text of an XHTML document and counts whitespace-
// separated tokens, skipping the parts a reader never renders.
func countWords(raw []byte) int {
	decoder := xml.NewDecoder(strings.NewReader(string(raw)))
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
