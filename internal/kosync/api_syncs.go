//
// File:        internal/kosync/api_syncs.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"fmt"

	"git.obth.eu/atjontv/kosync/internal/legacy"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"
)

func (app *Kosync) SyncsPostProgress(c fiber.Ctx) error {
	// Parse payload
	var data legacy.DocumentData
	if err := c.Bind().Body(&data); err != nil {
		return err
	}
	doc := DocumentDataToNew(&data, c.Locals("current_user_id").(string))

	app.PrintDebug("Syncs", requestid.FromContext(c), fmt.Sprintf("User '%s' sent progress for document '%s'", c.Locals("current_user_name").(string), data.Document))
	if err := app.Db.CreateOrUpdateDocument(&doc); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusOK)
}

func (app *Kosync) SyncsGetProgress(c fiber.Ctx) error {
	documentId := c.Params("document", "-")
	if documentId == "-" {
		return fiber.ErrNotFound
	}
	app.PrintDebug("Syncs", requestid.FromContext(c), fmt.Sprintf("User '%s' requested progress of document '%s'", c.Locals("current_user_name").(string), documentId))

	// Find document
	docData, found, err := app.Db.FindDocumentById(c.Locals("current_user_id").(string), documentId)
	if err != nil {
		return err
	}
	if !found {
		return fiber.ErrNotFound
	}

	return c.JSON(FileDataFromNew(docData))
}
