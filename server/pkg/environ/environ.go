//
// File:        pkg/environ/environ.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package environ

import (
	"os"
	"strconv"
	"strings"
)

func GetEnv(key, fallback string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		return fallback
	}
	return value
}

func GetEnvBool(key string, fallback bool) bool {
	if s, err := strconv.ParseBool(strings.ToLower(GetEnv(key, strconv.FormatBool(fallback)))); err != nil {
		return fallback
	} else {
		return s
	}
}

func GetEnvInt(key string, fallback int) int {
	if i, err := strconv.Atoi(GetEnv(key, strconv.Itoa(fallback))); err != nil {
		return fallback
	} else {
		return i
	}
}
