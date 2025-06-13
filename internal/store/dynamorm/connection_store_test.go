package dynamorm_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pay-theory/dynamorm/pkg/mocks"
	"github.com/pay-theory/streamer/internal/store"
	"github.com/pay-theory/streamer/internal/store/dynamorm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestNewConnectionStore tests the constructor
func TestNewConnectionStore(t *testing.T) {
	mockDB := new(mocks.MockDB)
	connStore := dynamorm.NewConnectionStore(mockDB)

	assert.NotNil(t, connStore)
	assert.Implements(t, (*store.ConnectionStore)(nil), connStore)
}

// TestConnectionStore_Save tests the Save method
func TestConnectionStore_Save(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		conn      *store.Connection
		setupMock func(*mocks.MockDB, *mocks.MockQuery)
		wantErr   bool
		errMsg    string
	}{
		{
			name: "successful save",
			conn: &store.Connection{
				ConnectionID: "conn123",
				UserID:       "user123",
				TenantID:     "tenant123",
				Endpoint:     "wss://example.com/ws",
				ConnectedAt:  time.Now(),
				LastPing:     time.Now(),
				Metadata:     map[string]string{"client": "web"},
			},
			setupMock: func(mockDB *mocks.MockDB, mockQuery *mocks.MockQuery) {
				mockDB.On("Model", mock.AnythingOfType("*dynamorm.Connection")).Return(mockQuery)
				mockQuery.On("Create").Return(nil)
			},
			wantErr: false,
		},
		{
			name: "save with auto-generated TTL",
			conn: &store.Connection{
				ConnectionID: "conn123",
				UserID:       "user123",
				TenantID:     "tenant123",
				Endpoint:     "wss://example.com/ws",
				ConnectedAt:  time.Now(),
				LastPing:     time.Now(),
				TTL:          0, // Should be auto-set
			},
			setupMock: func(mockDB *mocks.MockDB, mockQuery *mocks.MockQuery) {
				mockDB.On("Model", mock.AnythingOfType("*dynamorm.Connection")).Return(mockQuery)
				mockQuery.On("Create").Return(nil)
			},
			wantErr: false,
		},
		{
			name: "nil connection",
			conn: nil,
			setupMock: func(mockDB *mocks.MockDB, mockQuery *mocks.MockQuery) {
				// No mock expectations
			},
			wantErr: true,
			errMsg:  "cannot be nil",
		},
		{
			name: "empty connection ID",
			conn: &store.Connection{
				UserID:   "user123",
				TenantID: "tenant123",
				Endpoint: "wss://example.com/ws",
			},
			setupMock: func(mockDB *mocks.MockDB, mockQuery *mocks.MockQuery) {
				// No mock expectations
			},
			wantErr: true,
			errMsg:  "ConnectionID",
		},
		{
			name: "empty user ID",
			conn: &store.Connection{
				ConnectionID: "conn123",
				TenantID:     "tenant123",
				Endpoint:     "wss://example.com/ws",
			},
			setupMock: func(mockDB *mocks.MockDB, mockQuery *mocks.MockQuery) {
				// No mock expectations
			},
			wantErr: true,
			errMsg:  "UserID",
		},
		{
			name: "empty tenant ID",
			conn: &store.Connection{
				ConnectionID: "conn123",
				UserID:       "user123",
				Endpoint:     "wss://example.com/ws",
			},
			setupMock: func(mockDB *mocks.MockDB, mockQuery *mocks.MockQuery) {
				// No mock expectations
			},
			wantErr: true,
			errMsg:  "TenantID",
		},
		{
			name: "empty endpoint",
			conn: &store.Connection{
				ConnectionID: "conn123",
				UserID:       "user123",
				TenantID:     "tenant123",
			},
			setupMock: func(mockDB *mocks.MockDB, mockQuery *mocks.MockQuery) {
				// No mock expectations
			},
			wantErr: true,
			errMsg:  "Endpoint",
		},
		{
			name: "dynamorm create error",
			conn: &store.Connection{
				ConnectionID: "conn123",
				UserID:       "user123",
				TenantID:     "tenant123",
				Endpoint:     "wss://example.com/ws",
			},
			setupMock: func(mockDB *mocks.MockDB, mockQuery *mocks.MockQuery) {
				mockDB.On("Model", mock.AnythingOfType("*dynamorm.Connection")).Return(mockQuery)
				mockQuery.On("Create").Return(errors.New("dynamodb error"))
			},
			wantErr: true,
			errMsg:  "failed to save connection",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB := new(mocks.MockDB)
			mockQuery := new(mocks.MockQuery)

			if tt.setupMock != nil {
				tt.setupMock(mockDB, mockQuery)
			}

			connStore := dynamorm.NewConnectionStore(mockDB)
			err := connStore.Save(ctx, tt.conn)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
				// Verify TTL was set if it was 0
				if tt.conn != nil && tt.conn.TTL != 0 {
					assert.NotZero(t, tt.conn.TTL)
				}
			}

			mockDB.AssertExpectations(t)
			mockQuery.AssertExpectations(t)
		})
	}
}

