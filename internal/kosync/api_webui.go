//
// File:        internal/kosync/api_webui.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"github.com/gofiber/fiber/v3"
)

var logApiWeb = NewKlog("api/webui")

func (app *Kosync) ApiGetDocumentsAll(c fiber.Ctx) error {
	logApiWeb.Debug("ApiGetDocumentsAll")
	result, err := app.apiGetUserDocuments(c.Locals(CtxContextUserName).(string))
	if err != nil {
		return err
	}

	logApiWeb.Debug("Returning %d documents", len(*result))
	c.Set("Access-Control-Allow-Origin", "*")
	return c.JSON(*result)
}

func (app *Kosync) ApiPutDocument(c fiber.Ctx) error {
	logApiWeb.Debug("ApiPutDocument")
	user, _, err := app.Db.FindUserByUsername(c.Locals(CtxContextUserName).(string))
	if err != nil {
		logApiWeb.Error("Failed to find user '%s': %v", c.Locals(CtxContextUserName).(string), err.Error())
		return err
	}

	var document Document
	if err := c.Bind().Body(&document); err != nil {
		logApiWeb.Error("Failed to parse request body: %v", err.Error())
		return err
	}
	logApiWeb.Debug("User '%s' sent document '%s' update", user.Username, document.Id)

	if err := app.Db.CreateOrUpdateDocument(&document); err != nil {
		logApiWeb.Error("Failed to save document: %v", err.Error())
		return err
	}

	if app.Config.EnableWebSocketApi {
		go func(userId, docId string) {
			updatedDoc, _, e := app.Db.FindDocumentById(userId, docId)
			if e != nil {
				return
			}
			go func() {
				_ = app.PubSubAnnounce(userId, "user.documents", updatedDoc)
			}()
		}(c.Locals(CtxContextUserId).(string), document.Id)
	}

	logApiWeb.Debug("Successfully saved document '%s'", document.Id)
	return c.SendStatus(fiber.StatusNoContent)
}

func (app *Kosync) apiGetUserDocuments(username string) (*[]DocumentWithHistory, error) {
	user, found, err := app.Db.FindUserByUsername(username)
	if err != nil {
		logApiWeb.Error("Failed to find user '%s': %v", username, err.Error())
		return nil, err
	}
	if !found {
		logApiWeb.Error("User '%s' not found", username)
		return nil, fiber.ErrNotFound
	}

	logApiWeb.Debug("User '%s' requested all documents", user.Username)
	return app.Db.AllDocumentsOfUserWithHistory(user.Id)
}
