//
// File:        internal/kosync/models.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"fmt"

	"git.obth.eu/atjontv/kosync/pkg/decode"
)

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
	_ = decode.StructFromMap(&doc, "json", m)
	return doc
}

func (d *Document) Equals(b *Document) bool {
	return d.Id == b.Id &&
		d.OwnerId == b.OwnerId &&
		d.Title == b.Title &&
		d.CurrentLocation == b.CurrentLocation &&
		d.Progress == b.Progress &&
		d.LastReadOnDevice == b.LastReadOnDevice &&
		d.LastReadOnDeviceId == b.LastReadOnDeviceId &&
		d.LastReadAt == b.LastReadAt
}
