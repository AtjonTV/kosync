//
// File:        internal/kosync/database_models.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

type Database struct {
	Schema int                 `json:"schema"`
	Config ConfigData          `json:"config"`
	Users  map[string]UserData `json:"users"`
}

type ConfigData struct {
	ListenAddress       string `json:"listen_address"`
	DisableRegistration bool   `json:"disable_registration"`
	DebugLog            bool   `json:"enable_debug_log"`
	StoreHistory        bool   `json:"store_history"`
	BackupEncodingType  string `json:"backup_encoding_type"`
	BackupOnStartup     bool   `json:"backup_on_startup"`
	WebUi               bool   `json:"enable_webui"`
}

type UserData struct {
	Username  string                 `json:"username"`
	Password  string                 `json:"password"`
	Documents map[string]FileData    `json:"documents"`
	History   map[string]HistoryData `json:"history"`
}

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

type HistoryData struct {
	DocumentHistory []FileData `json:"document_history"`
}
