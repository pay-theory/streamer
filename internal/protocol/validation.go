package protocol

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ValidationError represents a validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationErrors represents multiple validation errors
type ValidationErrors []ValidationError

// Error implements the error interface
func (ve ValidationErrors) Error() string {
	if len(ve) == 0 {
		return ""
	}

	var messages []string
	for _, e := range ve {
		messages = append(messages, fmt.Sprintf("%s: %s", e.Field, e.Message))
	}
	return strings.Join(messages, "; ")
}

// ValidateIncomingMessage validates an incoming WebSocket message
func ValidateIncomingMessage(data []byte) (*IncomingMessage, error) {
	var msg IncomingMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	var errors ValidationErrors

	// Validate message type
	if msg.Type == "" {
		msg.Type = MessageTypeRequest // Default to request
	} else if msg.Type != MessageTypeRequest {
		errors = append(errors, ValidationError{
			Field:   "type",
			Message: fmt.Sprintf("invalid message type: %s", msg.Type),
		})
	}

	// Validate action
	if msg.Action == "" {
		errors = append(errors, ValidationError{
			Field:   "action",
			Message: "action is required",
		})
	} else {
		// Action must be alphanumeric with dots, dashes, and underscores
		if !isValidAction(msg.Action) {
			errors = append(errors, ValidationError{
				Field:   "action",
				Message: "action must contain only alphanumeric characters, dots, dashes, and underscores",
			})
		}
	}

	// Validate ID if provided
	if msg.ID != "" && len(msg.ID) > 128 {
		errors = append(errors, ValidationError{
			Field:   "id",
			Message: "id must not exceed 128 characters",
		})
	}

	// Validate payload if provided
	if msg.Payload != nil && len(msg.Payload) > 0 {
		var temp interface{}
		if err := json.Unmarshal(msg.Payload, &temp); err != nil {
			errors = append(errors, ValidationError{
				Field:   "payload",
				Message: "payload must be valid JSON",
			})
		}
	}

	if len(errors) > 0 {
		return nil, errors
	}

	return &msg, nil
}

// isValidAction checks if an action string is valid
func isValidAction(action string) bool {
	if action == "" {
		return false
	}

	for _, ch := range action {
		if !isAlphaNumeric(ch) && ch != '.' && ch != '-' && ch != '_' {
			return false
		}
	}

	return true
}

// isAlphaNumeric checks if a rune is alphanumeric
func isAlphaNumeric(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')
}

// ValidateHandler provides validation for specific handler payloads
type ValidateHandler interface {
	ValidatePayload(payload json.RawMessage) error
}

// PayloadValidator provides common payload validation functions
type PayloadValidator struct{}

// RequireFields checks that required fields exist in the payload
func (pv *PayloadValidator) RequireFields(payload json.RawMessage, fields ...string) error {
	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err != nil {
		return fmt.Errorf("invalid payload format: %w", err)
	}

	var errors ValidationErrors
	for _, field := range fields {
		if _, exists := data[field]; !exists {
			errors = append(errors, ValidationError{
				Field:   field,
				Message: "field is required",
			})
		}
	}

	if len(errors) > 0 {
		return errors
	}

	return nil
}

// ValidateString validates a string field
func (pv *PayloadValidator) ValidateString(data map[string]interface{}, field string, minLen, maxLen int) error {
	value, exists := data[field]
	if !exists {
		return nil // Field is optional
	}

	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("%s must be a string", field)
	}

	if minLen > 0 && len(str) < minLen {
		return fmt.Errorf("%s must be at least %d characters", field, minLen)
	}

	if maxLen > 0 && len(str) > maxLen {
		return fmt.Errorf("%s must not exceed %d characters", field, maxLen)
	}

	return nil
}

// ValidateNumber validates a numeric field
func (pv *PayloadValidator) ValidateNumber(data map[string]interface{}, field string, min, max float64) error {
	value, exists := data[field]
	if !exists {
		return nil // Field is optional
	}

	num, ok := value.(float64)
	if !ok {
		// Try to convert from int
		if intVal, ok := value.(int); ok {
			num = float64(intVal)
		} else {
			return fmt.Errorf("%s must be a number", field)
		}
	}

	if min != 0 && num < min {
		return fmt.Errorf("%s must be at least %f", field, min)
	}

	if max != 0 && num > max {
		return fmt.Errorf("%s must not exceed %f", field, max)
	}

	return nil
}
