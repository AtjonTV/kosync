//
// File:        internal/kosync/crypt.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"crypto/ed25519"
	"crypto/rand"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type CryptState struct {
	tempPublicKey  *ed25519.PublicKey
	tempPrivateKey *ed25519.PrivateKey
}

func NewCryptState() *CryptState {
	c := CryptState{}
	_ = c.GenerateKeys()
	return &c
}

func (c *CryptState) GenerateKeys() error {
	if c.tempPrivateKey == nil || c.tempPublicKey == nil {
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return err
		}
		c.tempPublicKey = &pub
		c.tempPrivateKey = &priv
	}
	return nil
}

func (c *CryptState) CreateToken(userId string) (token string, err error) {
	return jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.MapClaims{
		"sub": userId,
		"exp": time.Now().Add(time.Hour * 6).Unix(),
	}).SignedString(*c.tempPrivateKey)
}

func (c *CryptState) VerifyToken(token string) (valid bool, userId string) {
	jwtToken, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		return *c.tempPublicKey, nil
	})
	if err != nil {
		return
	}
	sub, err := jwtToken.Claims.GetSubject()
	if err != nil {
		return
	}
	exp, err := jwtToken.Claims.GetExpirationTime()
	if err != nil {
		return
	}
	if exp.Unix() < time.Now().Unix() {
		return
	}
	return jwtToken.Valid, sub
}
