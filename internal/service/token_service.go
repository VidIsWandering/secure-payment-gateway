package service

import (
	"fmt"
	"time"

	"secure-payment-gateway/internal/core/ports"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// JWTTokenService implements ports.TokenService using HS256 JWT.
type JWTTokenService struct {
	secret []byte
	expiry time.Duration
	issuer string
}

// NewJWTTokenService creates a new JWT token service.
func NewJWTTokenService(secret string, expiry time.Duration, issuer string) *JWTTokenService {
	return &JWTTokenService{
		secret: []byte(secret),
		expiry: expiry,
		issuer: issuer,
	}
}

// Generate creates a signed JWT for the given merchant.
func (s *JWTTokenService) Generate(merchantID uuid.UUID, accessKey string) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(s.expiry)

	claims := jwt.MapClaims{
		"sub":        merchantID.String(),
		"access_key": accessKey,
		"iat":        now.Unix(),
		"exp":        expiresAt.Unix(),
		"iss":        s.issuer,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(s.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("signing token: %w", err)
	}

	return tokenString, expiresAt, nil
}

// Validate parses and validates a JWT token, returning the claims.
func (s *JWTTokenService) Validate(tokenString string) (*ports.TokenClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("parsing token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	sub, ok := claims["sub"].(string)
	if !ok {
		return nil, fmt.Errorf("missing subject claim")
	}

	merchantID, err := uuid.Parse(sub)
	if err != nil {
		return nil, fmt.Errorf("invalid merchant ID in token: %w", err)
	}

	accessKey, _ := claims["access_key"].(string)

	return &ports.TokenClaims{
		MerchantID: merchantID,
		AccessKey:  accessKey,
	}, nil
}
