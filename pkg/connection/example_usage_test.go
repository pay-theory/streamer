package connection_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/pay-theory/streamer/internal/store"
	"github.com/pay-theory/streamer/pkg/connection"
)

// Example demonstrates how to use the ConnectionManager
func Example() {
	// Load AWS configuration
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	// Create DynamoDB client for the store
	dynamoClient := dynamodb.NewFromConfig(cfg)

	// Create connection store
	connStore := store.NewConnectionStore(dynamoClient, "streamer_connections")

	// Create API Gateway Management API client
	// The endpoint should be your WebSocket API endpoint
	endpoint := "https://abc123.execute-api.us-east-1.amazonaws.com/production"
	apiGatewayClient := apigatewaymanagementapi.NewFromConfig(cfg, func(o *apigatewaymanagementapi.Options) {
		o.EndpointResolver = apigatewaymanagementapi.EndpointResolverFunc(func(region string, options apigatewaymanagementapi.EndpointResolverOptions) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL: endpoint,
			}, nil
		})
	})

	// Create connection manager
	manager := connection.NewManager(connStore, apiGatewayClient, endpoint)

	// Optional: Set custom logger
	manager.SetLogger(func(format string, args ...interface{}) {
		log.Printf("[ConnectionManager] "+format, args...)
	})

	// Example 1: Send a message to a single connection
	err = manager.Send(context.Background(), "connection-123", map[string]interface{}{
		"type": "notification",
		"data": map[string]string{
			"message":   "Hello from the server!",
			"timestamp": time.Now().Format(time.RFC3339),
		},
	})
	if err != nil {
		switch {
		case errors.Is(err, connection.ErrConnectionNotFound):
			log.Printf("Connection not found")
		case errors.Is(err, connection.ErrConnectionStale):
			log.Printf("Connection is stale and has been removed")
		default:
			log.Printf("Failed to send message: %v", err)
		}
	}

	// Example 2: Broadcast to multiple connections
	connectionIDs := []string{"conn-1", "conn-2", "conn-3"}
	err = manager.Broadcast(context.Background(), connectionIDs, map[string]interface{}{
		"type": "broadcast",
		"data": map[string]string{
			"announcement": "System maintenance in 5 minutes",
		},
	})
	if err != nil {
		log.Printf("Broadcast had some failures: %v", err)
	}

	// Example 3: Check if a connection is active
	if manager.IsActive(context.Background(), "connection-123") {
		fmt.Println("Connection is active")
	} else {
		fmt.Println("Connection is not active")
	}
}

// Example_withRouter shows integration with Team 2's router
func Example_withRouter() {
	// Setup connection manager (as shown above)
	var manager *connection.Manager // ... initialized as above

	// This is how Team 2 would use it in their router
	// The manager implements the ConnectionManager interface expected by the router:
	//
	// type ConnectionManager interface {
	//     Send(ctx context.Context, connectionID string, message interface{}) error
	// }

	// In router.go:
	// router := streamer.NewRouter(requestStore, manager)

	// When router needs to send a response:
	response := map[string]interface{}{
		"type":       "response",
		"request_id": "req-123",
		"success":    true,
		"data": map[string]string{
			"result": "Operation completed",
		},
	}

	err := manager.Send(context.Background(), "connection-456", response)
	if err != nil {
		log.Printf("Failed to send response: %v", err)
	}
}

// Example_mockForTesting shows how Team 2 can mock the ConnectionManager for tests
type MockConnectionManager struct {
	SendFunc func(ctx context.Context, connectionID string, message interface{}) error
	calls    []SendCall
}

type SendCall struct {
	ConnectionID string
	Message      interface{}
}

func (m *MockConnectionManager) Send(ctx context.Context, connectionID string, message interface{}) error {
	m.calls = append(m.calls, SendCall{
		ConnectionID: connectionID,
		Message:      message,
	})

	if m.SendFunc != nil {
		return m.SendFunc(ctx, connectionID, message)
	}
	return nil
}

func Example_mockUsage() {
	// In Team 2's tests:
	mockManager := &MockConnectionManager{
		SendFunc: func(ctx context.Context, connectionID string, message interface{}) error {
			// Simulate successful send
			return nil
		},
	}

	// Use mock in router tests
	// router := streamer.NewRouter(requestStore, mockManager)

	// Verify calls were made
	fmt.Printf("Number of sends: %d\n", len(mockManager.calls))
	fmt.Printf("First call was to connection: %s\n", mockManager.calls[0].ConnectionID)
}
