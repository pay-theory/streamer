package connection

import (
	"errors"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi/types"
	"github.com/aws/smithy-go"
)

// isGoneError checks if the error indicates the connection is gone (410 status)
func isGoneError(err error) bool {
	if err == nil {
		return false
	}

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		// Check for GoneException or 410 status code
		return apiErr.ErrorCode() == "GoneException" ||
			strings.Contains(apiErr.Error(), "410") ||
			strings.Contains(apiErr.Error(), "Gone")
	}

	// Check for specific exception types
	var goneErr *types.GoneException
	return errors.As(err, &goneErr)
}

// isForbiddenError checks if the error indicates forbidden access
func isForbiddenError(err error) bool {
	if err == nil {
		return false
	}

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		return apiErr.ErrorCode() == "ForbiddenException"
	}

	var forbiddenErr *types.ForbiddenException
	return errors.As(err, &forbiddenErr)
}

// isPayloadTooLargeError checks if the error indicates payload is too large
func isPayloadTooLargeError(err error) bool {
	if err == nil {
		return false
	}

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		return apiErr.ErrorCode() == "PayloadTooLargeException"
	}

	var payloadErr *types.PayloadTooLargeException
	return errors.As(err, &payloadErr)
}
