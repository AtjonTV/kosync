//
// File:        internal/kosync/api_syncs.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"github.com/gofiber/fiber/v3"
)

var logApiSyncs = NewKlog("api/syncs")

func (app *Kosync) SyncsPostProgress(c fiber.Ctx) error {
	logApiSyncs.Debug("SyncsPostProgress")
	// Parse payload
	var data KoProgress
	if err := c.Bind().Body(&data); err != nil {
		logApiSyncs.Error("Failed to parse request body: %v", err.Error())
		return err
	}
	doc := KoProgressToDocument(&data, c.Locals(CtxContextUserId).(string))

	logApiSyncs.Debug("User '%s' sent document '%s' progress", c.Locals(CtxContextUserName).(string), doc.Id)
	if err := app.Db.CreateOrUpdateDocument(&doc); err != nil {
		logApiSyncs.Error("Failed to save document progress: %v", err.Error())
		return err
	}
	if !app.Config.DisableWebSocketApi {
		go func(userId string) {
			updatedDoc, found, err := app.Db.FindDocumentById(userId, doc.Id)
			if err != nil || !found {
				return
			}
			_ = app.PubSubAnnounce(userId, PubSubTopicUserDocuments, updatedDoc, "Document")
			app.PubSubAnnounceStatistics(userId, int64(updatedDoc.LastReadAt))
		}(doc.OwnerId)
	}

	logApiSyncs.Debug("Successfully saved document '%s' progress with '%s'", doc.Id, doc.ProgressAsString())
	return c.SendStatus(fiber.StatusOK)
}

func (app *Kosync) SyncsGetProgress(c fiber.Ctx) error {
	logApiSyncs.Debug("SyncsGetProgress")
	documentId := c.Params("document", "-")
	if documentId == "-" {
		logApiSyncs.Error("No document id provided")
		return fiber.ErrNotFound
	}
	logApiSyncs.Debug("User '%s' requested progress of document '%s'", c.Locals(CtxContextUserName).(string), documentId)

	// Find document
	docData, found, err := app.Db.FindDocumentById(c.Locals(CtxContextUserId).(string), documentId)
	if err != nil {
		logApiSyncs.Error("Failed to find document '%s': %v", documentId, err.Error())
		return err
	}
	if !found {
		logApiSyncs.Error("Document '%s' not found", documentId)
		return fiber.ErrNotFound
	}

	logApiSyncs.Debug("Sending document state")
	return c.JSON(DocumentToKoProgressWithTime(docData))
}
