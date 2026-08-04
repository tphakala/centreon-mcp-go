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

// TokenCache stores auth tokens keyed by host+username+password with TTL.
// Binding the key to the password ensures a request presenting a different
// (e.g. wrong) password misses the cache and is forced through a real login,
// rather than reusing a token minted for the correct password.
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

func cacheKey(host, username, password string) string {
	// Stream each field into the hash separately (NUL-separated) rather than
	// building one concatenated string, to avoid an extra in-memory copy of the
	// plaintext password. hash.Hash.Write never returns an error.
	h := sha256.New()
	_, _ = h.Write([]byte(host))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(username))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(password))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// Get returns a cached token if it exists and hasn't expired. The password is
// part of the key, so a different password will not match a cached entry.
func (c *TokenCache) Get(host, username, password string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := cacheKey(host, username, password)
	entry, ok := c.entries[key]
	if !ok || time.Now().After(entry.expires) {
		delete(c.entries, key)
		return "", false
	}
	return entry.token, true
}

// Set stores a token in the cache and sweeps expired entries.
func (c *TokenCache) Set(host, username, password, token string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for k, e := range c.entries {
		if now.After(e.expires) {
			delete(c.entries, k)
		}
	}

	key := cacheKey(host, username, password)
	c.entries[key] = tokenEntry{
		token:   token,
		expires: now.Add(c.ttl),
	}
}
