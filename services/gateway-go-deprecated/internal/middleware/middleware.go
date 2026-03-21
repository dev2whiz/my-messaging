package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/my-messaging/gateway/internal/auth"
)

type contextKey string

const UserKey contextKey = "user"

type UserClaims struct {
	UserID   string
	Username string
}

// Auth validates the Bearer JWT from the Authorization header.
func Auth(authSvc *auth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr := bearerToken(r)
			if tokenStr == "" {
				http.Error(w, `{"error":"missing token"}`, http.StatusUnauthorized)
				return
			}
			claims, err := authSvc.ValidateToken(r.Context(), tokenStr)
			if err != nil {
				http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), UserKey, &UserClaims{
				UserID:   claims.UserID,
				Username: claims.Username,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// TokenFromQuery reads the JWT from ?token= query param (used for WebSocket upgrade).
func TokenFromQuery(authSvc *auth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr := r.URL.Query().Get("token")
			if tokenStr == "" {
				http.Error(w, `{"error":"missing token"}`, http.StatusUnauthorized)
				return
			}
			claims, err := authSvc.ValidateToken(r.Context(), tokenStr)
			if err != nil {
				http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), UserKey, &UserClaims{
				UserID:   claims.UserID,
				Username: claims.Username,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetUser(ctx context.Context) *UserClaims {
	u, _ := ctx.Value(UserKey).(*UserClaims)
	return u
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	return ""
}

// CORS adds permissive CORS headers (dev-friendly; tighten in Stage 5).
func CORS(origin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
