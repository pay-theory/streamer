package protocol

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestValidationError tests the ValidationError type
func TestValidationError(t *testing.T) {
	// Test single error
	ve := ValidationErrors{
		{Field: "email", Message: "invalid format"},
	}
	assert.Equal(t, "email: invalid format", ve.Error())

	// Test multiple errors
	ve = ValidationErrors{
		{Field: "email", Message: "invalid format"},
		{Field: "password", Message: "too short"},
		{Field: "age", Message: "must be positive"},
	}
	errStr := ve.Error()
	assert.Contains(t, errStr, "email: invalid format")
	assert.Contains(t, errStr, "password: too short")
	assert.Contains(t, errStr, "age: must be positive")
	assert.Contains(t, errStr, "; ")

	// Test empty errors
	ve = ValidationErrors{}
	assert.Equal(t, "", ve.Error())
}

// TestValidateIncomingMessage tests the ValidateIncomingMessage function
func TestValidateIncomingMessage(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		wantErr bool
		errMsgs []string
		check   func(*testing.T, *IncomingMessage)
	}{
		{
			name: "valid message with all fields",
			data: `{
				"type": "request",
				"id": "msg123",
				"action": "user.create",
				"payload": {"name": "John", "email": "john@example.com"},
				"metadata": {"source": "web"}
			}`,
			wantErr: false,
			check: func(t *testing.T, msg *IncomingMessage) {
				assert.Equal(t, string(MessageTypeRequest), string(msg.Type))
				assert.Equal(t, "msg123", msg.ID)
				assert.Equal(t, "user.create", msg.Action)
				assert.NotNil(t, msg.Payload)
				assert.NotNil(t, msg.Metadata)
			},
		},
		{
			name:    "valid minimal message",
			data:    `{"action": "ping"}`,
			wantErr: false,
			check: func(t *testing.T, msg *IncomingMessage) {
				assert.Equal(t, string(MessageTypeRequest), string(msg.Type)) // Default
				assert.Equal(t, "ping", msg.Action)
				assert.Equal(t, "", msg.ID)
				assert.Nil(t, msg.Payload)
			},
		},
		{
			name:    "invalid JSON",
			data:    `{invalid json`,
			wantErr: true,
			errMsgs: []string{"invalid JSON"},
		},
		{
			name:    "missing action",
			data:    `{"type": "request"}`,
			wantErr: true,
			errMsgs: []string{"action is required"},
		},
		{
			name:    "invalid message type",
			data:    `{"type": "invalid", "action": "test"}`,
			wantErr: true,
			errMsgs: []string{"invalid message type: invalid"},
		},
		{
			name:    "action with invalid characters",
			data:    `{"action": "user@create!"}`,
			wantErr: true,
			errMsgs: []string{"action must contain only alphanumeric characters"},
		},
		{
			name:    "valid action with dots, dashes, underscores",
			data:    `{"action": "user.create-new_v2"}`,
			wantErr: false,
			check: func(t *testing.T, msg *IncomingMessage) {
				assert.Equal(t, "user.create-new_v2", msg.Action)
			},
		},
		{
			name:    "id too long",
			data:    `{"action": "test", "id": "` + strings.Repeat("a", 129) + `"}`,
			wantErr: true,
			errMsgs: []string{"id must not exceed 128 characters"},
		},
		{
			name:    "invalid payload JSON",
			data:    `{"action": "test", "payload": {invalid json}}`,
			wantErr: true,
			errMsgs: []string{"invalid JSON"},
		},
		{
			name:    "valid payload",
			data:    `{"action": "test", "payload": {"key": "value", "num": 123}}`,
			wantErr: false,
			check: func(t *testing.T, msg *IncomingMessage) {
				var payload map[string]interface{}
				err := json.Unmarshal(msg.Payload, &payload)
				assert.NoError(t, err)
				assert.Equal(t, "value", payload["key"])
				assert.Equal(t, float64(123), payload["num"])
			},
		},
		{
			name: "multiple validation errors",
			data: `{
				"type": "invalid",
				"action": "",
				"id": "` + strings.Repeat("x", 200) + `"
			}`,
			wantErr: true,
			errMsgs: []string{
				"invalid message type",
				"action is required",
				"id must not exceed 128 characters",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := ValidateIncomingMessage([]byte(tt.data))

			if tt.wantErr {
				assert.Error(t, err)
				for _, errMsg := range tt.errMsgs {
					assert.Contains(t, err.Error(), errMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, msg)
				if tt.check != nil {
					tt.check(t, msg)
				}
			}
		})
	}
}

