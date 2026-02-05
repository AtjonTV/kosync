//
// File:        internal/kosync/middleware.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"
)

func (app *Kosync) NewAuthMiddleware() fiber.Handler {
	enableUrl := []string{
		"/users/auth",
		"/syncs",
		"/api/documents.all",
		"/api/documents.update",
	}

	// Return new handler
	return func(c fiber.Ctx) error {
		doHandle := false
		for _, url := range enableUrl {
			if strings.HasPrefix(c.Path(), url) {
				doHandle = true
			}
		}
		if !doHandle {
			return c.Next()
		}

		// get the headers
		username := c.Get("x-auth-user", "")
		password := c.Get("x-auth-key", "")

		if username == "" || password == "" {
			return fiber.ErrUnauthorized
		}

		// try to find the user
		user, found, err := app.Db.FindUserByUsername(username)
		if err != nil {
			return err
		}
		if !found {
			app.PrintDebug("Auth", requestid.FromContext(c), fmt.Sprintf("Unauthorized request from unknown '%s'", username))
			return fiber.ErrUnauthorized
		}

		// verify the passwords match (both are md5 hashed)
		if user.Password != password {
			app.PrintDebug("Auth", requestid.FromContext(c), fmt.Sprintf("Unauthorized request from user '%s'", username))
			return fiber.ErrUnauthorized
		}

		c.Locals("current_user_id", user.Id)
		c.Locals("current_user_name", user.Username)
		app.PrintDebug("Auth", requestid.FromContext(c), fmt.Sprintf("Authorized user '%s'", username))
		return c.Next()
	}
}
