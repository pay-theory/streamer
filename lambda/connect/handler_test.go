package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/golang-jwt/jwt/v5"
	"github.com/pay-theory/streamer/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock connection store
type mockConnectionStore struct {
	mock.Mock
}

func (m *mockConnectionStore) Save(ctx context.Context, conn *store.Connection) error {
	args := m.Called(ctx, conn)
	return args.Error(0)
}

func (m *mockConnectionStore) Get(ctx context.Context, connectionID string) (*store.Connection, error) {
	args := m.Called(ctx, connectionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*store.Connection), args.Error(1)
}

func (m *mockConnectionStore) Delete(ctx context.Context, connectionID string) error {
	args := m.Called(ctx, connectionID)
	return args.Error(0)
}

func (m *mockConnectionStore) ListByUser(ctx context.Context, userID string) ([]*store.Connection, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*store.Connection), args.Error(1)
}

func (m *mockConnectionStore) ListByTenant(ctx context.Context, tenantID string) ([]*store.Connection, error) {
	args := m.Called(ctx, tenantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*store.Connection), args.Error(1)
}

func (m *mockConnectionStore) UpdateLastPing(ctx context.Context, connectionID string) error {
	args := m.Called(ctx, connectionID)
	return args.Error(0)
}

func (m *mockConnectionStore) DeleteStale(ctx context.Context, before time.Time) error {
	args := m.Called(ctx, before)
	return args.Error(0)
}

// Mock metrics publisher
type mockMetricsPublisher struct {
	mock.Mock
}

func (m *mockMetricsPublisher) PublishMetric(ctx context.Context, namespace, metricName string, value float64, unit types.StandardUnit, dimensions ...types.Dimension) error {
	args := m.Called(ctx, namespace, metricName, value, unit, dimensions)
	return args.Error(0)
}

func (m *mockMetricsPublisher) PublishLatency(ctx context.Context, namespace, metricName string, duration time.Duration, dimensions ...types.Dimension) error {
	args := m.Called(ctx, namespace, metricName, duration, dimensions)
	return args.Error(0)
}

// Test helper functions
func generateTestJWT(claims jwt.MapClaims, privateKey interface{}) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(privateKey)
}

func getTestPublicKey() string {
	return `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAu1SU1LfVLPHCozMxH2Mo
4lgOEePzNm0tRgeLezV6ffAt0gunVTLw7onLRnrq0/IzW7yWR7QkrmBL7jTKEn5u
+qKhbwKfBstIs+bMY2Zkp18gnTxKLxoS2tFczGkPLPgizskuemMghRniWaoLcyeh
kd3qqGElvW/VDL5AaWTg0nLVkjRo9z+40RQzuVaE8AkAFmxZzow3x+VJYKdjykkJ
0iT9wCS0DRTXu269V264Vf/3jvredZiKRkgwlL9xNAwxXFg0x/XFw005UWVRIkdg
cKWTjpBP2dPwVZ4WWC+9aGVd+Gyn1o0CLelf4rEjGoXbAAEgAqeGUxrcIlbjXfbc
mwIDAQAB
-----END PUBLIC KEY-----`
}