// TestConnectionStore_Get tests the Get method
func TestConnectionStore_Get(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		connectionID string
		setupMock    func(*mocks.MockDB, *mocks.MockQuery)
		want         *store.Connection
		wantErr      bool
		errMsg       string
	}{
		{
			name:         "successful get",
			connectionID: "conn123",
			setupMock: func(mockDB *mocks.MockDB, mockQuery *mocks.MockQuery) {
				expectedConn := &dynamorm.Connection{
					ConnectionID: "conn123",
					UserID:       "user123",
					TenantID:     "tenant123",
					Endpoint:     "wss://example.com/ws",
					ConnectedAt:  time.Now(),
					LastPing:     time.Now(),
				}
				expectedConn.SetKeys()

				mockDB.On("Model", mock.AnythingOfType("*dynamorm.Connection")).Return(mockQuery)
				mockQuery.On("Where", "pk", "=", expectedConn.PK).Return(mockQuery)
				mockQuery.On("Where", "sk", "=", expectedConn.SK).Return(mockQuery)
				mockQuery.On("First", mock.AnythingOfType("*dynamorm.Connection")).
					Run(func(args mock.Arguments) {
						dest := args.Get(0).(*dynamorm.Connection)
						*dest = *expectedConn
					}).Return(nil)
			},
			want: &store.Connection{
				ConnectionID: "conn123",
				UserID:       "user123",
				TenantID:     "tenant123",
				Endpoint:     "wss://example.com/ws",
			},
			wantErr: false,
		},
		{
			name:         "empty connection ID",
			connectionID: "",
			setupMock: func(mockDB *mocks.MockDB, mockQuery *mocks.MockQuery) {
				// No mock expectations
			},
			wantErr: true,
			errMsg:  "cannot be empty",
		},
		{
			name:         "not found",
			connectionID: "conn123",
			setupMock: func(mockDB *mocks.MockDB, mockQuery *mocks.MockQuery) {
				conn := &dynamorm.Connection{ConnectionID: "conn123"}
				conn.SetKeys()

				mockDB.On("Model", mock.AnythingOfType("*dynamorm.Connection")).Return(mockQuery)
				mockQuery.On("Where", "pk", "=", conn.PK).Return(mockQuery)
				mockQuery.On("Where", "sk", "=", conn.SK).Return(mockQuery)
				mockQuery.On("First", mock.AnythingOfType("*dynamorm.Connection")).Return(errors.New("item not found"))
			},
			wantErr: true,
			errMsg:  "item not found",
		},
		{
			name:         "dynamodb error",
			connectionID: "conn123",
			setupMock: func(mockDB *mocks.MockDB, mockQuery *mocks.MockQuery) {
				conn := &dynamorm.Connection{ConnectionID: "conn123"}
				conn.SetKeys()

				mockDB.On("Model", mock.AnythingOfType("*dynamorm.Connection")).Return(mockQuery)
				mockQuery.On("Where", "pk", "=", conn.PK).Return(mockQuery)
				mockQuery.On("Where", "sk", "=", conn.SK).Return(mockQuery)
				mockQuery.On("First", mock.AnythingOfType("*dynamorm.Connection")).Return(errors.New("dynamodb error"))
			},
			wantErr: true,
			errMsg:  "failed to get connection",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB := new(mocks.MockDB)
			mockQuery := new(mocks.MockQuery)

			if tt.setupMock != nil {
				tt.setupMock(mockDB, mockQuery)
			}

			connStore := dynamorm.NewConnectionStore(mockDB)
			got, err := connStore.Get(ctx, tt.connectionID)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, got)
				assert.Equal(t, tt.want.ConnectionID, got.ConnectionID)
				assert.Equal(t, tt.want.UserID, got.UserID)
				assert.Equal(t, tt.want.TenantID, got.TenantID)
			}

			mockDB.AssertExpectations(t)
			mockQuery.AssertExpectations(t)
		})
	}
}

