package proxy

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"html"
	"net/http"
	"strings"
	"sync"
	"time"

	"drip/internal/server/tunnel"
	"drip/internal/shared/protocol"
)

const authCookieName = "drip_auth"
const authSessionDuration = 24 * time.Hour

const (
	authRateLimitWindow           = 1 * time.Minute
	authRateLimitMax              = 10
	authRateLimitLockout          = 5 * time.Minute
	authRateLimitLockoutThreshold = 20
)

type authSession struct {
	subdomain string
	expiresAt time.Time
}

type authSessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*authSession
}

type authRateLimitEntry struct {
	failures    int
	windowStart time.Time
	lockedUntil time.Time
}

type authRateLimiter struct {
	mu      sync.RWMutex
	entries map[string]*authRateLimitEntry
}

var sessionStore = &authSessionStore{
	sessions: make(map[string]*authSession),
}

var authLimiter = &authRateLimiter{
	entries: make(map[string]*authRateLimitEntry),
}

func init() {
	go authLimiter.startCleanupLoop()
	go sessionStore.startCleanupLoop()
}

func (rl *authRateLimiter) startCleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.cleanup()
	}
}

func (s *authSessionStore) startCleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.cleanup()
	}
}

func (s *authSessionStore) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for token, session := range s.sessions {
		if now.After(session.expiresAt) {
			delete(s.sessions, token)
		}
	}
}

func (rl *authRateLimiter) isRateLimited(ip string) bool {
	if ip == "" {
		return false
	}

	rl.mu.RLock()
	entry, exists := rl.entries[ip]
	rl.mu.RUnlock()

	if !exists {
		return false
	}

	now := time.Now()

	if !entry.lockedUntil.IsZero() && now.Before(entry.lockedUntil) {
		return true
	}

	if now.Sub(entry.windowStart) < authRateLimitWindow && entry.failures >= authRateLimitMax {
		return true
	}

	return false
}

func (rl *authRateLimiter) recordFailure(ip string) {
	if ip == "" {
		return
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	entry, exists := rl.entries[ip]

	if !exists {
		rl.entries[ip] = &authRateLimitEntry{
			failures:    1,
			windowStart: now,
		}
		return
	}

	if now.Sub(entry.windowStart) >= authRateLimitWindow {
		entry.failures = 1
		entry.windowStart = now
		entry.lockedUntil = time.Time{}
		return
	}

	entry.failures++

	if entry.failures >= authRateLimitLockoutThreshold {
		entry.lockedUntil = now.Add(authRateLimitLockout)
	}
}

func (rl *authRateLimiter) resetFailures(ip string) {
	if ip == "" {
		return
	}

	rl.mu.Lock()
	delete(rl.entries, ip)
	rl.mu.Unlock()
}

func (rl *authRateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for ip, entry := range rl.entries {
		windowExpired := now.Sub(entry.windowStart) >= authRateLimitWindow
		lockoutExpired := entry.lockedUntil.IsZero() || now.After(entry.lockedUntil)
		if windowExpired && lockoutExpired {
			delete(rl.entries, ip)
		}
	}
}

func (s *authSessionStore) create(subdomain string) string {
	token := generateSessionToken()
	s.mu.Lock()
	s.sessions[token] = &authSession{
		subdomain: subdomain,
		expiresAt: time.Now().Add(authSessionDuration),
	}
	s.mu.Unlock()
	return token
}

func (s *authSessionStore) validate(token, subdomain string) bool {
	s.mu.RLock()
	session, ok := s.sessions[token]
	s.mu.RUnlock()

	if !ok {
		return false
	}
	if time.Now().After(session.expiresAt) {
		s.mu.Lock()
		delete(s.sessions, token)
		s.mu.Unlock()
		return false
	}
	return session.subdomain == subdomain
}

func generateSessionToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	hash := sha256.Sum256(b)
	return hex.EncodeToString(hash[:])
}

func isBearerProxyAuth(auth *protocol.ProxyAuth) bool {
	if auth == nil {
		return false
	}
	if auth.Type != "" {
		return strings.EqualFold(auth.Type, "bearer")
	}
	return auth.Token != ""
}

func extractBearerToken(header string) string {
	if header == "" {
		return ""
	}
	parts := strings.Fields(header)
	if len(parts) < 2 {
		return ""
	}
	if !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}

func (h *Handler) isProxyAuthenticated(r *http.Request, subdomain string) bool {
	cookie, err := r.Cookie(authCookieName + "_" + subdomain)
	if err != nil {
		return false
	}
	return sessionStore.validate(cookie.Value, subdomain)
}

func (h *Handler) isBearerAuthenticated(r *http.Request, auth *protocol.ProxyAuth) bool {
	token := extractBearerToken(r.Header.Get("Authorization"))
	if token == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(auth.Token)) == 1
}

