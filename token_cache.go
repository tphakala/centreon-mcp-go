package main

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"
)

type tokenEntry struct {
	token   string
	expires time.Time
}

// TokenCache stores auth tokens keyed by host+username with TTL.
type TokenCache struct {
	mu      sync.Mutex
	ttl     time.Duration
	entries map[string]tokenEntry
}

// NewTokenCache creates a new token cache with the given TTL.
func NewTokenCache(ttl time.Duration) *TokenCache {
	return &TokenCache{
		ttl:     ttl,
		entries: make(map[string]tokenEntry),
	}
}

func cacheKey(host, username string) string {
	h := sha256.Sum256([]byte(host + "\x00" + username))
	return fmt.Sprintf("%x", h)
}

// Get returns a cached token if it exists and hasn't expired.
func (c *TokenCache) Get(host, username string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := cacheKey(host, username)
	entry, ok := c.entries[key]
	if !ok || time.Now().After(entry.expires) {
		delete(c.entries, key)
		return "", false
	}
	return entry.token, true
}

// Set stores a token in the cache.
func (c *TokenCache) Set(host, username, token string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := cacheKey(host, username)
	c.entries[key] = tokenEntry{
		token:   token,
		expires: time.Now().Add(c.ttl),
	}
}
