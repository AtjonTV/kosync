//
// File:        pkg/environ/environ_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package environ_test

import (
	"fmt"
	"os"
	"testing"

	"git.obth.eu/atjontv/kosync/pkg/environ"
)

const (
	EnvTestStr       = "TEST_STR"
	EnvTestInt       = "TEST_INT"
	EnvTestBoolTrue  = "TEST_BOOL_TRUE"
	EnvTestBoolFalse = "TEST_BOOL_FALSE"
)

var envs = map[string]any{
	EnvTestStr:        "test",
	"TEST_INT":        123,
	"TEST_BOOL_TRUE":  true,
	"TEST_BOOL_FALSE": false,
}

func prepareEnv() {
	for key, value := range envs {
		_ = os.Setenv(key, fmt.Sprintf("%v", value))
	}
}

func TestGetEnv(t *testing.T) {
	prepareEnv()

	strVal := environ.GetEnv(EnvTestStr, "")
	if strVal != "test" {
		t.Errorf("Expected %s to be '%s', got '%s'", EnvTestStr, envs[EnvTestStr], strVal)
	}
}

func TestGetEnvInt(t *testing.T) {
	prepareEnv()

	intVal := environ.GetEnvInt(EnvTestInt, 0)
	if intVal != 123 {
		t.Errorf("Expected %s to be '%d', got '%d'", EnvTestInt, envs[EnvTestInt], intVal)
	}
}

func TestGetEnvBool(t *testing.T) {
	prepareEnv()

	trueVal := environ.GetEnvBool(EnvTestBoolTrue, false)
	if !trueVal {
		t.Errorf("Expected %s to be '%t', got '%t'", EnvTestBoolTrue, envs[EnvTestBoolTrue], trueVal)
	}

	falseVal := environ.GetEnvBool(EnvTestBoolFalse, true)
	if falseVal {
		t.Errorf("Expected %s to be '%t', got '%t'", EnvTestBoolFalse, envs[EnvTestBoolFalse], falseVal)
	}
}
