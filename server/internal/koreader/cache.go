//
// File:        internal/koreader/cache.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package koreader

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

// credentialCache remembers successfully verified KOReader credentials.
//
// Why this exists: the stored credential is bcrypt hashed, and KOReader is
// commonly configured to push progress every two pages. Without a cache every
// push would pay a full bcrypt verification, which is tens of milliseconds of
// CPU each and quickly saturates a small server.
//
// Only successful verifications are cached, so a brute force attempt still pays
// the full bcrypt cost on every single guess.
type credentialCache struct {
	mu      sync.Mutex
	ttl     time.Duration
	maxSize int
	entries map[string]cacheEntry
	// byAccount maps an account id to its cache keys so that a password change
	// or a deletion can drop the matching entries immediately.
	byAccount map[string]map[string]struct{}

	// now is swappable for the tests.
	now func() time.Time
}

type cacheEntry struct {
	accountId string
	ownerId   string
	expiresAt time.Time
}

func newCredentialCache(ttl time.Duration, maxSize int) *credentialCache {
	if maxSize < 1 {
		maxSize = 1
	}

	return &credentialCache{
		ttl:       ttl,
		maxSize:   maxSize,
		entries:   make(map[string]cacheEntry),
		byAccount: make(map[string]map[string]struct{}),
		now:       time.Now,
	}
}

// enabled reports whether the cache stores anything at all.
func (c *credentialCache) enabled() bool {
	return c.ttl > 0
}

// cacheKey derives an opaque key from the credentials.
//
// The digest is hashed again so that the cache does not hold the value that a
// KOReader device sends over the wire.
func cacheKey(username, md5hex string) string {
	sum := sha256.Sum256([]byte(username + "\x00" + md5hex))
	return hex.EncodeToString(sum[:])
}

// get returns the cached account and owner ids for the given credentials.
func (c *credentialCache) get(username, md5hex string) (accountId, ownerId string, found bool) {
	if !c.enabled() {
		return "", "", false
	}

	key := cacheKey(username, md5hex)

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[key]
	if !ok {
		return "", "", false
	}
	if !entry.expiresAt.After(c.now()) {
		c.removeLocked(key)
		return "", "", false
	}

	return entry.accountId, entry.ownerId, true
}

// put stores a verified credential.
func (c *credentialCache) put(username, md5hex, accountId, ownerId string) {
	if !c.enabled() {
		return
	}

	key := cacheKey(username, md5hex)

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.entries[key]; !exists && len(c.entries) >= c.maxSize {
		c.evictLocked()
	}

	c.entries[key] = cacheEntry{
		accountId: accountId,
		ownerId:   ownerId,
		expiresAt: c.now().Add(c.ttl),
	}

	keys, ok := c.byAccount[accountId]
	if !ok {
		keys = make(map[string]struct{})
		c.byAccount[accountId] = keys
	}
	keys[key] = struct{}{}
}

// invalidateAccount drops every cached credential of the given account. It runs
// whenever a credential is updated or deleted, so a rotated password stops
// working immediately instead of after the cache lifetime.
func (c *credentialCache) invalidateAccount(accountId string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key := range c.byAccount[accountId] {
		delete(c.entries, key)
	}
	delete(c.byAccount, accountId)
}

// len reports the number of cached entries, including expired ones that have
// not been evicted yet.
func (c *credentialCache) len() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return len(c.entries)
}

// evictLocked makes room for a new entry. Expired entries go first; if there
// are none, the entry closest to expiring is dropped. Because every entry gets
// the same lifetime, that is also the one that was added first.
func (c *credentialCache) evictLocked() {
	now := c.now()

	evicted := false
	for key, entry := range c.entries {
		if !entry.expiresAt.After(now) {
			c.removeLocked(key)
			evicted = true
		}
	}
	if evicted {
		return
	}

	var oldestKey string
	var oldestAt time.Time
	for key, entry := range c.entries {
		if oldestKey == "" || entry.expiresAt.Before(oldestAt) {
			oldestKey = key
			oldestAt = entry.expiresAt
		}
	}
	if oldestKey != "" {
		c.removeLocked(oldestKey)
	}
}

// removeLocked deletes a single entry and its account index. The caller holds the lock.
func (c *credentialCache) removeLocked(key string) {
	entry, ok := c.entries[key]
	if !ok {
		return
	}

	delete(c.entries, key)

	if keys, ok := c.byAccount[entry.accountId]; ok {
		delete(keys, key)
		if len(keys) == 0 {
			delete(c.byAccount, entry.accountId)
		}
	}
}
