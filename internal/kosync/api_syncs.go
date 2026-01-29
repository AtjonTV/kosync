//
// File:        internal/kosync/api_syncs.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"fmt"

	"git.obth.eu/atjontv/kosync/internal/legacy"
	"github.com/gofiber/fiber/v2"
)

func (app *Kosync) SyncsPostProgress(c *fiber.Ctx) error {
	// Parse payload
	var data legacy.DocumentData
	if err := c.BodyParser(&data); err != nil {
		return err
	}

	app.PrintDebug("Syncs", c.Locals("requestid").(string), fmt.Sprintf("User '%s' sent progress for document '%s'", c.Locals("current_user").(string), data.Document))
	if err := app.LegacyDb.AddOrUpdateDocument(c.Locals("current_user").(string), data); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusOK)
}

func (app *Kosync) SyncsGetProgress(c *fiber.Ctx) error {
	documentId := c.Params("document", "-")
	if documentId == "-" {
		return fiber.ErrNotFound
	}
	app.PrintDebug("Syncs", c.Locals("requestid").(string), fmt.Sprintf("User '%s' requested progress of document '%s'", c.Locals("current_user").(string), documentId))

	// Find document
	docData, found := app.LegacyDb.GetUser(c.Locals("current_user").(string)).Documents[documentId]
	if !found {
		return fiber.ErrNotFound
	}

	return c.JSON(legacy.DocumentData{ProgressData: docData.ProgressData, Document: documentId})
}
