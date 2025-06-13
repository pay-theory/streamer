package main

import (
	"encoding/json"
)

// HandlerConfig holds configuration for the handler
type HandlerConfig struct {
	TableName      string
	JWTPublicKey   string
	JWTIssuer      string
	AllowedTenants []string
	LogLevel       string
}

// jsonStringify converts a value to JSON string
func jsonStringify(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}