func getTestPrivateKey() interface{} {
	privateKeyPEM := `-----BEGIN RSA PRIVATE KEY-----
MIIEogIBAAKCAQEAu1SU1LfVLPHCozMxH2Mo4lgOEePzNm0tRgeLezV6ffAt0gun
VTLw7onLRnrq0/IzW7yWR7QkrmBL7jTKEn5u+qKhbwKfBstIs+bMY2Zkp18gnTxK
LxoS2tFczGkPLPgizskuemMghRniWaoLcyehkd3qqGElvW/VDL5AaWTg0nLVkjRo
9z+40RQzuVaE8AkAFmxZzow3x+VJYKdjykkJ0iT9wCS0DRTXu269V264Vf/3jvre
dZiKRkgwlL9xNAwxXFg0x/XFw005UWVRIkdgcKWTjpBP2dPwVZ4WWC+9aGVd+Gyn
1o0CLelf4rEjGoXbAAEgAqeGUxrcIlbjXfbcmwIDAQABAoIBACiARq2wkltjtcjs
kFvZ7w1JAORHbEufEO1Eu27zOIlqbgyAcAl7q+/1bip4Z/x1IVES84/yTaM8p0go
amMhvgry/mS8vNi1BN2SAZEnb/7xSxbflb70bX9RHLJqKnp5GZe2jexw+wyXlwaM
+bclUCrh9e1ltH7IvUrRrQnFJfh+is1fRon9Co9Li0GwoN0x0byrrngU8Ak3Y6D9
D8GjQA4Elm94ST3izJv8iCOLSDBmzsPsXfcCUZfmTfZ5DbUDMbMxRnSo3nQeoKGC
0Lj9FkWcfmLcpGlSXTO+Ww1L7EGq+PT3NtRae1FZPwjddQ1/4V905kyQFLamAA5Y
lSpE2wkCgYEAy1OPLQcZt4NQnQzPz2SBJqQN2P5u3vXl+zNVKP8w4eBv0vWuJJF+
hkGNnSxXQrTkvDOIUddSKOzHHgSg4nY6K02ecyT0PPm/UZvtRpWrnBjcEVtHEJNp
bU9pLD5iZ0J9sbzPU/LxPmuAP2Bs8JmTn6aFRspFrP7W0s1Nmk2jsm0CgYEA7Cum
yOTLjrYhqKjBqon7TZGhfNjo4QSGlulMEGpcQfKcE0g5DmcAVrGgMT8LvDgRVAKJ
uA6jZWJGpmj4yt/IIVs21PPrW3asp9LgqNeinoKBYpqOJXLcotn6eTuCMcbT8Lka
F8wGcWjod7PYYqBxDgdI5DhqCv5MAoZPLFe9SScCgYBGNjLjiL4Q/5j23RF1bNbq
aKKfEY1YV0R1F4AA87YU8V1XcN41d3F8eNoNn1VBK5UCpAHqkRlqL7SdBCwKDGPB
GTWGKXFZRiCr8gKNvn6xpVBtNpT0fAaZL9PoGZsqtRZvEnXlPdew7g4fOKEXS2VQ
nqze2krj/BtZtXsP7OvpQQKBgE7vU+QCqsyF5ppLNp5a7hpyzpNb3g7S1z0Acik8
/1IguGwHbPqPTb0c1JTQkrxKPn8hxCMVpaJ+IbTl96fqkAjSfFslL4tpPqVUh4eD
iU8kxEyy/7lhfIP1n7kAa5cMrOxsBgBhGAsNqKCEe8Y0SAevCvlwf0cxpkEJkzQO
9wBvAoGADKdJqgYT6EkuSacvP4x29a7Rqtf5lYTDmFPz7Is6fAQGGfNkjJSPOGKW
k5dDJbhhJJQJNx0LgZ5PJjbDUslnIPNrDvvlS6KbV/cz5hYYgV7A0Shkz5W6X8xf
jQb2xpOmjYgMKvCN3DJQqPR+YrQlI8LDGF9UkoqBTYvM6IfGSxE=
-----END RSA PRIVATE KEY-----`

	privateKey, _ := jwt.ParseRSAPrivateKeyFromPEM([]byte(privateKeyPEM))
	return privateKey
}

