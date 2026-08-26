//
// File:        internal/opds/json.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package opds

import (
	"encoding/json"
	"time"
)

// schemaBook is the type every publication in this catalog is.
const schemaBook = "http://schema.org/Book"

// JSONRenderer writes OPDS 2.0, which is the Readium manifest model in JSON: a
// feed is metadata plus links plus navigation or publications, and a
// publication is metadata plus links plus images.
type JSONRenderer struct{}

// ContentType implements Renderer.
func (JSONRenderer) ContentType() string {
	return MediaFeed
}

// Render implements Renderer.
func (JSONRenderer) Render(feed *Feed) ([]byte, error) {
	return json.Marshal(jsonFeed{
		Metadata:     jsonFeedMetadata(feed),
		Links:        jsonLinks(feed.Links),
		Navigation:   jsonLinks(feed.Navigation),
		Publications: jsonPublications(feed.Publications),
	})
}

type jsonFeed struct {
	Metadata     jsonMetadata      `json:"metadata"`
	Links        []jsonLink        `json:"links,omitempty"`
	Navigation   []jsonLink        `json:"navigation,omitempty"`
	Publications []jsonPublication `json:"publications,omitempty"`
}

// jsonMetadata describes the feed.
//
// The three page counters are pointers so that a feed which is not a page of a
// longer list leaves them out entirely, while an empty page of one still says
// "numberOfItems": 0 rather than staying silent about it. A client that has just
// searched for something wants the difference between "none" and "not counted".
type jsonMetadata struct {
	Title         string `json:"title"`
	NumberOfItems *int   `json:"numberOfItems,omitempty"`
	ItemsPerPage  *int   `json:"itemsPerPage,omitempty"`
	CurrentPage   *int   `json:"currentPage,omitempty"`
}

type jsonLink struct {
	Rel       string `json:"rel,omitempty"`
	Href      string `json:"href"`
	Type      string `json:"type,omitempty"`
	Title     string `json:"title,omitempty"`
	Templated bool   `json:"templated,omitempty"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
}

type jsonPublication struct {
	Metadata jsonPublicationMetadata `json:"metadata"`
	Links    []jsonLink              `json:"links,omitempty"`
	Images   []jsonLink              `json:"images,omitempty"`
}

type jsonPublicationMetadata struct {
	Type          string       `json:"@type,omitempty"`
	Identifier    string       `json:"identifier,omitempty"`
	Title         string       `json:"title"`
	Author        []jsonAuthor `json:"author,omitempty"`
	Language      string       `json:"language,omitempty"`
	NumberOfPages int          `json:"numberOfPages,omitempty"`
	Modified      string       `json:"modified,omitempty"`
	Description   string       `json:"description,omitempty"`
}

type jsonAuthor struct {
	Name string `json:"name"`
}

func jsonFeedMetadata(feed *Feed) jsonMetadata {
	metadata := jsonMetadata{Title: feed.Title}

	if feed.Page != nil {
		metadata.NumberOfItems = &feed.Page.Total
		metadata.ItemsPerPage = &feed.Page.Size
		metadata.CurrentPage = &feed.Page.Number
	}

	return metadata
}

func jsonLinks(links []Link) []jsonLink {
	if len(links) == 0 {
		return nil
	}

	out := make([]jsonLink, 0, len(links))
	for _, link := range links {
		// The two types happen to line up today, and a conversion would say so
		// permanently. They are separate because one is what the catalog thinks
		// in and the other is what OPDS 2.0 puts on the wire.
		//lint:ignore S1016 written out on purpose, see above
		out = append(out, jsonLink{
			Rel:       link.Rel,
			Href:      link.Href,
			Type:      link.Type,
			Title:     link.Title,
			Templated: link.Templated,
			Width:     link.Width,
			Height:    link.Height,
		})
	}

	return out
}

func jsonPublications(publications []Publication) []jsonPublication {
	if len(publications) == 0 {
		return nil
	}

	out := make([]jsonPublication, 0, len(publications))
	for _, publication := range publications {
		out = append(out, jsonPublication{
			Metadata: jsonPublicationMetadata{
				Type:          schemaBook,
				Identifier:    publication.Identifier,
				Title:         publication.Title,
				Author:        jsonAuthors(publication.Authors),
				Language:      publication.Language,
				NumberOfPages: publication.Pages,
				Modified:      jsonTime(publication.Updated),
				Description:   publication.Description,
			},
			Links:  jsonLinks(publication.Links),
			Images: jsonLinks(publication.Images),
		})
	}

	return out
}

func jsonAuthors(names []string) []jsonAuthor {
	if len(names) == 0 {
		return nil
	}

	authors := make([]jsonAuthor, 0, len(names))
	for _, name := range names {
		if name == "" {
			continue
		}
		authors = append(authors, jsonAuthor{Name: name})
	}
	if len(authors) == 0 {
		return nil
	}

	return authors
}

// jsonTime formats a moment the way the Readium model wants it, which is
// RFC 3339 in UTC.
func jsonTime(moment time.Time) string {
	if moment.IsZero() {
		return ""
	}

	return moment.UTC().Format(time.RFC3339)
}