// TestIsValidAction tests the isValidAction function
func TestIsValidAction(t *testing.T) {
	tests := []struct {
		action string
		valid  bool
	}{
		{"user.create", true},
		{"user_create", true},
		{"user-create", true},
		{"userCreate", true},
		{"user.create.v2", true},
		{"user123", true},
		{"User.Create", true},
		{"USER_CREATE", true},
		{"", false},
		{"user@create", false},
		{"user!create", false},
		{"user create", false},
		{"user#create", false},
		{"user$create", false},
		{"user%create", false},
		{"user&create", false},
		{"user*create", false},
		{"user(create)", false},
		{"user[create]", false},
		{"user{create}", false},
		{"user/create", false},
		{"user\\create", false},
		{"user|create", false},
		{"user+create", false},
		{"user=create", false},
		{"user?create", false},
		{"user<create>", false},
		{"user,create", false},
		{"user;create", false},
		{"user:create", false},
		{"user'create", false},
		{"user\"create", false},
	}

	for _, tt := range tests {
		t.Run(tt.action, func(t *testing.T) {
			got := isValidAction(tt.action)
			assert.Equal(t, tt.valid, got)
		})
	}
}

// TestIsAlphaNumeric tests the isAlphaNumeric function
func TestIsAlphaNumeric(t *testing.T) {
	tests := []struct {
		ch   rune
		want bool
	}{
		{'a', true},
		{'z', true},
		{'A', true},
		{'Z', true},
		{'0', true},
		{'9', true},
		{'m', true},
		{'M', true},
		{'5', true},
		{'.', false},
		{'-', false},
		{'_', false},
		{'@', false},
		{' ', false},
		{'!', false},
		{'#', false},
		{'$', false},
		{'%', false},
	}

	for _, tt := range tests {
		t.Run(string(tt.ch), func(t *testing.T) {
			got := isAlphaNumeric(tt.ch)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestPayloadValidator_RequireFields tests the RequireFields method
func TestPayloadValidator_RequireFields(t *testing.T) {
	pv := &PayloadValidator{}

	tests := []struct {
		name    string
		payload string
		fields  []string
		wantErr bool
		errMsgs []string
	}{
		{
			name:    "all required fields present",
			payload: `{"name": "John", "email": "john@example.com", "age": 30}`,
			fields:  []string{"name", "email"},
			wantErr: false,
		},
		{
			name:    "missing required field",
			payload: `{"name": "John"}`,
			fields:  []string{"name", "email"},
			wantErr: true,
			errMsgs: []string{"email", "field is required"},
		},
		{
			name:    "missing multiple required fields",
			payload: `{"name": "John"}`,
			fields:  []string{"email", "age", "address"},
			wantErr: true,
			errMsgs: []string{"email", "age", "address", "field is required"},
		},
		{
			name:    "no required fields",
			payload: `{"anything": "value"}`,
			fields:  []string{},
			wantErr: false,
		},
		{
			name:    "invalid JSON payload",
			payload: `{invalid}`,
			fields:  []string{"name"},
			wantErr: true,
			errMsgs: []string{"invalid payload format"},
		},
		{
			name:    "null values count as present",
			payload: `{"name": null, "email": null}`,
			fields:  []string{"name", "email"},
			wantErr: false,
		},
		{
			name:    "empty strings count as present",
			payload: `{"name": "", "email": ""}`,
			fields:  []string{"name", "email"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pv.RequireFields(json.RawMessage(tt.payload), tt.fields...)

			if tt.wantErr {
				assert.Error(t, err)
				for _, errMsg := range tt.errMsgs {
					assert.Contains(t, err.Error(), errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestPayloadValidator_ValidateString tests the ValidateString method
func TestPayloadValidator_ValidateString(t *testing.T) {
	pv := &PayloadValidator{}

	tests := []struct {
		name    string
		data    map[string]interface{}
		field   string
		minLen  int
		maxLen  int
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid string within bounds",
			data:    map[string]interface{}{"name": "John"},
			field:   "name",
			minLen:  1,
			maxLen:  10,
			wantErr: false,
		},
		{
			name:    "string at min length",
			data:    map[string]interface{}{"code": "AB"},
			field:   "code",
			minLen:  2,
			maxLen:  10,
			wantErr: false,
		},
		{
			name:    "string at max length",
			data:    map[string]interface{}{"code": "ABCDE"},
			field:   "code",
			minLen:  1,
			maxLen:  5,
			wantErr: false,
		},
		{
			name:    "string too short",
			data:    map[string]interface{}{"name": "Jo"},
			field:   "name",
			minLen:  3,
			maxLen:  10,
			wantErr: true,
			errMsg:  "name must be at least 3 characters",
		},
		{
			name:    "string too long",
			data:    map[string]interface{}{"name": "VeryLongName"},
			field:   "name",
			minLen:  1,
			maxLen:  10,
			wantErr: true,
			errMsg:  "name must not exceed 10 characters",
		},
		{
			name:    "field not present (optional)",
			data:    map[string]interface{}{},
			field:   "name",
			minLen:  1,
			maxLen:  10,
			wantErr: false,
		},
		{
			name:    "not a string",
			data:    map[string]interface{}{"name": 123},
			field:   "name",
			minLen:  1,
			maxLen:  10,
			wantErr: true,
			errMsg:  "name must be a string",
		},
		{
			name:    "zero min/max means no limit",
			data:    map[string]interface{}{"desc": "Any length description"},
			field:   "desc",
			minLen:  0,
			maxLen:  0,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pv.ValidateString(tt.data, tt.field, tt.minLen, tt.maxLen)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestPayloadValidator_ValidateNumber tests the ValidateNumber method
func TestPayloadValidator_ValidateNumber(t *testing.T) {
	pv := &PayloadValidator{}

	tests := []struct {
		name    string
		data    map[string]interface{}
		field   string
		min     float64
		max     float64
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid float within bounds",
			data:    map[string]interface{}{"price": 19.99},
			field:   "price",
			min:     0,
			max:     100,
			wantErr: false,
		},
		{
			name:    "valid int within bounds",
			data:    map[string]interface{}{"age": 25},
			field:   "age",
			min:     18,
			max:     100,
			wantErr: false,
		},
		{
			name:    "number at min",
			data:    map[string]interface{}{"score": 0.0},
			field:   "score",
			min:     0,
			max:     100,
			wantErr: false,
		},
		{
			name:    "number at max",
			data:    map[string]interface{}{"score": 100.0},
			field:   "score",
			min:     0,
			max:     100,
			wantErr: false,
		},
		{
			name:    "number too small",
			data:    map[string]interface{}{"age": 17},
			field:   "age",
			min:     18,
			max:     100,
			wantErr: true,
			errMsg:  "age must be at least 18",
		},
		{
			name:    "number too large",
			data:    map[string]interface{}{"age": 150},
			field:   "age",
			min:     0,
			max:     100,
			wantErr: true,
			errMsg:  "age must not exceed 100",
		},
		{
			name:    "field not present (optional)",
			data:    map[string]interface{}{},
			field:   "age",
			min:     0,
			max:     100,
			wantErr: false,
		},
		{
			name:    "not a number",
			data:    map[string]interface{}{"age": "twenty"},
			field:   "age",
			min:     0,
			max:     100,
			wantErr: true,
			errMsg:  "age must be a number",
		},
		{
			name:    "zero min/max means no limit",
			data:    map[string]interface{}{"value": -1000.5},
			field:   "value",
			min:     0,
			max:     0,
			wantErr: false,
		},
		{
			name:    "negative numbers",
			data:    map[string]interface{}{"temp": -10.5},
			field:   "temp",
			min:     -20,
			max:     40,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pv.ValidateNumber(tt.data, tt.field, tt.min, tt.max)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidationErrorsJSON tests JSON marshaling of ValidationErrors
func TestValidationErrorsJSON(t *testing.T) {
	ve := ValidationErrors{
		{Field: "email", Message: "invalid format"},
		{Field: "password", Message: "too short"},
	}

	// Marshal to JSON
	data, err := json.Marshal(ve)
	assert.NoError(t, err)

	// Unmarshal back
	var decoded ValidationErrors
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)

	assert.Len(t, decoded, 2)
	assert.Equal(t, ve[0].Field, decoded[0].Field)
	assert.Equal(t, ve[0].Message, decoded[0].Message)
	assert.Equal(t, ve[1].Field, decoded[1].Field)
	assert.Equal(t, ve[1].Message, decoded[1].Message)
}

// TestValidateIncomingMessageEdgeCases tests edge cases for ValidateIncomingMessage
func TestValidateIncomingMessageEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		wantErr bool
	}{
		{
			name:    "empty JSON object",
			data:    `{}`,
			wantErr: true, // Missing action
		},
		{
			name:    "null values",
			data:    `{"action": "test", "payload": null, "metadata": null}`,
			wantErr: false,
		},
		{
			name:    "empty action",
			data:    `{"action": ""}`,
			wantErr: true,
		},
		{
			name:    "action with only dots",
			data:    `{"action": "..."}`,
			wantErr: false, // Dots are allowed
		},
		{
			name:    "action with spaces trimmed",
			data:    `{"action": " test "}`,
			wantErr: true, // Spaces are not valid
		},
		{
			name:    "unicode in action",
			data:    `{"action": "测试"}`,
			wantErr: true, // Only ASCII alphanumeric allowed
		},
		{
			name:    "very long action",
			data:    `{"action": "` + strings.Repeat("a", 1000) + `"}`,
			wantErr: false, // No length limit on action
		},
		{
			name:    "id exactly 128 chars",
			data:    `{"action": "test", "id": "` + strings.Repeat("x", 128) + `"}`,
			wantErr: false,
		},
		{
			name:    "nested payload",
			data:    `{"action": "test", "payload": {"nested": {"deep": {"value": 123}}}}`,
			wantErr: false,
		},
		{
			name:    "array payload",
			data:    `{"action": "test", "payload": [1, 2, 3, 4, 5]}`,
			wantErr: false,
		},
		{
			name:    "string payload",
			data:    `{"action": "test", "payload": "simple string"}`,
			wantErr: false,
		},
		{
			name:    "number payload",
			data:    `{"action": "test", "payload": 42}`,
			wantErr: false,
		},
		{
			name:    "boolean payload",
			data:    `{"action": "test", "payload": true}`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := ValidateIncomingMessage([]byte(tt.data))

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, msg)
			}
		})
	}
}

// TestValidationErrorsAsError tests that ValidationErrors implements error interface
func TestValidationErrorsAsError(t *testing.T) {
	ve := ValidationErrors{
		{Field: "test", Message: "failed"},
	}

	// Can be used as error
	var err error = ve
	assert.NotNil(t, err)
	assert.Equal(t, "test: failed", err.Error())

	// Can be type asserted back
	var ve2 ValidationErrors
	ok := errors.As(err, &ve2)
	assert.True(t, ok)
	assert.Equal(t, ve, ve2)
}
