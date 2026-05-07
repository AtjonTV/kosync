//
// File:        internal/kosync/config_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"os"
	"testing"
)

func TestNewConfig(t *testing.T) {
	// Clear relevant env vars
	os.Unsetenv("DATABASE_FILE")
	os.Unsetenv("LISTEN_ADDRESS")

	conf := NewConfig(nil)
	if conf == nil {
		t.Fatal("Expected config, got nil")
	}

	// Check defaults
	if conf.DatabaseFile != "./kosync.db" {
		t.Errorf("Expected default database file ./kosync.db, got %s", conf.DatabaseFile)
	}
	if conf.ListenAddress != ":8080" {
		t.Errorf("Expected default listen address :8080, got %s", conf.ListenAddress)
	}
}

func TestNewConfigWithEnv(t *testing.T) {
	os.Setenv("DATABASE_FILE", "/tmp/test.db")
	defer os.Unsetenv("DATABASE_FILE")

	conf := NewConfig(nil)
	if conf.DatabaseFile != "/tmp/test.db" {
		t.Errorf("Expected database file /tmp/test.db, got %s", conf.DatabaseFile)
	}
}

func TestNewConfigWithFallback(t *testing.T) {
	os.Unsetenv("ENABLE_WEBUI")

	fallback := &Config{
		EnableWebUi: true,
	}

	conf := NewConfig(fallback)
	if !conf.EnableWebUi {
		t.Error("Expected EnableWebUi to be true due to fallback")
	}
}
