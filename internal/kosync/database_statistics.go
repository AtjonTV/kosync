//
// File:        internal/kosync/database_statistics.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"database/sql"
	"fmt"
)

var logDbStats = NewKlog("db/statistics")

func (db *Database) GetReadStatistics(ownerId string, days int) ([]ReadStatistics, error) {
	logDbStats.Debug("GetReadStatistics(ownerId='%s', days=%d)", ownerId, days)

	var query = `
        WITH RECURSIVE days(date) AS (
            SELECT date('now', ?)
            UNION ALL
            SELECT date(date, '+1 day') FROM days WHERE date < date('now')
        ),
        all_states AS (
            SELECT id as document_id, progress, last_read_at, owner_id FROM documents WHERE deleted_at IS NULL
            UNION ALL
            SELECT document_id, progress, last_read_at, owner_id FROM document_history WHERE deleted_at IS NULL
        ),
        daily_stats_raw AS (
            SELECT
                date(last_read_at/10000.0, 'unixepoch') as day,
                document_id,
                MAX(progress) as max_progress,
                COUNT(DISTINCT last_read_at) as update_count,
                MIN(last_read_at) as first_update_of_day
            FROM all_states
            WHERE owner_id = ?
            GROUP BY day, document_id
        ),
        daily_increase AS (
            SELECT
                s.day,
                s.document_id,
                s.update_count,
                s.max_progress,
                (SELECT MAX(a.progress) FROM all_states a WHERE a.document_id = s.document_id AND a.owner_id = ? AND a.last_read_at < s.first_update_of_day) as prev_max_progress
            FROM daily_stats_raw s
        ),
        stats AS (
            SELECT
                day,
                SUM(update_count) as total_updates,
                SUM(max_progress - COALESCE(prev_max_progress, 0)) as total_increase
            FROM daily_increase
            GROUP BY day
        )
        SELECT
            d.date,
            COALESCE(s.total_updates, 0),
            COALESCE(s.total_increase, 0)
        FROM days d
        LEFT JOIN stats s ON s.day = d.date
        ORDER BY d.date ASC;
    `

	daysParam := fmt.Sprintf("-%d days", days-1)
	rows, err := db.rawDb.Query(query, daysParam, ownerId, ownerId)
	if err != nil {
		logDbStats.Error("Failed to fetch read statistics: %v", err.Error())
		return nil, err
	}
	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	var result []ReadStatistics
	for rows.Next() {
		var stat ReadStatistics
		var increase float32
		if err := rows.Scan(&stat.Date, &stat.UpdateCount, &increase); err != nil {
			logDbStats.Error("Failed to scan read statistics: %v", err.Error())
			return nil, err
		}
		// Convert to percentage points
		stat.ProgressIncrease = increase * 100
		result = append(result, stat)
	}

	return result, nil
}

func (db *Database) GetReadStatisticsByDay(ownerId string, day string) (*ReadStatistics, error) {
	logDbStats.Debug("GetReadStatisticsByDay(ownerId='%s', day='%s')", ownerId, day)

	var query = `
        WITH all_states AS (
            SELECT id as document_id, progress, last_read_at, owner_id FROM documents WHERE deleted_at IS NULL
            UNION ALL
            SELECT document_id, progress, last_read_at, owner_id FROM document_history WHERE deleted_at IS NULL
        ),
        daily_stats_raw AS (
            SELECT
                date(last_read_at/10000.0, 'unixepoch') as day,
                document_id,
                MAX(progress) as max_progress,
                COUNT(DISTINCT last_read_at) as update_count,
                MIN(last_read_at) as first_update_of_day
            FROM all_states
            WHERE owner_id = ? AND date(last_read_at/10000.0, 'unixepoch') = ?
            GROUP BY day, document_id
        ),
        daily_increase AS (
            SELECT
                s.day,
                s.document_id,
                s.update_count,
                s.max_progress,
                (SELECT MAX(a.progress) FROM all_states a WHERE a.document_id = s.document_id AND a.owner_id = ? AND a.last_read_at < s.first_update_of_day) as prev_max_progress
            FROM daily_stats_raw s
        )
        SELECT
            day,
            SUM(update_count) as total_updates,
            SUM(max_progress - COALESCE(prev_max_progress, 0)) as total_increase
        FROM daily_increase
        GROUP BY day;
    `

	row := db.rawDb.QueryRow(query, ownerId, day, ownerId)
	var stat ReadStatistics
	stat.Date = day
	var increase float32
	err := row.Scan(&stat.Date, &stat.UpdateCount, &increase)
	if err != nil {
		if err == sql.ErrNoRows {
			// No updates for this day, return 0s
			return &ReadStatistics{Date: day, UpdateCount: 0, ProgressIncrease: 0}, nil
		}
		logDbStats.Error("Failed to fetch read statistics for day: %v", err.Error())
		return nil, err
	}
	stat.ProgressIncrease = increase * 100
	return &stat, nil
}
