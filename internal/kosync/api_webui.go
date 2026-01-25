//
// File:        internal/kosync/api_webui.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"encoding/base64"
	"encoding/json"

	"github.com/gofiber/fiber/v2"
)

func (app *Kosync) ApiGetDocumentsAll(c *fiber.Ctx) error {
	data, found := app.Db.Users[c.Locals("current_user").(string)]
	if !found {
		return fiber.ErrNotFound
	}

	type ResponseRecord struct {
		Id       string   `json:"id"`
		Document FileData `json:"document"`
		HistoryData
	}

	result := make([]ResponseRecord, 0, len(data.Documents))
	for id, doc := range data.Documents {
		history, found := data.History[doc.DocumentId]
		if !found {
			result = append(result, ResponseRecord{id, doc, HistoryData{}})
		} else {
			result = append(result, ResponseRecord{id, doc, history})
		}
	}

	c.Set("Access-Control-Allow-Origin", "*")
	return c.JSON(result)
}

func (app *Kosync) ApiAuthBasic(c *fiber.Ctx) error {
	user, _ := app.Db.Users[c.Locals("current_user").(string)]
	type UserData struct {
		Username string `json:"username"`
		Key      string `json:"key"`
	}
	bytes, _ := json.Marshal(UserData{user.Username, user.Password})
	userObj := base64.StdEncoding.EncodeToString(bytes)
	return c.Redirect("/web?user="+userObj, fiber.StatusTemporaryRedirect)
}
