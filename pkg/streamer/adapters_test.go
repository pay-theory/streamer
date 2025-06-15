package streamer

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/pay-theory/streamer/internal/store"
)

// mockRequestQueue implements store.RequestQueue for testing
type mockRequestQueue struct {
	enqueuedRequests []*store.AsyncRequest
	requests         map[string]*store.AsyncRequest
	enqueueErr       error
	getErr           error
	updateErr        error
}

func (m *mockRequestQueue) Enqueue(ctx context.Context, req *store.AsyncRequest) error {
	if m.enqueueErr != nil {
		return m.enqueueErr
	}
	m.enqueuedRequests = append(m.enqueuedRequests, req)
	if m.requests == nil {
		m.requests = make(map[string]*store.AsyncRequest)
	}
	m.requests[req.RequestID] = req
	return nil
}

func (m *mockRequestQueue) Get(ctx context.Context, requestID string) (*store.AsyncRequest, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	req, ok := m.requests[requestID]
	if !ok {
		return nil, store.ErrNotFound
	}
	return req, nil
}

func (m *mockRequestQueue) UpdateProgress(ctx context.Context, requestID string, progress float64, message string, details map[string]interface{}) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	return nil
}

func (m *mockRequestQueue) CompleteRequest(ctx context.Context, requestID string, result map[string]interface{}) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	return nil
}

func (m *mockRequestQueue) FailRequest(ctx context.Context, requestID string, errMsg string) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	return nil
}

// Stub remaining methods to satisfy interface
func (m *mockRequestQueue) Dequeue(ctx context.Context, limit int) ([]*store.AsyncRequest, error) {
	return nil, nil
}
func (m *mockRequestQueue) UpdateStatus(ctx context.Context, requestID string, status store.RequestStatus, message string) error {
	return nil
}
func (m *mockRequestQueue) GetByConnection(ctx context.Context, connectionID string, limit int) ([]*store.AsyncRequest, error) {
	return nil, nil
}
func (m *mockRequestQueue) GetByStatus(ctx context.Context, status store.RequestStatus, limit int) ([]*store.AsyncRequest, error) {
	return nil, nil
}
func (m *mockRequestQueue) Delete(ctx context.Context, requestID string) error { return nil }