// TestConnectionStore_Delete tests the Delete method
func TestConnectionStore_Delete(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		connectionID string
		setupMock    func(*mocks.MockDB, *mocks.MockQuery)
		wantErr      bool
		errMsg       string
	}{
		{
			name:         "successful delete",
			connectionID: "conn123",
			setupMock: func(mockDB *mocks.MockDB, mockQuery *mocks.MockQuery) {
				mockDB.On("Model", mock.AnythingOfType("*dynamorm.Connection")).Return(mockQuery)
				mockQuery.On("Delete").Return(nil)
			},
			wantErr: false,
		},
		{
			name:         "empty connection ID",
			connectionID: "",
			setupMock: func(mockDB *mocks.MockDB, mockQuery *mocks.MockQuery) {
				// No mock expectations
			},
			wantErr: true,
			errMsg:  "cannot be empty",
		},
		{
			name:         "dynamodb error",
			connectionID: "conn123",
			setupMock: func(mockDB *mocks.MockDB, mockQuery *mocks.MockQuery) {
				mockDB.On("Model", mock.AnythingOfType("*dynamorm.Connection")).Return(mockQuery)
				mockQuery.On("Delete").Return(errors.New("dynamodb error"))
			},
			wantErr: true,
			errMsg:  "failed to delete connection",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB := new(mocks.MockDB)
			mockQuery := new(mocks.MockQuery)

			if tt.setupMock != nil {
				tt.setupMock(mockDB, mockQuery)
			}

			connStore := dynamorm.NewConnectionStore(mockDB)
			err := connStore.Delete(ctx, tt.connectionID)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}

			mockDB.AssertExpectations(t)
			mockQuery.AssertExpectations(t)
		})
	}
}

