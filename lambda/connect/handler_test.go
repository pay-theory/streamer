//go:build !lift
// +build !lift

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
	"github.com/pay-theory/streamer/lambda/shared"
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

// Mock JWT Verifier
type mockJWTVerifier struct {
	mock.Mock
}

func (m *mockJWTVerifier) Verify(token string) (*Claims, error) {
	args := m.Called(token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Claims), args.Error(1)
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

func TestHandler_Handle_Success(t *testing.T) {
	// Setup mocks
	mockStore := new(mockConnectionStore)
	mockMetrics := new(mockMetricsPublisher)
	mockVerifier := new(mockJWTVerifier)

	config := &HandlerConfig{
		JWTPublicKey: "test-key",
		JWTIssuer:    "test-issuer",
	}

	handler := NewHandlerWithVerifier(mockStore, config, mockMetrics, mockVerifier)

	// Create test event
	event := events.APIGatewayWebsocketProxyRequest{
		RequestContext: events.APIGatewayWebsocketProxyRequestContext{
			ConnectionID: "test-connection-123",
			DomainName:   "api.example.com",
			Stage:        "prod",
			Identity: events.APIGatewayRequestIdentity{
				SourceIP: "192.168.1.1",
			},
		},
		QueryStringParameters: map[string]string{
			"Authorization": "valid-token",
		},
		Headers: map[string]string{
			"User-Agent": "test-agent",
		},
	}

	// Setup mock expectations
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: "user123",
		},
		TenantID:    "tenant456",
		Permissions: []string{"read", "write"},
	}
	mockVerifier.On("Verify", "valid-token").Return(claims, nil)
	mockStore.On("Save", mock.Anything, mock.MatchedBy(func(conn *store.Connection) bool {
		return conn.ConnectionID == "test-connection-123" &&
			conn.UserID == "user123" &&
			conn.TenantID == "tenant456"
	})).Return(nil)
	mockMetrics.On("PublishMetric", mock.Anything, "", shared.CommonMetrics.ConnectionEstablished,
		float64(1), types.StandardUnitCount, mock.Anything).Return(nil)
	mockMetrics.On("PublishLatency", mock.Anything, "", "ProcessingLatency",
		mock.AnythingOfType("time.Duration"), mock.Anything).Return(nil)

	// Execute
	response, err := handler.Handle(context.Background(), event)

	// Verify
	assert.NoError(t, err)
	assert.Equal(t, 200, response.StatusCode)
	assert.Contains(t, response.Body, "Connected successfully")

	mockStore.AssertExpectations(t)
	mockMetrics.AssertExpectations(t)
	mockVerifier.AssertExpectations(t)
}

func TestHandler_Handle_MissingToken(t *testing.T) {
	mockStore := new(mockConnectionStore)
	mockMetrics := new(mockMetricsPublisher)
	mockVerifier := new(mockJWTVerifier)

	config := &HandlerConfig{
		JWTPublicKey: "test-key",
		JWTIssuer:    "test-issuer",
	}

	handler := NewHandlerWithVerifier(mockStore, config, mockMetrics, mockVerifier)

	event := events.APIGatewayWebsocketProxyRequest{
		RequestContext: events.APIGatewayWebsocketProxyRequestContext{
			ConnectionID: "test-connection-123",
			Identity: events.APIGatewayRequestIdentity{
				SourceIP: "192.168.1.1",
			},
		},
		QueryStringParameters: map[string]string{},
		Headers:               map[string]string{},
	}

	mockMetrics.On("PublishMetric", mock.Anything, "", shared.CommonMetrics.AuthenticationFailed,
		float64(1), types.StandardUnitCount, mock.Anything).Return(nil)

	response, err := handler.Handle(context.Background(), event)

	assert.NoError(t, err)
	assert.Equal(t, 401, response.StatusCode)
	assert.Contains(t, response.Body, "Missing authorization token")

	mockMetrics.AssertExpectations(t)
}

func TestHandler_Handle_InvalidToken(t *testing.T) {
	mockStore := new(mockConnectionStore)
	mockMetrics := new(mockMetricsPublisher)
	mockVerifier := new(mockJWTVerifier)

	config := &HandlerConfig{
		JWTPublicKey: "test-key",
		JWTIssuer:    "test-issuer",
	}

	handler := NewHandlerWithVerifier(mockStore, config, mockMetrics, mockVerifier)

	event := events.APIGatewayWebsocketProxyRequest{
		RequestContext: events.APIGatewayWebsocketProxyRequestContext{
			ConnectionID: "test-connection-123",
			Identity: events.APIGatewayRequestIdentity{
				SourceIP: "192.168.1.1",
			},
		},
		QueryStringParameters: map[string]string{
			"Authorization": "invalid-token",
		},
		Headers: map[string]string{},
	}

	mockVerifier.On("Verify", "invalid-token").Return(nil, errors.New("invalid signature"))
	mockMetrics.On("PublishMetric", mock.Anything, "", shared.CommonMetrics.AuthenticationFailed,
		float64(1), types.StandardUnitCount, mock.Anything).Return(nil)

	response, err := handler.Handle(context.Background(), event)

	assert.NoError(t, err)
	assert.Equal(t, 401, response.StatusCode)
	assert.Contains(t, response.Body, "Invalid token")

	mockVerifier.AssertExpectations(t)
	mockMetrics.AssertExpectations(t)
}

