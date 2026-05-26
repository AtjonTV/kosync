//
// File:        internal/kosync/i18n_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		name       string
		acceptLang string
		queryLang  string
		expected   Language
	}{
		{"Query DE", "", "de", DE},
		{"Query EN", "", "en", EN},
		{"Query DE prefix", "", "de-DE", DE},
		{"Query EN prefix", "", "en-US", EN},
		{"Query overrides Accept-Lang", "en", "de", DE},
		{"Accept-Lang DE", "de", "", DE},
		{"Accept-Lang EN", "en", "", EN},
		{"Accept-Lang sub-tag DE", "de-CH", "", DE},
		{"Accept-Lang sub-tag EN", "en-GB", "", EN},
		{"Accept-Lang with quality weight DE preferred", "de, en-US;q=0.7", "", DE},
		{"Accept-Lang with quality weight EN preferred", "en, de;q=0.7", "", EN},
		{"Accept-Lang with quality weight unsupported preferred", "da, de;q=0.8", "", DE},
		{"Accept-Lang fallback", "fr-FR", "", EN},
		{"Empty fallback", "", "", EN},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := DetectLanguage(tt.acceptLang, tt.queryLang)
			if res != tt.expected {
				t.Errorf("DetectLanguage(%q, %q) = %v; expected %v", tt.acceptLang, tt.queryLang, res, tt.expected)
			}
		})
	}
}

func TestTranslate(t *testing.T) {
	tests := []struct {
		lang     Language
		key      string
		expected string
	}{
		{EN, "err_user_already_exists", "user already exists"},
		{DE, "err_user_already_exists", "Benutzer existiert bereits"},
		{EN, "err_webui_disabled", "WebUI is not enabled. If you want to use the web interface, restart KOsync with the --webui flag."},
		{DE, "err_webui_disabled", "Die WebUI ist nicht aktiviert. Wenn du die Weboberfläche nutzen möchtest, starte KOsync mit dem Flag --webui neu."},
		{EN, "unknown_key", "unknown_key"},
		{Language("fr"), "err_user_already_exists", "user already exists"}, // Fallback to EN
	}

	for _, tt := range tests {
		t.Run(string(tt.lang)+"_"+tt.key, func(t *testing.T) {
			res := Translate(tt.lang, tt.key)
			if res != tt.expected {
				t.Errorf("Translate(%v, %q) = %q; expected %q", tt.lang, tt.key, res, tt.expected)
			}
		})
	}
}

func TestI18nMiddleware(t *testing.T) {
	app := fiber.New()
	koapp := &Kosync{}
	app.Use(koapp.NewI18nMiddleware())

	app.Get("/test-lang", func(c fiber.Ctx) error {
		lang := GetLanguageFromFiber(c)
		return c.SendString(string(lang))
	})

	t.Run("Default English", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test-lang", nil)
		resp, _ := app.Test(req)
		body := make([]byte, 100)
		n, _ := resp.Body.Read(body)
		lang := string(body[:n])
		if lang != "en" {
			t.Errorf("expected en, got %s", lang)
		}
	})

	t.Run("Accept-Language German", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test-lang", nil)
		req.Header.Set("Accept-Language", "de-DE,de;q=0.9,en;q=0.8")
		resp, _ := app.Test(req)
		body := make([]byte, 100)
		n, _ := resp.Body.Read(body)
		lang := string(body[:n])
		if lang != "de" {
			t.Errorf("expected de, got %s", lang)
		}
	})

	t.Run("Query Parameter German", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test-lang?lang=de", nil)
		resp, _ := app.Test(req)
		body := make([]byte, 100)
		n, _ := resp.Body.Read(body)
		lang := string(body[:n])
		if lang != "de" {
			t.Errorf("expected de, got %s", lang)
		}
	})
}
