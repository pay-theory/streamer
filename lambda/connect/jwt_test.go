//go:build !lift
// +build !lift

package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helper to create a signed JWT token
func createTestToken(t *testing.T, privateKey *rsa.PrivateKey, claims *Claims) string {
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(privateKey)
	require.NoError(t, err)
	return tokenString
}

func TestNewJWTVerifier_Success(t *testing.T) {
	_, publicKeyPEM := generateTestKeyPair(t)

	verifier, err := NewJWTVerifier(publicKeyPEM, "test-issuer")

	assert.NoError(t, err)
	assert.NotNil(t, verifier)
	assert.NotNil(t, verifier.publicKey)
	assert.Equal(t, "test-issuer", verifier.issuer)
}

func TestNewJWTVerifier_InvalidPEM(t *testing.T) {
	invalidPEM := "invalid-pem-data"

	verifier, err := NewJWTVerifier(invalidPEM, "test-issuer")

	assert.Error(t, err)
	assert.Nil(t, verifier)
	assert.Contains(t, err.Error(), "failed to parse public key")
}

func TestJWTVerifier_Verify_Success(t *testing.T) {
	privateKey, publicKeyPEM := generateTestKeyPair(t)
	verifier, err := NewJWTVerifier(publicKeyPEM, "test-issuer")
	require.NoError(t, err)

	now := time.Now()
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user123",
			Issuer:    "test-issuer",
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
		TenantID:    "tenant456",
		Permissions: []string{"read", "write"},
	}

	tokenString := createTestToken(t, privateKey, claims)

	result, err := verifier.Verify(tokenString)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "user123", result.Subject)
	assert.Equal(t, "tenant456", result.TenantID)
	assert.Equal(t, []string{"read", "write"}, result.Permissions)
}

func TestJWTVerifier_Verify_WithBearerPrefix(t *testing.T) {
	privateKey, publicKeyPEM := generateTestKeyPair(t)
	verifier, err := NewJWTVerifier(publicKeyPEM, "test-issuer")
	require.NoError(t, err)

	now := time.Now()
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user123",
			Issuer:    "test-issuer",
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
		TenantID:    "tenant456",
		Permissions: []string{"read"},
	}

	tokenString := createTestToken(t, privateKey, claims)
	bearerToken := "Bearer " + tokenString

	result, err := verifier.Verify(bearerToken)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "user123", result.Subject)
}

func TestJWTVerifier_Verify_ExpiredToken(t *testing.T) {
	privateKey, publicKeyPEM := generateTestKeyPair(t)
	verifier, err := NewJWTVerifier(publicKeyPEM, "test-issuer")
	require.NoError(t, err)

	now := time.Now()
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user123",
			Issuer:    "test-issuer",
			ExpiresAt: jwt.NewNumericDate(now.Add(-time.Hour)), // Expired
			IssuedAt:  jwt.NewNumericDate(now.Add(-2 * time.Hour)),
		},
		TenantID:    "tenant456",
		Permissions: []string{"read"},
	}

	tokenString := createTestToken(t, privateKey, claims)

	result, err := verifier.Verify(tokenString)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "token is expired")
}

func TestJWTVerifier_Verify_NotYetValid(t *testing.T) {
	privateKey, publicKeyPEM := generateTestKeyPair(t)
	verifier, err := NewJWTVerifier(publicKeyPEM, "test-issuer")
	require.NoError(t, err)

	now := time.Now()
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user123",
			Issuer:    "test-issuer",
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now.Add(time.Hour)), // Not yet valid
		},
		TenantID:    "tenant456",
		Permissions: []string{"read"},
	}

	tokenString := createTestToken(t, privateKey, claims)

	result, err := verifier.Verify(tokenString)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "token is not valid yet")
}

func TestJWTVerifier_Verify_FutureIssuedAt(t *testing.T) {
	privateKey, publicKeyPEM := generateTestKeyPair(t)
	verifier, err := NewJWTVerifier(publicKeyPEM, "test-issuer")
	require.NoError(t, err)

	now := time.Now()
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user123",
			Issuer:    "test-issuer",
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now.Add(2 * time.Minute)), // Too far in future
		},
		TenantID:    "tenant456",
		Permissions: []string{"read"},
	}

	tokenString := createTestToken(t, privateKey, claims)

	result, err := verifier.Verify(tokenString)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "token issued in the future")
}

