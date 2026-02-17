//
// File:        internal/kosync/api_auth.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import "github.com/gofiber/fiber/v3"

func (app *Kosync) ApiAuthForToken(c fiber.Ctx) error {
	logApiWeb.Debug("ApiAuthForToken")
	userId := c.Locals(CtxContextUserId).(string)
	userName := c.Locals(CtxContextUserName).(string)
	token, err := app.Crypt.CreateToken(userId, userName)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusOK).SendString(token)
}

func (app *Kosync) ApiAuthBasic(c fiber.Ctx) error {
	logApiWeb.Debug("ApiAuthBasic")
	userId := c.Locals(CtxContextUserId).(string)
	userName := c.Locals(CtxContextUserName).(string)
	token, err := app.Crypt.CreateToken(userId, userName)
	if err != nil {
		return err
	}
	logApiWeb.Debug("Redirecting to WebUI with token '%s'", token)
	return c.Redirect().Status(fiber.StatusTemporaryRedirect).To("/web?token=" + token)
}
