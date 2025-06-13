package dynamorm_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	// DynamORM mocks
	dynamocks "github.com/pay-theory/dynamorm/pkg/mocks"

	"github.com/pay-theory/streamer/internal/store"
	"github.com/pay-theory/streamer/internal/store/dynamorm"
)

// Test using DynamORM mocks for detailed behavior verification
func TestRequestQueue_Enqueue_WithDynamORMMocks(t *testing.T) {
	tests := []struct {
		name        string
		request     *store.AsyncRequest
		setupMock   func(*dynamocks.MockDB, *dynamocks.MockQuery)
		expectError bool
		errorMsg    string
	}{
		{
			name: "successful enqueue",
			request: &store.AsyncRequest{
				RequestID:    "req-123",
				ConnectionID: "conn-456",
				UserID:       "user-789",
				TenantID:     "tenant-abc",
				Action:       "test-action",
				Payload:      map[string]interface{}{"data": "test"},
			},
			setupMock: func(db *dynamocks.MockDB, q *dynamocks.MockQuery) {
				db.On("Model", mock.AnythingOfType("*dynamorm.AsyncRequest")).Return(q)
				q.On("Create").Return(nil)
			},
			expectError: false,
		},
		{
			name: "database error",
			request: &store.AsyncRequest{
				RequestID:    "req-123",
				ConnectionID: "conn-456",
				UserID:       "user-789",
				TenantID:     "tenant-abc",
				Action:       "test-action",
			},
			setupMock: func(db *dynamocks.MockDB, q *dynamocks.MockQuery) {
				db.On("Model", mock.AnythingOfType("*dynamorm.AsyncRequest")).Return(q)
				q.On("Create").Return(errors.New("DynamoDB service unavailable"))
			},
			expectError: true,
			errorMsg:    "failed to enqueue request",
		},
		{
			name:        "nil request",
			request:     nil,
			setupMock:   func(db *dynamocks.MockDB, q *dynamocks.MockQuery) {},
			expectError: true,
			errorMsg:    "cannot be nil",
		},
		{
			name: "missing request ID",
			request: &store.AsyncRequest{
				ConnectionID: "conn-456",
				UserID:       "user-789",
				TenantID:     "tenant-abc",
				Action:       "test-action",
			},
			setupMock:   func(db *dynamocks.MockDB, q *dynamocks.MockQuery) {},
			expectError: true,
			errorMsg:    "RequestID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB := new(dynamocks.MockDB)
			mockQuery := new(dynamocks.MockQuery)

			tt.setupMock(mockDB, mockQuery)

			queue := dynamorm.NewRequestQueue(mockDB)
			err := queue.Enqueue(context.Background(), tt.request)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				// Verify auto-generated fields
				assert.Equal(t, store.StatusPending, tt.request.Status)
				assert.NotZero(t, tt.request.CreatedAt)
				assert.NotZero(t, tt.request.TTL)
			}

			mockDB.AssertExpectations(t)
			mockQuery.AssertExpectations(t)
		})
	}
}

