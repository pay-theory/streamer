package connection

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi/types"
	"github.com/pay-theory/streamer/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockConnectionStore mocks the store.ConnectionStore interface
type MockConnectionStore struct {
	mock.Mock
}

func (m *MockConnectionStore) Save(ctx context.Context, conn *store.Connection) error {
	args := m.Called(ctx, conn)
	return args.Error(0)
}

func (m *MockConnectionStore) Get(ctx context.Context, connectionID string) (*store.Connection, error) {
	args := m.Called(ctx, connectionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*store.Connection), args.Error(1)
}

func (m *MockConnectionStore) Delete(ctx context.Context, connectionID string) error {
	args := m.Called(ctx, connectionID)
	return args.Error(0)
}

func (m *MockConnectionStore) ListByUser(ctx context.Context, userID string) ([]*store.Connection, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*store.Connection), args.Error(1)
}

func (m *MockConnectionStore) ListByTenant(ctx context.Context, tenantID string) ([]*store.Connection, error) {
	args := m.Called(ctx, tenantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*store.Connection), args.Error(1)
}

func (m *MockConnectionStore) UpdateLastPing(ctx context.Context, connectionID string) error {
	args := m.Called(ctx, connectionID)
	return args.Error(0)
}

func (m *MockConnectionStore) DeleteStale(ctx context.Context, before time.Time) error {
	args := m.Called(ctx, before)
	return args.Error(0)
}

// Note: All API Gateway mocks have been moved to mocks.go for centralization
// Use MockAPIGatewayClient or TestableAPIGatewayClient from mocks.go

// TestNewManager tests the Manager constructor
func TestNewManager(t *testing.T) {
	mockStore := new(MockConnectionStore)
	mockAPIGateway := NewMockAPIGatewayClient()
	endpoint := "wss://example.com"

	manager := NewManager(mockStore, mockAPIGateway, endpoint)

	assert.NotNil(t, manager)
	assert.Equal(t, endpoint, manager.endpoint)
	assert.NotNil(t, manager.workerPool)
	assert.NotNil(t, manager.circuitBreaker)
	assert.NotNil(t, manager.metrics)
	assert.NotNil(t, manager.shutdownCh)
}

// TestManager_Send tests the Send method
func TestManager_Send(t *testing.T) {
	tests := []struct {
		name          string
		connectionID  string
		message       interface{}
		setupMocks    func(*MockConnectionStore, *MockAPIGatewayClient)
		expectError   bool
		expectedError error
	}{
		{
			name:         "successful send",
			connectionID: "conn123",
			message:      map[string]string{"type": "test", "data": "hello"},
			setupMocks: func(mockStore *MockConnectionStore, api *MockAPIGatewayClient) {
				// Mock successful connection lookup
				mockStore.On("Get", mock.Anything, "conn123").Return(&store.Connection{
					ConnectionID: "conn123",
					UserID:       "user123",
					TenantID:     "tenant123",
					Endpoint:     "wss://example.com",
					LastPing:     time.Now(),
				}, nil)

				// Mock successful API Gateway send
				api.On("PostToConnection", mock.Anything, "conn123", mock.AnythingOfType("[]uint8")).Return(nil)

				// Mock successful UpdateLastPing (async)
				mockStore.On("UpdateLastPing", mock.Anything, "conn123").Return(nil).Maybe()
			},
			expectError: false,
		},
		{
			name:         "connection not found",
			connectionID: "conn123",
			message:      map[string]string{"type": "test"},
			setupMocks: func(mockStore *MockConnectionStore, api *MockAPIGatewayClient) {
				mockStore.On("Get", mock.Anything, "conn123").Return(nil, store.ErrNotFound)
			},
			expectError:   true,
			expectedError: ErrConnectionNotFound,
		},
		{
			name:         "marshal error with invalid message",
			connectionID: "conn123",
			message:      make(chan int), // channels cannot be marshaled to JSON
			setupMocks: func(mockStore *MockConnectionStore, api *MockAPIGatewayClient) {
				mockStore.On("Get", mock.Anything, "conn123").Return(&store.Connection{
					ConnectionID: "conn123",
					UserID:       "user123",
					TenantID:     "tenant123",
					Endpoint:     "wss://example.com",
					LastPing:     time.Now(),
				}, nil)
			},
			expectError: true,
		},
		{
			name:         "stale connection (410 Gone)",
			connectionID: "conn123",
			message:      map[string]string{"type": "test"},
			setupMocks: func(mockStore *MockConnectionStore, api *MockAPIGatewayClient) {
				mockStore.On("Get", mock.Anything, "conn123").Return(&store.Connection{
					ConnectionID: "conn123",
					UserID:       "user123",
					TenantID:     "tenant123",
					Endpoint:     "wss://example.com",
					LastPing:     time.Now(),
				}, nil)

				// Mock 410 Gone error
				api.On("PostToConnection", mock.Anything, "conn123", mock.AnythingOfType("[]uint8")).
					Return(&types.GoneException{Message: aws.String("Connection no longer exists")})

				// Should attempt to delete stale connection
				mockStore.On("Delete", mock.Anything, "conn123").Return(nil).Maybe()
			},
			expectError:   true,
			expectedError: ErrConnectionStale,
		},
		{
			name:         "network error with retry",
			connectionID: "conn123",
			message:      map[string]string{"type": "test"},
			setupMocks: func(mockStore *MockConnectionStore, api *MockAPIGatewayClient) {
				mockStore.On("Get", mock.Anything, "conn123").Return(&store.Connection{
					ConnectionID: "conn123",
					UserID:       "user123",
					TenantID:     "tenant123",
					Endpoint:     "wss://example.com",
					LastPing:     time.Now(),
				}, nil)

				// Mock network error (will retry 3 times)
				api.On("PostToConnection", mock.Anything, "conn123", mock.AnythingOfType("[]uint8")).
					Return(errors.New("network error")).Times(3)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := new(MockConnectionStore)
			mockAPIGateway := NewMockAPIGatewayClient()

			if tt.setupMocks != nil {
				tt.setupMocks(mockStore, mockAPIGateway)
			}

			manager := NewManager(mockStore, mockAPIGateway, "wss://example.com")
			// Set a custom logger to avoid output during tests
			manager.SetLogger(func(format string, args ...interface{}) {})

			err := manager.Send(context.Background(), tt.connectionID, tt.message)

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedError != nil {
					assert.True(t, errors.Is(err, tt.expectedError))
				}
			} else {
				assert.NoError(t, err)
			}

			// Allow time for async operations
			time.Sleep(10 * time.Millisecond)

			mockStore.AssertExpectations(t)
			mockAPIGateway.AssertExpectations(t)
		})
	}
}

// Test coverage has been improved with the comprehensive tests above
