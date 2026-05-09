//
// File:        internal/kosync/api_auth_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"git.obth.eu/atjontv/kosync/pkg/jmp"
	"github.com/gofiber/fiber/v3"
)

func TestApiAuthForToken(t *testing.T) {
	app := fiber.New()
	koapp := &Kosync{
		Crypt: NewDefaultCryptState(),
	}

	app.Get("/auth", func(c fiber.Ctx) error {
		c.Locals(CtxContextUserId, "user123")
		c.Locals(CtxContextUserName, "testuser")
		return koapp.ApiAuthForToken(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/auth", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK, got %v", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	token := string(body)
	if token == "" {
		t.Error("Expected token in response body")
	}

	valid, userId := koapp.Crypt.VerifyToken(token)
	if !valid || userId != "user123" {
		t.Errorf("Invalid token returned: %s", token)
	}
}

func TestApiAuthBasic(t *testing.T) {
	app := fiber.New()
	koapp := &Kosync{
		Crypt: NewDefaultCryptState(),
	}

	app.Get("/auth-basic", func(c fiber.Ctx) error {
		c.Locals(CtxContextUserId, "user123")
		c.Locals(CtxContextUserName, "testuser")
		return koapp.ApiAuthBasic(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/auth-basic", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}

	if resp.StatusCode != http.StatusTemporaryRedirect {
		t.Errorf("Expected status Temporary Redirect, got %v", resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if location == "" || !contains(location, "token=") {
		t.Errorf("Expected token in redirect location, got %s", location)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s[:len(substr)] == substr || contains(s[1:], substr))
}

func TestNewAuthMiddleware(t *testing.T) {
	const syncsRoute = "/syncs"
	db, _ := NewTemporaryDatabase(true)
	defer db.Close()

	koapp := &Kosync{
		Db:    db,
		Crypt: NewDefaultCryptState(),
		Jmp:   jmp.New(),
	}

	user, _ := db.CreateUser("testuser", "md5password")

	app := fiber.New()
	app.Use(koapp.NewAuthMiddleware())
	app.Get(syncsRoute, func(c fiber.Ctx) error {
		return c.SendString("OK")
	})
	app.Get("/public", func(c fiber.Ctx) error {
		return c.SendString("PUBLIC")
	})

	// 1. Missing auth on protected route
	req := httptest.NewRequest(http.MethodGet, syncsRoute, nil)
	resp, _ := app.Test(req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 for protected route without auth, got %v", resp.StatusCode)
	}

	// 2. Public route
	req = httptest.NewRequest(http.MethodGet, "/public", nil)
	resp, _ = app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 for public route, got %v", resp.StatusCode)
	}

	// 3. Auth with Bearer Token
	token, _ := koapp.Crypt.CreateToken(user.Id, user.Username)
	req = httptest.NewRequest(http.MethodGet, syncsRoute, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, _ = app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 with valid token, got %v", resp.StatusCode)
	}

	// 4. Auth with headers
	req = httptest.NewRequest(http.MethodGet, syncsRoute, nil)
	req.Header.Set("x-auth-user", user.Username)
	req.Header.Set("x-auth-key", "md5password")
	resp, _ = app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 with valid headers, got %v", resp.StatusCode)
	}

	// 5. Auth with invalid headers
	req = httptest.NewRequest(http.MethodGet, syncsRoute, nil)
	req.Header.Set("x-auth-user", user.Username)
	req.Header.Set("x-auth-key", "wrongpassword")
	resp, _ = app.Test(req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 with invalid password, got %v", resp.StatusCode)
	}

	// 6. AllowFail route
	app.Get("/api/ws", func(c fiber.Ctx) error {
		return c.SendString("WS")
	})
	req = httptest.NewRequest(http.MethodGet, "/api/ws", nil)
	resp, _ = app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 for allowFail route without auth, got %v", resp.StatusCode)
	}

	// 7. Invalid Bearer Token
	req = httptest.NewRequest(http.MethodGet, syncsRoute, nil)
	req.Header.Set("Authorization", "Bearer invalidtoken")
	resp, _ = app.Test(req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 with invalid token, got %v", resp.StatusCode)
	}
}