func TestJWTVerifier_Verify_InvalidIssuer(t *testing.T) {
	privateKey, publicKeyPEM := generateTestKeyPair(t)
	verifier, err := NewJWTVerifier(publicKeyPEM, "expected-issuer")
	require.NoError(t, err)

	now := time.Now()
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user123",
			Issuer:    "wrong-issuer",
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
		TenantID:    "tenant456",
		Permissions: []string{"read"},
	}

	tokenString := createTestToken(t, privateKey, claims)

	result, err := verifier.Verify(tokenString)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "invalid issuer")
}

func TestJWTVerifier_Verify_MissingSubject(t *testing.T) {
	privateKey, publicKeyPEM := generateTestKeyPair(t)
	verifier, err := NewJWTVerifier(publicKeyPEM, "test-issuer")
	require.NoError(t, err)

	now := time.Now()
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			// Subject missing
			Issuer:    "test-issuer",
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
		TenantID:    "tenant456",
		Permissions: []string{"read"},
	}

	tokenString := createTestToken(t, privateKey, claims)

	result, err := verifier.Verify(tokenString)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "missing subject")
}

func TestJWTVerifier_Verify_MissingTenantID(t *testing.T) {
	privateKey, publicKeyPEM := generateTestKeyPair(t)
	verifier, err := NewJWTVerifier(publicKeyPEM, "test-issuer")
	require.NoError(t, err)

	now := time.Now()
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user123",
			Issuer:    "test-issuer",
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
		// TenantID missing
		Permissions: []string{"read"},
	}

	tokenString := createTestToken(t, privateKey, claims)

	result, err := verifier.Verify(tokenString)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "missing tenant ID")
}

func TestJWTVerifier_Verify_InvalidSignature(t *testing.T) {
	_, publicKeyPEM := generateTestKeyPair(t)
	verifier, err := NewJWTVerifier(publicKeyPEM, "test-issuer")
	require.NoError(t, err)

	// Create token with different private key
	wrongPrivateKey, _ := generateTestKeyPair(t)
	now := time.Now()
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user123",
			Issuer:    "test-issuer",
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
		TenantID:    "tenant456",
		Permissions: []string{"read"},
	}

	tokenString := createTestToken(t, wrongPrivateKey, claims)

	result, err := verifier.Verify(tokenString)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to parse token")
}

func TestJWTVerifier_Verify_MalformedToken(t *testing.T) {
	_, publicKeyPEM := generateTestKeyPair(t)
	verifier, err := NewJWTVerifier(publicKeyPEM, "test-issuer")
	require.NoError(t, err)

	malformedToken := "not.a.valid.jwt.token"

	result, err := verifier.Verify(malformedToken)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to parse token")
}

func TestJWTVerifier_VerifyWithoutIssuerCheck(t *testing.T) {
	privateKey, publicKeyPEM := generateTestKeyPair(t)
	verifier, err := NewJWTVerifier(publicKeyPEM, "expected-issuer")
	require.NoError(t, err)

	now := time.Now()
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user123",
			Issuer:    "different-issuer", // This would normally fail
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
		TenantID:    "tenant456",
		Permissions: []string{"read"},
	}

	tokenString := createTestToken(t, privateKey, claims)

	result, err := verifier.VerifyWithoutIssuerCheck(tokenString)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "user123", result.Subject)
	assert.Equal(t, "tenant456", result.TenantID)
}

func TestParsePublicKey_PKCS1Format(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// Create PKCS#1 format PEM
	publicKeyBytes := x509.MarshalPKCS1PublicKey(&privateKey.PublicKey)
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	publicKey, err := parsePublicKey(string(publicKeyPEM))

	assert.NoError(t, err)
	assert.NotNil(t, publicKey)
	assert.Equal(t, &privateKey.PublicKey, publicKey)
}

func TestParsePublicKey_PKCS8Format(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// Create PKCS#8 format PEM
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	require.NoError(t, err)

	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	publicKey, err := parsePublicKey(string(publicKeyPEM))

	assert.NoError(t, err)
	assert.NotNil(t, publicKey)
	assert.Equal(t, &privateKey.PublicKey, publicKey)
}

func TestParsePublicKey_InvalidPEM(t *testing.T) {
	invalidPEM := "not-a-pem-block"

	publicKey, err := parsePublicKey(invalidPEM)

	assert.Error(t, err)
	assert.Nil(t, publicKey)
	assert.Contains(t, err.Error(), "failed to decode PEM block")
}

func TestParsePublicKey_UnsupportedKeyType(t *testing.T) {
	// Create a PEM with unsupported type
	unsupportedPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: []byte("dummy-data"),
	})

	publicKey, err := parsePublicKey(string(unsupportedPEM))

	assert.Error(t, err)
	assert.Nil(t, publicKey)
	assert.Contains(t, err.Error(), "unsupported key type")
}
