package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

type Claims struct {
	TokenType string `json:"typ"`
	jwt.RegisteredClaims
}

type TokenManager struct {
	accessSecret  []byte
	refreshSecret []byte
	accessTTL     time.Duration
	refreshTTL    time.Duration
}

func NewTokenManager(accessSecret string, refreshSecret string, accessTTL time.Duration, refreshTTL time.Duration) *TokenManager {
	return &TokenManager{
		accessSecret:  []byte(accessSecret),
		refreshSecret: []byte(refreshSecret),
		accessTTL:     accessTTL,
		refreshTTL:    refreshTTL,
	}
}

func (m *TokenManager) AccessTTL() time.Duration {
	return m.accessTTL
}

func (m *TokenManager) RefreshTTL() time.Duration {
	return m.refreshTTL
}

func (m *TokenManager) IssueAccess(userID uuid.UUID) (string, time.Time, error) {
	return m.issue(userID, TokenTypeAccess, m.accessSecret, m.accessTTL)
}

func (m *TokenManager) IssueRefresh(userID uuid.UUID) (string, time.Time, error) {
	return m.issue(userID, TokenTypeRefresh, m.refreshSecret, m.refreshTTL)
}

func (m *TokenManager) VerifyAccess(tokenString string) (*Claims, error) {
	return m.verify(tokenString, TokenTypeAccess, m.accessSecret)
}

func (m *TokenManager) VerifyRefresh(tokenString string) (*Claims, error) {
	return m.verify(tokenString, TokenTypeRefresh, m.refreshSecret)
}

func (m *TokenManager) issue(userID uuid.UUID, tokenType string, secret []byte, ttl time.Duration) (string, time.Time, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(ttl)
	claims := Claims{
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			ID:        uuid.NewString(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(secret)
	return signed, expiresAt, err
}

func (m *TokenManager) verify(tokenString string, expectedType string, secret []byte) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return secret, nil
	})
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	if claims.TokenType != expectedType {
		return nil, fmt.Errorf("invalid token type")
	}
	if _, err := uuid.Parse(claims.Subject); err != nil {
		return nil, fmt.Errorf("invalid token subject")
	}
	return claims, nil
}