func TestRequestQueue_Get_WithDynamORMMocks(t *testing.T) {
	tests := []struct {
		name        string
		requestID   string
		setupMock   func(*dynamocks.MockDB, *dynamocks.MockQuery)
		expectError bool
		errorMsg    string
		expected    *store.AsyncRequest
	}{
		{
			name:      "successful get",
			requestID: "req-123",
			setupMock: func(db *dynamocks.MockDB, q *dynamocks.MockQuery) {
				db.On("Model", mock.AnythingOfType("*dynamorm.AsyncRequest")).Return(q)
				q.On("Where", "pk", "=", mock.AnythingOfType("string")).Return(q)
				q.On("All", mock.AnythingOfType("*[]dynamorm.AsyncRequest")).Run(func(args mock.Arguments) {
					dest := args.Get(0).(*[]dynamorm.AsyncRequest)
					*dest = []dynamorm.AsyncRequest{
						{
							RequestID:    "req-123",
							ConnectionID: "conn-456",
							Action:       "test-action",
							Status:       store.StatusPending,
						},
					}
				}).Return(nil)
			},
			expectError: false,
			expected: &store.AsyncRequest{
				RequestID:    "req-123",
				ConnectionID: "conn-456",
				Action:       "test-action",
				Status:       store.StatusPending,
			},
		},
		{
			name:        "empty request ID",
			requestID:   "",
			setupMock:   func(db *dynamocks.MockDB, q *dynamocks.MockQuery) {},
			expectError: true,
			errorMsg:    "cannot be empty",
		},
		{
			name:      "request not found",
			requestID: "req-nonexistent",
			setupMock: func(db *dynamocks.MockDB, q *dynamocks.MockQuery) {
				db.On("Model", mock.AnythingOfType("*dynamorm.AsyncRequest")).Return(q)
				q.On("Where", "pk", "=", mock.AnythingOfType("string")).Return(q)
				q.On("All", mock.AnythingOfType("*[]dynamorm.AsyncRequest")).Run(func(args mock.Arguments) {
					dest := args.Get(0).(*[]dynamorm.AsyncRequest)
					*dest = []dynamorm.AsyncRequest{} // Empty slice
				}).Return(nil)
			},
			expectError: true,
			errorMsg:    "item not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB := new(dynamocks.MockDB)
			mockQuery := new(dynamocks.MockQuery)

			tt.setupMock(mockDB, mockQuery)

			queue := dynamorm.NewRequestQueue(mockDB)
			result, err := queue.Get(context.Background(), tt.requestID)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.expected.RequestID, result.RequestID)
				assert.Equal(t, tt.expected.Action, result.Action)
			}

			mockDB.AssertExpectations(t)
			mockQuery.AssertExpectations(t)
		})
	}
}

func TestRequestQueue_UpdateStatus_WithDynamORMMocks(t *testing.T) {
	tests := []struct {
		name        string
		requestID   string
		newStatus   store.RequestStatus
		message     string
		setupMock   func(*dynamocks.MockDB, *dynamocks.MockQuery)
		expectError bool
		errorMsg    string
	}{
		{
			name:      "successful status update",
			requestID: "req-123",
			newStatus: store.StatusProcessing,
			message:   "Processing started",
			setupMock: func(db *dynamocks.MockDB, q *dynamocks.MockQuery) {
				// Mock Get call
				db.On("Model", mock.AnythingOfType("*dynamorm.AsyncRequest")).Return(q)
				q.On("Where", "pk", "=", mock.AnythingOfType("string")).Return(q)
				q.On("All", mock.AnythingOfType("*[]dynamorm.AsyncRequest")).Run(func(args mock.Arguments) {
					dest := args.Get(0).(*[]dynamorm.AsyncRequest)
					*dest = []dynamorm.AsyncRequest{
						{
							RequestID:    "req-123",
							ConnectionID: "conn-456",
							Action:       "test-action",
							Status:       store.StatusPending,
						},
					}
				}).Return(nil)

				// Mock Delete old entry
				q.On("Delete").Return(nil)

				// Mock Create new entry
				q.On("Create").Return(nil)
			},
			expectError: false,
		},
		{
			name:        "empty request ID",
			requestID:   "",
			newStatus:   store.StatusProcessing,
			message:     "Processing started",
			setupMock:   func(db *dynamocks.MockDB, q *dynamocks.MockQuery) {},
			expectError: true,
			errorMsg:    "cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB := new(dynamocks.MockDB)
			mockQuery := new(dynamocks.MockQuery)

			tt.setupMock(mockDB, mockQuery)

			queue := dynamorm.NewRequestQueue(mockDB)
			err := queue.UpdateStatus(context.Background(), tt.requestID, tt.newStatus, tt.message)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}

			mockDB.AssertExpectations(t)
			mockQuery.AssertExpectations(t)
		})
	}
}

