//
// File:        internal/kosync/middleware.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"errors"
	"slices"
	"strings"

	"github.com/gofiber/fiber/v3"
)

func (app *Kosync) handleTokenAuth(authHeader string, allowFail bool) (bool, *User, error) {
	log := NewKlog("auth")
	tokenType := "Bearer"
	rawToken, _ := strings.CutPrefix(authHeader, tokenType)
	token := strings.TrimSpace(rawToken)

	valid, userIdentifier := app.Crypt.VerifyToken(token)
	if valid {
		user, found, err := app.Db.FindUserById(userIdentifier)
		return found, user, err
	}

	if !allowFail {
		log.Error("Invalid token '%s'", token)
	}
	return false, nil, fiber.ErrUnauthorized
}

func (app *Kosync) handleHeaderAuth(c fiber.Ctx) (bool, *User, *string, error) {
	userIdentifier := c.Get("x-auth-user", "")
	password := c.Get("x-auth-key", "")

	if userIdentifier == "" || password == "" {
		return false, nil, nil, fiber.ErrUnauthorized
	}

	user, found, err := app.Db.FindUserByUsername(userIdentifier)
	return found, user, &password, err
}

func (app *Kosync) NewAuthMiddleware() fiber.Handler {
	enableUrl := []string{
		"/users/auth",
		"/syncs",
		"/api/documents.all",
		"/api/statistics.read",
		"/api/documents.update",
		"/api/documents.delete",
		"/api/documents.history.delete",
		"/api/documents.history.restore",
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
			user         *User
			found        bool
			err          error
			userPassword *string
		)

		authHeader := c.Get("Authorization", "")
		if strings.Contains(authHeader, "Bearer") && len(authHeader) > 6 {
			found, user, err = app.handleTokenAuth(authHeader, allowFail)
		} else {
			found, user, userPassword, err = app.handleHeaderAuth(c)
		}

		if err != nil && !allowFail {
			if errors.Is(err, fiber.ErrUnauthorized) {
				return err
			}
			log.Error("Auth error: %v", err.Error())
			return err
		}

		if (!found || err != nil) && allowFail {
			return c.Next()
		}

		if !found {
			log.Error("User not found or credentials missing")
			return fiber.ErrUnauthorized
		}

		if userPassword != nil && user.Password != *userPassword {
			if allowFail {
				return c.Next()
			}
			log.Error("Passwords do not match for user '%s'", user.Username)
			return fiber.ErrUnauthorized
		}

		c.Locals(CtxContextUserId, user.Id)
		c.Locals(CtxContextUserName, user.Username)
		log.Debug("Successful login for user '%s'", user.Username)
		return c.Next()
	}
}
