package server

import (
	"testing"
	"time"

	"github.com/heptau/pgarachne/internal/config"
)

func TestLoginLimiterAllow(t *testing.T) {
	l := newLoginLimiter(2, 50*time.Millisecond)

	if !l.Allow("key") {
		t.Fatal("first attempt should be allowed")
	}
	if !l.Allow("key") {
		t.Fatal("second attempt should be allowed")
	}
	if l.Allow("key") {
		t.Fatal("third attempt within the window should be denied")
	}
	// A different key has its own budget.
	if !l.Allow("other") {
		t.Fatal("attempt with a different key should be allowed")
	}

	time.Sleep(60 * time.Millisecond)
	if !l.Allow("key") {
		t.Fatal("attempt after the window expired should be allowed")
	}
}

func TestLoginLimiterFailsClosedWhenFull(t *testing.T) {
	l := newLoginLimiter(5, time.Minute)
	l.maxEntries = 1

	if !l.Allow("first") {
		t.Fatal("first key should be admitted")
	}
	if l.Allow("second") {
		t.Fatal("new key should be denied when the map is full (fail closed)")
	}
	if !l.Allow("first") {
		t.Fatal("existing key should still be allowed when the map is full")
	}
}

func TestNewIPLoginLimiterFromConfig(t *testing.T) {
	if l := newIPLoginLimiterFromConfig(&config.Config{LoginRateLimitPerIP: 0}); l != nil {
		t.Error("LoginRateLimitPerIP=0 should disable the per-IP limiter")
	}

	l := newIPLoginLimiterFromConfig(&config.Config{LoginRateLimitPerIP: 1, LoginRateWindow: time.Minute})
	if l == nil {
		t.Fatal("expected a limiter for LoginRateLimitPerIP=1")
	}
	if !l.Allow("10.0.0.1") {
		t.Error("first attempt should be allowed")
	}
	if l.Allow("10.0.0.1") {
		t.Error("second attempt should be denied at limit 1")
	}
}
