//
// File:        internal/webdav/statistics.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package webdav

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"

	// The SQLite driver PocketBase already uses. Registered under "sqlite".
	_ "modernc.org/sqlite"
)

// FileName is the only name this endpoint accepts.
//
// It is what KOReader's statistics plugin uploads to a cloud target, and
// pinning it is most of what keeps a sync endpoint from being a file host: a
// name nobody can choose is a name nobody can fill a disk with.
const FileName = "statistics.sqlite3"

// header is what every SQLite file begins with. Cheap to check and the first
// thing that rules out a JPEG somebody renamed.
var header = []byte("SQLite format 3\x00")

// wanted is the shape a KOReader statistics database has.
//
// Only the columns this server will actually read are required. A KOReader
// release that adds a column must not be refused by a server that has not heard
// of it yet — that would turn a routine device update into a sync that silently
// stops working.
var wanted = map[string][]string{
	"book":           {"id", "md5", "title", "pages"},
	"page_stat_data": {"id_book", "page", "start_time", "duration", "total_pages"},
}

// ErrNotStatistics is returned when an upload is not a KOReader statistics
// database.
var ErrNotStatistics = errors.New("not a KOReader statistics database")

// Validate reports whether the file at the given path is one of these.
//
// This runs on a file a device has just written and nothing has vouched for, so
// it is deliberately shy: the header is checked before SQLite is asked to open
// anything, the connection is read only, trusted_schema is off so that a view or
// a default in a hostile file cannot arrange to run a function, and the only
// statements are reads of the schema.
//
// It answers "is this the thing we asked for", not "is every row in it sane".
// The import will have to be careful about the contents on its own account; this
// is the gate that keeps a photo album, a log file, or a rename of something
// else from being kept at all.
func Validate(path string) error {
	// bearer:disable go_gosec_filesystem_filereadtaint
	file, err := os.Open(path) // #nosec G304 -- the path is built by this package
	if err != nil {
		return fmt.Errorf("open the upload: %w", err)
	}

	prefix := make([]byte, len(header))
	_, err = file.Read(prefix)
	closeErr := file.Close()

	if err != nil || string(prefix) != string(header) {
		return fmt.Errorf("%w: it is not a SQLite file", ErrNotStatistics)
	}
	if closeErr != nil {
		return fmt.Errorf("close the upload: %w", closeErr)
	}

	// Read only, immutable, and with the schema untrusted.
	//
	// immutable is the one that is easy to leave out and wrong to. KOReader's
	// database is in WAL mode, and opening a WAL database — even for reading —
	// makes SQLite create the -shm and -wal files beside it. Beside it means
	// inside the account's sync directory, where the only thing that should ever
	// appear is the one permitted name. Promising the file cannot change makes
	// SQLite read it as it lies and leave nothing behind, which is exactly true
	// of an upload that has already been written and closed.
	db, err := sql.Open("sqlite",
		"file:"+path+"?mode=ro&immutable=1&_pragma=trusted_schema(off)&_pragma=query_only(true)")
	if err != nil {
		return fmt.Errorf("%w: it cannot be opened", ErrNotStatistics)
	}
	defer db.Close()

	for table, columns := range wanted {
		found, err := columnsOf(db, table)
		if err != nil {
			return fmt.Errorf("%w: %s", ErrNotStatistics, err)
		}

		for _, column := range columns {
			if !found[column] {
				return fmt.Errorf("%w: %s.%s is missing", ErrNotStatistics, table, column)
			}
		}
	}

	return nil
}

// columnsOf returns the columns of one table.
//
// table_info takes the name as an argument rather than as text spliced into the
// statement, which matters here for the ordinary reason and for one more: the
// names are this package's own constants, and keeping them parameters means they
// stay that way if somebody later makes them configurable.
func columnsOf(db *sql.DB, table string) (map[string]bool, error) {
	rows, err := db.Query("SELECT name FROM pragma_table_info(?)", table)
	if err != nil {
		return nil, fmt.Errorf("read the schema of %s", table)
	}
	defer rows.Close()

	columns := map[string]bool{}
	for rows.Next() {
		name := ""
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("read the schema of %s", table)
		}
		columns[strings.ToLower(name)] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read the schema of %s", table)
	}
	if len(columns) == 0 {
		return nil, fmt.Errorf("there is no %s table", table)
	}

	return columns, nil
}