// TestConnectionStore_ListByUser tests the ListByUser method
func TestConnectionStore_ListByUser(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		userID    string
		setupMock func(*mocks.MockDB, *mocks.MockQuery)
		want      int // number of connections expected
		wantErr   bool
		errMsg    string
	}{
		{
			name:   "successful list",
			userID: "user123",
			setupMock: func(mockDB *mocks.MockDB, mockQuery *mocks.MockQuery) {
				expectedConnections := []dynamorm.Connection{
					{
						ConnectionID: "conn1",
						UserID:       "user123",
						TenantID:     "tenant123",
					},
					{
						ConnectionID: "conn2",
						UserID:       "user123",
						TenantID:     "tenant456",
					},
				}

				mockDB.On("Model", &dynamorm.Connection{}).Return(mockQuery)
				mockQuery.On("Index", "user-index").Return(mockQuery)
				mockQuery.On("Where", "user_id", "=", "user123").Return(mockQuery)
				mockQuery.On("All", mock.AnythingOfType("*[]dynamorm.Connection")).
					Run(func(args mock.Arguments) {
						dest := args.Get(0).(*[]dynamorm.Connection)
						*dest = expectedConnections
					}).Return(nil)
			},
			want:    2,
			wantErr: false,
		},
		{
			name:   "empty user ID",
			userID: "",
			setupMock: func(mockDB *mocks.MockDB, mockQuery *mocks.MockQuery) {
				// No mock expectations
			},
			wantErr: true,
			errMsg:  "cannot be empty",
		},
		{
			name:   "dynamodb error",
			userID: "user123",
			setupMock: func(mockDB *mocks.MockDB, mockQuery *mocks.MockQuery) {
				mockDB.On("Model", &dynamorm.Connection{}).Return(mockQuery)
				mockQuery.On("Index", "user-index").Return(mockQuery)
				mockQuery.On("Where", "user_id", "=", "user123").Return(mockQuery)
				mockQuery.On("All", mock.AnythingOfType("*[]dynamorm.Connection")).Return(errors.New("dynamodb error"))
			},
			wantErr: true,
			errMsg:  "failed to list connections by user",
		},
		{
			name:   "no connections found",
			userID: "user123",
			setupMock: func(mockDB *mocks.MockDB, mockQuery *mocks.MockQuery) {
				mockDB.On("Model", &dynamorm.Connection{}).Return(mockQuery)
				mockQuery.On("Index", "user-index").Return(mockQuery)
				mockQuery.On("Where", "user_id", "=", "user123").Return(mockQuery)
				mockQuery.On("All", mock.AnythingOfType("*[]dynamorm.Connection")).
					Run(func(args mock.Arguments) {
						dest := args.Get(0).(*[]dynamorm.Connection)
						*dest = []dynamorm.Connection{}
					}).Return(nil)
			},
			want:    0,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB := new(mocks.MockDB)
			mockQuery := new(mocks.MockQuery)

			if tt.setupMock != nil {
				tt.setupMock(mockDB, mockQuery)
			}

			connStore := dynamorm.NewConnectionStore(mockDB)
			got, err := connStore.ListByUser(ctx, tt.userID)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.Len(t, got, tt.want)
			}

			mockDB.AssertExpectations(t)
			mockQuery.AssertExpectations(t)
		})
	}
}

// TestConnectionStore_ListByTenant tests the ListByTenant method
func TestConnectionStore_ListByTenant(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		tenantID  string
		setupMock func(*mocks.MockDB, *mocks.MockQuery)
		want      int // number of connections expected
		wantErr   bool
		errMsg    string
	}{
		{
			name:     "successful list",
			tenantID: "tenant123",
			setupMock: func(mockDB *mocks.MockDB, mockQuery *mocks.MockQuery) {
				expectedConnections := []dynamorm.Connection{
					{
						ConnectionID: "conn1",
						UserID:       "user1",
						TenantID:     "tenant123",
					},
					{
						ConnectionID: "conn2",
						UserID:       "user2",
						TenantID:     "tenant123",
					},
				}

				mockDB.On("Model", &dynamorm.Connection{}).Return(mockQuery)
				mockQuery.On("Index", "tenant-index").Return(mockQuery)
				mockQuery.On("Where", "tenant_id", "=", "tenant123").Return(mockQuery)
				mockQuery.On("All", mock.AnythingOfType("*[]dynamorm.Connection")).
					Run(func(args mock.Arguments) {
						dest := args.Get(0).(*[]dynamorm.Connection)
						*dest = expectedConnections
					}).Return(nil)
			},
			want:    2,
			wantErr: false,
		},
		{
			name:     "empty tenant ID",
			tenantID: "",
			setupMock: func(mockDB *mocks.MockDB, mockQuery *mocks.MockQuery) {
				// No mock expectations
			},
			wantErr: true,
			errMsg:  "cannot be empty",
		},
		{
			name:     "dynamodb error",
			tenantID: "tenant123",
			setupMock: func(mockDB *mocks.MockDB, mockQuery *mocks.MockQuery) {
				mockDB.On("Model", &dynamorm.Connection{}).Return(mockQuery)
				mockQuery.On("Index", "tenant-index").Return(mockQuery)
				mockQuery.On("Where", "tenant_id", "=", "tenant123").Return(mockQuery)
				mockQuery.On("All", mock.AnythingOfType("*[]dynamorm.Connection")).Return(errors.New("dynamodb error"))
			},
			wantErr: true,
			errMsg:  "failed to list connections by tenant",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB := new(mocks.MockDB)
			mockQuery := new(mocks.MockQuery)

			if tt.setupMock != nil {
				tt.setupMock(mockDB, mockQuery)
			}

			connStore := dynamorm.NewConnectionStore(mockDB)
			got, err := connStore.ListByTenant(ctx, tt.tenantID)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.Len(t, got, tt.want)
			}

			mockDB.AssertExpectations(t)
			mockQuery.AssertExpectations(t)
		})
	}
}