func TestRequestQueue_GetByConnection_WithDynamORMMocks(t *testing.T) {
	mockDB := new(dynamocks.MockDB)
	mockQuery := new(dynamocks.MockQuery)

	// Setup mock for successful query
	mockDB.On("Model", mock.AnythingOfType("*dynamorm.AsyncRequest")).Return(mockQuery)
	mockQuery.On("Index", "connection-index").Return(mockQuery)
	mockQuery.On("Where", "connection_id", "=", "conn-456").Return(mockQuery)
	mockQuery.On("Limit", 10).Return(mockQuery)
	mockQuery.On("All", mock.AnythingOfType("*[]dynamorm.AsyncRequest")).Run(func(args mock.Arguments) {
		dest := args.Get(0).(*[]dynamorm.AsyncRequest)
		*dest = []dynamorm.AsyncRequest{
			{
				RequestID:    "req-1",
				ConnectionID: "conn-456",
				Action:       "action-1",
				Status:       store.StatusPending,
			},
			{
				RequestID:    "req-2",
				ConnectionID: "conn-456",
				Action:       "action-2",
				Status:       store.StatusProcessing,
			},
		}
	}).Return(nil)

	queue := dynamorm.NewRequestQueue(mockDB)
	result, err := queue.GetByConnection(context.Background(), "conn-456", 10)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "req-1", result[0].RequestID)
	assert.Equal(t, "req-2", result[1].RequestID)

	mockDB.AssertExpectations(t)
	mockQuery.AssertExpectations(t)
}

func TestRequestQueue_GetByStatus_WithDynamORMMocks(t *testing.T) {
	mockDB := new(dynamocks.MockDB)
	mockQuery := new(dynamocks.MockQuery)

	// Setup mock for successful query
	mockDB.On("Model", mock.AnythingOfType("*dynamorm.AsyncRequest")).Return(mockQuery)
	mockQuery.On("Index", "status-index").Return(mockQuery)
	mockQuery.On("Where", "status", "=", store.StatusPending).Return(mockQuery)
	mockQuery.On("Limit", 5).Return(mockQuery)
	mockQuery.On("All", mock.AnythingOfType("*[]dynamorm.AsyncRequest")).Run(func(args mock.Arguments) {
		dest := args.Get(0).(*[]dynamorm.AsyncRequest)
		*dest = []dynamorm.AsyncRequest{
			{
				RequestID:    "req-1",
				ConnectionID: "conn-456",
				Action:       "action-1",
				Status:       store.StatusPending,
			},
		}
	}).Return(nil)

	queue := dynamorm.NewRequestQueue(mockDB)
	result, err := queue.GetByStatus(context.Background(), store.StatusPending, 5)

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "req-1", result[0].RequestID)
	assert.Equal(t, store.StatusPending, result[0].Status)

	mockDB.AssertExpectations(t)
	mockQuery.AssertExpectations(t)
}

func TestRequestQueue_Delete_WithDynamORMMocks(t *testing.T) {
	mockDB := new(dynamocks.MockDB)
	mockQuery := new(dynamocks.MockQuery)

	// Setup mock for Get call
	mockDB.On("Model", mock.AnythingOfType("*dynamorm.AsyncRequest")).Return(mockQuery)
	mockQuery.On("Where", "pk", "=", mock.AnythingOfType("string")).Return(mockQuery)
	mockQuery.On("All", mock.AnythingOfType("*[]dynamorm.AsyncRequest")).Run(func(args mock.Arguments) {
		dest := args.Get(0).(*[]dynamorm.AsyncRequest)
		*dest = []dynamorm.AsyncRequest{
			{
				RequestID:    "req-123",
				ConnectionID: "conn-456",
				Action:       "test-action",
				Status:       store.StatusCompleted,
			},
		}
	}).Return(nil)

	// Setup mock for Delete call
	mockQuery.On("Delete").Return(nil)

	queue := dynamorm.NewRequestQueue(mockDB)
	err := queue.Delete(context.Background(), "req-123")

	assert.NoError(t, err)
	mockDB.AssertExpectations(t)
	mockQuery.AssertExpectations(t)
}

