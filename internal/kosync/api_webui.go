//
// File:        internal/kosync/api_webui.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"git.obth.eu/atjontv/kosync/internal/legacy"
	"github.com/gofiber/fiber/v3"
)

type UiDocumentData struct {
	Id string `json:"id"`
	legacy.FileData
	History []legacy.FileData `json:"history"`
}

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

	var document UiDocumentData
	if err := c.Bind().Body(&document); err != nil {
		logApiWeb.Error("Failed to parse request body: %v", err.Error())
		return err
	}
	logApiWeb.Debug("User '%s' sent document '%s' update", user.Username, document.Id)

	doc := FileDataToNew(&document.FileData, user.Id)
	if err := app.Db.CreateOrUpdateDocument(&doc); err != nil {
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
				_ = app.PubSubAnnounce(userId, "user.documents", UiDocumentData{Id: docId, FileData: FileDataFromNew(updatedDoc)})
			}()
		}(c.Locals(CtxContextUserId).(string), doc.Id)
	}

	logApiWeb.Debug("Successfully saved document '%s'", doc.Id)
	return c.SendStatus(fiber.StatusNoContent)
}

func (app *Kosync) apiGetUserDocuments(username string) (*[]UiDocumentData, error) {
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
	docs, err := app.Db.AllDocumentsOfUser(user.Id)
	if err != nil {
		logApiWeb.Error("Failed to get documents for user '%s': %v", user.Username, err.Error())
		return nil, err
	}
	result := make([]UiDocumentData, 0, len(docs))
	for i := range docs {
		history, err := app.Db.GetDocumentHistory(user.Id, docs[i].Id)
		if err != nil {
			logApiWeb.Error("Failed to get history for document '%s': %v", docs[i].Id, err.Error())
			continue
		}

		docHistory := make([]legacy.FileData, 0, len(history))
		for j := range history {
			docHistory = append(docHistory, FileDataFromNew(&history[j]))
		}

		result = append(result, UiDocumentData{Id: docs[i].Id, FileData: FileDataFromNew(&docs[i]), History: docHistory})
	}
	return &result, nil
}
