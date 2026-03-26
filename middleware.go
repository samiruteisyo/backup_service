package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Session struct {
	Token     string
	ExpiresAt time.Time
}

var (
	sessions  = make(map[string]Session)
	sessionMu sync.RWMutex
)

func generateSessionToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func hashPassword(password string) string {
	h := sha256.Sum256([]byte(password))
	return hex.EncodeToString(h[:])
}

func createSession() string {
	token := generateSessionToken()
	sessionMu.Lock()
	sessions[token] = Session{
		Token:     token,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	sessionMu.Unlock()
	return token
}

func validateSession(token string) bool {
	sessionMu.RLock()
	s, ok := sessions[token]
	sessionMu.RUnlock()

	if !ok {
		return false
	}

	if time.Now().After(s.ExpiresAt) {
		sessionMu.Lock()
		delete(sessions, token)
		sessionMu.Unlock()
		return false
	}

	return true
}

func destroySession(token string) {
	sessionMu.Lock()
	delete(sessions, token)
	sessionMu.Unlock()
}

func cleanupSessions() {
	sessionMu.Lock()
	defer sessionMu.Unlock()
	now := time.Now()
	for token, s := range sessions {
		if now.After(s.ExpiresAt) {
			delete(sessions, token)
		}
	}
}

func startSessionCleanup() {
	ticker := time.NewTicker(1 * time.Hour)
	go func() {
		for range ticker.C {
			cleanupSessions()
		}
	}()
}

func getSessionToken(r *http.Request) string {
	cookie, err := r.Cookie("session")
	if err != nil {
		return ""
	}
	return cookie.Value
}

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" || r.URL.Path == "/api/login" {
			next.ServeHTTP(w, r)
			return
		}

		token := getSessionToken(r)
		if !validateSession(token) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				w.Header().Set("Content-Type", "application/json")
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func requireAuth(r *http.Request) bool {
	token := getSessionToken(r)
	return validateSession(token)
}

func setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   86400,
	})
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	fmt.Fprint(w, marshalJSON(data))
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func marshalJSON(data interface{}) string {
	b, err := json.Marshal(data)
	if err != nil {
		return `{"error":"internal error"}`
	}
	return string(b)
}
