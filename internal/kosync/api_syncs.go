//
// File:        internal/kosync/api_syncs.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"git.obth.eu/atjontv/kosync/internal/legacy"
	"github.com/gofiber/fiber/v3"
)

var logApiSyncs = NewKlog("api/syncs")

func (app *Kosync) SyncsPostProgress(c fiber.Ctx) error {
	logApiSyncs.Debug("SyncsPostProgress")
	// Parse payload
	var data legacy.DocumentData
	if err := c.Bind().Body(&data); err != nil {
		logApiSyncs.Error("Failed to parse request body: %v", err.Error())
		return err
	}
	doc := DocumentDataToNew(&data, c.Locals("current_user_id").(string))

	logApiSyncs.Debug("User '%s' sent document '%s' progress", c.Locals("current_user_name").(string), doc.Id)
	if err := app.Db.CreateOrUpdateDocument(&doc); err != nil {
		logApiSyncs.Error("Failed to save document progress: %v", err.Error())
		return err
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
	logApiSyncs.Debug("User '%s' requested progress of document '%s'", c.Locals("current_user_name").(string), documentId)

	// Find document
	docData, found, err := app.Db.FindDocumentById(c.Locals("current_user_id").(string), documentId)
	if err != nil {
		logApiSyncs.Error("Failed to find document '%s': %v", documentId, err.Error())
		return err
	}
	if !found {
		logApiSyncs.Error("Document '%s' not found", documentId)
		return fiber.ErrNotFound
	}

	logApiSyncs.Debug("Sending document state")
	return c.JSON(FileDataFromNew(docData))
}