func TestHandler_Handle_TenantNotAllowed(t *testing.T) {
	mockStore := new(mockConnectionStore)
	mockMetrics := new(mockMetricsPublisher)
	mockVerifier := new(mockJWTVerifier)

	config := &HandlerConfig{
		JWTPublicKey:   "test-key",
		JWTIssuer:      "test-issuer",
		AllowedTenants: []string{"allowed-tenant"},
	}

	handler := NewHandlerWithVerifier(mockStore, config, mockMetrics, mockVerifier)

	event := events.APIGatewayWebsocketProxyRequest{
		RequestContext: events.APIGatewayWebsocketProxyRequestContext{
			ConnectionID: "test-connection-123",
			Identity: events.APIGatewayRequestIdentity{
				SourceIP: "192.168.1.1",
			},
		},
		QueryStringParameters: map[string]string{
			"Authorization": "valid-token",
		},
		Headers: map[string]string{},
	}

	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: "user123",
		},
		TenantID:    "not-allowed-tenant",
		Permissions: []string{"read"},
	}
	mockVerifier.On("Verify", "valid-token").Return(claims, nil)
	mockMetrics.On("PublishMetric", mock.Anything, "", shared.CommonMetrics.AuthenticationFailed,
		float64(1), types.StandardUnitCount, mock.Anything).Return(nil)

	response, err := handler.Handle(context.Background(), event)

	assert.NoError(t, err)
	assert.Equal(t, 401, response.StatusCode)
	assert.Contains(t, response.Body, "Tenant not allowed")

	mockVerifier.AssertExpectations(t)
	mockMetrics.AssertExpectations(t)
}

func TestHandler_Handle_StorageError(t *testing.T) {
	mockStore := new(mockConnectionStore)
	mockMetrics := new(mockMetricsPublisher)
	mockVerifier := new(mockJWTVerifier)

	config := &HandlerConfig{
		JWTPublicKey: "test-key",
		JWTIssuer:    "test-issuer",
	}

	handler := NewHandlerWithVerifier(mockStore, config, mockMetrics, mockVerifier)

	event := events.APIGatewayWebsocketProxyRequest{
		RequestContext: events.APIGatewayWebsocketProxyRequestContext{
			ConnectionID: "test-connection-123",
			DomainName:   "api.example.com",
			Stage:        "prod",
			Identity: events.APIGatewayRequestIdentity{
				SourceIP: "192.168.1.1",
			},
		},
		QueryStringParameters: map[string]string{
			"Authorization": "valid-token",
		},
		Headers: map[string]string{},
	}

	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: "user123",
		},
		TenantID:    "tenant456",
		Permissions: []string{"read"},
	}
	mockVerifier.On("Verify", "valid-token").Return(claims, nil)
	mockStore.On("Save", mock.Anything, mock.Anything).Return(errors.New("DynamoDB error"))

	response, err := handler.Handle(context.Background(), event)

	assert.NoError(t, err)
	assert.Equal(t, 500, response.StatusCode)
	assert.Contains(t, response.Body, "Failed to establish connection")

	mockVerifier.AssertExpectations(t)
	mockStore.AssertExpectations(t)
}

func TestHandler_Handle_TokenFromHeader(t *testing.T) {
	mockStore := new(mockConnectionStore)
	mockMetrics := new(mockMetricsPublisher)
	mockVerifier := new(mockJWTVerifier)

	config := &HandlerConfig{
		JWTPublicKey: "test-key",
		JWTIssuer:    "test-issuer",
	}

	handler := NewHandlerWithVerifier(mockStore, config, mockMetrics, mockVerifier)

	event := events.APIGatewayWebsocketProxyRequest{
		RequestContext: events.APIGatewayWebsocketProxyRequestContext{
			ConnectionID: "test-connection-123",
			DomainName:   "api.example.com",
			Stage:        "prod",
			Identity: events.APIGatewayRequestIdentity{
				SourceIP: "192.168.1.1",
			},
		},
		QueryStringParameters: map[string]string{},
		Headers: map[string]string{
			"Authorization": "Bearer header-token",
		},
	}

	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: "user123",
		},
		TenantID:    "tenant456",
		Permissions: []string{"read"},
	}
	mockVerifier.On("Verify", "Bearer header-token").Return(claims, nil)
	mockStore.On("Save", mock.Anything, mock.Anything).Return(nil)
	mockMetrics.On("PublishMetric", mock.Anything, "", shared.CommonMetrics.ConnectionEstablished,
		float64(1), types.StandardUnitCount, mock.Anything).Return(nil)
	mockMetrics.On("PublishLatency", mock.Anything, "", "ProcessingLatency",
		mock.AnythingOfType("time.Duration"), mock.Anything).Return(nil)

	response, err := handler.Handle(context.Background(), event)

	assert.NoError(t, err)
	assert.Equal(t, 200, response.StatusCode)

	mockVerifier.AssertExpectations(t)
	mockStore.AssertExpectations(t)
	mockMetrics.AssertExpectations(t)
}

