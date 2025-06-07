package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func main() {
	var (
		userID      = flag.String("user-id", "demo-user", "User ID for the JWT")
		tenantID    = flag.String("tenant-id", "demo-tenant", "Tenant ID for the JWT")
		permissions = flag.String("permissions", "read,write", "Comma-separated permissions")
		expired     = flag.Bool("expired", false, "Generate an expired token")
		privateKey  = flag.String("private-key", "demo/private.pem", "Path to RSA private key")
		issuer      = flag.String("issuer", "https://auth.pay-theory.com", "JWT issuer")
	)
	flag.Parse()

	// Generate or load private key
	key, err := loadOrGeneratePrivateKey(*privateKey)
	if err != nil {
		log.Fatalf("Failed to load/generate private key: %v", err)
	}

	// Create claims
	claims := jwt.MapClaims{
		"sub":         *userID,
		"tenant_id":   *tenantID,
		"iss":         *issuer,
		"iat":         time.Now().Unix(),
		"permissions": strings.Split(*permissions, ","),
	}

	if *expired {
		// Set expiration to 1 hour ago
		claims["exp"] = time.Now().Add(-1 * time.Hour).Unix()
	} else {
		// Set expiration to 24 hours from now
		claims["exp"] = time.Now().Add(24 * time.Hour).Unix()
	}

	// Create token
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)

	// Sign token
	tokenString, err := token.SignedString(key)
	if err != nil {
		log.Fatalf("Failed to sign token: %v", err)
	}

	fmt.Println(tokenString)
}

func loadOrGeneratePrivateKey(path string) (*rsa.PrivateKey, error) {
	// Try to load existing key
	keyData, err := ioutil.ReadFile(path)
	if err == nil {
		// Parse existing key
		block, _ := pem.Decode(keyData)
		if block == nil {
			return nil, fmt.Errorf("failed to decode PEM block")
		}

		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}

		return key, nil
	}

	// Generate new key
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	// Save private key
	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}

	privateKeyFile, err := ioutil.TempFile("", "private-*.pem")
	if err != nil {
		return nil, fmt.Errorf("failed to create private key file: %w", err)
	}
	defer privateKeyFile.Close()

	if err := pem.Encode(privateKeyFile, privateKeyPEM); err != nil {
		return nil, fmt.Errorf("failed to write private key: %w", err)
	}

	// Save public key
	publicKeyPEM := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: x509.MarshalPKCS1PublicKey(&key.PublicKey),
	}

	publicKeyFile, err := ioutil.TempFile("", "public-*.pem")
	if err != nil {
		return nil, fmt.Errorf("failed to create public key file: %w", err)
	}
	defer publicKeyFile.Close()

	if err := pem.Encode(publicKeyFile, publicKeyPEM); err != nil {
		return nil, fmt.Errorf("failed to write public key: %w", err)
	}

	fmt.Printf("Generated new key pair:\n")
	fmt.Printf("Private key: %s\n", privateKeyFile.Name())
	fmt.Printf("Public key: %s\n", publicKeyFile.Name())

	return key, nil
}
