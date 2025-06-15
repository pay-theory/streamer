// Example: Lift v2 WebSocket Broadcast Application
// This demonstrates how to use Lift for WebSocket applications with authentication
package main

import (
	"log"
	"time"

	"github.com/pay-theory/lift/pkg/lift"
)

// This is a simplified example showing Lift WebSocket patterns
// For a complete implementation, you would need to add:
// - Actual middleware configuration (see lambda/connect/main_lift_optimized.go for working example)
// - Database/storage layer
// - Broadcasting logic
// - Stream management

func main() {
	// Create app with WebSocket support
	app := lift.New(lift.WithWebSocketSupport())

	// Note: In a real implementation, you would add middleware here:
	// See lambda/connect/main_lift_optimized.go for working middleware configuration
	// app.Use(middleware.JWT(jwtConfig))
	// app.Use(middleware.ObservabilityMiddleware(obsConfig))
	// app.Use(middleware.WebSocketMetrics(metricsCollector))

	// Connection handler - viewers join streams
	app.WebSocket("$connect", handleViewerConnect)

	// Stream management routes (simplified)
	app.WebSocket("stream.start", handleStreamStart)
	app.WebSocket("stream.join", handleStreamJoin)
	app.WebSocket("chat.message", handleChatMessage)

	// Disconnect handler
	app.WebSocket("$disconnect", handleViewerDisconnect)

	// Start the application
	log.Println("Starting Lift WebSocket broadcast example...")
	app.Start()
}

// handleViewerConnect processes new WebSocket connections
func handleViewerConnect(ctx *lift.Context) error {
	// In a real implementation with JWT middleware, you would get user ID from claims
	userID, _ := ctx.Get("userId").(string)
	if userID == "" {
		// For this example, we'll use a placeholder
		userID = "example-user"
	}

	// Set initial connection metadata
	ctx.Set("streams", []string{}) // Streams the user is watching
	ctx.Set("is_host", false)

	log.Printf("Viewer connected")

	return ctx.Status(200).JSON(map[string]interface{}{
		"status": "connected",
		"userId": userID,
	})
}

// handleStreamStart initiates a new stream
func handleStreamStart(ctx *lift.Context) error {
	userID, _ := ctx.Get("userId").(string)
	if userID == "" {
		userID = "example-user"
	}

	// In a real implementation, you would:
	// 1. Parse the request body for stream details
	// 2. Create a stream record in your database
	// 3. Broadcast to all connected users
	// 4. Set up stream metadata

	log.Printf("Stream started")

	return ctx.Status(200).JSON(map[string]interface{}{
		"status":   "stream_started",
		"streamId": "example-stream-123",
		"hostId":   userID,
	})
}

// handleStreamJoin allows viewers to join a stream
func handleStreamJoin(ctx *lift.Context) error {
	userID, _ := ctx.Get("userId").(string)
	if userID == "" {
		userID = "example-user"
	}

	// In a real implementation, you would:
	// 1. Parse the stream ID from the request
	// 2. Validate the stream exists
	// 3. Add user to stream viewers
	// 4. Update viewer count
	// 5. Notify other participants

	log.Printf("User joined stream")

	return ctx.Status(200).JSON(map[string]interface{}{
		"status":  "joined",
		"message": "Successfully joined stream",
	})
}

// handleChatMessage processes chat messages in streams
func handleChatMessage(ctx *lift.Context) error {
	userID, _ := ctx.Get("userId").(string)
	if userID == "" {
		userID = "example-user"
	}

	// In a real implementation, you would:
	// 1. Parse the message from the request
	// 2. Validate user is in the stream
	// 3. Broadcast message to all stream viewers
	// 4. Store message history if needed

	log.Printf("Chat message received")

	return ctx.Status(200).JSON(map[string]interface{}{
		"status":  "sent",
		"message": "Message sent successfully",
	})
}

// handleViewerDisconnect processes WebSocket disconnections
func handleViewerDisconnect(ctx *lift.Context) error {
	userID, _ := ctx.Get("userId").(string)
	if userID == "" {
		userID = "example-user"
	}

	// In a real implementation, you would:
	// 1. Remove user from all streams
	// 2. Update viewer counts
	// 3. Notify other participants
	// 4. Clean up user data

	log.Printf("Viewer disconnected")

	return ctx.Status(200).JSON(map[string]interface{}{
		"status": "disconnected",
	})
}

// Helper functions that would be implemented in a real application:

// generateStreamID would create a unique stream identifier
func generateStreamID() string {
	return "stream-" + time.Now().Format("20060102-150405")
}

// validateStreamToken would validate WebSocket authentication tokens
func validateStreamToken(token string) (map[string]interface{}, error) {
	// Implementation would validate JWT token
	return map[string]interface{}{
		"user_id":   "example-user",
		"tenant_id": "example-tenant",
	}, nil
}

// createConnectionStore would set up the connection storage
func createConnectionStore() interface{} {
	// Implementation would return actual connection store
	return nil
}