func TestHandler_Handle_EmptyTenantList(t *testing.T) {
	mockStore := new(mockConnectionStore)
	mockMetrics := new(mockMetricsPublisher)
	mockVerifier := new(mockJWTVerifier)

	config := &HandlerConfig{
		JWTPublicKey:   "test-key",
		JWTIssuer:      "test-issuer",
		AllowedTenants: []string{}, // Empty list should allow all tenants
	}

	handler := NewHandlerWithVerifier(mockStore, config, mockMetrics, mockVerifier)

	event := events.APIGatewayWebsocketProxyRequest{
		RequestContext: events.APIGatewayWebsocketProxyRequestContext{
			ConnectionID: "test-connection-123",
			DomainName:   "api.example.com",
			Stage:        "prod",
			Identity: events.APIGatewayRequestIdentity{
				SourceIP: "192.168.1.1",
			},
		},
		QueryStringParameters: map[string]string{
			"Authorization": "valid-token",
		},
		Headers: map[string]string{},
	}

	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: "user123",
		},
		TenantID:    "any-tenant",
		Permissions: []string{"read"},
	}
	mockVerifier.On("Verify", "valid-token").Return(claims, nil)
	mockStore.On("Save", mock.Anything, mock.Anything).Return(nil)
	mockMetrics.On("PublishMetric", mock.Anything, "", shared.CommonMetrics.ConnectionEstablished,
		float64(1), types.StandardUnitCount, mock.Anything).Return(nil)
	mockMetrics.On("PublishLatency", mock.Anything, "", "ProcessingLatency",
		mock.AnythingOfType("time.Duration"), mock.Anything).Return(nil)

	response, err := handler.Handle(context.Background(), event)

	assert.NoError(t, err)
	assert.Equal(t, 200, response.StatusCode)

	mockVerifier.AssertExpectations(t)
	mockStore.AssertExpectations(t)
	mockMetrics.AssertExpectations(t)
}

func TestNewHandler(t *testing.T) {
	mockStore := new(mockConnectionStore)
	mockMetrics := new(mockMetricsPublisher)

	// Generate a valid RSA key pair for testing
	_, validPublicKey := generateTestKeyPair(t)

	config := &HandlerConfig{
		JWTPublicKey: validPublicKey,
		JWTIssuer:    "test-issuer",
	}

	handler := NewHandler(mockStore, config, mockMetrics)

	assert.NotNil(t, handler)
	assert.Equal(t, mockStore, handler.store)
	assert.Equal(t, config, handler.config)
	assert.Equal(t, mockMetrics, handler.metrics)
	assert.NotNil(t, handler.jwtVerifier)
	assert.NotNil(t, handler.logger)
}

func TestJsonStringify(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "string slice",
			input:    []string{"read", "write", "delete"},
			expected: `["read","write","delete"]`,
		},
		{
			name:     "empty slice",
			input:    []string{},
			expected: `[]`,
		},
		{
			name:     "nil input",
			input:    nil,
			expected: `null`,
		},
		{
			name:     "map input",
			input:    map[string]string{"key": "value"},
			expected: `{"key":"value"}`,
		},
		{
			name:     "unmarshalable input",
			input:    make(chan int),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := jsonStringify(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUnauthorizedResponse(t *testing.T) {
	response, err := unauthorizedResponse("Test error message")

	assert.NoError(t, err)
	assert.Equal(t, 401, response.StatusCode)
	assert.Equal(t, "application/json", response.Headers["Content-Type"])
	assert.Contains(t, response.Body, "Test error message")
	assert.Contains(t, response.Body, "UNAUTHORIZED")
}

func TestInternalErrorResponse(t *testing.T) {
	response, err := internalErrorResponse("Test internal error")

	assert.NoError(t, err)
	assert.Equal(t, 500, response.StatusCode)
	assert.Equal(t, "application/json", response.Headers["Content-Type"])
	assert.Contains(t, response.Body, "Test internal error")
	assert.Contains(t, response.Body, "INTERNAL_ERROR")
}