// TestConnectionStore_UpdateLastPing tests the UpdateLastPing method
func TestConnectionStore_UpdateLastPing(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		connectionID string
		setupMock    func(*mocks.MockDB, *mocks.MockQuery, *mocks.MockUpdateBuilder)
		wantErr      bool
		errMsg       string
	}{
		{
			name:         "successful update",
			connectionID: "conn123",
			setupMock: func(mockDB *mocks.MockDB, mockQuery *mocks.MockQuery, mockUpdateBuilder *mocks.MockUpdateBuilder) {
				mockDB.On("Model", mock.AnythingOfType("*dynamorm.Connection")).Return(mockQuery)
				mockQuery.On("UpdateBuilder").Return(mockUpdateBuilder)
				mockUpdateBuilder.On("Set", "last_ping", mock.AnythingOfType("time.Time")).Return(mockUpdateBuilder)
				mockUpdateBuilder.On("Set", "ttl", mock.AnythingOfType("int64")).Return(mockUpdateBuilder)
				mockUpdateBuilder.On("Execute").Return(nil)
			},
			wantErr: false,
		},
		{
			name:         "empty connection ID",
			connectionID: "",
			setupMock: func(mockDB *mocks.MockDB, mockQuery *mocks.MockQuery, mockUpdateBuilder *mocks.MockUpdateBuilder) {
				// No mock expectations
			},
			wantErr: true,
			errMsg:  "cannot be empty",
		},
		{
			name:         "not found",
			connectionID: "conn123",
			setupMock: func(mockDB *mocks.MockDB, mockQuery *mocks.MockQuery, mockUpdateBuilder *mocks.MockUpdateBuilder) {
				mockDB.On("Model", mock.AnythingOfType("*dynamorm.Connection")).Return(mockQuery)
				mockQuery.On("UpdateBuilder").Return(mockUpdateBuilder)
				mockUpdateBuilder.On("Set", "last_ping", mock.AnythingOfType("time.Time")).Return(mockUpdateBuilder)
				mockUpdateBuilder.On("Set", "ttl", mock.AnythingOfType("int64")).Return(mockUpdateBuilder)
				mockUpdateBuilder.On("Execute").Return(errors.New("item not found"))
			},
			wantErr: true,
			errMsg:  "item not found",
		},
		{
			name:         "dynamodb error",
			connectionID: "conn123",
			setupMock: func(mockDB *mocks.MockDB, mockQuery *mocks.MockQuery, mockUpdateBuilder *mocks.MockUpdateBuilder) {
				mockDB.On("Model", mock.AnythingOfType("*dynamorm.Connection")).Return(mockQuery)
				mockQuery.On("UpdateBuilder").Return(mockUpdateBuilder)
				mockUpdateBuilder.On("Set", "last_ping", mock.AnythingOfType("time.Time")).Return(mockUpdateBuilder)
				mockUpdateBuilder.On("Set", "ttl", mock.AnythingOfType("int64")).Return(mockUpdateBuilder)
				mockUpdateBuilder.On("Execute").Return(errors.New("dynamodb error"))
			},
			wantErr: true,
			errMsg:  "failed to update last ping",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB := new(mocks.MockDB)
			mockQuery := new(mocks.MockQuery)
			mockUpdateBuilder := new(mocks.MockUpdateBuilder)

			if tt.setupMock != nil {
				tt.setupMock(mockDB, mockQuery, mockUpdateBuilder)
			}

			connStore := dynamorm.NewConnectionStore(mockDB)
			err := connStore.UpdateLastPing(ctx, tt.connectionID)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}

			mockDB.AssertExpectations(t)
			mockQuery.AssertExpectations(t)
			mockUpdateBuilder.AssertExpectations(t)
		})
	}
}

