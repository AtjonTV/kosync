//
// File:        internal/kosync/crypt.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"time"
)

type CryptState struct {
	tempPublicKey  *ed25519.PublicKey
	tempPrivateKey *ed25519.PrivateKey
	lastGenerated  int64
}

func NewCryptState() *CryptState {
	c := CryptState{}
	_ = c.RotateKeys()
	return &c
}

func (c *CryptState) Sign(payload []byte) ([]byte, error) {
	if err := c.RotateKeys(); err != nil {
		return nil, err
	}
	return ed25519.Sign(*c.tempPrivateKey, payload), nil
}

func (c *CryptState) Verify(payload, signature []byte) bool {
	return ed25519.Verify(*c.tempPublicKey, payload, signature)
}

func (c *CryptState) RotateKeys() error {
	if c.tempPrivateKey == nil || c.tempPublicKey == nil || c.areKeysExpired() {
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return err
		}
		c.tempPublicKey = &pub
		c.tempPrivateKey = &priv
		c.lastGenerated = time.Now().Unix()
	}
	return nil
}

func (c *CryptState) areKeysExpired() bool {
	return time.Now().Unix()-c.lastGenerated > 3600 // older than one hour?
}

func (c *CryptState) CreateToken(userId string) (token string, err error) {
	hash := []byte(userId)
	sign, err := c.Sign(hash[:])
	if err != nil {
		return
	}
	token = base64.URLEncoding.EncodeToString(hash[:]) + "." + base64.URLEncoding.EncodeToString(sign)
	return
}

func (c *CryptState) VerifyToken(token string) (valid bool, userId string) {
	valid = false
	if token == "" {
		LogDebug("No token provided")
		return
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		LogDebug("Invalid token format")
		return
	}
	hash, err := base64.URLEncoding.DecodeString(parts[0])
	if err != nil {
		LogDebug("Failed to decode token hash: %v", err.Error())
		return
	}
	userId = string(hash)
	sign, err := base64.URLEncoding.DecodeString(parts[1])
	if err != nil {
		LogDebug("Failed to decode token signature: %v", err.Error())
		return
	}
	return c.Verify(hash, sign), userId
}
