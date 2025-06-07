package main

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims represents the JWT claims we expect
type Claims struct {
	jwt.RegisteredClaims
	TenantID    string   `json:"tenant_id"`
	Permissions []string `json:"permissions"`
}

// JWTVerifier handles JWT token verification
type JWTVerifier struct {
	publicKey *rsa.PublicKey
	issuer    string
}

// NewJWTVerifier creates a new JWT verifier with the given public key
func NewJWTVerifier(publicKeyPEM, issuer string) (*JWTVerifier, error) {
	// Parse the public key
	publicKey, err := parsePublicKey(publicKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	return &JWTVerifier{
		publicKey: publicKey,
		issuer:    issuer,
	}, nil
}

// Verify validates a JWT token and returns the claims
func (v *JWTVerifier) Verify(tokenString string) (*Claims, error) {
	// Remove "Bearer " prefix if present
	tokenString = strings.TrimPrefix(tokenString, "Bearer ")
	tokenString = strings.TrimSpace(tokenString)

	// Parse and validate the token
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify the algorithm is RS256
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return v.publicKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	// Check if token is valid
	if !token.Valid {
		return nil, errors.New("token is invalid")
	}

	// Extract claims
	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, errors.New("failed to extract claims")
	}

	// Validate standard claims
	if err := v.validateClaims(claims); err != nil {
		return nil, err
	}

	return claims, nil
}

// validateClaims performs additional validation on the claims
func (v *JWTVerifier) validateClaims(claims *Claims) error {
	now := time.Now()

	// Check expiration
	if claims.ExpiresAt != nil && claims.ExpiresAt.Before(now) {
		return errors.New("token has expired")
	}

	// Check not before
	if claims.NotBefore != nil && claims.NotBefore.After(now) {
		return errors.New("token not yet valid")
	}

	// Check issued at (reject tokens from the future)
	if claims.IssuedAt != nil && claims.IssuedAt.After(now.Add(1*time.Minute)) {
		return errors.New("token issued in the future")
	}

	// Validate issuer if configured
	if v.issuer != "" && claims.Issuer != v.issuer {
		return fmt.Errorf("invalid issuer: expected %s, got %s", v.issuer, claims.Issuer)
	}

	// Validate required fields
	if claims.Subject == "" {
		return errors.New("missing subject (user ID)")
	}

	if claims.TenantID == "" {
		return errors.New("missing tenant ID")
	}

	return nil
}

// parsePublicKey parses a PEM-encoded RSA public key
func parsePublicKey(pemString string) (*rsa.PublicKey, error) {
	// Decode PEM block
	block, _ := pem.Decode([]byte(pemString))
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}

	// Parse the public key
	switch block.Type {
	case "RSA PUBLIC KEY":
		// PKCS#1 format
		pub, err := x509.ParsePKCS1PublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse PKCS1 public key: %w", err)
		}
		return pub, nil

	case "PUBLIC KEY":
		// PKCS#8 format
		pub, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse PKIX public key: %w", err)
		}

		rsaPub, ok := pub.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("not an RSA public key")
		}
		return rsaPub, nil

	default:
		return nil, fmt.Errorf("unsupported key type: %s", block.Type)
	}
}
