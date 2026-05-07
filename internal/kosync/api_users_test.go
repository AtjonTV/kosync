//
// File:        internal/kosync/api_users_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestUsersAuth(t *testing.T) {
	koapp := &Kosync{}
	app := fiber.New()
	app.Get("/users/auth", koapp.UsersAuth)

	req := httptest.NewRequest(http.MethodGet, "/users/auth", nil)
	resp, _ := app.Test(req)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %v", resp.StatusCode)
	}
}

func TestUsersCreate(t *testing.T) {
	db, err := NewTemporaryDatabase(true)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	t.Run("RegistrationEnabled", func(t *testing.T) {
		koapp := &Kosync{
			Db:     db,
			Config: &Config{DisableRegistration: false},
		}
		app := fiber.New()
		app.Post("/users/create", koapp.UsersCreate)

		data := map[string]string{
			"username": "newuser",
			"password": "password123",
		}
		body, _ := json.Marshal(data)
		req := httptest.NewRequest(http.MethodPost, "/users/create", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req)

		if resp.StatusCode != http.StatusCreated {
			t.Errorf("Expected 201, got %v", resp.StatusCode)
		}

		// Verify user in DB
		user, found, _ := db.FindUserByUsername("newuser")
		if !found || user.Username != "newuser" {
			t.Errorf("User not found in DB")
		}
	})

	t.Run("RegistrationDisabled", func(t *testing.T) {
		koapp := &Kosync{
			Db:     db,
			Config: &Config{DisableRegistration: true},
		}
		app := fiber.New()
		app.Post("/users/create", koapp.UsersCreate)

		data := map[string]string{
			"username": "anotheruser",
			"password": "password123",
		}
		body, _ := json.Marshal(data)
		req := httptest.NewRequest(http.MethodPost, "/users/create", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req)

		if resp.StatusCode != http.StatusPaymentRequired {
			t.Errorf("Expected 402, got %v", resp.StatusCode)
		}
	})
}
