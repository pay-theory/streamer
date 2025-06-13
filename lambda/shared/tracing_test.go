package shared

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/stretchr/testify/assert"
)

// Mock X-Ray segment for testing
type mockSegment struct {
	annotations map[string]string
	metadata    map[string]map[string]interface{}
	error       error
	closed      bool
}

func newMockSegment() *mockSegment {
	return &mockSegment{
		annotations: make(map[string]string),
		metadata:    make(map[string]map[string]interface{}),
	}
}

func (m *mockSegment) AddAnnotation(key string, value string) {
	m.annotations[key] = value
}

func (m *mockSegment) AddMetadata(namespace string, metadata map[string]interface{}) {
	if m.metadata[namespace] == nil {
		m.metadata[namespace] = make(map[string]interface{})
	}
	for k, v := range metadata {
		m.metadata[namespace][k] = v
	}
}

func (m *mockSegment) AddError(err error) {
	m.error = err
}

func (m *mockSegment) Close(err error) {
	m.closed = true
	if err != nil {
		m.error = err
	}
}

func TestStartSubsegment(t *testing.T) {
	tests := []struct {
		name     string
		segment  string
		data     TraceSegment
		validate func(t *testing.T, ctx context.Context, seg *xray.Segment)
	}{
		{
			name:    "subsegment with full data",
			segment: "TestOperation",
			data: TraceSegment{
				ConnectionID: "conn-123",
				UserID:       "user-456",
				TenantID:     "tenant-789",
				MessageType:  "request",
				MessageSize:  1024,
				Action:       "process",
			},
			validate: func(t *testing.T, ctx context.Context, seg *xray.Segment) {
				// In actual X-Ray SDK, we would check annotations and metadata
				// For now, just verify context and segment are returned
				assert.NotNil(t, ctx)
			},
		},
		{
			name:    "subsegment with partial data",
			segment: "PartialOperation",
			data: TraceSegment{
				ConnectionID: "conn-999",
				Action:       "validate",
			},
			validate: func(t *testing.T, ctx context.Context, seg *xray.Segment) {
				assert.NotNil(t, ctx)
			},
		},
		{
			name:    "subsegment with empty data",
			segment: "EmptyOperation",
			data:    TraceSegment{},
			validate: func(t *testing.T, ctx context.Context, seg *xray.Segment) {
				assert.NotNil(t, ctx)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Start subsegment
			newCtx, seg := StartSubsegment(ctx, tt.segment, tt.data)

			// Validate
			if tt.validate != nil {
				tt.validate(t, newCtx, seg)
			}

			// Clean up
			if seg != nil {
				EndSubsegment(seg, nil)
			}
		})
	}
}

func TestEndSubsegment(t *testing.T) {
	tests := []struct {
		name string
		seg  *xray.Segment
		err  error
	}{
		{
			name: "end subsegment with nil error",
			seg:  nil, // In test environment, segments are often nil
			err:  nil,
		},
		{
			name: "end subsegment with error",
			seg:  nil,
			err:  errors.New("test error"),
		},
		{
			name: "end nil subsegment",
			seg:  nil,
			err:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This should not panic
			EndSubsegment(tt.seg, tt.err)
		})
	}
}

