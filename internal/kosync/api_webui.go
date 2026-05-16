//
// File:        internal/kosync/api_webui.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
)

var logApiWeb = NewKlog("api/webui")

func (app *Kosync) ApiGetDocumentsAll(c fiber.Ctx) error {
	logApiWeb.Debug("ApiGetDocumentsAll")
	userNameVal := c.Locals(CtxContextUserName)
	if userNameVal == nil {
		logApiWeb.Error("User Name not found in context")
		return fiber.ErrUnauthorized
	}
	result, err := app.apiGetUserDocuments(userNameVal.(string))
	if err != nil {
		return err
	}

	logApiWeb.Debug("Returning %d documents", len(*result))
	c.Set("Access-Control-Allow-Origin", "*")
	return c.JSON(*result)
}

func (app *Kosync) ApiPutDocument(c fiber.Ctx) error {
	logApiWeb.Debug("ApiPutDocument")
	userNameVal := c.Locals(CtxContextUserName)
	if userNameVal == nil {
		logApiWeb.Error("User Name not found in context")
		return fiber.ErrUnauthorized
	}
	user, _, err := app.Db.FindUserByUsername(userNameVal.(string))
	if err != nil {
		logApiWeb.Error("Failed to find user '%s': %v", userNameVal.(string), err.Error())
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

	if !app.Config.DisableWebSocketApi {
		userIdVal := c.Locals(CtxContextUserId)
		if userIdVal != nil {
			go func(userId, docId string) {
				updatedDoc, _, e := app.Db.FindDocumentById(userId, docId)
				if e != nil {
					return
				}
				_ = app.PubSubAnnounce(userId, PubSubTopicUserDocuments, updatedDoc, "Document")
				app.PubSubAnnounceStatistics(userId, int64(updatedDoc.LastReadAt))
			}(userIdVal.(string), document.Id)
		}
	}

	logApiWeb.Debug("Successfully saved document '%s'", document.Id)
	return c.SendStatus(fiber.StatusNoContent)
}

func (app *Kosync) ApiDeleteDocument(c fiber.Ctx) error {
	logApiWeb.Debug("ApiDeleteDocument")
	documentId := c.Query("id")
	if documentId == "" {
		logApiWeb.Error("Missing document id in query")
		return fiber.ErrBadRequest
	}

	userIdVal := c.Locals(CtxContextUserId)
	if userIdVal == nil {
		logApiWeb.Error("User ID not found in context")
		return fiber.ErrUnauthorized
	}
	userId := userIdVal.(string)
	logApiWeb.Debug("User '%s' requested deletion of document '%s'", userId, documentId)

	if err := app.Db.DeleteDocumentById(userId, documentId); err != nil {
		logApiWeb.Error("Failed to delete document: %v", err.Error())
		return err
	}

	if !app.Config.DisableWebSocketApi {
		go func(documentId string) {
			type DocumentDeletion struct {
				DocumentId string `json:"document_id"`
			}
			_ = app.PubSubAnnounce(userId, PubSubTopicUserDocuments, DocumentDeletion{DocumentId: documentId}, "DocumentDeletion")
		}(documentId)
	}

	logApiWeb.Debug("Successfully deleted document '%s'", documentId)
	return c.SendStatus(fiber.StatusNoContent)
}

func (app *Kosync) ApiDeleteDocumentHistory(c fiber.Ctx) error {
	logApiWeb.Debug("ApiDeleteDocumentHistory")
	documentId := c.Query("id")
	lastReadAtStr := c.Query("last_read_at")
	if documentId == "" || lastReadAtStr == "" {
		logApiWeb.Error("Missing document id or last_read_at in query")
		return fiber.ErrBadRequest
	}

	lastReadAt, err := strconv.ParseInt(lastReadAtStr, 10, 64)
	if err != nil {
		logApiWeb.Error("Failed to parse last_read_at: %v", err.Error())
		return fiber.ErrBadRequest
	}

	userIdVal := c.Locals(CtxContextUserId)
	if userIdVal == nil {
		logApiWeb.Error("User ID not found in context")
		return fiber.ErrUnauthorized
	}
	userId := userIdVal.(string)
	logApiWeb.Debug("User '%s' requested deletion of history item for document '%s' at %d", userId, documentId, lastReadAt)

	if err := app.Db.DeleteDocumentHistoryItem(userId, documentId, lastReadAt); err != nil {
		logApiWeb.Error("Failed to delete history item: %v", err.Error())
		return err
	}

	if !app.Config.DisableWebSocketApi {
		go func(userId, documentId string, lastReadAt int64) {
			type HistoryDeletion struct {
				DocumentId string `json:"document_id"`
				LastReadAt int64  `json:"last_read_at"`
			}
			_ = app.PubSubAnnounce(userId, PubSubTopicUserDocuments, HistoryDeletion{DocumentId: documentId, LastReadAt: lastReadAt}, "HistoryDeletion")
		}(userId, documentId, lastReadAt)
	}

	logApiWeb.Debug("Successfully deleted history item for document '%s'", documentId)
	return c.SendStatus(fiber.StatusNoContent)
}

func (app *Kosync) ApiRestoreDocumentHistory(c fiber.Ctx) error {
	logApiWeb.Debug("ApiRestoreDocumentHistory")
	documentId := c.Query("id")
	lastReadAtStr := c.Query("last_read_at")
	if documentId == "" || lastReadAtStr == "" {
		logApiWeb.Error("Missing document id or last_read_at in query")
		return fiber.ErrBadRequest
	}

	lastReadAt, err := strconv.ParseInt(lastReadAtStr, 10, 64)
	if err != nil {
		logApiWeb.Error("Failed to parse last_read_at: %v", err.Error())
		return fiber.ErrBadRequest
	}

	userIdVal := c.Locals(CtxContextUserId)
	if userIdVal == nil {
		logApiWeb.Error("User ID not found in context")
		return fiber.ErrUnauthorized
	}
	userId := userIdVal.(string)
	logApiWeb.Debug("User '%s' requested restoration of history item for document '%s' at %d", userId, documentId, lastReadAt)

	if err := app.Db.RestoreDocumentHistoryItem(userId, documentId, lastReadAt); err != nil {
		logApiWeb.Error("Failed to restore history item: %v", err.Error())
		return err
	}

	if !app.Config.DisableWebSocketApi {
		go func(userId, documentId string, lastReadAt int64) {
			type HistoryRestore struct {
				DocumentId string `json:"document_id"`
				LastReadAt int64  `json:"last_read_at"`
			}
			_ = app.PubSubAnnounce(userId, PubSubTopicUserDocuments, HistoryRestore{DocumentId: documentId, LastReadAt: lastReadAt}, "HistoryRestore")
		}(userId, documentId, lastReadAt)
	}

	logApiWeb.Debug("Successfully restored history item for document '%s'", documentId)
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
