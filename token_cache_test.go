package main

import (
	"testing"
	"time"
)

func TestTokenCache_SetAndGet(t *testing.T) {
	tc := NewTokenCache(50 * time.Minute)
	tc.Set("host1", "user1", "token-abc")

	tok, ok := tc.Get("host1", "user1")
	if !ok {
		t.Fatal("expected token to be found")
	}
	if tok != "token-abc" {
		t.Errorf("expected token-abc, got %q", tok)
	}
}

func TestTokenCache_Expired(t *testing.T) {
	tc := NewTokenCache(1 * time.Millisecond)
	tc.Set("host1", "user1", "token-abc")

	time.Sleep(5 * time.Millisecond)

	_, ok := tc.Get("host1", "user1")
	if ok {
		t.Error("expected token to be expired")
	}
}

func TestTokenCache_DifferentKeys(t *testing.T) {
	tc := NewTokenCache(50 * time.Minute)
	tc.Set("host1", "user1", "token-1")
	tc.Set("host2", "user2", "token-2")

	tok1, ok1 := tc.Get("host1", "user1")
	tok2, ok2 := tc.Get("host2", "user2")
	_, ok3 := tc.Get("host1", "user2")

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
