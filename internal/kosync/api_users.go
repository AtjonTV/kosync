//
// File:        internal/kosync/api_users.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"errors"

	"github.com/gofiber/fiber/v3"
)

var logApiUser = NewKlog("api/users")

func (app *Kosync) UsersAuth(c fiber.Ctx) error {
	logApiUser.Debug("User auth check was successful")
	return c.SendStatus(fiber.StatusOK)
}

func (app *Kosync) UsersCreate(c fiber.Ctx) error {
	if app.Config.DisableRegistration {
		logApiUser.Debug("User registration is disabled, could not create new user.")
		return fiber.ErrPaymentRequired // KORSS also returns 402
	}

	var data struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := c.Bind().Body(&data); err != nil {
		LogError("Failed to parse request body: %v", err.Error())
		return err
	}

	logApiUser.Debug("Trying to process new user signup: '%s'", data.Username)
	if _, err := app.Db.CreateUser(data.Username, data.Password); err != nil {
		LogError("Failed to create new user: %v", err.Error())
		if errors.Is(err, ErrUserAlreadyExists) {
			lang := GetLanguageFromFiber(c)
			return fiber.NewError(fiber.StatusConflict, Translate(lang, "err_user_already_exists"))
		}
		return err
	}

	logApiUser.Debug("Successfully created new user: '%s'", data.Username)
	return c.SendStatus(fiber.StatusCreated)
}
