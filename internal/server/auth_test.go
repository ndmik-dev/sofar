package server

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestSessionSurvivesOnlyItsOwnKey(t *testing.T) {
	now := time.Now()
	exp := now.Add(time.Hour).Unix()
	token := signSession(signingKey("hunter2"), exp)

	if !validSession(signingKey("hunter2"), token, now) {
		t.Fatal("a freshly signed session was rejected")
	}
	// Changing the password is the only revocation this design has; if an old
	// cookie survived it, there would be none at all.
	if validSession(signingKey("something else"), token, now) {
		t.Error("session survived a password change")
	}
}

func TestSessionExpires(t *testing.T) {
	now := time.Now()
	token := signSession(signingKey("pw"), now.Add(-time.Second).Unix())
	if validSession(signingKey("pw"), token, now) {
		t.Error("expired session accepted")
	}
}

func TestSessionRejectsTampering(t *testing.T) {
	now := time.Now()
	key := signingKey("pw")
	far := now.Add(10 * time.Hour).Unix()
	token := signSession(key, now.Add(time.Hour).Unix())

	_, sig, _ := strings.Cut(token, ".")
	forged := strconv.FormatInt(far, 10) + "." + sig
	if validSession(key, forged, now) {
		t.Error("a session with a stretched expiry was accepted")
	}
	if validSession(key, token+"x", now) {
		t.Error("a session with a mangled signature was accepted")
	}
}

func TestLockoutIsPerAddress(t *testing.T) {
	now := time.Now()
	a := newAttempts()
	for i := 0; i < maxAttempts; i++ {
		a.failed("10.0.0.1", now)
	}
	if !a.blocked("10.0.0.1", now) {
		t.Error("guessing address was not locked out")
	}
	if a.blocked("10.0.0.2", now) {
		t.Error("an unrelated address was locked out too")
	}
	if a.blocked("10.0.0.1", now.Add(lockout+time.Minute)) {
		t.Error("lockout never lifted")
	}
	a.reset("10.0.0.1")
	if a.blocked("10.0.0.1", now) {
		t.Error("a correct password did not clear the counter")
	}
}