func TestCaptureFunc(t *testing.T) {
	tests := []struct {
		name     string
		funcName string
		fn       func(context.Context) error
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "successful function capture",
			funcName: "SuccessOperation",
			fn: func(ctx context.Context) error {
				// Simulate some work
				return nil
			},
			wantErr: false,
		},
		{
			name:     "function capture with error",
			funcName: "ErrorOperation",
			fn: func(ctx context.Context) error {
				return errors.New("operation failed")
			},
			wantErr: true,
			errMsg:  "operation failed",
		},
		{
			name:     "function with context usage",
			funcName: "ContextOperation",
			fn: func(ctx context.Context) error {
				// Try to use context
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
					return nil
				}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			err := CaptureFunc(ctx, tt.funcName, tt.fn)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCaptureFuncWithData(t *testing.T) {
	tests := []struct {
		name     string
		funcName string
		data     TraceSegment
		fn       func(context.Context) error
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "capture with full trace data",
			funcName: "DataOperation",
			data: TraceSegment{
				ConnectionID: "conn-123",
				UserID:       "user-456",
				TenantID:     "tenant-789",
				MessageType:  "response",
				MessageSize:  2048,
				Action:       "send",
			},
			fn: func(ctx context.Context) error {
				return nil
			},
			wantErr: false,
		},
		{
			name:     "capture with error and trace data",
			funcName: "ErrorDataOperation",
			data: TraceSegment{
				ConnectionID: "conn-error",
				Action:       "fail",
			},
			fn: func(ctx context.Context) error {
				return errors.New("data operation failed")
			},
			wantErr: true,
			errMsg:  "data operation failed",
		},
		{
			name:     "capture with empty trace data",
			funcName: "EmptyDataOperation",
			data:     TraceSegment{},
			fn: func(ctx context.Context) error {
				return nil
			},
			wantErr: false,
		},
		{
			name:     "capture with only message metadata",
			funcName: "MessageOperation",
			data: TraceSegment{
				MessageType: "notification",
				MessageSize: 512,
			},
			fn: func(ctx context.Context) error {
				return nil
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			err := CaptureFuncWithData(ctx, tt.funcName, tt.data, tt.fn)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAddTraceMetadata(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		key       string
		value     interface{}
	}{
		{
			name:      "add string metadata",
			namespace: "test",
			key:       "stringKey",
			value:     "stringValue",
		},
		{
			name:      "add numeric metadata",
			namespace: "metrics",
			key:       "count",
			value:     42,
		},
		{
			name:      "add boolean metadata",
			namespace: "flags",
			key:       "enabled",
			value:     true,
		},
		{
			name:      "add complex metadata",
			namespace: "complex",
			key:       "data",
			value: map[string]interface{}{
				"nested": "value",
				"count":  123,
			},
		},
		{
			name:      "add nil metadata",
			namespace: "test",
			key:       "nilKey",
			value:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// This should not panic even without a segment
			AddTraceMetadata(ctx, tt.namespace, tt.key, tt.value)
		})
	}
}

func TestAddTraceAnnotation(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{
			name:  "add simple annotation",
			key:   "user_id",
			value: "user-123",
		},
		{
			name:  "add empty annotation",
			key:   "empty",
			value: "",
		},
		{
			name:  "add annotation with special characters",
			key:   "special",
			value: "value!@#$%^&*()",
		},
		{
			name:  "add long annotation",
			key:   "long",
			value: "this is a very long annotation value that contains a lot of text to test how the system handles longer values",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// This should not panic even without a segment
			AddTraceAnnotation(ctx, tt.key, tt.value)
		})
	}
}

func TestRecordError(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{
			name: "record simple error",
			err:  errors.New("simple error"),
		},
		{
			name: "record nil error",
			err:  nil,
		},
		{
			name: "record formatted error",
			err:  errors.New("error with code: E123"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// This should not panic even without a segment
			RecordError(ctx, tt.err)
		})
	}
}

func TestTraceSegmentStruct(t *testing.T) {
	// Test that TraceSegment struct is properly defined
	segment := TraceSegment{
		ConnectionID: "test-conn",
		UserID:       "test-user",
		TenantID:     "test-tenant",
		MessageType:  "test-type",
		MessageSize:  1024,
		Action:       "test-action",
	}

	assert.Equal(t, "test-conn", segment.ConnectionID)
	assert.Equal(t, "test-user", segment.UserID)
	assert.Equal(t, "test-tenant", segment.TenantID)
	assert.Equal(t, "test-type", segment.MessageType)
	assert.Equal(t, 1024, segment.MessageSize)
	assert.Equal(t, "test-action", segment.Action)
}

func TestIntegrationScenario(t *testing.T) {
	// Test a realistic integration scenario
	ctx := context.Background()

	// Start a subsegment for connection handling
	ctx, seg := StartSubsegment(ctx, "HandleConnection", TraceSegment{
		ConnectionID: "conn-integration",
		UserID:       "user-integration",
		TenantID:     "tenant-integration",
	})

	// Add some metadata during processing
	AddTraceMetadata(ctx, "processing", "stage", "validation")
	AddTraceAnnotation(ctx, "status", "processing")

	// Simulate an operation
	err := CaptureFuncWithData(ctx, "ProcessMessage", TraceSegment{
		MessageType: "request",
		MessageSize: 512,
		Action:      "process",
	}, func(ctx context.Context) error {
		// Add more metadata inside the function
		AddTraceMetadata(ctx, "message", "processed", true)
		return nil
	})

	assert.NoError(t, err)

	// Record completion
	EndSubsegment(seg, nil)
}

func TestErrorScenario(t *testing.T) {
	// Test error handling scenario
	ctx := context.Background()

	// Start a subsegment
	ctx, seg := StartSubsegment(ctx, "ErrorOperation", TraceSegment{
		ConnectionID: "conn-error",
		Action:       "fail",
	})

	// Simulate an error
	testErr := errors.New("test error occurred")
	RecordError(ctx, testErr)

	// End with error
	EndSubsegment(seg, testErr)
}

// Test that nil segments don't cause panics
func TestNilSegmentHandling(t *testing.T) {
	// All these operations should handle nil segments gracefully
	ctx := context.Background()

	// These should not panic
	ctx, _ = StartSubsegment(ctx, "Test", TraceSegment{})
	assert.NotNil(t, ctx)

	EndSubsegment(nil, nil)
	EndSubsegment(nil, errors.New("error"))

	AddTraceMetadata(ctx, "namespace", "key", "value")
	AddTraceAnnotation(ctx, "key", "value")
	RecordError(ctx, errors.New("error"))
}