// TestConnectionStore_DeleteStale tests the DeleteStale method
func TestConnectionStore_DeleteStale(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	staleTime := now.Add(-1 * time.Hour)

	tests := []struct {
		name      string
		before    time.Time
		setupMock func(*mocks.MockDB, *mocks.MockQuery)
		wantErr   bool
		errMsg    string
	}{
		{
			name:   "successful delete stale",
			before: staleTime,
			setupMock: func(mockDB *mocks.MockDB, mockQuery *mocks.MockQuery) {
				staleConnections := []dynamorm.Connection{
					{ConnectionID: "conn1", LastPing: staleTime.Add(-30 * time.Minute)},
					{ConnectionID: "conn2", LastPing: staleTime.Add(-2 * time.Hour)},
				}

				// Mock the scan
				mockDB.On("Model", &dynamorm.Connection{}).Return(mockQuery)
				mockQuery.On("Where", "last_ping", "<", staleTime).Return(mockQuery)
				mockQuery.On("Scan", mock.AnythingOfType("*[]dynamorm.Connection")).
					Run(func(args mock.Arguments) {
						dest := args.Get(0).(*[]dynamorm.Connection)
						*dest = staleConnections
					}).Return(nil)

				// Mock the deletes
				for range staleConnections {
					deleteQuery := new(mocks.MockQuery)
					mockDB.On("Model", mock.AnythingOfType("*dynamorm.Connection")).Return(deleteQuery).Once()
					deleteQuery.On("Delete").Return(nil)
				}
			},
			wantErr: false,
		},
		{
			name:   "scan error",
			before: staleTime,
			setupMock: func(mockDB *mocks.MockDB, mockQuery *mocks.MockQuery) {
				mockDB.On("Model", &dynamorm.Connection{}).Return(mockQuery)
				mockQuery.On("Where", "last_ping", "<", staleTime).Return(mockQuery)
				mockQuery.On("Scan", mock.AnythingOfType("*[]dynamorm.Connection")).Return(errors.New("scan error"))
			},
			wantErr: true,
			errMsg:  "failed to scan stale connections",
		},
		{
			name:   "no stale connections",
			before: staleTime,
			setupMock: func(mockDB *mocks.MockDB, mockQuery *mocks.MockQuery) {
				mockDB.On("Model", &dynamorm.Connection{}).Return(mockQuery)
				mockQuery.On("Where", "last_ping", "<", staleTime).Return(mockQuery)
				mockQuery.On("Scan", mock.AnythingOfType("*[]dynamorm.Connection")).
					Run(func(args mock.Arguments) {
						dest := args.Get(0).(*[]dynamorm.Connection)
						*dest = []dynamorm.Connection{}
					}).Return(nil)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB := new(mocks.MockDB)
			mockQuery := new(mocks.MockQuery)

			if tt.setupMock != nil {
				tt.setupMock(mockDB, mockQuery)
			}

			connStore := dynamorm.NewConnectionStore(mockDB)
			err := connStore.DeleteStale(ctx, tt.before)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}

			mockDB.AssertExpectations(t)
			mockQuery.AssertExpectations(t)
		})
	}
}
