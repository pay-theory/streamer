package connection

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi/types"
)

// AWSAPIGatewayAdapter adapts the AWS SDK client to our interface
type AWSAPIGatewayAdapter struct {
	client *apigatewaymanagementapi.Client
}

// NewAWSAPIGatewayAdapter creates a new adapter
func NewAWSAPIGatewayAdapter(client *apigatewaymanagementapi.Client) *AWSAPIGatewayAdapter {
	return &AWSAPIGatewayAdapter{
		client: client,
	}
}

// Ensure AWSAPIGatewayAdapter implements APIGatewayClient interface
var _ APIGatewayClient = (*AWSAPIGatewayAdapter)(nil)

// PostToConnection sends data to a WebSocket connection
func (a *AWSAPIGatewayAdapter) PostToConnection(ctx context.Context, connectionID string, data []byte) error {
	input := &apigatewaymanagementapi.PostToConnectionInput{
		ConnectionId: aws.String(connectionID),
		Data:         data,
	}

	_, err := a.client.PostToConnection(ctx, input)
	if err != nil {
		// Convert AWS SDK errors to our error types
		return a.convertError(err, connectionID)
	}

	return nil
}

// DeleteConnection terminates a WebSocket connection
func (a *AWSAPIGatewayAdapter) DeleteConnection(ctx context.Context, connectionID string) error {
	input := &apigatewaymanagementapi.DeleteConnectionInput{
		ConnectionId: aws.String(connectionID),
	}

	_, err := a.client.DeleteConnection(ctx, input)
	if err != nil {
		// Convert AWS SDK errors to our error types
		return a.convertError(err, connectionID)
	}

	return nil
}

// GetConnection retrieves connection information
func (a *AWSAPIGatewayAdapter) GetConnection(ctx context.Context, connectionID string) (*ConnectionInfo, error) {
	input := &apigatewaymanagementapi.GetConnectionInput{
		ConnectionId: aws.String(connectionID),
	}

	output, err := a.client.GetConnection(ctx, input)
	if err != nil {
		return nil, a.convertError(err, connectionID)
	}

	info := &ConnectionInfo{
		ConnectionID: connectionID,
	}

	if output.ConnectedAt != nil {
		info.ConnectedAt = output.ConnectedAt.String()
	}
	if output.LastActiveAt != nil {
		info.LastActiveAt = output.LastActiveAt.String()
	}
	if output.Identity != nil {
		if output.Identity.SourceIp != nil {
			info.SourceIP = *output.Identity.SourceIp
		}
		if output.Identity.UserAgent != nil {
			info.UserAgent = *output.Identity.UserAgent
		}
	}

	return info, nil
}

// convertError converts AWS SDK errors to our error types
func (a *AWSAPIGatewayAdapter) convertError(err error, connectionID string) error {
	if err == nil {
		return nil
	}

	// Check for specific AWS SDK error types
	var goneErr *types.GoneException
	if errors.As(err, &goneErr) {
		return GoneError{
			ConnectionID: connectionID,
			Message:      fmt.Sprintf("connection %s is gone: %v", connectionID, err),
		}
	}

	var forbiddenErr *types.ForbiddenException
	if errors.As(err, &forbiddenErr) {
		return ForbiddenError{
			ConnectionID: connectionID,
			Message:      fmt.Sprintf("forbidden access to connection %s: %v", connectionID, err),
		}
	}

	var payloadErr *types.PayloadTooLargeException
	if errors.As(err, &payloadErr) {
		return PayloadTooLargeError{
			ConnectionID: connectionID,
			Message:      fmt.Sprintf("payload too large for connection %s: %v", connectionID, err),
		}
	}

	// AWS SDK v2 doesn't have a specific throttling exception type
	// We'll check for it via the HTTP status code below

	// Check for HTTP status codes in generic API errors
	var apiErr interface{ HTTPStatusCode() int }
	if errors.As(err, &apiErr) {
		switch apiErr.HTTPStatusCode() {
		case 410:
			return GoneError{
				ConnectionID: connectionID,
				Message:      fmt.Sprintf("connection %s returned 410: %v", connectionID, err),
			}
		case 403:
			return ForbiddenError{
				ConnectionID: connectionID,
				Message:      fmt.Sprintf("connection %s returned 403: %v", connectionID, err),
			}
		case 413:
			return PayloadTooLargeError{
				ConnectionID: connectionID,
				Message:      fmt.Sprintf("connection %s returned 413: %v", connectionID, err),
			}
		case 429:
			return ThrottlingError{
				ConnectionID: connectionID,
				Message:      fmt.Sprintf("connection %s returned 429: %v", connectionID, err),
			}
		case 500, 502, 503, 504:
			return InternalServerError{
				Message: fmt.Sprintf("server error for connection %s: %v", connectionID, err),
			}
		}
	}

	// Return the original error if we can't convert it
	return err
}
