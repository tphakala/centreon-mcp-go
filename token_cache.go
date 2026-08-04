package main

import (
	"container/list"
	"crypto/sha256"
	"fmt"
	"sync"
	"time"
)

// tokenCacheMaxEntries bounds how many auth tokens the gateway caches at once.
// In gateway mode the cache key is derived from request headers (host, username,
// password), so an unbounded cache lets a caller with distinct credentials grow
// it without limit. The cap turns that into a fixed memory ceiling; legitimate
// deployments have far fewer distinct credential sets than this.
const tokenCacheMaxEntries = 4096

type tokenEntry struct {
	host    string // Centreon host this token authenticates against; needed to log the session out (the cache key is a hash, so host is not recoverable from it)
	token   string
	expires time.Time
	el      *list.Element // position in ll; el.Value holds the map key (string)
}

// TokenCache is a bounded, TTL'd LRU of auth tokens keyed by
// host+username+password. Binding the key to the password ensures a request
// presenting a different (e.g. wrong) password misses the cache and is forced
// through a real login, rather than reusing a token minted for the correct
// password.
//
// The cache is bounded to a fixed number of entries and evicts the
// least-recently-used entry on overflow, so distinct (attacker-influenced)
// gateway keys cannot exhaust memory. Expired entries are dropped lazily on
// access; there is no whole-map sweep on the write path.
type TokenCache struct {
	mu         sync.Mutex
	ttl        time.Duration
	maxEntries int
	ll         *list.List             // front = most recently used; Value is the key
	entries    map[string]*tokenEntry // key -> entry
}

// NewTokenCache creates a token cache with the given TTL and the default cap.
func NewTokenCache(ttl time.Duration) *TokenCache {
	return newTokenCache(ttl, tokenCacheMaxEntries)
}

// newTokenCache creates a token cache with an explicit entry cap. Split out from
// NewTokenCache so eviction can be exercised with a small cap in tests.
func newTokenCache(ttl time.Duration, maxEntries int) *TokenCache {
	if maxEntries < 1 {
		maxEntries = 1
	}
	return &TokenCache{
		ttl:        ttl,
		maxEntries: maxEntries,
		ll:         list.New(),
		entries:    make(map[string]*tokenEntry),
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
// part of the key, so a different password will not match a cached entry. A hit
// marks the entry most-recently-used; an expired entry is dropped.
func (c *TokenCache) Get(host, username, password string) (string, bool) {
	// Hash before taking the lock: cacheKey is pure, and the password comes from
	// an attacker-influenced request header, so hashing it must not hold the
	// global mutex. Under the lock only the O(1) map/list work runs.
	key := cacheKey(host, username, password)

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[key]
	if !ok {
		return "", false
	}
	if time.Now().After(entry.expires) {
		c.remove(key, entry)
		return "", false
	}
	c.ll.MoveToFront(entry.el)
	return entry.token, true
}

// Set stores a token, refreshing an existing key in place. When the cache is at
// capacity and the key is new, the least-recently-used entry is evicted first,
// so the cache stays within its fixed bound.
func (c *TokenCache) Set(host, username, password, token string) {
	// Hash before taking the lock (see Get): the password is attacker-influenced
	// and cacheKey is pure, so the hash must not run under the global mutex.
	key := cacheKey(host, username, password)

	c.mu.Lock()
	defer c.mu.Unlock()

	expires := time.Now().Add(c.ttl)

	if entry, ok := c.entries[key]; ok {
		entry.token = token
		entry.expires = expires
		c.ll.MoveToFront(entry.el)
		return
	}

	if c.ll.Len() >= c.maxEntries {
		c.removeOldest()
	}
	el := c.ll.PushFront(key)
	c.entries[key] = &tokenEntry{host: host, token: token, expires: expires, el: el}
}

// remove drops a known key/entry pair from both the list and the index.
// The caller holds mu.
func (c *TokenCache) remove(key string, entry *tokenEntry) {
	c.ll.Remove(entry.el)
	delete(c.entries, key)
}

// removeOldest evicts the least-recently-used entry (the back of the list).
// The caller holds mu.
func (c *TokenCache) removeOldest() {
	back := c.ll.Back()
	if back == nil {
		return
	}
	key, _ := back.Value.(string)
	c.ll.Remove(back)
	delete(c.entries, key)
}

// cachedToken is a (host, token) pair returned by Drain. The caller uses it to
// log the token's Centreon session out; the host is carried explicitly because
// the cache key is a one-way hash and cannot yield it back.
type cachedToken struct {
	host  string
	token string
}

// Drain removes every entry and returns their (host, token) pairs, then leaves
// the cache empty. Entries whose TTL has already passed are included on purpose:
// their Centreon session can outlive the cache TTL, so a graceful shutdown must
// still attempt to log them out. Drain performs no I/O; the caller does the
// logouts outside the lock.
func (c *TokenCache) Drain() []cachedToken {
	c.mu.Lock()
	defer c.mu.Unlock()

	drained := make([]cachedToken, 0, len(c.entries))
	for _, entry := range c.entries {
		drained = append(drained, cachedToken{host: entry.host, token: entry.token})
	}
	c.ll.Init()
	c.entries = make(map[string]*tokenEntry)
	return drained
}
