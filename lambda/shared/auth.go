package shared

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTClaims represents the expected JWT claims
type JWTClaims struct {
	UserID      string   `json:"user_id"`
	TenantID    string   `json:"tenant_id"`
	Permissions []string `json:"permissions,omitempty"`
	jwt.RegisteredClaims
}

// ValidateJWT validates a JWT token and returns the claims
func ValidateJWT(tokenString, secret string) (map[string]interface{}, error) {
	// Remove Bearer prefix if present
	tokenString = strings.TrimPrefix(tokenString, "Bearer ")
	tokenString = strings.TrimSpace(tokenString)

	// Parse the token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Validate the signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	// Check if token is valid
	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	// Extract claims
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("invalid token claims")
	}

	// Validate expiration
	if exp, ok := claims["exp"].(float64); ok {
		if time.Now().Unix() > int64(exp) {
			return nil, errors.New("token expired")
		}
	} else {
		return nil, errors.New("missing expiration claim")
	}

	// Validate required fields
	if _, ok := claims["user_id"].(string); !ok {
		return nil, errors.New("missing user_id claim")
	}

	if _, ok := claims["tenant_id"].(string); !ok {
		return nil, errors.New("missing tenant_id claim")
	}

	return claims, nil
}

// GenerateJWT generates a JWT token for testing
func GenerateJWT(userID, tenantID string, permissions []string, secret string, duration time.Duration) (string, error) {
	claims := JWTClaims{
		UserID:      userID,
		TenantID:    tenantID,
		Permissions: permissions,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(duration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}
