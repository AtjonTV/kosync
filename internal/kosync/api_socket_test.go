//
// File:        internal/kosync/api_socket_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestHandleOpenWebsocket(t *testing.T) {
	app := fiber.New()
	koapp := &Kosync{
		Crypt: NewDefaultCryptState(),
	}

	app.Get("/api/ws", koapp.HandleOpenWebsocket)

	// Case 1: Without auth
	req := httptest.NewRequest(http.MethodGet, "/api/ws", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("Expected status 303 (See Other), got %d", resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if !strings.Contains(location, "/api/ws/") {
		t.Errorf("Expected location to contain /api/ws/, got %s", location)
	}

	// Case 2: With auth
	app = fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals(CtxContextUserId, "user1")
		c.Locals(CtxContextUserName, "testuser")
		return c.Next()
	})
	app.Get("/api/ws", koapp.HandleOpenWebsocket)

	req = httptest.NewRequest(http.MethodGet, "/api/ws", nil)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("Expected status 303 (See Other), got %d", resp.StatusCode)
	}

	location = resp.Header.Get("Location")
	if !strings.Contains(location, "/api/ws/") {
		t.Errorf("Expected location to contain /api/ws/, got %s", location)
	}

	// Should contain token
	parts := strings.Split(location, "/api/ws/")
	if len(parts) < 2 || parts[1] == "" {
		t.Errorf("Expected token in location, got %s", location)
	}
}
