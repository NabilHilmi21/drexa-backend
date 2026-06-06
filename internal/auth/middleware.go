package auth

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const userClaimsKey contextKey = "user_claims"

// JWTMiddleware validates the access token on every request.
// Token is read from the Authorization header first, then the access_token cookie.
// Injects *JWTClaims into the request context on success.
func JWTMiddleware(tokenSvc TokenService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractBearerToken(r)
			if token == "" {
				sendJSON(w, http.StatusUnauthorized, MessageResponse{Error: "unauthorized"})
				return
			}

			claims, err := tokenSvc.ValidateAccessToken(r.Context(), token)
			if err != nil {
				sendJSON(w, http.StatusUnauthorized, MessageResponse{Error: "unauthorized"})
				return
			}

			ctx := context.WithValue(r.Context(), userClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserFromContext retrieves the JWT claims injected by JWTMiddleware.
func UserFromContext(ctx context.Context) (*JWTClaims, bool) {
	claims, ok := ctx.Value(userClaimsKey).(*JWTClaims)
	return claims, ok
}

func extractBearerToken(r *http.Request) string {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	if c, err := r.Cookie("access_token"); err == nil {
		return c.Value
	}
	return ""
}