func TestRequestQueueAdapter_Enqueue(t *testing.T) {
	tests := []struct {
		name    string
		request *Request
		wantErr bool
		verify  func(t *testing.T, asyncReq *store.AsyncRequest)
	}{
		{
			name: "basic request conversion",
			request: &Request{
				ID:           "req-123",
				ConnectionID: "conn-456",
				Action:       "generate_report",
				Payload:      json.RawMessage(`{"type":"monthly","month":"2024-01"}`),
				CreatedAt:    time.Now(),
			},
			wantErr: false,
			verify: func(t *testing.T, asyncReq *store.AsyncRequest) {
				if asyncReq.RequestID != "req-123" {
					t.Errorf("RequestID = %v, want %v", asyncReq.RequestID, "req-123")
				}
				if asyncReq.ConnectionID != "conn-456" {
					t.Errorf("ConnectionID = %v, want %v", asyncReq.ConnectionID, "conn-456")
				}
				if asyncReq.Action != "generate_report" {
					t.Errorf("Action = %v, want %v", asyncReq.Action, "generate_report")
				}
				if asyncReq.Status != store.StatusPending {
					t.Errorf("Status = %v, want %v", asyncReq.Status, store.StatusPending)
				}
				if asyncReq.Progress != 0 {
					t.Errorf("Progress = %v, want %v", asyncReq.Progress, 0)
				}
				if asyncReq.MaxRetries != 3 {
					t.Errorf("MaxRetries = %v, want %v", asyncReq.MaxRetries, 3)
				}
			},
		},
		{
			name: "request with metadata",
			request: &Request{
				ID:           "req-789",
				ConnectionID: "conn-012",
				Action:       "process_data",
				Metadata: map[string]string{
					"user_id":   "user-123",
					"tenant_id": "tenant-456",
					"custom":    "value",
				},
				CreatedAt: time.Now(),
			},
			wantErr: false,
			verify: func(t *testing.T, asyncReq *store.AsyncRequest) {
				if asyncReq.UserID != "user-123" {
					t.Errorf("UserID = %v, want %v", asyncReq.UserID, "user-123")
				}
				if asyncReq.TenantID != "tenant-456" {
					t.Errorf("TenantID = %v, want %v", asyncReq.TenantID, "tenant-456")
				}
				// Check metadata is preserved in payload
				metadata, ok := asyncReq.Payload["_metadata"].(map[string]string)
				if !ok {
					t.Fatal("_metadata not found in payload")
				}
				if metadata["custom"] != "value" {
					t.Errorf("custom metadata = %v, want %v", metadata["custom"], "value")
				}
			},
		},
		{
			name: "invalid payload JSON",
			request: &Request{
				ID:           "req-bad",
				ConnectionID: "conn-bad",
				Action:       "bad_action",
				Payload:      json.RawMessage(`{"invalid": json}`),
				CreatedAt:    time.Now(),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockRequestQueue{}
			adapter := NewRequestQueueAdapter(mock)

			err := adapter.Enqueue(context.Background(), tt.request)
			if (err != nil) != tt.wantErr {
				t.Errorf("Enqueue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.verify != nil {
				if len(mock.enqueuedRequests) != 1 {
					t.Fatalf("Expected 1 enqueued request, got %d", len(mock.enqueuedRequests))
				}
				tt.verify(t, mock.enqueuedRequests[0])
			}
		})
	}
}

func TestConvertAsyncRequestToRequest(t *testing.T) {
	tests := []struct {
		name     string
		asyncReq *store.AsyncRequest
		want     *Request
		wantErr  bool
	}{
		{
			name: "basic conversion",
			asyncReq: &store.AsyncRequest{
				RequestID:    "req-123",
				ConnectionID: "conn-456",
				Action:       "generate_report",
				CreatedAt:    time.Now(),
				Payload: map[string]interface{}{
					"type":  "monthly",
					"month": "2024-01",
				},
			},
			want: &Request{
				ID:           "req-123",
				ConnectionID: "conn-456",
				Action:       "generate_report",
			},
			wantErr: false,
		},
		{
			name: "with metadata",
			asyncReq: &store.AsyncRequest{
				RequestID:    "req-789",
				ConnectionID: "conn-012",
				Action:       "process_data",
				CreatedAt:    time.Now(),
				UserID:       "user-123",
				TenantID:     "tenant-456",
				Payload: map[string]interface{}{
					"_metadata": map[string]interface{}{
						"custom": "value",
						"foo":    "bar",
					},
					"data": "test",
				},
			},
			want: &Request{
				ID:           "req-789",
				ConnectionID: "conn-012",
				Action:       "process_data",
				Metadata: map[string]string{
					"user_id":   "user-123",
					"tenant_id": "tenant-456",
					"custom":    "value",
					"foo":       "bar",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ConvertAsyncRequestToRequest(tt.asyncReq)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConvertAsyncRequestToRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.ID != tt.want.ID {
					t.Errorf("ID = %v, want %v", got.ID, tt.want.ID)
				}
				if got.ConnectionID != tt.want.ConnectionID {
					t.Errorf("ConnectionID = %v, want %v", got.ConnectionID, tt.want.ConnectionID)
				}
				if got.Action != tt.want.Action {
					t.Errorf("Action = %v, want %v", got.Action, tt.want.Action)
				}
				// Verify metadata
				for k, v := range tt.want.Metadata {
					if got.Metadata[k] != v {
						t.Errorf("Metadata[%s] = %v, want %v", k, got.Metadata[k], v)
					}
				}
			}
		})
	}
}

func TestMapStoreError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantCode  string
		wantError bool
	}{
		{
			name:      "nil error",
			err:       nil,
			wantError: false,
		},
		{
			name:      "not found error",
			err:       store.ErrNotFound,
			wantCode:  ErrCodeNotFound,
			wantError: true,
		},
		{
			name:      "already exists error",
			err:       store.ErrAlreadyExists,
			wantCode:  ErrCodeValidation,
			wantError: true,
		},
		{
			name:      "validation error",
			err:       &store.ValidationError{Field: "payload", Message: "invalid format"},
			wantCode:  ErrCodeValidation,
			wantError: true,
		},
		{
			name:      "generic error",
			err:       errors.New("some error"),
			wantCode:  ErrCodeInternalError,
			wantError: true,
		},
		{
			name: "wrapped store error",
			err: &store.StoreError{
				Op:    "Get",
				Table: "test_table",
				Key:   "test_key",
				Err:   store.ErrNotFound,
			},
			wantCode:  ErrCodeNotFound,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapStoreError(tt.err)

			// Check if we got an error when we didn't expect one
			if !tt.wantError && got != nil {
				t.Errorf("mapStoreError() = %v, want nil", got)
				return
			}

			// Check if we didn't get an error when we expected one
			if tt.wantError && got == nil {
				t.Errorf("mapStoreError() = nil, want error")
				return
			}

			// If we expected an error, verify it's the right type and code
			if tt.wantError && got != nil {
				streamerErr, ok := got.(*Error)
				if !ok {
					t.Errorf("mapStoreError() returned non-Error type: %T", got)
					return
				}
				if streamerErr.Code != tt.wantCode {
					t.Errorf("Error code = %v, want %v", streamerErr.Code, tt.wantCode)
				}
			}
		})
	}
}

