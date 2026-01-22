//
// File:        internal/kosync/middleware.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
)

func (app *Kosync) NewAuthMiddleware() fiber.Handler {
	// Return new handler
	return func(c *fiber.Ctx) error {
		// Do not require auth for signup
		if c.Path() == "/users/create" {
			return c.Next()
		}

		// get the headers
		username := c.Get("x-auth-user")
		password := c.Get("x-auth-key")

		// try to find the user
		user, found := app.Db.Users[username]
		if !found {
			app.PrintDebug("Auth", c.Locals("requestid").(string), fmt.Sprintf("Unauthorized request from unknown '%s'", username))
			return fiber.ErrUnauthorized
		}

		// verify the passwords match (both are md5 hashed)
		if user.Password != password {
			app.PrintDebug("Auth", c.Locals("requestid").(string), fmt.Sprintf("Unauthorized request from user '%s'", username))
			return fiber.ErrUnauthorized
		}

		c.Locals("current_user", user.Username)
		app.PrintDebug("Auth", c.Locals("requestid").(string), fmt.Sprintf("Authorized user '%s'", username))
		return c.Next()
	}
}
