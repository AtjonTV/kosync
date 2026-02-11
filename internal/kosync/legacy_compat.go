//
// File:        internal/kosync/legacy_compat.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"time"
)

type KoProgress struct {
	Document   string  `json:"document"`
	Progress   string  `json:"progress"`
	Percentage float32 `json:"percentage"`
	Device     string  `json:"device"`
	DeviceId   string  `json:"device_id"`
}

type KoProgressWithTime struct {
	KoProgress
	Timestamp float64 `json:"timestamp"`
}

func DocumentToKoProgressWithTime(d *Document) KoProgressWithTime {
	return KoProgressWithTime{
		KoProgress: KoProgress{
			Document:   d.Id,
			Progress:   d.CurrentLocation,
			Percentage: d.Progress,
			Device:     d.LastReadOnDevice,
			DeviceId:   d.LastReadOnDeviceId,
		},
		Timestamp: d.LastReadAt,
	}
}

func KoProgressToDocument(f *KoProgress, ownerId string) Document {
	return Document{
		Id:                 f.Document,
		OwnerId:            ownerId,
		CurrentLocation:    f.Progress,
		Progress:           f.Percentage,
		LastReadOnDevice:   f.Device,
		LastReadOnDeviceId: f.DeviceId,
		LastReadAt:         float64(time.Now().UnixMicro() / 100.0),
	}
}
