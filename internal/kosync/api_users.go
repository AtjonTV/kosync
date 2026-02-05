//
// File:        internal/kosync/api_users.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"fmt"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"
)

func (app *Kosync) UsersAuth(c fiber.Ctx) error {
	app.PrintDebug("Users", requestid.FromContext(c), fmt.Sprintf("Login of user '%s'", c.Locals("current_user_name").(string)))
	return c.SendStatus(fiber.StatusOK)
}

func (app *Kosync) UsersCreate(c fiber.Ctx) error {
	if app.Config.DisableRegistration {
		return fiber.ErrPaymentRequired // KORSS also returns 402
	}

	var data struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := c.Bind().Body(&data); err != nil {
		return err
	}

	app.PrintDebug("Users", requestid.FromContext(c), fmt.Sprintf("Signup of new user '%s'", data.Username))
	if _, err := app.Db.CreateUser(data.Username, data.Password); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusCreated)
}
