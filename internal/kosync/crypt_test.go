//
// File:        internal/kosync/crypt_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"testing"
)

func TestCryptStateInitialization(t *testing.T) {
	// Test with seed
	conf := CryptConfig{StaticKeySeed: "random-32-character-seed-for-key"}
	state := NewCryptState(conf)
	if !state.HasKeys() {
		t.Error("Private key should be generated with seed")
	}

	// Test without seed
	state = NewCryptState(CryptConfig{})
	if !state.HasKeys() {
		t.Error("Private key should be generated randomly")
	}
}

func TestGenerateKeys(t *testing.T) {
	state := NewDefaultCryptState()
	// Test with seed
	seed := "random-32-character-seed-for-key"
	err := state.GenerateKeys(&seed)
	if err != nil {
		t.Error("Should generate keys with seed")
	}

	// Reset state for random key test
	state = &CryptState{}
	err = state.GenerateKeys(nil)
	if err != nil {
		t.Error("Should generate keys randomly")
	}
}

func TestKeysAsPem(t *testing.T) {
	state := NewDefaultCryptState()
	seed := "random-32-character-seed-for-key"
	err := state.GenerateKeys(&seed)
	if err != nil {
		t.Error("Should generate keys before testing PEM")
	}

	pub, pri, err := state.KeysAsPem()
	if err != nil {
		t.Error("Should encode keys to PEM")
	}
	if pub == "" || pri == "" {
		t.Error("Should return non-empty PEM strings")
	}
}

func TestCreateToken(t *testing.T) {
	state := NewDefaultCryptState()
	seed := "random-32-character-seed-for-key"
	err := state.GenerateKeys(&seed)
	if err != nil {
		t.Error("Should generate keys before testing token")
	}

	token, err := state.CreateToken("user123", "testuser")
	if err != nil {
		t.Error("Should create valid token")
	}
	if token == "" {
		t.Error("Should return non-empty token")
	}
}

func TestVerifyToken(t *testing.T) {
	state := NewDefaultCryptState()
	seed := "random-32-character-seed-for-key"
	err := state.GenerateKeys(&seed)
	if err != nil {
		t.Error("Should generate keys before testing token")
	}

	// Valid token
	token, _ := state.CreateToken("user123", "testuser")
	valid, _ := state.VerifyToken(token)
	if !valid {
		t.Error("Should validate valid token")
	}

	// Invalid token
	valid, _ = state.VerifyToken("invalidtoken")
	if valid {
		t.Error("Should reject invalid token")
	}
}
