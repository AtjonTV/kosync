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
		"/api/documents.delete",
		"/api/auth.jwt",
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
		var (
			userIdentifier string
			user           *User
			found          bool
			err            error
			userPassword   *string
		)

		authHeader := c.Get("Authorization", "")
		tokenType := "Bearer"
		if strings.Contains(authHeader, tokenType) && len(authHeader) > len(tokenType) {
			rawToken, _ := strings.CutPrefix(authHeader, tokenType)
			token := strings.TrimSpace(rawToken)

			var valid bool
			valid, userIdentifier = app.Crypt.VerifyToken(token)
			if valid {
				user, found, err = app.Db.FindUserById(userIdentifier)
			} else if !valid && allowFail {
				return c.Next()
			} else if !valid && !allowFail {
				log.Error("Invalid token '%s'", token)
				return fiber.ErrUnauthorized
			}
		} else {
			// get the headers
			userIdentifier = c.Get("x-auth-user", "")
			password := c.Get("x-auth-key", "")

			if userIdentifier == "" || password == "" {
				if allowFail {
					return c.Next()
				}
				log.Error("Username or Password missing from request headers: username='%s', password='%s'", userIdentifier, password)
				return fiber.ErrUnauthorized
			}
			userPassword = &password

			// try to find the user
			user, found, err = app.Db.FindUserByUsername(userIdentifier)
		}

		if err != nil {
			if allowFail {
				return c.Next()
			}

			log.Error("Failed to find user '%s': %v", userIdentifier, err.Error())
			return err
		}
		if !found {
			if allowFail {
				return c.Next()
			}

			log.Error("Could not find user '%s'", userIdentifier)
			return fiber.ErrUnauthorized
		}

		if userPassword != nil {
			// verify the passwords match (both are md5 hashed)
			if user.Password != *userPassword {
				if allowFail {
					return c.Next()
				}

				log.Error("Passwords do not match for user '%s'", user.Username)
				return fiber.ErrUnauthorized
			}
		}

		c.Locals(CtxContextUserId, user.Id)
		c.Locals(CtxContextUserName, user.Username)
		log.Debug("Successful login for user '%s'", user.Username)
		return c.Next()
	}
}