func TestRequestQueueAdapter_GetAsyncRequest(t *testing.T) {
	tests := []struct {
		name      string
		requestID string
		setupMock func(*mockRequestQueue)
		want      *Request
		wantErr   bool
		wantCode  string
	}{
		{
			name:      "successful get",
			requestID: "req-123",
			setupMock: func(m *mockRequestQueue) {
				m.requests = map[string]*store.AsyncRequest{
					"req-123": {
						RequestID:    "req-123",
						ConnectionID: "conn-456",
						Action:       "test_action",
						UserID:       "user-789",
						TenantID:     "tenant-012",
						CreatedAt:    time.Now(),
						Payload: map[string]interface{}{
							"data": "test data",
						},
					},
				}
			},
			want: &Request{
				ID:           "req-123",
				ConnectionID: "conn-456",
				Action:       "test_action",
				Metadata: map[string]string{
					"user_id":   "user-789",
					"tenant_id": "tenant-012",
				},
			},
			wantErr: false,
		},
		{
			name:      "not found error",
			requestID: "non-existent",
			setupMock: func(m *mockRequestQueue) {
				m.requests = make(map[string]*store.AsyncRequest)
			},
			wantErr:  true,
			wantCode: ErrCodeNotFound,
		},
		{
			name:      "store error",
			requestID: "req-error",
			setupMock: func(m *mockRequestQueue) {
				m.getErr = errors.New("database error")
			},
			wantErr:  true,
			wantCode: ErrCodeInternalError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockRequestQueue{}
			if tt.setupMock != nil {
				tt.setupMock(mock)
			}
			adapter := &RequestQueueAdapter{queue: mock}

			got, err := adapter.GetAsyncRequest(context.Background(), tt.requestID)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetAsyncRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil {
				streamerErr, ok := err.(*Error)
				if !ok {
					t.Errorf("Expected Error type, got %T", err)
					return
				}
				if streamerErr.Code != tt.wantCode {
					t.Errorf("Error code = %v, want %v", streamerErr.Code, tt.wantCode)
				}
				return
			}

			if !tt.wantErr && got != nil {
				if got.ID != tt.want.ID {
					t.Errorf("ID = %v, want %v", got.ID, tt.want.ID)
				}
				if got.ConnectionID != tt.want.ConnectionID {
					t.Errorf("ConnectionID = %v, want %v", got.ConnectionID, tt.want.ConnectionID)
				}
				if got.Action != tt.want.Action {
					t.Errorf("Action = %v, want %v", got.Action, tt.want.Action)
				}
				// Check metadata
				for k, v := range tt.want.Metadata {
					if got.Metadata[k] != v {
						t.Errorf("Metadata[%s] = %v, want %v", k, got.Metadata[k], v)
					}
				}
			}
		})
	}
}

func TestRequestQueueAdapter_UpdateProgress(t *testing.T) {
	tests := []struct {
		name      string
		requestID string
		progress  float64
		message   string
		setupMock func(*mockRequestQueue)
		wantErr   bool
		wantCode  string
	}{
		{
			name:      "successful update",
			requestID: "req-123",
			progress:  50.0,
			message:   "Processing halfway done",
			setupMock: func(m *mockRequestQueue) {
				// No error setup needed
			},
			wantErr: false,
		},
		{
			name:      "update error",
			requestID: "req-456",
			progress:  75.5,
			message:   "Almost done",
			setupMock: func(m *mockRequestQueue) {
				m.updateErr = errors.New("update failed")
			},
			wantErr:  true,
			wantCode: ErrCodeInternalError,
		},
		{
			name:      "not found error",
			requestID: "non-existent",
			progress:  10.0,
			message:   "Starting",
			setupMock: func(m *mockRequestQueue) {
				m.updateErr = store.ErrNotFound
			},
			wantErr:  true,
			wantCode: ErrCodeNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockRequestQueue{}
			if tt.setupMock != nil {
				tt.setupMock(mock)
			}
			adapter := &RequestQueueAdapter{queue: mock}

			err := adapter.UpdateProgress(context.Background(), tt.requestID, tt.progress, tt.message)

			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateProgress() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil {
				streamerErr, ok := err.(*Error)
				if !ok {
					t.Errorf("Expected Error type, got %T", err)
					return
				}
				if streamerErr.Code != tt.wantCode {
					t.Errorf("Error code = %v, want %v", streamerErr.Code, tt.wantCode)
				}
			}
		})
	}
}

