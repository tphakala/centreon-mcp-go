package main

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestTokenCache_SetAndGet(t *testing.T) {
	tc := NewTokenCache(50 * time.Minute)
	tc.Set("host1", "user1", "pass1", "token-abc")

	tok, ok := tc.Get("host1", "user1", "pass1")
	if !ok {
		t.Fatal("expected token to be found")
	}
	if tok != "token-abc" {
		t.Errorf("expected token-abc, got %q", tok)
	}
}

func TestTokenCache_Expired(t *testing.T) {
	tc := NewTokenCache(1 * time.Millisecond)
	tc.Set("host1", "user1", "pass1", "token-abc")

	time.Sleep(5 * time.Millisecond)

	_, ok := tc.Get("host1", "user1", "pass1")
	if ok {
		t.Error("expected token to be expired")
	}
	// The lazy expiry drop must clear the entry from BOTH the map and the LRU
	// list; if it only deleted the map key the list would leak a dead element.
	if n := len(tc.entries); n != 0 {
		t.Errorf("expired Get should drop the map entry: %d remain", n)
	}
	if n := tc.ll.Len(); n != 0 {
		t.Errorf("expired Get should drop the LRU element: %d remain", n)
	}
}

func TestTokenCache_DifferentKeys(t *testing.T) {
	tc := NewTokenCache(50 * time.Minute)
	tc.Set("host1", "user1", "pass1", "token-1")
	tc.Set("host2", "user2", "pass2", "token-2")

	tok1, ok1 := tc.Get("host1", "user1", "pass1")
	tok2, ok2 := tc.Get("host2", "user2", "pass2")
	_, ok3 := tc.Get("host1", "user2", "pass1")

	if !ok1 || tok1 != "token-1" {
		t.Errorf("expected token-1, got %q", tok1)
	}
	if !ok2 || tok2 != "token-2" {
		t.Errorf("expected token-2, got %q", tok2)
	}
	if ok3 {
		t.Error("expected no token for host1+user2")
	}
}

// TestTokenCache_WrongPasswordMisses pins the #26 fix: a cached token minted
// for one password must not be served to a request presenting a different
// password. A different password produces a different key, so it misses the
// cache and is forced through a real login.
func TestTokenCache_WrongPasswordMisses(t *testing.T) {
	tc := NewTokenCache(50 * time.Minute)
	tc.Set("host1", "user1", "correct-pass", "token-abc")

	if _, ok := tc.Get("host1", "user1", "wrong-pass"); ok {
		t.Error("expected cache miss for a different password, got a hit")
	}

	tok, ok := tc.Get("host1", "user1", "correct-pass")
	if !ok || tok != "token-abc" {
		t.Errorf("expected the correct password to hit and return token-abc, got %q ok=%v", tok, ok)
	}
}

// TestTokenCache_BoundedToMaxEntries pins the #29 DoS fix: distinct keys, which
// in gateway mode come from attacker-influenced request headers, must not grow
// the cache without bound. Flooding it with far more keys than the cap leaves
// the cache at the cap, not at the flood size.
func TestTokenCache_BoundedToMaxEntries(t *testing.T) {
	tc := newTokenCache(50*time.Minute, 3)
	for i := range 100 {
		tc.Set("host", "user", fmt.Sprintf("pass-%d", i), "tok")
	}
	// The flood deterministically fills the cap, so the cache must sit at
	// exactly 3: fewer would mean an over-eviction bug, more an unbounded one.
	if got := len(tc.entries); got != 3 {
		t.Fatalf("cache should be filled to exactly the cap: got %d entries, want 3", got)
	}
	if ll, m := tc.ll.Len(), len(tc.entries); ll != m {
		t.Fatalf("list/map desync: list=%d map=%d", ll, m)
	}
}

// TestTokenCache_EvictsLeastRecentlyUsed pins that eviction is LRU: a Get keeps
// an entry alive, and the untouched one is the one dropped when the cap is hit.
func TestTokenCache_EvictsLeastRecentlyUsed(t *testing.T) {
	tc := newTokenCache(50*time.Minute, 2)
	tc.Set("h", "u", "A", "tok-a")
	tc.Set("h", "u", "B", "tok-b")

	// Touch A so B becomes the least-recently-used entry.
	if _, ok := tc.Get("h", "u", "A"); !ok {
		t.Fatal("A should still be present before eviction")
	}

	// Inserting C is over cap, so the LRU entry (B) is evicted.
	tc.Set("h", "u", "C", "tok-c")

	if _, ok := tc.Get("h", "u", "B"); ok {
		t.Error("B should have been evicted as least-recently-used")
	}
	if tok, ok := tc.Get("h", "u", "A"); !ok || tok != "tok-a" {
		t.Errorf("A should survive eviction, got %q ok=%v", tok, ok)
	}
	if tok, ok := tc.Get("h", "u", "C"); !ok || tok != "tok-c" {
		t.Errorf("C should be present, got %q ok=%v", tok, ok)
	}
}

// TestTokenCache_SetExistingKeyRefreshes pins that re-Setting the same key
// updates the token in place instead of adding a second entry.
func TestTokenCache_SetExistingKeyRefreshes(t *testing.T) {
	tc := newTokenCache(50*time.Minute, 5)
	tc.Set("h", "u", "p", "tok-1")
	tc.Set("h", "u", "p", "tok-2")

	if got := len(tc.entries); got != 1 {
		t.Fatalf("re-Set of the same key should not grow the cache index: got %d entries", got)
	}
	if got := tc.ll.Len(); got != 1 {
		t.Fatalf("re-Set of the same key should not grow the LRU list: got %d elements", got)
	}
	if tok, ok := tc.Get("h", "u", "p"); !ok || tok != "tok-2" {
		t.Errorf("expected the refreshed token tok-2, got %q ok=%v", tok, ok)
	}
}

