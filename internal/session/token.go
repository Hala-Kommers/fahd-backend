package session

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type TokenClaims struct {
	SessionID string `json:"sid"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

type TokenService struct {
	secret []byte
	ttl    time.Duration
}

func NewTokenService(secret string, ttl time.Duration) *TokenService {
	return &TokenService{secret: []byte(secret), ttl: ttl}
}

func (s *TokenService) Generate(sessionID string) (string, int64, error) {
	now := time.Now()
	exp := now.Add(s.ttl)
	claims := jwt.MapClaims{
		"sid": sessionID,
		"iat": now.Unix(),
		"exp": exp.Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.secret)
	if err != nil {
		return "", 0, fmt.Errorf("sign token: %w", err)
	}
	return signed, exp.Unix(), nil
}

func (s *TokenService) Validate(tokenStr string) (*TokenClaims, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	sid, _ := claims["sid"].(string)
	iatFloat, _ := claims["iat"].(float64)
	expFloat, _ := claims["exp"].(float64)
	if sid == "" {
		return nil, fmt.Errorf("missing session id")
	}
	return &TokenClaims{
		SessionID: sid,
		IssuedAt:  int64(iatFloat),
		ExpiresAt: int64(expFloat),
	}, nil
}
