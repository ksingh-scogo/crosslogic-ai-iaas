package gateway

import (
	"net/http"
	"strings"
)

// SecurityConfig holds configuration for security middleware
type SecurityConfig struct {
	// EnableHSTS enables HTTP Strict Transport Security
	EnableHSTS bool
	// HSTSMaxAge is the max-age for HSTS header in seconds (default: 31536000 = 1 year)
	HSTSMaxAge int
	// EnableFrameOptions enables X-Frame-Options header
	EnableFrameOptions bool
	// FrameOptionsValue is the value for X-Frame-Options (default: DENY)
	FrameOptionsValue string
	// EnableContentTypeOptions enables X-Content-Type-Options header
	EnableContentTypeOptions bool
	// EnableXSSProtection enables X-XSS-Protection header
	EnableXSSProtection bool
	// ContentSecurityPolicy is the CSP header value (empty = not set)
	ContentSecurityPolicy string
	// ReferrerPolicy is the Referrer-Policy header value
	ReferrerPolicy string
	// PermissionsPolicy is the Permissions-Policy header value
	PermissionsPolicy string
	// AllowedHosts is a list of allowed host headers (empty = allow all)
	AllowedHosts []string
}

// DefaultSecurityConfig returns a secure default configuration
func DefaultSecurityConfig() SecurityConfig {
	return SecurityConfig{
		EnableHSTS:               true,
		HSTSMaxAge:               31536000, // 1 year
		EnableFrameOptions:       true,
		FrameOptionsValue:        "DENY",
		EnableContentTypeOptions: true,
		EnableXSSProtection:      true,
		ContentSecurityPolicy:    "default-src 'self'; frame-ancestors 'none'",
		ReferrerPolicy:           "strict-origin-when-cross-origin",
		PermissionsPolicy:        "geolocation=(), microphone=(), camera=()",
		AllowedHosts:             []string{}, // Allow all by default
	}
}

// SecurityMiddleware adds security headers to all responses
func SecurityMiddleware(config SecurityConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Host header validation (prevent host header injection)
			if len(config.AllowedHosts) > 0 {
				host := r.Host
				// Remove port if present
				if colonIdx := strings.LastIndex(host, ":"); colonIdx != -1 {
					host = host[:colonIdx]
				}
				allowed := false
				for _, allowedHost := range config.AllowedHosts {
					if strings.EqualFold(host, allowedHost) {
						allowed = true
						break
					}
				}
				if !allowed {
					http.Error(w, "Invalid host header", http.StatusBadRequest)
					return
				}
			}

			// HTTP Strict Transport Security
			if config.EnableHSTS {
				w.Header().Set("Strict-Transport-Security",
					"max-age=31536000; includeSubDomains; preload")
			}

			// Prevent clickjacking
			if config.EnableFrameOptions {
				w.Header().Set("X-Frame-Options", config.FrameOptionsValue)
			}

			// Prevent MIME type sniffing
			if config.EnableContentTypeOptions {
				w.Header().Set("X-Content-Type-Options", "nosniff")
			}

			// XSS Protection (legacy but still useful)
			if config.EnableXSSProtection {
				w.Header().Set("X-XSS-Protection", "1; mode=block")
			}

			// Content Security Policy
			if config.ContentSecurityPolicy != "" {
				w.Header().Set("Content-Security-Policy", config.ContentSecurityPolicy)
			}

			// Referrer Policy
			if config.ReferrerPolicy != "" {
				w.Header().Set("Referrer-Policy", config.ReferrerPolicy)
			}

			// Permissions Policy (formerly Feature-Policy)
			if config.PermissionsPolicy != "" {
				w.Header().Set("Permissions-Policy", config.PermissionsPolicy)
			}

			// Prevent caching of sensitive data
			// Only apply to API responses, not static assets
			if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/v1/") || strings.HasPrefix(r.URL.Path, "/admin/") {
				w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
				w.Header().Set("Pragma", "no-cache")
				w.Header().Set("Expires", "0")
			}

			// Remove server identification headers
			w.Header().Del("Server")
			w.Header().Del("X-Powered-By")

			next.ServeHTTP(w, r)
		})
	}
}

// APISecurityMiddleware adds API-specific security measures
func APISecurityMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Ensure JSON content type for API responses
			if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/v1/") || strings.HasPrefix(r.URL.Path, "/admin/") {
				// Check Content-Type for POST/PUT/PATCH requests
				if r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH" {
					contentType := r.Header.Get("Content-Type")
					// Allow empty content-type for some endpoints (like health checks)
					if contentType != "" && !strings.Contains(contentType, "application/json") && !strings.Contains(contentType, "multipart/form-data") {
						http.Error(w, `{"error":{"message":"Content-Type must be application/json","type":"invalid_request_error"}}`, http.StatusUnsupportedMediaType)
						return
					}
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequestSizeLimitMiddleware limits the size of incoming request bodies
func RequestSizeLimitMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Limit request body size (default 10MB for inference requests)
			if maxBytes > 0 {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// SanitizeErrorMiddleware prevents leaking sensitive information in error responses
type sanitizedResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *sanitizedResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

// SensitivePatterns contains patterns that should be redacted from error messages
var sensitivePatterns = []string{
	"password",
	"secret",
	"token",
	"key",
	"credential",
	"authorization",
	"bearer",
	"postgres://",
	"redis://",
	"mysql://",
	"mongodb://",
}

// ContainsSensitiveInfo checks if a string contains sensitive information
func ContainsSensitiveInfo(s string) bool {
	lower := strings.ToLower(s)
	for _, pattern := range sensitivePatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

// AnonymizeAPIKey masks an API key for logging, showing only prefix
func AnonymizeAPIKey(key string) string {
	if len(key) <= 12 {
		return "****"
	}
	return key[:12] + "****"
}

// AnonymizeEmail masks an email for logging
func AnonymizeEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "****"
	}
	if len(parts[0]) <= 2 {
		return "**@" + parts[1]
	}
	return parts[0][:2] + "****@" + parts[1]
}
