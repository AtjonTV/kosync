//
// File:        internal/kosync/api_webui.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"encoding/base64"
	"encoding/json"

	"git.obth.eu/atjontv/kosync/internal/legacy"
	"github.com/gofiber/fiber/v3"
)

type UiDocumentData struct {
	Id string `json:"id"`
	legacy.FileData
	History []legacy.FileData `json:"history"`
}

func (app *Kosync) ApiGetDocumentsAll(c fiber.Ctx) error {
	user, found, err := app.Db.FindUserByUsername(c.Locals("current_user_name").(string))
	if err != nil {
		return err
	}
	if !found {
		return fiber.ErrNotFound
	}

	docs, err := app.Db.AllDocumentsOfUser(user.Id)
	if err != nil {
		return err
	}
	result := make([]UiDocumentData, 0, len(docs))
	for i := range docs {
		history, err := app.Db.GetDocumentHistory(user.Id, docs[i].Id)
		if err != nil {
			continue
		}

		docHistory := make([]legacy.FileData, 0, len(history))
		for j := range history {
			docHistory = append(docHistory, FileDataFromNew(&history[j]))
		}

		result = append(result, UiDocumentData{Id: docs[i].Id, FileData: FileDataFromNew(&docs[i]), History: docHistory})
	}

	c.Set("Access-Control-Allow-Origin", "*")
	return c.JSON(result)
}

func (app *Kosync) ApiPutDocument(c fiber.Ctx) error {
	user, _, err := app.Db.FindUserByUsername(c.Locals("current_user_name").(string))
	if err != nil {
		return err
	}

	var document UiDocumentData
	if err := c.Bind().Body(&document); err != nil {
		return err
	}

	doc := FileDataToNew(&document.FileData, user.Id)
	if err := app.Db.CreateOrUpdateDocument(&doc); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func (app *Kosync) ApiAuthBasic(c fiber.Ctx) error {
	user, _, err := app.Db.FindUserByUsername(c.Locals("current_user_name").(string))
	if err != nil {
		return err
	}
	type UserData struct {
		Username string `json:"username"`
		Key      string `json:"key"`
	}
	bytes, _ := json.Marshal(UserData{user.Username, user.Password})
	userObj := base64.StdEncoding.EncodeToString(bytes)
	return c.Redirect().Status(fiber.StatusTemporaryRedirect).To("/web?user=" + userObj)
}
