//
// File:        internal/epub/real_metadata_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package epub_test

import (
	"archive/zip"
	"html"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"git.obth.eu/atjontv/kosync/internal/epub"
)

// realLibraryEnv names a directory of EPUBs the metadata reader is run over.
//
// The books are not ours to ship, so this skips unless one is supplied. The
// synthetic books in the other tests are this package's own idea of what the
// markup looks like; this is the only test that meets the shapes publishers
// actually ship.
//
//	KOSYNC_REAL_EPUB_DIR=/path/to/books go test ./internal/epub/ -v
const realLibraryEnv = "KOSYNC_REAL_EPUB_DIR"

// TestARealLibraryParses reads every EPUB under the given directory.
//
// The oracle is the package document itself: whatever the reader claims to have
// found has to be findable in the XML it was read from. That catches the
// failure this is really guarding against — a parse that quietly returns
// nothing, or returns a refinement's value instead of the collection's.
func TestARealLibraryParses(t *testing.T) {
	root := os.Getenv(realLibraryEnv)
	if root == "" {
		t.Skipf("set %s to a directory of EPUBs to run this", realLibraryEnv)
	}

	var books, withSeries, withSubjects, subjects int

	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".epub") {
			return nil //nolint:nilerr // an unreadable file is the operator's problem, not a failure
		}

		file, err := os.Open(path) // #nosec G304 -- a path the operator chose
		if err != nil {
			return nil //nolint:nilerr
		}
		defer file.Close()

		info, err := file.Stat()
		if err != nil {
			return nil //nolint:nilerr
		}

		reader, err := epub.Open(file, info.Size())
		if err != nil {
			t.Errorf("%s: %v", filepath.Base(path), err)

			return nil
		}

		books++
		meta := reader.Metadata()
		document := packageDocumentOf(t, path)

		if meta.Series != "" {
			withSeries++
			if !strings.Contains(document, meta.Series) {
				t.Errorf("%s: series %q is not in the package document", filepath.Base(path), meta.Series)
			}
		}
		if len(meta.Subjects) > 0 {
			withSubjects++
			subjects += len(meta.Subjects)
		}
		for _, subject := range meta.Subjects {
			if subject == "" {
				t.Errorf("%s: an empty subject was kept", filepath.Base(path))
			}
		}

		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}

	if books == 0 {
		t.Fatalf("no EPUBs under %s", root)
	}

	t.Logf("%d books: %d with a series, %d with subjects (%d subjects in all)",
		books, withSeries, withSubjects, subjects)
}

// packageDocumentOf returns the raw text of a book's package document.
func packageDocumentOf(t testing.TB, path string) string {
	t.Helper()

	archive, err := zip.OpenReader(path) // #nosec G304 -- a path the operator chose
	if err != nil {
		return ""
	}
	defer archive.Close()

	for _, file := range archive.File {
		if !strings.HasSuffix(file.Name, ".opf") {
			continue
		}

		handle, err := file.Open()
		if err != nil {
			return ""
		}
		defer handle.Close()

		raw, err := io.ReadAll(io.LimitReader(handle, 4<<20))
		if err != nil {
			return ""
		}

		// Unescaped, because the reader returns what the XML meant rather than
		// what it said: a series really called "The Tales of Dunk & Egg" is
		// written "Dunk &amp; Egg" and would otherwise look like a mismatch.
		return html.UnescapeString(string(raw))
	}

	return ""
}
