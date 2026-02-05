//
// File:        internal/legacy/database_models.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package legacy

type ProgressData struct {
	Progress   string  `json:"progress"`
	Percentage float32 `json:"percentage"`
	Device     string  `json:"device"`
	DeviceId   string  `json:"device_id"`
}

type DocumentData struct {
	ProgressData
	Document string `json:"document"`
}

type FileData struct {
	ProgressData
	DocumentId string `json:"document"`
	Timestamp  int64  `json:"timestamp"`
	PrettyName string `json:"pretty_name"` // User given name of Document, set via WebUI
}