func TestHandler_Handle(t *testing.T) {
	tests := []struct {
		name           string
		event          events.APIGatewayWebsocketProxyRequest
		setupMocks     func(*mockConnectionStore)
		expectedStatus int
		expectedError  bool
		checkBody      func(t *testing.T, body string)
	}{
		{
			name: "successful connection with valid JWT",
			event: events.APIGatewayWebsocketProxyRequest{
				RequestContext: events.APIGatewayWebsocketProxyRequestContext{
					ConnectionID: "test-connection-123",
					DomainName:   "test.execute-api.us-east-1.amazonaws.com",
					Stage:        "prod",
					Identity: events.APIGatewayRequestIdentity{
						SourceIP: "192.168.1.1",
					},
				},
				QueryStringParameters: map[string]string{
					"Authorization": generateValidToken(t),
				},
				Headers: map[string]string{
					"User-Agent": "TestClient/1.0",
				},
			},
			setupMocks: func(store *mockConnectionStore) {
				store.On("Save", mock.Anything, mock.MatchedBy(func(conn *store.Connection) bool {
					return conn.ConnectionID == "test-connection-123" &&
						conn.UserID == "user-123" &&
						conn.TenantID == "tenant-456"
				})).Return(nil)
			},
			expectedStatus: 200,
			expectedError:  false,
			checkBody: func(t *testing.T, body string) {
				assert.Contains(t, body, "Connected successfully")
			},
		},
		{
			name: "missing authorization token",
			event: events.APIGatewayWebsocketProxyRequest{
				RequestContext: events.APIGatewayWebsocketProxyRequestContext{
					ConnectionID: "test-connection-123",
				},
				QueryStringParameters: map[string]string{},
			},
			setupMocks:     func(store *mockConnectionStore) {},
			expectedStatus: 401,
			expectedError:  false,
			checkBody: func(t *testing.T, body string) {
				assert.Contains(t, body, "Missing authorization token")
			},
		},
		{
			name: "expired JWT token",
			event: events.APIGatewayWebsocketProxyRequest{
				RequestContext: events.APIGatewayWebsocketProxyRequestContext{
					ConnectionID: "test-connection-123",
				},
				QueryStringParameters: map[string]string{
					"Authorization": generateExpiredToken(t),
				},
			},
			setupMocks:     func(store *mockConnectionStore) {},
			expectedStatus: 401,
			expectedError:  false,
			checkBody: func(t *testing.T, body string) {
				assert.Contains(t, body, "token has expired")
			},
		},
		{
			name: "invalid JWT signature",
			event: events.APIGatewayWebsocketProxyRequest{
				RequestContext: events.APIGatewayWebsocketProxyRequestContext{
					ConnectionID: "test-connection-123",
				},
				QueryStringParameters: map[string]string{
					"Authorization": "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ1c2VyLTEyMyIsInRlbmFudF9pZCI6InRlbmFudC00NTYiLCJleHAiOjk5OTk5OTk5OTl9.invalid-signature",
				},
			},
			setupMocks:     func(store *mockConnectionStore) {},
			expectedStatus: 401,
			expectedError:  false,
			checkBody: func(t *testing.T, body string) {
				assert.Contains(t, body, "Invalid token")
			},
		},
		{
			name: "DynamoDB save error",
			event: events.APIGatewayWebsocketProxyRequest{
				RequestContext: events.APIGatewayWebsocketProxyRequestContext{
					ConnectionID: "test-connection-123",
					DomainName:   "test.execute-api.us-east-1.amazonaws.com",
					Stage:        "prod",
				},
				QueryStringParameters: map[string]string{
					"Authorization": generateValidToken(t),
				},
			},
			setupMocks: func(store *mockConnectionStore) {
				store.On("Save", mock.Anything, mock.Anything).Return(errors.New("DynamoDB error"))
			},
			expectedStatus: 500,
			expectedError:  false,
			checkBody: func(t *testing.T, body string) {
				assert.Contains(t, body, "Failed to establish connection")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock store
			mockStore := new(mockConnectionStore)
			if tt.setupMocks != nil {
				tt.setupMocks(mockStore)
			}

			// Create handler with test configuration
			config := &HandlerConfig{
				TableName:    "test-table",
				JWTPublicKey: getTestPublicKey(),
				JWTIssuer:    "test-issuer",
			}

			// Create mock metrics publisher
			mockMetrics := new(mockMetricsPublisher)
			mockMetrics.On("PublishMetric", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

			handler := NewHandler(mockStore, config, mockMetrics)

			// Execute handler
			response, err := handler.Handle(context.Background(), tt.event)

			// Check error
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Check status code
			assert.Equal(t, tt.expectedStatus, response.StatusCode)

			// Check response body
			if tt.checkBody != nil {
				tt.checkBody(t, response.Body)
			}

			// Verify mock expectations
			mockStore.AssertExpectations(t)
		})
	}
}

func TestHandler_TenantRestrictions(t *testing.T) {
	mockStore := new(mockConnectionStore)

	// Create handler with tenant restrictions
	config := &HandlerConfig{
		TableName:      "test-table",
		JWTPublicKey:   getTestPublicKey(),
		JWTIssuer:      "test-issuer",
		AllowedTenants: []string{"allowed-tenant-1", "allowed-tenant-2"},
	}

	// Create mock metrics publisher
	mockMetrics := new(mockMetricsPublisher)
	mockMetrics.On("PublishMetric", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	handler := NewHandler(mockStore, config, mockMetrics)

	// Test allowed tenant
	event := events.APIGatewayWebsocketProxyRequest{
		RequestContext: events.APIGatewayWebsocketProxyRequestContext{
			ConnectionID: "test-connection-123",
		},
		QueryStringParameters: map[string]string{
			"Authorization": generateTokenWithTenant(t, "allowed-tenant-1"),
		},
	}

	mockStore.On("Save", mock.Anything, mock.Anything).Return(nil).Once()

	response, err := handler.Handle(context.Background(), event)
	assert.NoError(t, err)
	assert.Equal(t, 200, response.StatusCode)

	// Test disallowed tenant
	event.QueryStringParameters["Authorization"] = generateTokenWithTenant(t, "disallowed-tenant")

	response, err = handler.Handle(context.Background(), event)
	assert.NoError(t, err)
	assert.Equal(t, 401, response.StatusCode)
	assert.Contains(t, response.Body, "Tenant not allowed")

	mockStore.AssertExpectations(t)
}

// Helper functions to generate test tokens
func generateValidToken(t *testing.T) string {
	claims := jwt.MapClaims{
		"sub":         "user-123",
		"tenant_id":   "tenant-456",
		"permissions": []string{"read", "write"},
		"exp":         time.Now().Add(1 * time.Hour).Unix(),
		"iat":         time.Now().Unix(),
		"iss":         "test-issuer",
	}

	token, err := generateTestJWT(claims, getTestPrivateKey())
	assert.NoError(t, err)
	return token
}

func generateExpiredToken(t *testing.T) string {
	claims := jwt.MapClaims{
		"sub":       "user-123",
		"tenant_id": "tenant-456",
		"exp":       time.Now().Add(-1 * time.Hour).Unix(),
		"iat":       time.Now().Add(-2 * time.Hour).Unix(),
		"iss":       "test-issuer",
	}

	token, err := generateTestJWT(claims, getTestPrivateKey())
	assert.NoError(t, err)
	return token
}

func generateTokenWithTenant(t *testing.T, tenantID string) string {
	claims := jwt.MapClaims{
		"sub":       "user-123",
		"tenant_id": tenantID,
		"exp":       time.Now().Add(1 * time.Hour).Unix(),
		"iat":       time.Now().Unix(),
		"iss":       "test-issuer",
	}

	token, err := generateTestJWT(claims, getTestPrivateKey())
	assert.NoError(t, err)
	return token
}