// TestTokenCache_SetPromotesExistingKey pins that re-Setting an existing key
// also marks it most-recently-used, so a subsequently-inserted key evicts the
// other (now least-recently-used) entry rather than the just-refreshed one.
func TestTokenCache_SetPromotesExistingKey(t *testing.T) {
	tc := newTokenCache(50*time.Minute, 2)
	tc.Set("h", "u", "A", "tok-a")
	tc.Set("h", "u", "B", "tok-b")

	// Refresh A: this must promote A to MRU, leaving B least-recently-used.
	tc.Set("h", "u", "A", "tok-a2")

	// Inserting C is over cap, so the LRU entry (B) is evicted, not A.
	tc.Set("h", "u", "C", "tok-c")

	if _, ok := tc.Get("h", "u", "B"); ok {
		t.Error("B should have been evicted after A was refreshed to MRU")
	}
	if tok, ok := tc.Get("h", "u", "A"); !ok || tok != "tok-a2" {
		t.Errorf("A should survive with its refreshed token, got %q ok=%v", tok, ok)
	}
}

// TestTokenCache_DrainReturnsAllAndEmpties pins the #5 shutdown-drain support:
// Drain returns every cached (host, token) pair so each Centreon session can be
// logged out, and leaves the cache empty (both the index and the LRU list).
func TestTokenCache_DrainReturnsAllAndEmpties(t *testing.T) {
	tc := newTokenCache(50*time.Minute, 10)
	tc.Set("https://h1", "u", "p1", "tok-1")
	tc.Set("https://h2", "u", "p2", "tok-2")
	tc.Set("https://h3", "u", "p3", "tok-3")

	drained := tc.Drain()
	if len(drained) != 3 {
		t.Fatalf("Drain should return every cached entry: got %d, want 3", len(drained))
	}

	got := make(map[string]string, len(drained))
	for _, ct := range drained {
		got[ct.Host] = ct.Token
	}
	want := map[string]string{"https://h1": "tok-1", "https://h2": "tok-2", "https://h3": "tok-3"}
	for h, tok := range want {
		if got[h] != tok {
			t.Errorf("drained token for %s = %q, want %q", h, got[h], tok)
		}
	}

	if n := len(tc.entries); n != 0 {
		t.Errorf("Drain should empty the entry index: %d remain", n)
	}
	if n := tc.ll.Len(); n != 0 {
		t.Errorf("Drain should empty the LRU list: %d remain", n)
	}
	if _, ok := tc.Get("https://h1", "u", "p1"); ok {
		t.Error("Get should miss after Drain")
	}
}

// TestTokenCache_DrainIncludesExpiredEntries pins that Drain returns entries
// whose cache TTL has already passed but that have not yet been lazily dropped.
// The Centreon server session can outlive the cache TTL (activity resets its
// idle timer), so a graceful shutdown must still attempt to log these out.
func TestTokenCache_DrainIncludesExpiredEntries(t *testing.T) {
	// A negative TTL makes the entry already expired the instant it is stored.
	tc := newTokenCache(-time.Minute, 10)
	tc.Set("https://h1", "u", "p1", "tok-expired")

	drained := tc.Drain()
	if len(drained) != 1 {
		t.Fatalf("Drain must include TTL-expired entries: got %d, want 1", len(drained))
	}
	if drained[0].Host != "https://h1" || drained[0].Token != "tok-expired" {
		t.Errorf("unexpected drained entry: %+v", drained[0])
	}
}

// TestTokenCache_SkipsOverlongHost pins the defensive size bound: an
// unreasonably large (attacker-influenced) host is not retained, so the bounded
// cache cannot be inflated in per-entry size, only in count.
func TestTokenCache_SkipsOverlongHost(t *testing.T) {
	tc := newTokenCache(50*time.Minute, 10)

	longHost := "https://" + strings.Repeat("a", maxCachedHostLen)
	tc.Set(longHost, "u", "p", "tok")

	if _, ok := tc.Get(longHost, "u", "p"); ok {
		t.Error("an overlong host must not be cached")
	}
	if n := len(tc.entries); n != 0 {
		t.Errorf("overlong host should leave the cache empty, got %d entries", n)
	}

	// A normal host is unaffected.
	tc.Set("https://ok.example.com", "u", "p", "tok")
	if _, ok := tc.Get("https://ok.example.com", "u", "p"); !ok {
		t.Error("a normal host should still be cached")
	}
}

// TestTokenCache_DrainEmptyReturnsNothing pins that draining an empty cache is a
// safe no-op that returns no work.
func TestTokenCache_DrainEmptyReturnsNothing(t *testing.T) {
	tc := newTokenCache(50*time.Minute, 10)
	if drained := tc.Drain(); len(drained) != 0 {
		t.Fatalf("Drain on an empty cache should return nothing, got %d", len(drained))
	}
}

// TestTokenCache_ConcurrentAccessStaysBounded runs Set/Get from many goroutines
// (with -race) to confirm the cache stays within its cap and free of data races.
func TestTokenCache_ConcurrentAccessStaysBounded(t *testing.T) {
	const goroutines, perGoroutine, capacity = 8, 200, 8
	tc := newTokenCache(50*time.Minute, capacity)

	var wg sync.WaitGroup
	for g := range goroutines {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := range perGoroutine {
				p := fmt.Sprintf("p-%d-%d", g, i)
				tc.Set("h", "u", p, "tok")
				tc.Get("h", "u", p)
			}
		}(g)
	}
	wg.Wait()

	if got := len(tc.entries); got > capacity {
		t.Fatalf("cache exceeded cap under concurrency: got %d, want <= %d", got, capacity)
	}
}
