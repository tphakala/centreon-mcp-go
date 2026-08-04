package main

import (
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
