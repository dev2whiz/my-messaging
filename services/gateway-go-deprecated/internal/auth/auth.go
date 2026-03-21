package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

var ErrTokenBlocked = errors.New("auth: token has been revoked")

type Service struct {
	secret     []byte
	expiryHrs  int
	valkey     *redis.Client
}

type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

func NewService(secret string, expiryHours int, valkey *redis.Client) *Service {
	return &Service{
		secret:    []byte(secret),
		expiryHrs: expiryHours,
		valkey:    valkey,
	}
}

// IssueToken creates a signed JWT for the given user.
func (s *Service) IssueToken(userID, username string) (string, error) {
	exp := time.Now().Add(time.Duration(s.expiryHrs) * time.Hour)
	claims := Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   userID,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

// ValidateToken parses and validates a JWT string, checking the Valkey blocklist.
func (s *Service) ValidateToken(ctx context.Context, tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("auth: unexpected signing method: %v", t.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("auth: invalid token claims")
	}

	// Check Valkey blocklist (logout)
	blocked, err := s.valkey.Exists(ctx, blocklistKey(tokenStr)).Result()
	if err != nil {
		return nil, fmt.Errorf("auth: valkey check: %w", err)
	}
	if blocked > 0 {
		return nil, ErrTokenBlocked
	}
	return claims, nil
}

// RevokeToken adds the token to the Valkey blocklist until its natural expiry.
func (s *Service) RevokeToken(ctx context.Context, tokenStr string, exp time.Time) error {
	ttl := time.Until(exp)
	if ttl <= 0 {
		return nil // already expired
	}
	return s.valkey.Set(ctx, blocklistKey(tokenStr), 1, ttl).Err()
}

func blocklistKey(token string) string {
	return "auth:blocklist:" + token
}
