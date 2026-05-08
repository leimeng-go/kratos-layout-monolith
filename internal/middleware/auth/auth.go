package auth

import (
	"context"
	"strings"
	"time"

	"github.com/go-kratos/kratos-layout-monolith/internal/conf"

	"github.com/go-kratos/kratos/v2/middleware"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const TokenKey contextKey = "jwt_token"

// Claims holds JWT claims.
type Claims struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// GenerateToken creates a new JWT token.
func GenerateToken(secret string, userID int64, username string, expireSeconds int64) (string, error) {
	claims := Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expireSeconds) * time.Second)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ParseToken validates and parses a JWT token.
func ParseToken(secret, tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, jwt.ErrTokenInvalidClaims
}

// FromContext extracts the JWT claims from the context.
func FromContext(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(TokenKey).(*Claims)
	return claims, ok
}

// Server returns an HTTP middleware that validates JWT tokens.
func Server(ac *conf.Auth, secret string) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			r, ok := khttp.RequestFromServerContext(ctx)
			if !ok {
				return next(ctx, req)
			}

			// Check whitelist - skip auth for these paths
			path := r.URL.Path
			for _, w := range ac.Whitelist {
				if path == w {
					return next(ctx, req)
				}
			}

			// Extract Bearer token from header or query parameter
			authHeader := r.Header.Get("Authorization")
			var tokenStr string
			if authHeader != "" {
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
					tokenStr = parts[1]
				}
			}
			if tokenStr == "" {
				tokenStr = r.URL.Query().Get("token")
			}
			if tokenStr != "" {
				claims, err := ParseToken(secret, tokenStr)
				if err == nil && claims != nil {
					ctx = context.WithValue(ctx, TokenKey, claims)
				}
			}

			return next(ctx, req)
		}
	}
}
