//
// File:        internal/koreader/cache_test.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package koreader

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

// clock is a controllable time source for the cache tests.
type clock struct {
	now time.Time
}

func (c *clock) Now() time.Time {
	return c.now
}

func newTestCache(ttl time.Duration, maxSize int) (*credentialCache, *clock) {
	c := &clock{now: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)}

	cache := newCredentialCache(ttl, maxSize)
	cache.now = c.Now

	return cache, c
}

func TestCacheReturnsAStoredCredential(t *testing.T) {
	cache, _ := newTestCache(5*time.Minute, 16)

	cache.put("alice", "digest", "account-id", "owner-id")

	accountId, ownerId, found := cache.get("alice", "digest")
	if !found {
		t.Fatalf("expected the credential to be cached")
	}
	if accountId != "account-id" || ownerId != "owner-id" {
		t.Errorf("expected account-id/owner-id, got %s/%s", accountId, ownerId)
	}
}

func TestCacheMissesOnADifferentDigest(t *testing.T) {
	cache, _ := newTestCache(5*time.Minute, 16)

	cache.put("alice", "digest", "account-id", "owner-id")

	if _, _, found := cache.get("alice", "other-digest"); found {
		t.Errorf("expected a different password digest to miss the cache")
	}
	if _, _, found := cache.get("bob", "digest"); found {
		t.Errorf("expected a different username to miss the cache")
	}
}

func TestCacheExpiresEntries(t *testing.T) {
	cache, clk := newTestCache(5*time.Minute, 16)

	cache.put("alice", "digest", "account-id", "owner-id")

	clk.now = clk.now.Add(5 * time.Minute)
	if _, _, found := cache.get("alice", "digest"); found {
		t.Errorf("expected the entry to be expired exactly at its lifetime")
	}
	if cache.len() != 0 {
		t.Errorf("expected the expired entry to be dropped on read, %d left", cache.len())
	}
}

func TestCacheInvalidatesAnAccount(t *testing.T) {
	cache, _ := newTestCache(5*time.Minute, 16)

	// The same credential can be cached under several digests, for example
	// right after a password rotation.
	cache.put("alice", "old-digest", "account-id", "owner-id")
	cache.put("alice", "new-digest", "account-id", "owner-id")
	cache.put("bob", "digest", "other-account", "other-owner")

	cache.invalidateAccount("account-id")

	if _, _, found := cache.get("alice", "old-digest"); found {
		t.Errorf("expected the invalidated credential to be gone")
	}
	if _, _, found := cache.get("alice", "new-digest"); found {
		t.Errorf("expected every entry of the invalidated account to be gone")
	}
	if _, _, found := cache.get("bob", "digest"); !found {
		t.Errorf("expected other accounts to survive the invalidation")
	}
}

func TestCacheStaysWithinItsSizeLimit(t *testing.T) {
	cache, _ := newTestCache(5*time.Minute, 3)

	for i := range 10 {
		cache.put("user"+strconv.Itoa(i), "digest", "account"+strconv.Itoa(i), "owner")
	}

	if cache.len() > 3 {
		t.Errorf("expected at most 3 cached entries, got %d", cache.len())
	}

	// The most recently stored credential has to be the one that survived.
	if _, _, found := cache.get("user9", "digest"); !found {
		t.Errorf("expected the newest entry to be kept")
	}
}

func TestCacheEvictsExpiredEntriesFirst(t *testing.T) {
	cache, clk := newTestCache(5*time.Minute, 2)

	cache.put("stale", "digest", "account-stale", "owner")
	clk.now = clk.now.Add(6 * time.Minute) // "stale" is now expired
	cache.put("fresh", "digest", "account-fresh", "owner")
	cache.put("newest", "digest", "account-newest", "owner")

	if _, _, found := cache.get("fresh", "digest"); !found {
		t.Errorf("expected the live entry to be kept over the expired one")
	}
	if _, _, found := cache.get("newest", "digest"); !found {
		t.Errorf("expected the newest entry to be stored")
	}
}

func TestDisabledCacheStoresNothing(t *testing.T) {
	cache, _ := newTestCache(0, 16)

	cache.put("alice", "digest", "account-id", "owner-id")

	if cache.enabled() {
		t.Errorf("expected a zero lifetime to disable the cache")
	}
	if _, _, found := cache.get("alice", "digest"); found {
		t.Errorf("expected a disabled cache to never report a hit")
	}
	if cache.len() != 0 {
		t.Errorf("expected a disabled cache to stay empty, got %d entries", cache.len())
	}
}

func TestCacheKeyDoesNotContainTheDigest(t *testing.T) {
	key := cacheKey("alice", "5f4dcc3b5aa765d61d8327deb882cf99")

	if key == "" {
		t.Fatalf("expected a cache key")
	}
	if strings.Contains(key, "5f4dcc3b5aa765d61d8327deb882cf99") || strings.Contains(key, "alice") {
		t.Errorf("the cache key must not embed the credentials it stands for")
	}
}
