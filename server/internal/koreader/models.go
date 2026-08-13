//
// File:        internal/koreader/models.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package koreader

// ProgressRequest is the payload a KOReader device pushes to /koreader/syncs/progress.
//
// The field names are dictated by the KOReader sync plugin and must not change.
type ProgressRequest struct {
	Document   string  `json:"document"`
	Progress   string  `json:"progress"`
	Percentage float64 `json:"percentage"`
	Device     string  `json:"device"`
	DeviceId   string  `json:"device_id"`
}

// ProgressResponse is what a device receives from GET /koreader/syncs/progress/{document}.
//
// Timestamp is in Unix seconds. The legacy KOsync server returned its internal
// 1/10000 second unit here, which no KOReader build expects.
type ProgressResponse struct {
	Document   string  `json:"document"`
	Progress   string  `json:"progress"`
	Percentage float64 `json:"percentage"`
	Device     string  `json:"device"`
	DeviceId   string  `json:"device_id"`
	Timestamp  int64   `json:"timestamp"`
}

// PushResponse acknowledges a stored progress push.
type PushResponse struct {
	Document  string `json:"document"`
	Timestamp int64  `json:"timestamp"`
}

// AuthResponse acknowledges valid device credentials.
type AuthResponse struct {
	Authorized string `json:"authorized"`
}
