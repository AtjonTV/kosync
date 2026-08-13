//
// File:        internal/koreader/auth_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package koreader

import (
	"testing"

	"git.obth.eu/atjontv/kosync/internal/config"
	"git.obth.eu/atjontv/kosync/internal/schema"
	"git.obth.eu/atjontv/kosync/internal/testutil"
	"github.com/pocketbase/pocketbase/tests"
)

// registeredApp returns a seeded app with a caching handler attached.
func registeredApp(t testing.TB) (*tests.TestApp, *Handler) {
	t.Helper()

	app := testutil.SeededApp(t)
	conf := &config.Config{KoreaderAuthCacheTtl: 300}
	conf.Normalize()

	return app, Register(app, conf)
}

func TestAuthenticateAcceptsTheStoredDigest(t *testing.T) {
	_, handler := registeredApp(t)

	account, err := handler.authenticate(testutil.KoUsernameA, testutil.Md5Hex(testutil.KoPasswordA))
	if err != nil {
		t.Fatalf("expected the seeded credential to authenticate: %v", err)
	}
	if account.Id != testutil.IdAccountA {
		t.Errorf("expected account %q, got %q", testutil.IdAccountA, account.Id)
	}
	if account.OwnerId != testutil.IdUserA {
		t.Errorf("expected owner %q, got %q", testutil.IdUserA, account.OwnerId)
	}
}

func TestAuthenticateCachesOnlySuccesses(t *testing.T) {
	_, handler := registeredApp(t)

	if _, err := handler.authenticate(testutil.KoUsernameA, "wrong-digest"); err == nil {
		t.Fatalf("expected a wrong digest to be rejected")
	}
	if handler.cache.len() != 0 {
		t.Errorf("expected failures not to be cached, %d entries stored", handler.cache.len())
	}

	if _, err := handler.authenticate(testutil.KoUsernameA, testutil.Md5Hex(testutil.KoPasswordA)); err != nil {
		t.Fatalf("expected the seeded credential to authenticate: %v", err)
	}
	if handler.cache.len() != 1 {
		t.Errorf("expected the successful verification to be cached, %d entries stored", handler.cache.len())
	}
}

func TestRotatingAPasswordLocksOutTheOldOneImmediately(t *testing.T) {
	app, handler := registeredApp(t)

	// Warm the cache with the current password.
	if _, err := handler.authenticate(testutil.KoUsernameA, testutil.Md5Hex(testutil.KoPasswordA)); err != nil {
		t.Fatalf("expected the seeded credential to authenticate: %v", err)
	}

	account, err := app.FindRecordById(schema.CollectionKoreaderAccounts, testutil.IdAccountA)
	if err != nil {
		t.Fatalf("failed to load the credential: %v", err)
	}
	account.SetPassword(testutil.Md5Hex("a-brand-new-password"))
	if err := app.Save(account); err != nil {
		t.Fatalf("failed to rotate the password: %v", err)
	}

	// Without the invalidation hook the cache would still answer with the old
	// password for the rest of its lifetime.
	if _, err := handler.authenticate(testutil.KoUsernameA, testutil.Md5Hex(testutil.KoPasswordA)); err == nil {
		t.Errorf("expected the old password to stop working immediately")
	}
	if _, err := handler.authenticate(testutil.KoUsernameA, testutil.Md5Hex("a-brand-new-password")); err != nil {
		t.Errorf("expected the new password to work: %v", err)
	}
}

func TestDisablingACredentialLocksItOutImmediately(t *testing.T) {
	app, handler := registeredApp(t)

	if _, err := handler.authenticate(testutil.KoUsernameA, testutil.Md5Hex(testutil.KoPasswordA)); err != nil {
		t.Fatalf("expected the seeded credential to authenticate: %v", err)
	}

	account, err := app.FindRecordById(schema.CollectionKoreaderAccounts, testutil.IdAccountA)
	if err != nil {
		t.Fatalf("failed to load the credential: %v", err)
	}
	account.Set(schema.FieldDisabled, true)
	if err := app.Save(account); err != nil {
		t.Fatalf("failed to disable the credential: %v", err)
	}

	if _, err := handler.authenticate(testutil.KoUsernameA, testutil.Md5Hex(testutil.KoPasswordA)); err == nil {
		t.Errorf("expected a disabled credential to be rejected even when it was cached")
	}
}

func TestDeletingACredentialLocksItOutImmediately(t *testing.T) {
	app, handler := registeredApp(t)

	if _, err := handler.authenticate(testutil.KoUsernameA, testutil.Md5Hex(testutil.KoPasswordA)); err != nil {
		t.Fatalf("expected the seeded credential to authenticate: %v", err)
	}

	account, err := app.FindRecordById(schema.CollectionKoreaderAccounts, testutil.IdAccountA)
	if err != nil {
		t.Fatalf("failed to load the credential: %v", err)
	}
	if err := app.Delete(account); err != nil {
		t.Fatalf("failed to delete the credential: %v", err)
	}

	if _, err := handler.authenticate(testutil.KoUsernameA, testutil.Md5Hex(testutil.KoPasswordA)); err == nil {
		t.Errorf("expected a deleted credential to be rejected even when it was cached")
	}
}

func TestAuthenticateRejectsACredentialWithoutAnOwner(t *testing.T) {
	app, handler := registeredApp(t)

	// A credential can lose its owner only through a bug or a manual edit, but
	// it must never authenticate as "nobody".
	account, err := app.FindRecordById(schema.CollectionKoreaderAccounts, testutil.IdAccountA)
	if err != nil {
		t.Fatalf("failed to load the credential: %v", err)
	}
	if _, err := app.DB().
		NewQuery("UPDATE {{" + schema.CollectionKoreaderAccounts + "}} SET [[owner]] = '' WHERE [[id]] = '" + account.Id + "'").
		Execute(); err != nil {
		t.Fatalf("failed to clear the owner: %v", err)
	}

	if _, err := handler.authenticate(testutil.KoUsernameA, testutil.Md5Hex(testutil.KoPasswordA)); err == nil {
		t.Errorf("expected a credential without an owner to be rejected")
	}
}