func TestRequestQueue_UpdateProgress_WithDynamORMMocks(t *testing.T) {
	mockDB := new(dynamocks.MockDB)
	mockQuery := new(dynamocks.MockQuery)
	mockUpdateBuilder := new(dynamocks.MockUpdateBuilder)

	// Setup mock for Get call
	mockDB.On("Model", mock.AnythingOfType("*dynamorm.AsyncRequest")).Return(mockQuery)
	mockQuery.On("Where", "pk", "=", mock.AnythingOfType("string")).Return(mockQuery)
	mockQuery.On("All", mock.AnythingOfType("*[]dynamorm.AsyncRequest")).Run(func(args mock.Arguments) {
		dest := args.Get(0).(*[]dynamorm.AsyncRequest)
		*dest = []dynamorm.AsyncRequest{
			{
				RequestID:    "req-123",
				ConnectionID: "conn-456",
				Action:       "test-action",
				Status:       store.StatusProcessing,
			},
		}
	}).Return(nil)

	// Setup mock for UpdateBuilder chain
	mockQuery.On("UpdateBuilder").Return(mockUpdateBuilder)
	mockUpdateBuilder.On("Set", "progress", 50.0).Return(mockUpdateBuilder)
	mockUpdateBuilder.On("Set", "progress_message", "Half complete").Return(mockUpdateBuilder)
	mockUpdateBuilder.On("Set", "progress_details", mock.AnythingOfType("map[string]interface {}")).Return(mockUpdateBuilder)
	mockUpdateBuilder.On("Execute").Return(nil)

	queue := dynamorm.NewRequestQueue(mockDB)
	err := queue.UpdateProgress(context.Background(), "req-123", 50.0, "Half complete", map[string]interface{}{"step": 2})

	assert.NoError(t, err)
	mockDB.AssertExpectations(t)
	mockQuery.AssertExpectations(t)
	mockUpdateBuilder.AssertExpectations(t)
}

func TestRequestQueue_CompleteRequest_WithDynamORMMocks(t *testing.T) {
	mockDB := new(dynamocks.MockDB)
	mockQuery := new(dynamocks.MockQuery)

	// Setup mock for Get call
	mockDB.On("Model", mock.AnythingOfType("*dynamorm.AsyncRequest")).Return(mockQuery)
	mockQuery.On("Where", "pk", "=", mock.AnythingOfType("string")).Return(mockQuery)
	mockQuery.On("All", mock.AnythingOfType("*[]dynamorm.AsyncRequest")).Run(func(args mock.Arguments) {
		dest := args.Get(0).(*[]dynamorm.AsyncRequest)
		*dest = []dynamorm.AsyncRequest{
			{
				RequestID:    "req-123",
				ConnectionID: "conn-456",
				Action:       "test-action",
				Status:       store.StatusProcessing,
			},
		}
	}).Return(nil)

	// Setup mock for UpdateStatus calls (Delete + Create)
	mockQuery.On("Delete").Return(nil)
	mockQuery.On("Create").Return(nil)

	queue := dynamorm.NewRequestQueue(mockDB)
	result := map[string]interface{}{"success": true, "data": "completed"}
	err := queue.CompleteRequest(context.Background(), "req-123", result)

	assert.NoError(t, err)
	mockDB.AssertExpectations(t)
	mockQuery.AssertExpectations(t)
}

func TestRequestQueue_FailRequest_WithDynamORMMocks(t *testing.T) {
	mockDB := new(dynamocks.MockDB)
	mockQuery := new(dynamocks.MockQuery)

	// Setup mock for Get call
	mockDB.On("Model", mock.AnythingOfType("*dynamorm.AsyncRequest")).Return(mockQuery)
	mockQuery.On("Where", "pk", "=", mock.AnythingOfType("string")).Return(mockQuery)
	mockQuery.On("All", mock.AnythingOfType("*[]dynamorm.AsyncRequest")).Run(func(args mock.Arguments) {
		dest := args.Get(0).(*[]dynamorm.AsyncRequest)
		*dest = []dynamorm.AsyncRequest{
			{
				RequestID:    "req-123",
				ConnectionID: "conn-456",
				Action:       "test-action",
				Status:       store.StatusProcessing,
			},
		}
	}).Return(nil)

	// Setup mock for UpdateStatus calls (Delete + Create)
	mockQuery.On("Delete").Return(nil)
	mockQuery.On("Create").Return(nil)

	queue := dynamorm.NewRequestQueue(mockDB)
	err := queue.FailRequest(context.Background(), "req-123", "Processing failed due to timeout")

	assert.NoError(t, err)
	mockDB.AssertExpectations(t)
	mockQuery.AssertExpectations(t)
}

