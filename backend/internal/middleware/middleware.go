package middleware

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"baramail/backend/internal/config"
	"baramail/backend/internal/models"
	"baramail/backend/internal/services"
)

type contextKey string

const (
	ctxKeyRequestID    contextKey = "request_id"
	ctxKeyClientIP     contextKey = "client_ip"
	ctxKeyAdminUser    contextKey = "admin_user"
	ctxKeyAdminSession contextKey = "admin_session"
	ctxKeyMailbox      contextKey = "mailbox"
	AdminSessionCookie string     = "baramail_admin_session"
)

func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyRequestID).(string); ok {
		return v
	}
	return ""
}

func ClientIPFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyClientIP).(string); ok && v != "" {
		return v
	}
	return "0.0.0.0"
}

func AdminUserFromContext(ctx context.Context) *models.AdminUser {
	if v, ok := ctx.Value(ctxKeyAdminUser).(*models.AdminUser); ok {
		return v
	}
	return nil
}

func AdminSessionFromContext(ctx context.Context) *models.AdminSession {
	if v, ok := ctx.Value(ctxKeyAdminSession).(*models.AdminSession); ok {
		return v
	}
	return nil
}

func MailboxFromContext(ctx context.Context) *models.Mailbox {
	if v, ok := ctx.Value(ctxKeyMailbox).(*models.Mailbox); ok {
		return v
	}
	return nil
}

// ExtractClientIP trusts CF-Connecting-IP and X-Forwarded-For only when RemoteAddr belongs to TRUSTED_PROXIES.
func ExtractClientIP(r *http.Request, cfg *config.Config) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	host = strings.TrimSpace(host)

	if cfg != nil && cfg.IsTrustedProxy(host) {
		if cfIP := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); cfIP != "" {
			if net.ParseIP(cfIP) != nil {
				return cfIP
			}
		}
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			first := strings.TrimSpace(strings.Split(xff, ",")[0])
			if net.ParseIP(first) != nil {
				return first
			}
		}
	}
	return host
}

func RequestContextAndLogger(cfg *config.Config, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			reqID := r.Header.Get("X-Request-ID")
			if reqID == "" {
				var b [8]byte
				_, _ = rand.Read(b[:])
				reqID = hex.EncodeToString(b[:])
			}
			clientIP := ExtractClientIP(r, cfg)

			w.Header().Set("X-Request-ID", reqID)
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Referrer-Policy", "no-referrer")

			ctx := context.WithValue(r.Context(), ctxKeyRequestID, reqID)
			ctx = context.WithValue(ctx, ctxKeyClientIP, clientIP)

			rw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rw, r.WithContext(ctx))

			if logger != nil {
				logger.Info("http_request",
					slog.String("request_id", reqID),
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.Int("status", rw.status),
					slog.String("ip", clientIP),
					slog.Int64("duration_ms", time.Since(start).Milliseconds()),
				)
			}
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func RequireAdmin(cfg *config.Config, authSvc *services.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(AdminSessionCookie)
			if err != nil || cookie.Value == "" {
				writeMiddlewareError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Autentikasi admin diperlukan.")
				return
			}

			user, session, err := authSvc.ValidateSessionCookie(r.Context(), cookie.Value)
			if err != nil {
				writeMiddlewareError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Sesi admin tidak valid atau telah berakhir.")
				return
			}

			if isStateChangingMethod(r.Method) {
				csrfHeader := strings.TrimSpace(r.Header.Get("X-CSRF-Token"))
				if csrfHeader == "" || subtle.ConstantTimeCompare([]byte(csrfHeader), []byte(session.CSRFToken)) != 1 {
					writeMiddlewareError(w, http.StatusForbidden, "CSRF_INVALID", "Token CSRF tidak valid.")
					return
				}
				if origin := strings.TrimSpace(r.Header.Get("Origin")); origin != "" && cfg.AppBaseURL != "" {
					if !strings.EqualFold(strings.TrimRight(origin, "/"), cfg.AppBaseURL) && cfg.AppEnv == "production" {
						writeMiddlewareError(w, http.StatusForbidden, "ORIGIN_INVALID", "Header Origin tidak diizinkan.")
						return
					}
				}
			}

			ctx := context.WithValue(r.Context(), ctxKeyAdminUser, user)
			ctx = context.WithValue(ctx, ctxKeyAdminSession, session)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireInboxToken(authSvc *services.AuthService, mailboxSvc *services.MailboxService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
			if !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
				writeMiddlewareError(w, http.StatusUnauthorized, "INVALID_TOKEN", "Token inbox diperlukan.")
				return
			}
			rawToken := strings.TrimSpace(authHeader[7:])
			claims, err := authSvc.VerifyInboxToken(rawToken)
			if err != nil {
				writeMiddlewareError(w, http.StatusUnauthorized, "INVALID_TOKEN", "Token inbox tidak valid atau telah kedaluwarsa.")
				return
			}

			// Reload mailbox from DB on every request so disable/expire/domain-retire takes effect immediately.
			mb, err := mailboxSvc.GetAccessibleMailboxByID(r.Context(), claims.MailboxID)
			if err != nil {
				writeMiddlewareError(w, http.StatusNotFound, "MAILBOX_NOT_AVAILABLE", "Mailbox tidak tersedia.")
				return
			}

			ctx := context.WithValue(r.Context(), ctxKeyMailbox, mb)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*rateBucket
	limit   int
	window  time.Duration
}

type rateBucket struct {
	count   int
	resetAt time.Time
}

func NewRateLimiter(limitPerMinute int) *RateLimiter {
	return &RateLimiter{
		buckets: make(map[string]*rateBucket),
		limit:   limitPerMinute,
		window:  time.Minute,
	}
}

func (rl *RateLimiter) Allow(key string) bool {
	if rl.limit <= 0 {
		return true
	}
	now := time.Now()
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if len(rl.buckets) > 10000 {
		for k, b := range rl.buckets {
			if now.After(b.resetAt) {
				delete(rl.buckets, k)
			}
		}
	}

	b, exists := rl.buckets[key]
	if !exists || now.After(b.resetAt) {
		rl.buckets[key] = &rateBucket{
			count:   1,
			resetAt: now.Add(rl.window),
		}
		return true
	}
	if b.count >= rl.limit {
		return false
	}
	b.count++
	return true
}

func RateLimitByIP(limiter *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := ClientIPFromContext(r.Context())
			if !limiter.Allow("ip:" + ip) {
				writeMiddlewareError(w, http.StatusTooManyRequests, "RATE_LIMITED", "Terlalu banyak permintaan. Silakan coba lagi sesaat.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RateLimitByInboxToken(limiter *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("Authorization")
			if key == "" {
				key = ClientIPFromContext(r.Context())
			}
			if !limiter.Allow("tok:" + key) {
				writeMiddlewareError(w, http.StatusTooManyRequests, "RATE_LIMITED", "Terlalu banyak permintaan. Silakan coba lagi sesaat.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func isStateChangingMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func writeMiddlewareError(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": false,
		"error": map[string]string{
			"code":    code,
			"message": msg,
		},
	})
}

