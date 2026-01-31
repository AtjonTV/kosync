package kosync

import (
	"os"
	"time"

	"git.obth.eu/atjontv/kosync/internal/legacy"
	"github.com/gofiber/fiber/v2/log"
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
		LastReadAt:         time.Now().Unix(),
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

func MigrateData(oldDb *legacy.LegacyDb, newDb *Database) error {
	oldDb.DbLock.Lock()
	defer (func() {
		oldDb.Db.Schema = 99
		oldDb.DbLock.Unlock()
	})()

	for _, user := range oldDb.Db.Users {
		createdUser, err := newDb.CreateUser(user.Username, user.Password)
		if err != nil {
			log.Errorf("failed to create user '%s': %v", user.Username, err)
			continue
		}

		for docId, histories := range user.History {
			for _, hist := range histories.DocumentHistory {
				newDoc := FileDataToNew(&hist, createdUser.Id)
				newDoc.Id = docId
				err := newDb.CreateOrUpdateDocument(&newDoc)
				if err != nil {
					log.Errorf("failed to create document history '%s' for user '%s': %v", newDoc.Id, createdUser.Username, err)
				}
			}
		}

		for _, doc := range user.Documents {
			newDoc := FileDataToNew(&doc, createdUser.Id)
			err := newDb.CreateOrUpdateDocument(&newDoc)
			if err != nil {
				log.Errorf("failed to create document '%s' for user '%s': %v", newDoc.Id, createdUser.Username, err)
				return err
			}
		}
	}

	boolToString := func(val bool) string {
		if val {
			return "true"
		} else {
			return "false"
		}
	}

	config := map[string]string{}
	config["LISTEN_ADDRESS"] = oldDb.Db.Config.ListenAddress
	config["ENABLE_DEBUG_LOG"] = boolToString(oldDb.Db.Config.DebugLog)
	config["DISABLE_REGISTRATION"] = boolToString(oldDb.Db.Config.DisableRegistration)
	config["ENABLE_WEBUI"] = boolToString(oldDb.Db.Config.WebUi)

	configFile, err := os.OpenFile("kosync.env", os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer func(configFile *os.File) {
		_ = configFile.Close()
	}(configFile)
	for key, value := range config {
		_, err := configFile.WriteString(key + "=" + value + "\n")
		if err != nil {
			return err
		}
	}

	return nil
}
