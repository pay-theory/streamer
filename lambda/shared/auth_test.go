package shared

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecret = "test-secret-key"

func TestValidateJWT(t *testing.T) {
	tests := []struct {
		name      string
		token     string
		secret    string
		setupFunc func() string
		wantErr   bool
		errMsg    string
		checkFunc func(t *testing.T, claims map[string]interface{})
	}{
		{
			name: "valid token",
			setupFunc: func() string {
				token, _ := GenerateJWT("user-123", "tenant-456", []string{"read", "write"}, testSecret, time.Hour)
				return token
			},
			secret:  testSecret,
			wantErr: false,
			checkFunc: func(t *testing.T, claims map[string]interface{}) {
				assert.Equal(t, "user-123", claims["user_id"])
				assert.Equal(t, "tenant-456", claims["tenant_id"])

				// Check permissions
				perms, ok := claims["permissions"].([]interface{})
				require.True(t, ok)
				assert.Len(t, perms, 2)
			},
		},
		{
			name: "valid token with Bearer prefix",
			setupFunc: func() string {
				token, _ := GenerateJWT("user-456", "tenant-789", nil, testSecret, time.Hour)
				return "Bearer " + token
			},
			secret:  testSecret,
			wantErr: false,
			checkFunc: func(t *testing.T, claims map[string]interface{}) {
				assert.Equal(t, "user-456", claims["user_id"])
				assert.Equal(t, "tenant-789", claims["tenant_id"])
			},
		},
		{
			name:    "invalid token format",
			token:   "invalid.token.format",
			secret:  testSecret,
			wantErr: true,
			errMsg:  "failed to parse token",
		},
		{
			name:    "empty token",
			token:   "",
			secret:  testSecret,
			wantErr: true,
			errMsg:  "failed to parse token",
		},
		{
			name: "expired token",
			setupFunc: func() string {
				token, _ := GenerateJWT("user-123", "tenant-456", nil, testSecret, -time.Hour)
				return token
			},
			secret:  testSecret,
			wantErr: true,
			errMsg:  "token is expired",
		},
		{
			name: "wrong secret",
			setupFunc: func() string {
				token, _ := GenerateJWT("user-123", "tenant-456", nil, testSecret, time.Hour)
				return token
			},
			secret:  "wrong-secret",
			wantErr: true,
			errMsg:  "failed to parse token",
		},
		{
			name: "missing user_id claim",
			setupFunc: func() string {
				claims := jwt.MapClaims{
					"tenant_id": "tenant-123",
					"exp":       time.Now().Add(time.Hour).Unix(),
				}
				token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
				tokenString, _ := token.SignedString([]byte(testSecret))
				return tokenString
			},
			secret:  testSecret,
			wantErr: true,
			errMsg:  "missing user_id claim",
		},
		{
			name: "missing tenant_id claim",
			setupFunc: func() string {
				claims := jwt.MapClaims{
					"user_id": "user-123",
					"exp":     time.Now().Add(time.Hour).Unix(),
				}
				token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
				tokenString, _ := token.SignedString([]byte(testSecret))
				return tokenString
			},
			secret:  testSecret,
			wantErr: true,
			errMsg:  "missing tenant_id claim",
		},
		{
			name: "missing expiration claim",
			setupFunc: func() string {
				claims := jwt.MapClaims{
					"user_id":   "user-123",
					"tenant_id": "tenant-456",
				}
				token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
				tokenString, _ := token.SignedString([]byte(testSecret))
				return tokenString
			},
			secret:  testSecret,
			wantErr: true,
			errMsg:  "missing expiration claim",
		},
		{
			name: "different signing algorithm",
			setupFunc: func() string {
				// Try to use RS256 instead of HS256
				claims := jwt.MapClaims{
					"user_id":   "user-123",
					"tenant_id": "tenant-456",
					"exp":       time.Now().Add(time.Hour).Unix(),
				}
				token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
				tokenString, _ := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
				return tokenString
			},
			secret:  testSecret,
			wantErr: true,
			errMsg:  "unexpected signing method",
		},
		{
			name:    "token with spaces",
			token:   "  Bearer token-with-spaces  ",
			secret:  testSecret,
			wantErr: true,
			errMsg:  "failed to parse token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := tt.token
			if tt.setupFunc != nil {
				token = tt.setupFunc()
			}

			claims, err := ValidateJWT(token, tt.secret)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				assert.Nil(t, claims)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, claims)
				if tt.checkFunc != nil {
					tt.checkFunc(t, claims)
				}
			}
		})
	}
}

