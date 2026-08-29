package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	sessionCookie = "sofar"
	sessionLife   = 365 * 24 * time.Hour
	maxAttempts   = 8
	lockout       = 15 * time.Minute
)

// The signing key is derived from the password, so changing the password
// invalidates every cookie ever issued. That is the only revocation a
// single-password service can offer, and it should be one line to use.
func signingKey(password string) []byte {
	sum := sha256.Sum256([]byte("sofar-session\x00" + password))
	return sum[:]
}

func signSession(key []byte, expires int64) string {
	exp := strconv.FormatInt(expires, 10)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(exp))
	return exp + "." + hex.EncodeToString(mac.Sum(nil))
}

func validSession(key []byte, value string, now time.Time) bool {
	exp, _, found := strings.Cut(value, ".")
	if !found {
		return false
	}
	at, err := strconv.ParseInt(exp, 10, 64)
	if err != nil || now.Unix() >= at {
		return false
	}
	want := signSession(key, at)
	return subtle.ConstantTimeCompare([]byte(value), []byte(want)) == 1
}

// attempts throttles guessing. One password and no accounts means there is
// nothing else standing between a bot and the whole database.
type attempts struct {
	mu sync.Mutex
	at map[string]*attempt
}

type attempt struct {
	n     int
	until time.Time
}

func newAttempts() *attempts { return &attempts{at: map[string]*attempt{}} }

func (a *attempts) blocked(ip string, now time.Time) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	e := a.at[ip]
	return e != nil && e.n >= maxAttempts && now.Before(e.until)
}

func (a *attempts) failed(ip string, now time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	e := a.at[ip]
	if e == nil || now.After(e.until) {
		e = &attempt{}
		a.at[ip] = e
	}
	e.n++
	e.until = now.Add(lockout)
}

func (a *attempts) reset(ip string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.at, ip)
}

// Behind Cloudflare and Dokploy every request carries the proxy's address, so
// throttling on RemoteAddr would lock out the whole world on one bad guess.
// These headers are trusted for counting only: a forged one buys an attacker a
// fresh counter, which rotating IPs would buy anyway.
func clientIP(r *http.Request) string {
	if ip := r.Header.Get("CF-Connecting-IP"); ip != "" {
		return ip
	}
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		first, _, _ := strings.Cut(fwd, ",")
		if first = strings.TrimSpace(first); first != "" {
			return first
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// open paths answer before the password: the login form itself, the assets it
// needs to render, and the health check the platform polls.
func openPath(p string) bool {
	return p == "/login" || p == "/healthz" || strings.HasPrefix(p, "/static/")
}

func (s *Server) guard(next http.Handler) http.Handler {
	if s.cfg.Password == "" {
		return next
	}
	key := signingKey(s.cfg.Password)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if openPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		if c, err := r.Cookie(sessionCookie); err == nil &&
			validSession(key, c.Value, time.Now()) {
			next.ServeHTTP(w, r)
			return
		}
		// htmx swaps whatever comes back, so a redirect would land the login
		// page inside a table row. This header makes the browser navigate.
		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("HX-Redirect", "/login")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		http.Redirect(w, r, "/login", http.StatusFound)
	})
}

type loginPage struct {
	Title  string
	Failed bool
	Locked bool
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Password == "" {
		http.Redirect(w, r, "/active", http.StatusFound)
		return
	}
	s.renderLogin(w, r, loginPage{})
}

func (s *Server) handleLoginPost(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Password == "" {
		http.Redirect(w, r, "/active", http.StatusFound)
		return
	}
	now := time.Now()
	ip := clientIP(r)
	if s.attempts.blocked(ip, now) {
		s.renderLogin(w, r, loginPage{Locked: true})
		return
	}

	given := r.FormValue("password")
	if subtle.ConstantTimeCompare([]byte(given), []byte(s.cfg.Password)) != 1 {
		s.attempts.failed(ip, now)
		s.log.Warn("failed login", "ip", ip)
		s.renderLogin(w, r, loginPage{Failed: true})
		return
	}
	s.attempts.reset(ip)

	expires := now.Add(sessionLife)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    signSession(signingKey(s.cfg.Password), expires.Unix()),
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		// Behind Caddy the browser only ever sees https, and a dev instance
		// on http would silently drop a Secure cookie.
		Secure: !s.cfg.Dev,
	})
	http.Redirect(w, r, "/active", http.StatusFound)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   !s.cfg.Dev,
	})
	http.Redirect(w, r, "/login", http.StatusFound)
}

func (s *Server) renderLogin(w http.ResponseWriter, r *http.Request, page loginPage) {
	page.Title = "Вхід"
	if page.Failed || page.Locked {
		w.Header().Set("Cache-Control", "no-store")
	}
	s.render(w, r, "login.html", page)
}
