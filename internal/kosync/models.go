//
// File:        internal/kosync/models.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

type User struct {
	Id       string
	Username string
	Password string
}

type Document struct {
	Id      string // document
	OwnerId string

	Title              string
	CurrentLocation    string  // progress
	Progress           float32 // percentage
	LastReadOnDevice   string  // device
	LastReadOnDeviceId string  // device_id
	LastReadAt         int64   // timestamp
}

type DocumentHistory struct {
	Id         string // document
	OwnerId    string
	LastReadAt int64 // timestamp

	Title              string
	CurrentLocation    string  // progress
	Progress           float32 // percentage
	LastReadOnDevice   string  // device
	LastReadOnDeviceId string  // device_id
}
