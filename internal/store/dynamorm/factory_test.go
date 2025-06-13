package dynamorm

import (
	"testing"

	"github.com/pay-theory/dynamorm/pkg/session"
	"github.com/stretchr/testify/assert"
)

func TestNewStoreFactory(t *testing.T) {
	tests := []struct {
		name        string
		config      session.Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			config: session.Config{
				Region: "us-east-1",
			},
			expectError: false,
		},
		{
			name: "empty region",
			config: session.Config{
				Region: "",
			},
			expectError: false, // DynamORM might have defaults
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory, err := NewStoreFactory(tt.config)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, factory)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, factory)
				assert.NotNil(t, factory.db)
				assert.NotNil(t, factory.connectionStore)
				assert.NotNil(t, factory.requestQueue)

				// Test getter methods
				assert.NotNil(t, factory.ConnectionStore())
				assert.NotNil(t, factory.RequestQueue())
				assert.Nil(t, factory.SubscriptionStore()) // Currently not implemented
				assert.NotNil(t, factory.DB())
			}
		})
	}
}

func TestNewStoreFactoryFromClient(t *testing.T) {
	// This function currently just creates a new factory with region
	// In the future it might use the actual client
	factory, err := NewStoreFactoryFromClient(nil, "us-west-2")

	assert.NoError(t, err)
	assert.NotNil(t, factory)
	assert.NotNil(t, factory.ConnectionStore())
	assert.NotNil(t, factory.RequestQueue())
	assert.NotNil(t, factory.DB())
}
