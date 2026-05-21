package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/LYP-leo/xzxg-shop/backend/src/domain"
)

type contextKey string

const (
	requestIDKey contextKey = "request_id"
	accountKey   contextKey = "account"
)

type rateLimiter struct {
	mu      sync.Mutex
	clients map[string]rateLimitState
	limit   int
	window  time.Duration
}

type rateLimitState struct {
	count       int
	windowStart time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		clients: make(map[string]rateLimitState),
		limit:   limit,
		window:  window,
	}
}

func (l *rateLimiter) allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	state := l.clients[key]
	if state.windowStart.IsZero() || now.Sub(state.windowStart) > l.window {
		l.clients[key] = rateLimitState{count: 1, windowStart: now}
		return true
	}
	if state.count >= l.limit {
		return false
	}
	state.count++
	l.clients[key] = state
	return true
}

func (s *Server) withRequestContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = newRequestID()
		}
		w.Header().Set("X-Request-ID", requestID)
		ctx := context.WithValue(r.Context(), requestIDKey, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) withAccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		s.logger.Info("http request completed",
			"request_id", requestIDFromContext(r.Context()),
			"method", r.Method,
			"path", r.URL.Path,
			"status", recorder.status,
			"duration_ms", time.Since(startedAt).Milliseconds(),
			"client_ip", clientIP(r),
		)
	})
}

func (s *Server) withRateLimit(next http.Handler) http.Handler {
	limiter := newRateLimiter(120, time.Minute)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions || r.URL.Path == "/api/v1/health" {
			next.ServeHTTP(w, r)
			return
		}
		key := clientIP(r)
		if account, ok := accountFromContext(r.Context()); ok {
			key = string(account.Role) + ":" + account.AccountID
		}
		if !limiter.allow(key, time.Now()) {
			writeError(w, http.StatusTooManyRequests, "rate_limited", "请求过于频繁，请稍后再试")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) withBodyLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil && r.Method != http.MethodGet && r.Method != http.MethodHead {
			r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		account, hasAccount := s.accountFromRequest(r)
		if hasAccount {
			r = r.WithContext(context.WithValue(r.Context(), accountKey, account))
		}

		required, allowedRoles := routePolicy(r.Method, r.URL.Path)
		if !required {
			next.ServeHTTP(w, r)
			return
		}
		if !hasAccount {
			writeError(w, http.StatusUnauthorized, "unauthorized", "请先登录")
			return
		}
		if !roleAllowed(account.Role, allowedRoles) {
			writeError(w, http.StatusForbidden, "forbidden", "当前账号无权访问")
			return
		}
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func routePolicy(method string, path string) (bool, []domain.AccountRole) {
	if path == "/api/v1/health" || path == "/api/v1/auth/login" {
		return false, nil
	}
	if method == http.MethodGet && (path == "/api/v1/categories/tree" || path == "/api/v1/merchants" || path == "/api/v1/products" || strings.HasPrefix(path, "/api/v1/products/")) {
		return false, nil
	}
	if path == "/api/v1/auth/me" {
		return true, []domain.AccountRole{domain.AccountRoleUser, domain.AccountRoleMerchant, domain.AccountRoleAdmin}
	}
	if strings.HasPrefix(path, "/api/v1/cart") || strings.HasPrefix(path, "/api/v1/agent") || strings.HasPrefix(path, "/api/v1/orders") {
		return true, []domain.AccountRole{domain.AccountRoleUser}
	}
	if strings.HasPrefix(path, "/api/v1/merchant") {
		return true, []domain.AccountRole{domain.AccountRoleMerchant}
	}
	if strings.HasPrefix(path, "/api/v1/admin") {
		return true, []domain.AccountRole{domain.AccountRoleAdmin}
	}
	return false, nil
}

func roleAllowed(role domain.AccountRole, allowed []domain.AccountRole) bool {
	for _, item := range allowed {
		if item == role {
			return true
		}
	}
	return false
}

func accountFromContext(ctx context.Context) (domain.Account, bool) {
	account, ok := ctx.Value(accountKey).(domain.Account)
	return account, ok
}

func requestIDFromContext(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDKey).(string)
	return requestID
}

func newRequestID() string {
	var bytes [12]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return time.Now().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(bytes[:])
}

func clientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func logAttrsForAccount(logger *slog.Logger, account domain.Account) *slog.Logger {
	return logger.With("account_id", account.AccountID, "role", account.Role)
}