func TestRequestQueueAdapter_CompleteRequest(t *testing.T) {
	tests := []struct {
		name      string
		requestID string
		result    *Result
		setupMock func(*mockRequestQueue)
		wantErr   bool
		wantCode  string
	}{
		{
			name:      "successful completion with data",
			requestID: "req-123",
			result: &Result{
				RequestID: "req-123",
				Success:   true,
				Data: map[string]interface{}{
					"report_url": "https://example.com/report.pdf",
					"generated":  true,
				},
				Metadata: map[string]string{
					"processor": "report-gen-v2",
				},
			},
			setupMock: func(m *mockRequestQueue) {
				// No error setup needed
			},
			wantErr: false,
		},
		{
			name:      "completion with error result",
			requestID: "req-456",
			result: &Result{
				RequestID: "req-456",
				Success:   false,
				Error: &Error{
					Code:    ErrCodeValidation,
					Message: "Invalid report parameters",
					Details: map[string]interface{}{
						"field": "date_range",
					},
				},
			},
			setupMock: func(m *mockRequestQueue) {
				// No error setup needed
			},
			wantErr: false,
		},
		{
			name:      "completion fails",
			requestID: "req-789",
			result: &Result{
				RequestID: "req-789",
				Success:   true,
			},
			setupMock: func(m *mockRequestQueue) {
				m.updateErr = errors.New("database error")
			},
			wantErr:  true,
			wantCode: ErrCodeInternalError,
		},
		{
			name:      "not found error",
			requestID: "non-existent",
			result: &Result{
				RequestID: "non-existent",
				Success:   true,
			},
			setupMock: func(m *mockRequestQueue) {
				m.updateErr = store.ErrNotFound
			},
			wantErr:  true,
			wantCode: ErrCodeNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockRequestQueue{}
			if tt.setupMock != nil {
				tt.setupMock(mock)
			}
			adapter := &RequestQueueAdapter{queue: mock}

			err := adapter.CompleteRequest(context.Background(), tt.requestID, tt.result)

			if (err != nil) != tt.wantErr {
				t.Errorf("CompleteRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil {
				streamerErr, ok := err.(*Error)
				if !ok {
					t.Errorf("Expected Error type, got %T", err)
					return
				}
				if streamerErr.Code != tt.wantCode {
					t.Errorf("Error code = %v, want %v", streamerErr.Code, tt.wantCode)
				}
			}
		})
	}
}

func TestRequestQueueAdapter_FailRequest(t *testing.T) {
	tests := []struct {
		name      string
		requestID string
		err       error
		setupMock func(*mockRequestQueue)
		wantErr   bool
		wantCode  string
	}{
		{
			name:      "fail with streamer error",
			requestID: "req-123",
			err: &Error{
				Code:    ErrCodeTimeout,
				Message: "Operation timed out",
			},
			setupMock: func(m *mockRequestQueue) {
				// No error setup needed
			},
			wantErr: false,
		},
		{
			name:      "fail with generic error",
			requestID: "req-456",
			err:       errors.New("unexpected error occurred"),
			setupMock: func(m *mockRequestQueue) {
				// No error setup needed
			},
			wantErr: false,
		},
		{
			name:      "fail request fails",
			requestID: "req-789",
			err:       errors.New("processing error"),
			setupMock: func(m *mockRequestQueue) {
				m.updateErr = errors.New("database error")
			},
			wantErr:  true,
			wantCode: ErrCodeInternalError,
		},
		{
			name:      "not found error",
			requestID: "non-existent",
			err:       errors.New("some error"),
			setupMock: func(m *mockRequestQueue) {
				m.updateErr = store.ErrNotFound
			},
			wantErr:  true,
			wantCode: ErrCodeNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockRequestQueue{}
			if tt.setupMock != nil {
				tt.setupMock(mock)
			}
			adapter := &RequestQueueAdapter{queue: mock}

			err := adapter.FailRequest(context.Background(), tt.requestID, tt.err)

			if (err != nil) != tt.wantErr {
				t.Errorf("FailRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil {
				streamerErr, ok := err.(*Error)
				if !ok {
					t.Errorf("Expected Error type, got %T", err)
					return
				}
				if streamerErr.Code != tt.wantCode {
					t.Errorf("Error code = %v, want %v", streamerErr.Code, tt.wantCode)
				}
			}
		})
	}
}