func TestRequestQueue_Dequeue_WithDynamORMMocks(t *testing.T) {
	mockDB := new(dynamocks.MockDB)
	mockQuery := new(dynamocks.MockQuery)

	// Setup mock for GetByStatus call
	mockDB.On("Model", mock.AnythingOfType("*dynamorm.AsyncRequest")).Return(mockQuery)
	mockQuery.On("Index", "status-index").Return(mockQuery)
	mockQuery.On("Where", "status", "=", store.StatusPending).Return(mockQuery)
	mockQuery.On("Limit", 5).Return(mockQuery)
	mockQuery.On("All", mock.AnythingOfType("*[]dynamorm.AsyncRequest")).Run(func(args mock.Arguments) {
		dest := args.Get(0).(*[]dynamorm.AsyncRequest)
		*dest = []dynamorm.AsyncRequest{
			{
				RequestID:    "req-1",
				ConnectionID: "conn-456",
				Action:       "action-1",
				Status:       store.StatusPending,
			},
			{
				RequestID:    "req-2",
				ConnectionID: "conn-789",
				Action:       "action-2",
				Status:       store.StatusPending,
			},
		}
	}).Return(nil)

	// Setup mocks for UpdateStatus calls for each request
	// For each request, we need Get + Delete + Create
	mockQuery.On("Where", "pk", "=", mock.AnythingOfType("string")).Return(mockQuery)
	mockQuery.On("Delete").Return(nil)
	mockQuery.On("Create").Return(nil)

	queue := dynamorm.NewRequestQueue(mockDB)
	result, err := queue.Dequeue(context.Background(), 5)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "req-1", result[0].RequestID)
	assert.Equal(t, "req-2", result[1].RequestID)

	mockDB.AssertExpectations(t)
	mockQuery.AssertExpectations(t)
}

// Edge case and error handling tests
func TestRequestQueue_ValidationErrors(t *testing.T) {
	mockDB := new(dynamocks.MockDB)
	queue := dynamorm.NewRequestQueue(mockDB)

	tests := []struct {
		name     string
		request  *store.AsyncRequest
		errorMsg string
	}{
		{
			name:     "nil request",
			request:  nil,
			errorMsg: "cannot be nil",
		},
		{
			name: "missing connection ID",
			request: &store.AsyncRequest{
				RequestID: "req-123",
				UserID:    "user-789",
				TenantID:  "tenant-abc",
				Action:    "test-action",
			},
			errorMsg: "ConnectionID",
		},
		{
			name: "missing user ID",
			request: &store.AsyncRequest{
				RequestID:    "req-123",
				ConnectionID: "conn-456",
				TenantID:     "tenant-abc",
				Action:       "test-action",
			},
			errorMsg: "UserID",
		},
		{
			name: "missing tenant ID",
			request: &store.AsyncRequest{
				RequestID:    "req-123",
				ConnectionID: "conn-456",
				UserID:       "user-789",
				Action:       "test-action",
			},
			errorMsg: "TenantID",
		},
		{
			name: "missing action",
			request: &store.AsyncRequest{
				RequestID:    "req-123",
				ConnectionID: "conn-456",
				UserID:       "user-789",
				TenantID:     "tenant-abc",
			},
			errorMsg: "Action",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := queue.Enqueue(context.Background(), tt.request)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errorMsg)
		})
	}
}