func TestGenerateJWT(t *testing.T) {
	tests := []struct {
		name        string
		userID      string
		tenantID    string
		permissions []string
		secret      string
		duration    time.Duration
		wantErr     bool
	}{
		{
			name:        "valid token generation",
			userID:      "user-123",
			tenantID:    "tenant-456",
			permissions: []string{"read", "write", "delete"},
			secret:      testSecret,
			duration:    time.Hour,
			wantErr:     false,
		},
		{
			name:        "token with no permissions",
			userID:      "user-789",
			tenantID:    "tenant-012",
			permissions: nil,
			secret:      testSecret,
			duration:    30 * time.Minute,
			wantErr:     false,
		},
		{
			name:        "token with empty permissions",
			userID:      "user-999",
			tenantID:    "tenant-888",
			permissions: []string{},
			secret:      testSecret,
			duration:    24 * time.Hour,
			wantErr:     false,
		},
		{
			name:        "short duration token",
			userID:      "user-short",
			tenantID:    "tenant-short",
			permissions: []string{"read"},
			secret:      testSecret,
			duration:    1 * time.Minute,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := GenerateJWT(tt.userID, tt.tenantID, tt.permissions, tt.secret, tt.duration)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Empty(t, token)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, token)

				// Verify the generated token
				claims, err := ValidateJWT(token, tt.secret)
				require.NoError(t, err)

				assert.Equal(t, tt.userID, claims["user_id"])
				assert.Equal(t, tt.tenantID, claims["tenant_id"])

				// Check permissions if provided
				if tt.permissions != nil {
					perms, ok := claims["permissions"].([]interface{})
					if len(tt.permissions) > 0 {
						require.True(t, ok)
						assert.Len(t, perms, len(tt.permissions))
					}
				}

				// Check expiration
				exp, ok := claims["exp"].(float64)
				require.True(t, ok)
				expTime := time.Unix(int64(exp), 0)
				assert.True(t, expTime.After(time.Now()))
				assert.True(t, expTime.Before(time.Now().Add(tt.duration+time.Minute)))
			}
		})
	}
}

func TestJWTRoundTrip(t *testing.T) {
	// Test that we can generate and validate tokens consistently
	testCases := []struct {
		userID      string
		tenantID    string
		permissions []string
	}{
		{"user-1", "tenant-1", []string{"read"}},
		{"user-2", "tenant-2", []string{"read", "write"}},
		{"user-3", "tenant-3", []string{}},
		{"user-4", "tenant-4", nil},
	}

	for _, tc := range testCases {
		t.Run(tc.userID, func(t *testing.T) {
			// Generate token
			token, err := GenerateJWT(tc.userID, tc.tenantID, tc.permissions, testSecret, time.Hour)
			require.NoError(t, err)

			// Validate token
			claims, err := ValidateJWT(token, testSecret)
			require.NoError(t, err)

			// Verify claims
			assert.Equal(t, tc.userID, claims["user_id"])
			assert.Equal(t, tc.tenantID, claims["tenant_id"])
		})
	}
}

func TestJWTClaims(t *testing.T) {
	// Test the JWTClaims struct directly
	now := time.Now()
	claims := JWTClaims{
		UserID:      "test-user",
		TenantID:    "test-tenant",
		Permissions: []string{"admin", "user"},
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "test-issuer",
			Subject:   "test-subject",
			ID:        "test-id",
			Audience:  []string{"test-audience"},
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(testSecret))
	require.NoError(t, err)

	// Parse the token back
	parsedToken, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(testSecret), nil
	})
	require.NoError(t, err)
	require.True(t, parsedToken.Valid)

	// Verify custom claims are preserved
	parsedClaims, ok := parsedToken.Claims.(jwt.MapClaims)
	require.True(t, ok)

	assert.Equal(t, "test-user", parsedClaims["user_id"])
	assert.Equal(t, "test-tenant", parsedClaims["tenant_id"])
}
