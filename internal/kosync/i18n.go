//
// File:        internal/kosync/i18n.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"strings"

	"git.obth.eu/atjontv/kosync/pkg/jmp"
	"github.com/gofiber/contrib/v3/websocket"
	"github.com/gofiber/fiber/v3"
)

type Language string

const (
	EN Language = "en"
	DE Language = "de"
)

const CtxContextLanguage = "current_language"

var defaultLanguage = EN

var messages = map[Language]map[string]string{
	EN: {
		"err_user_already_exists":       "user already exists",
		"err_access_token_invalid":      "Your access token is invalid. Make sure it is supplied like this: /api/ws/{token}",
		"err_rpc_missing_document":      "RPC call is missing the argument 'document'",
		"err_rpc_invalid_document_type": "RPC argument 'document' has invalid type",
		"err_rpc_missing_argument":       "RPC call is missing the argument '%s'",
		"err_rpc_invalid_argument_type":  "RPC call has invalid type for argument '%s'",
		"err_webui_disabled":            "WebUI is not enabled. If you want to use the web interface, restart KOsync with the --webui flag.",
	},
	DE: {
		"err_user_already_exists":       "Benutzer existiert bereits",
		"err_access_token_invalid":      "Dein Zugriffs-Token ist ungültig. Stelle sicher, dass es so übergeben wird: /api/ws/{token}",
		"err_rpc_missing_document":      "Dem RPC-Aufruf fehlt das Argument 'document'",
		"err_rpc_invalid_document_type": "Das RPC-Argument 'document' hat einen ungültigen Typ",
		"err_rpc_missing_argument":       "Dem RPC-Aufruf fehlt das Argument '%s'",
		"err_rpc_invalid_argument_type":  "Das RPC-Argument '%s' hat einen ungültigen Typ",
		"err_webui_disabled":            "Die WebUI ist nicht aktiviert. Wenn du die Weboberfläche nutzen möchtest, starte KOsync mit dem Flag --webui neu.",
	},
}

func DetectLanguage(acceptLanguage string, queryLang string) Language {
	if queryLang != "" {
		ql := strings.ToLower(queryLang)
		if ql == "de" || strings.HasPrefix(ql, "de") {
			return DE
		}
		if ql == "en" || strings.HasPrefix(ql, "en") {
			return EN
		}
	}
	if acceptLanguage != "" {
		parts := strings.Split(acceptLanguage, ",")
		for _, part := range parts {
			clean := strings.Split(part, ";")[0]
			clean = strings.TrimSpace(clean)
			clean = strings.ToLower(clean)
			if clean == "de" || strings.HasPrefix(clean, "de-") {
				return DE
			}
			if clean == "en" || strings.HasPrefix(clean, "en-") {
				return EN
			}
		}
	}
	return defaultLanguage
}

func Translate(lang Language, key string) string {
	if msgs, ok := messages[lang]; ok {
		if val, exists := msgs[key]; exists {
			return val
		}
	}
	if msgs, ok := messages[EN]; ok {
		if val, exists := msgs[key]; exists {
			return val
		}
	}
	return key
}

func GetLanguage(ctx *jmp.Context) Language {
	if ctx == nil {
		return defaultLanguage
	}
	if val, ok := ctx.Data[CtxContextLanguage]; ok {
		if lang, ok := val.(Language); ok {
			return lang
		}
		if langStr, ok := val.(string); ok {
			return Language(langStr)
		}
	}
	return defaultLanguage
}

func GetLanguageFromFiber(c fiber.Ctx) Language {
	if val := c.Locals(CtxContextLanguage); val != nil {
		if lang, ok := val.(Language); ok {
			return lang
		}
		if langStr, ok := val.(string); ok {
			return Language(langStr)
		}
	}
	return defaultLanguage
}

func GetLanguageFromWebsocket(c *websocket.Conn) Language {
	if q := c.Query("lang"); q != "" {
		return DetectLanguage("", q)
	}
	if l := c.Locals(CtxContextLanguage); l != nil {
		if lang, ok := l.(Language); ok {
			return lang
		}
		if langStr, ok := l.(string); ok {
			return Language(langStr)
		}
	}
	return defaultLanguage
}
