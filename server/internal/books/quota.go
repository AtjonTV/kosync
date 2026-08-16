//
// File:        internal/books/quota.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package books

import (
	"database/sql"
	"errors"
	"fmt"

	"git.obth.eu/atjontv/kosync/internal/schema"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// Usage is how much room one account's library takes.
type Usage struct {
	Books int   `json:"books"`
	Used  int64 `json:"used"`
	Quota int64 `json:"quota"`
}

// Free returns the bytes still available, or -1 when there is no limit.
func (u Usage) Free() int64 {
	if u.Quota <= 0 {
		return -1
	}
	if u.Used >= u.Quota {
		return 0
	}

	return u.Quota - u.Used
}

// UsedBy returns how many bytes an account's books take, and how many there are.
//
// Read from the stored sizes rather than from the filesystem: the sum is wanted
// on every upload and on every look at the library, and a stat call per book
// would make both slower with every book added. The two go out of step only if
// something writes to the storage behind the server's back, which nothing does.
func UsedBy(app core.App, ownerId string) (int64, int, error) {
	if ownerId == "" {
		return 0, 0, nil
	}

	totals := struct {
		Used  int64 `db:"used"`
		Books int   `db:"books"`
	}{}

	err := app.DB().
		NewQuery(`
			SELECT COALESCE(SUM([[` + schema.FieldFileSize + `]]), 0) AS used, COUNT(*) AS books
			FROM {{` + schema.CollectionBooks + `}}
			WHERE [[` + schema.FieldOwner + `]] = {:owner}
		`).
		Bind(dbx.Params{"owner": ownerId}).
		One(&totals)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, 0, fmt.Errorf("measure the library of %s: %w", ownerId, err)
	}

	return totals.Used, totals.Books, nil
}

// UsageOf reports an account's library against the configured quota.
func UsageOf(app core.App, quota int64, ownerId string) (Usage, error) {
	used, count, err := UsedBy(app, ownerId)
	if err != nil {
		return Usage{}, err
	}

	return Usage{Books: count, Used: used, Quota: quota}, nil
}

// registerQuota refuses uploads that would take an account over its limit.
//
// On the request hook rather than the record one, so that the importer, the
// migrations and the tests can still write books server side. A quota is a rule
// about what people may ask for, not an invariant of the data: an operator who
// lowers the limit below what an account already holds has not corrupted
// anything, and the account keeps every book it has — it simply cannot add one.
func registerQuota(app core.App, quota int64) {
	if quota <= 0 {
		return
	}

	app.OnRecordCreateRequest(schema.CollectionBooks).BindFunc(func(e *core.RecordRequestEvent) error {
		// The operator's own uploads through the superuser interface are not
		// somebody's library filling up, and locking the administrator out of
		// the instance they administer would be absurd.
		if e.HasSuperuserAuth() {
			return e.Next()
		}

		owner := e.Record.GetString(schema.FieldOwner)
		incoming := int64(e.Record.GetInt(schema.FieldFileSize))
		if owner == "" || incoming <= 0 {
			return e.Next()
		}

		used, _, err := UsedBy(e.App, owner)
		if err != nil {
			return err
		}

		if used+incoming > quota {
			return e.BadRequestError(fmt.Sprintf(
				"There is not enough room for that book: it needs %s and only %s of your %s is free.",
				FormatBytes(incoming), FormatBytes(quota-used), FormatBytes(quota)), nil)
		}

		return e.Next()
	})
}

// FormatBytes writes a size the way a person would say it.
//
// Binary units under their decimal names, which is what every file manager and
// every e-reader does. Being right about the prefix here would only mean
// disagreeing with the number the reader sees everywhere else.
func FormatBytes(bytes int64) string {
	if bytes < 0 {
		bytes = 0
	}

	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	value := float64(bytes)
	units := []string{"KB", "MB", "GB", "TB"}
	name := units[0]

	for _, next := range units {
		name = next
		value /= unit
		if value < unit {
			break
		}
	}

	if value < 10 {
		return fmt.Sprintf("%.1f %s", value, name)
	}

	return fmt.Sprintf("%.0f %s", value, name)
}
