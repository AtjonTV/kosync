//
// File:        internal/kosync/models_statistics.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

type ReadStatistics struct {
	Date             string  `json:"date"`
	UpdateCount      int     `json:"update_count"`
	ProgressIncrease float32 `json:"progress_increase"`
	ReadingTime      int     `json:"reading_time"`
}
