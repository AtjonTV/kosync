//
// File:        internal/kosync/models.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import "fmt"

type User struct {
	Id       string
	Username string
	Password string
}

type Document struct {
	Id                 string  `json:"id"`
	OwnerId            string  `json:"owner_id"`
	Title              string  `json:"title"`
	CurrentLocation    string  `json:"current_location"`
	Progress           float32 `json:"progress"`
	LastReadOnDevice   string  `json:"last_read_on_device"`
	LastReadOnDeviceId string  `json:"last_read_on_device_id"`
	LastReadAt         float64 `json:"last_read_at"`
}

type DocumentHistory = Document

type DocumentWithHistory struct {
	Document
	History []DocumentHistory `json:"history"`
}

func (d *Document) ProgressAsString() string {
	return fmt.Sprintf("%.2f%%", d.Progress*100)
}

func DocumentFromMap(m map[string]interface{}) Document {
	doc := Document{}
	DecodeStructFromMap(m, doc, "json")
	return doc
}
