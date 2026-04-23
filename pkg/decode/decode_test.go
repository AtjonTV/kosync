//
// File:        pkg/decode/decode_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package decode

import (
	"testing"
)

type SampleStruct struct {
	StringField string `env:"TEST_STR" default:"TeSt"`
	IntField    int    `env:"TEST_INT" default:"1337"`
	BoolField   bool   `env:"TEST_BOOL" default:"true"`
}

var data = map[string]any{
	"TEST_STR":  "Other Test",
	"TEST_INT":  42,
	"TEST_BOOL": false,
}

func TestStructFromMap(t *testing.T) {
	var s = SampleStruct{}
	err := StructFromMap(&s, "env", data)
	if err != nil {
		t.Fatalf("Failed to decode struct: %v", err)
	}

	if s.StringField != "Other Test" {
		t.Errorf("Expected 'Other Test', got '%s'", s.StringField)
	}

	if s.IntField != 42 {
		t.Errorf("Expected 42, got '%d'", s.IntField)
	}

	if s.BoolField {
		t.Errorf("Expected false, got '%t'", s.BoolField)
	}
}

func TestStructFromEnv(t *testing.T) {
	var s = SampleStruct{}
	err := StructFromEnv(&s)
	if err != nil {
		t.Fatalf("Failed to decode struct from environment: %v", err)
	}

	if s.StringField != "TeSt" {
		t.Errorf("Expected 'TeSt', got '%s'", s.StringField)
	}

	if s.IntField != 1337 {
		t.Errorf("Expected 1337, got '%d'", s.IntField)
	}

	if !s.BoolField {
		t.Errorf("Expected true, got '%t'", s.BoolField)
	}
}
