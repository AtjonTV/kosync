//
// File:        internal/kosync/crypt.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type CryptState struct {
	tempPublicKey  *ed25519.PublicKey
	tempPrivateKey *ed25519.PrivateKey
}

func NewCryptState(staticBase string) *CryptState {
	c := CryptState{}
	if len(staticBase) > 0 {
		_ = c.GenerateKeys(&staticBase)
	} else {
		_ = c.GenerateKeys(nil)
	}
	return &c
}

func (c *CryptState) GenerateKeys(fromStatic *string) error {
	if fromStatic != nil {
		pri := ed25519.NewKeyFromSeed([]byte(*fromStatic))
		pub := pri.Public().(ed25519.PublicKey)
		c.tempPublicKey = &pub
		c.tempPrivateKey = &pri
	} else {
		pub, pri, err := ed25519.GenerateKey(nil)
		if err != nil {
			return err
		}
		c.tempPublicKey = &pub
		c.tempPrivateKey = &pri
	}
	return nil
}

func (c *CryptState) KeysAsPem() (pub, pri string, err error) {
	if c.tempPublicKey == nil || c.tempPrivateKey == nil {
		err = errors.New("No crypt keys available. Call GenerateKeys() first.")
		return
	}

	pubX509, err := x509.MarshalPKIXPublicKey(*c.tempPublicKey)
	if err != nil {
		return
	}
	pubPem := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubX509,
	})

	priX509, err := x509.MarshalPKCS8PrivateKey(*c.tempPrivateKey)
	if err != nil {
		return
	}
	priPem := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: priX509,
	})

	return string(pubPem), string(priPem), nil
}

func (c *CryptState) CreateToken(userId, userName string) (token string, err error) {
	return jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.MapClaims{
		"sub":      userId,
		"username": userName,
		"exp":      time.Now().Add(time.Hour * 6).Unix(),
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
