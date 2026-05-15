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
        stats_raw AS (
            SELECT 
                document_id, 
                date(created_at/1000, 'unixepoch') as day,
                progress as progress_before,
                cnt as update_count
            FROM (
                SELECT 
                    document_id, 
                    progress, 
                    created_at,
                    ROW_NUMBER() OVER (PARTITION BY date(created_at/1000, 'unixepoch'), document_id ORDER BY created_at ASC) as rn,
                    COUNT(*) OVER (PARTITION BY date(created_at/1000, 'unixepoch'), document_id) as cnt
                FROM document_history
                WHERE owner_id = ? AND deleted_at IS NULL
            )
            WHERE rn = 1
        ),
        all_states AS (
            SELECT document_id, progress, created_at, owner_id, 0 as is_current FROM document_history WHERE deleted_at IS NULL
            UNION ALL
            SELECT id as document_id, progress, COALESCE(updated_at, created_at) as created_at, owner_id, 1 as is_current FROM documents WHERE deleted_at IS NULL
        ),
        daily_increases AS (
            SELECT
                s.day,
                s.document_id,
                s.update_count,
                s.progress_before,
                (SELECT a.progress FROM all_states a WHERE a.document_id = s.document_id AND a.owner_id = ? AND date(a.created_at/1000, 'unixepoch') <= s.day ORDER BY a.created_at DESC, a.is_current DESC LIMIT 1) as progress_after
            FROM stats_raw s
        ),
        stats AS (
            SELECT
                day,
                SUM(update_count) as total_updates,
                SUM(progress_after - progress_before) as total_increase
            FROM daily_increases
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
