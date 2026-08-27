//
// File:        internal/books/storage.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package books

import (
	"errors"
	"fmt"
	"io"
	"sync"

	"git.obth.eu/atjontv/kosync/internal/epub"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/pocketbase/core"
)

// ErrNoFile is returned for a book record that has no stored file. A record can
// exist without one — a failed upload, a file removed underneath the row — and
// that is not a reason for a request to fail with a server error.
var ErrNoFile = errors.New("books: the book has no stored file")

// Stored is one book's archive, open on the storage backend.
//
// It is an *epub.Reader with the two handles that keep it usable behind it, so
// a caller reads the book through the ordinary methods and closes it once.
type Stored struct {
	*epub.Reader

	closers []io.Closer
}

// Close releases the storage handles the archive was read through.
func (s *Stored) Close() error {
	var err error
	// Innermost first: the file, then the filesystem it was opened on.
	for _, closer := range s.closers {
		if closeErr := closer.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}

	return err
}

// Open opens a stored book without reading it into memory.
//
// The other way to read a stored book is openStoredBook, which pulls the whole
// file through memory in one sequential read. That is the right trade for the
// cover pass: it wants two things out of the archive, at four in the morning,
// on a backend that may be an object store where every seek is a request.
//
// It is the wrong trade for a request an operator is waiting on. A book may be
// as large as the upload limit allows, and reading all of it to show one
// chapter would mean that much memory and that much latency per page turn.
// archive/zip only ever needs the central directory and the one entry asked
// for, so what this does instead is give it something it can seek in.
func Open(app core.App, record *core.Record) (*Stored, error) {
	filename := record.GetString(schema.FieldFile)
	if filename == "" {
		return nil, ErrNoFile
	}

	system, err := app.NewFilesystem()
	if err != nil {
		return nil, fmt.Errorf("open the file storage: %w", err)
	}

	file, err := system.GetReader(record.BaseFilesPath() + "/" + filename)
	if err != nil {
		_ = system.Close()

		return nil, fmt.Errorf("open the stored book: %w", err)
	}

	reader, err := epub.Open(&seekingReaderAt{source: file}, file.Size())
	if err != nil {
		_ = file.Close()
		_ = system.Close()

		return nil, err
	}

	return &Stored{Reader: reader, closers: []io.Closer{file, system}}, nil
}

// seekingReaderAt turns a read-seeker into the io.ReaderAt that archive/zip
// wants.
//
// The mutex is not decoration. io.ReaderAt is documented as safe for parallel
// use, and this implementation is anything but: it seeks a shared handle and
// then reads from it, so two callers without the lock would each read from
// where the other had just moved to. Nothing in KOsync reads one open book from
// two goroutines today, and this is here so that nothing has to remember that.
type seekingReaderAt struct {
	mu     sync.Mutex
	source io.ReadSeeker
}

func (r *seekingReaderAt) ReadAt(into []byte, offset int64) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, err := r.source.Seek(offset, io.SeekStart); err != nil {
		return 0, err
	}

	read, err := io.ReadFull(r.source, into)
	// ReadAt reports a short read at the end of the file as io.EOF; ReadFull
	// calls the same thing unexpected. Handing archive/zip the latter makes a
	// perfectly ordinary read of the last bytes of the archive look like a
	// truncated file.
	if errors.Is(err, io.ErrUnexpectedEOF) {
		err = io.EOF
	}

	return read, err
}
