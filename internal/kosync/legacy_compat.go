//
// File:        internal/kosync/legacy_compat.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"time"

	"git.obth.eu/atjontv/kosync/internal/legacy"
)

func FileDataFromNew(d *Document) legacy.FileData {
	return legacy.FileData{
		ProgressData: legacy.ProgressData{
			Progress:   d.CurrentLocation,
			Percentage: d.Progress,
			Device:     d.LastReadOnDevice,
			DeviceId:   d.LastReadOnDeviceId,
		},
		DocumentId: d.Id,
		PrettyName: d.Title,
		Timestamp:  d.LastReadAt,
	}
}

func DocumentDataToNew(f *legacy.DocumentData, ownerId string) Document {
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

func FileDataToNew(f *legacy.FileData, ownerId string) Document {
	return Document{
		Id:                 f.DocumentId,
		OwnerId:            ownerId,
		Title:              f.PrettyName,
		CurrentLocation:    f.Progress,
		Progress:           f.Percentage,
		LastReadOnDevice:   f.Device,
		LastReadOnDeviceId: f.DeviceId,
		LastReadAt:         f.Timestamp,
	}
}

func FileDataFromMap(m map[string]interface{}) legacy.FileData {
	return legacy.FileData{
		ProgressData: legacy.ProgressData{
			Progress:   m["progress"].(string),
			Percentage: float32(m["percentage"].(float64)),
			Device:     m["device"].(string),
			DeviceId:   m["device_id"].(string),
		},
		DocumentId: m["document"].(string),
		PrettyName: m["pretty_name"].(string),
	}
}
