package shared

import (
	"context"

	"github.com/aws/aws-xray-sdk-go/xray"
)

// TraceSegment represents custom X-Ray segment data
type TraceSegment struct {
	ConnectionID string `json:"connection_id,omitempty"`
	UserID       string `json:"user_id,omitempty"`
	TenantID     string `json:"tenant_id,omitempty"`
	MessageType  string `json:"message_type,omitempty"`
	MessageSize  int    `json:"message_size,omitempty"`
	Action       string `json:"action,omitempty"`
}

// StartSubsegment starts a new X-Ray subsegment with custom data
func StartSubsegment(ctx context.Context, name string, data TraceSegment) (context.Context, *xray.Segment) {
	ctx, seg := xray.BeginSubsegment(ctx, name)

	// Add annotations for searchable fields
	if data.ConnectionID != "" {
		seg.AddAnnotation("connection_id", data.ConnectionID)
	}
	if data.UserID != "" {
		seg.AddAnnotation("user_id", data.UserID)
	}
	if data.TenantID != "" {
		seg.AddAnnotation("tenant_id", data.TenantID)
	}
	if data.Action != "" {
		seg.AddAnnotation("action", data.Action)
	}

	// Add metadata for detailed info
	metadata := make(map[string]interface{})
	if data.MessageType != "" {
		metadata["type"] = data.MessageType
	}
	if data.MessageSize > 0 {
		metadata["size"] = data.MessageSize
	}
	if len(metadata) > 0 {
		seg.AddMetadata("message", metadata)
	}

	return ctx, seg
}

// EndSubsegment ends an X-Ray subsegment and records any error
func EndSubsegment(seg *xray.Segment, err error) {
	if err != nil {
		seg.AddError(err)
	}
	seg.Close(err)
}

// CaptureFunc wraps a function with X-Ray tracing
func CaptureFunc(ctx context.Context, name string, fn func(context.Context) error) error {
	return xray.Capture(ctx, name, func(ctx1 context.Context) error {
		return fn(ctx1)
	})
}

// CaptureFuncWithData wraps a function with X-Ray tracing and custom data
func CaptureFuncWithData(ctx context.Context, name string, data TraceSegment, fn func(context.Context) error) error {
	return xray.Capture(ctx, name, func(ctx1 context.Context) error {
		seg := xray.GetSegment(ctx1)

		// Add custom data to segment
		if data.ConnectionID != "" {
			seg.AddAnnotation("connection_id", data.ConnectionID)
		}
		if data.UserID != "" {
			seg.AddAnnotation("user_id", data.UserID)
		}
		if data.TenantID != "" {
			seg.AddAnnotation("tenant_id", data.TenantID)
		}
		if data.Action != "" {
			seg.AddAnnotation("action", data.Action)
		}
		// Add message metadata
		msgMetadata := make(map[string]interface{})
		if data.MessageType != "" {
			msgMetadata["type"] = data.MessageType
		}
		if data.MessageSize > 0 {
			msgMetadata["size"] = data.MessageSize
		}
		if len(msgMetadata) > 0 {
			seg.AddMetadata("message", msgMetadata)
		}

		return fn(ctx1)
	})
}

// AddTraceMetadata adds metadata to the current segment
func AddTraceMetadata(ctx context.Context, namespace, key string, value interface{}) {
	if seg := xray.GetSegment(ctx); seg != nil {
		metadata := map[string]interface{}{key: value}
		seg.AddMetadata(namespace, metadata)
	}
}

// AddTraceAnnotation adds an annotation to the current segment
func AddTraceAnnotation(ctx context.Context, key string, value string) {
	if seg := xray.GetSegment(ctx); seg != nil {
		seg.AddAnnotation(key, value)
	}
}

// RecordError records an error in the current segment
func RecordError(ctx context.Context, err error) {
	if seg := xray.GetSegment(ctx); seg != nil {
		seg.AddError(err)
	}
}