func (h *Handler) serveBearerAuthRequired(w http.ResponseWriter, realm string) {
	if realm == "" {
		realm = "drip"
	}
	w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer realm="%s"`, realm))
	w.Header().Set("Cache-Control", "no-store")
	http.Error(w, "Unauthorized: provide bearer token via Authorization header", http.StatusUnauthorized)
}

func (h *Handler) handleProxyLogin(w http.ResponseWriter, r *http.Request, tconn *tunnel.Connection, subdomain string) {
	h.handleProxyLoginWithRateLimit(w, r, tconn, subdomain, "")
}

func (h *Handler) handleProxyLoginWithRateLimit(w http.ResponseWriter, r *http.Request, tconn *tunnel.Connection, subdomain string, clientIP string) {
	if r.Method != http.MethodPost {
		h.serveLoginPage(w, r, subdomain, "")
		return
	}

	if clientIP != "" && authLimiter.isRateLimited(clientIP) {
		w.Header().Set("Retry-After", "60")
		http.Error(w, "Too many failed authentication attempts. Please try again later.", http.StatusTooManyRequests)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.serveLoginPage(w, r, subdomain, "Invalid form data")
		return
	}

	password := r.FormValue("password")

	if !tconn.ValidateProxyAuth(password) {
		if clientIP != "" {
			authLimiter.recordFailure(clientIP)
		}
		h.serveLoginPage(w, r, subdomain, "Invalid password")
		return
	}

	if clientIP != "" {
		authLimiter.resetFailures(clientIP)
	}

	token := sessionStore.create(subdomain)
	http.SetCookie(w, &http.Cookie{
		Name:     authCookieName + "_" + subdomain,
		Value:    token,
		Path:     "/",
		MaxAge:   int(authSessionDuration.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	redirectURL := r.FormValue("redirect")
	if redirectURL == "" || redirectURL == "/_drip/login" {
		redirectURL = "/"
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

func (h *Handler) serveLoginPage(w http.ResponseWriter, r *http.Request, subdomain string, errorMsg string) {
	redirectURL := r.URL.Path
	if r.URL.RawQuery != "" {
		redirectURL += "?" + r.URL.RawQuery
	}
	if redirectURL == "/_drip/login" {
		redirectURL = "/"
	}

	errorHTML := ""
	if errorMsg != "" {
		errorHTML = fmt.Sprintf(`<p class="error">%s</p>`, html.EscapeString(errorMsg))
	}

	safeRedirectURL := html.EscapeString(redirectURL)

	htmlContent := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8" />
	<meta name="viewport" content="width=device-width, initial-scale=1.0" />
	<title>%s - Drip</title>
	`+faviconLink+`
	<style>
		* { margin: 0; padding: 0; box-sizing: border-box; }
		body {
			font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
			background: #fff;
			color: #24292f;
			line-height: 1.6;
		}
		.container { max-width: 720px; margin: 0 auto; padding: 48px 24px; }
		header { margin-bottom: 48px; }
		h1 { font-size: 28px; font-weight: 600; margin-bottom: 8px; }
		h1 span { margin-right: 8px; }
		.desc { color: #57606a; font-size: 16px; }
		p { margin-bottom: 24px; }
		.error { color: #cf222e; margin-bottom: 16px; }
		.input-wrap {
			position: relative;
			background: #f6f8fa;
			border: 1px solid #d0d7de;
			border-radius: 6px;
			margin-bottom: 12px;
			display: flex;
		}
		.input-wrap input {
			flex: 1;
			margin: 0;
			padding: 12px 16px;
			font-family: ui-monospace, SFMono-Regular, 'SF Mono', Menlo, Consolas, monospace;
			font-size: 14px;
			background: transparent;
			border: none;
			outline: none;
		}
		.input-wrap button {
			background: #24292f;
			color: #fff;
			border: none;
			padding: 8px 16px;
			margin: 4px;
			border-radius: 4px;
			font-size: 14px;
			cursor: pointer;
		}
		.input-wrap button:hover { background: #32383f; }
		footer { margin-top: 48px; padding-top: 24px; border-top: 1px solid #d0d7de; }
		footer a { color: #57606a; text-decoration: none; font-size: 14px; }
		footer a:hover { color: #0969da; }
	</style>
</head>
<body>
	<div class="container">
		<header>
			<h1><span>🔒</span>%s</h1>
			<p class="desc">This tunnel is password protected</p>
		</header>

		%s
		<form method="POST" action="/_drip/login">
			<input type="hidden" name="redirect" value="%s" />
			<div class="input-wrap">
				<input type="password" name="password" placeholder="Enter password" required autofocus />
				<button type="submit">Continue</button>
			</div>
		</form>

		<footer>
			<a href="https://github.com/Gouryella/drip" target="_blank">GitHub</a>
		</footer>
	</div>
</body>
</html>`, subdomain, subdomain, errorHTML, safeRedirectURL)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(htmlContent))
}
