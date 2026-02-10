//
// File:        internal/kosync/middleware.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"slices"
	"strings"

	"github.com/gofiber/fiber/v3"
)

func (app *Kosync) NewAuthMiddleware() fiber.Handler {
	enableUrl := []string{
		"/users/auth",
		"/syncs",
		"/api/documents.all",
		"/api/documents.update",
		"/api/auth.ws",
		"/api/ws",
	}
	allowFailUrl := []string{
		"/api/ws",
	}

	log := NewKlog("auth")
	// Return new handler
	return func(c fiber.Ctx) error {
		doHandle := false
		allowFail := false
		for _, url := range enableUrl {
			if strings.HasPrefix(c.Path(), url) {
				doHandle = true
				if slices.Contains(allowFailUrl, url) {
					allowFail = true
				}
				break
			}
		}
		if !doHandle {
			log.Debug("Skipping auth check for route '%s'", c.Path())
			return c.Next()
		}

		// get the headers
		username := c.Get("x-auth-user", "")
		password := c.Get("x-auth-key", "")

		if username == "" || password == "" {
			if allowFail {
				return c.Next()
			}
			log.Error("Username or Password missing from request headers: username='%s', password='%s'", username, password)
			return fiber.ErrUnauthorized
		}

		// try to find the user
		user, found, err := app.Db.FindUserByUsername(username)
		if err != nil {
			if allowFail {
				return c.Next()
			}

			log.Error("Failed to find user '%s': %v", username, err.Error())
			return err
		}
		if !found {
			if allowFail {
				return c.Next()
			}

			log.Error("Could not find user '%s'", username)
			return fiber.ErrUnauthorized
		}

		// verify the passwords match (both are md5 hashed)
		if user.Password != password {
			if allowFail {
				return c.Next()
			}

			log.Error("Passwords do not match for user '%s'", username)
			return fiber.ErrUnauthorized
		}

		c.Locals(CtxContextUserId, user.Id)
		c.Locals(CtxContextUserName, user.Username)
		log.Debug("Successful login for user '%s'", username)
		return c.Next()
	}
}
