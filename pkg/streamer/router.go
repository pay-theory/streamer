package streamer

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-lambda-go/events"
)

// Router handles incoming WebSocket messages and routes them to appropriate handlers
type Router interface {
	// Handle registers a handler for a specific action
	Handle(action string, handler Handler) error

	// Route processes an incoming WebSocket event
	Route(ctx context.Context, event events.APIGatewayWebsocketProxyRequest) error

	// SetAsyncThreshold sets the duration threshold for async processing
	SetAsyncThreshold(duration time.Duration)

	// SetMiddleware adds middleware to the router
	SetMiddleware(middleware ...Middleware)
}

// Middleware defines a function that wraps handler execution
type Middleware func(Handler) Handler

// RequestStore defines the interface for storing async requests
type RequestStore interface {
	Enqueue(ctx context.Context, request *Request) error
}

// ConnectionManager defines the interface for managing WebSocket connections
type ConnectionManager interface {
	Send(ctx context.Context, connectionID string, message interface{}) error
}

// DefaultRouter implements the Router interface
type DefaultRouter struct {
	handlers       map[string]Handler
	asyncThreshold time.Duration
	requestStore   RequestStore
	connManager    ConnectionManager
	middlewares    []Middleware
	mu             sync.RWMutex
}

// NewRouter creates a new router instance
func NewRouter(store RequestStore, connManager ConnectionManager) *DefaultRouter {
	return &DefaultRouter{
		handlers:       make(map[string]Handler),
		asyncThreshold: 5 * time.Second, // Default threshold
		requestStore:   store,
		connManager:    connManager,
		middlewares:    []Middleware{},
	}
}

// Handle registers a handler for a specific action
func (r *DefaultRouter) Handle(action string, handler Handler) error {
	if action == "" {
		return fmt.Errorf("action cannot be empty")
	}
	if handler == nil {
		return fmt.Errorf("handler cannot be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.handlers[action]; exists {
		return fmt.Errorf("handler already registered for action: %s", action)
	}

	// Apply middlewares to the handler
	wrappedHandler := handler
	for i := len(r.middlewares) - 1; i >= 0; i-- {
		wrappedHandler = r.middlewares[i](wrappedHandler)
	}

	r.handlers[action] = wrappedHandler
	return nil
}

// SetAsyncThreshold sets the duration threshold for async processing
func (r *DefaultRouter) SetAsyncThreshold(duration time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.asyncThreshold = duration
}

// SetMiddleware adds middleware to the router
func (r *DefaultRouter) SetMiddleware(middleware ...Middleware) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.middlewares = append(r.middlewares, middleware...)
}

// Route processes an incoming WebSocket event
func (r *DefaultRouter) Route(ctx context.Context, event events.APIGatewayWebsocketProxyRequest) error {
	// Parse the incoming message
	var message map[string]interface{}
	if err := json.Unmarshal([]byte(event.Body), &message); err != nil {
		return r.sendError(ctx, event.RequestContext.ConnectionID,
			NewError(ErrCodeValidation, "Invalid message format"))
	}

	// Extract action from message
	action, ok := message["action"].(string)
	if !ok || action == "" {
		return r.sendError(ctx, event.RequestContext.ConnectionID,
			NewError(ErrCodeValidation, "Missing or invalid action"))
	}

	// Create request object
	request := &Request{
		ID:           generateRequestID(),
		ConnectionID: event.RequestContext.ConnectionID,
		Action:       action,
		CreatedAt:    time.Now(),
		Metadata:     make(map[string]string),
	}

	// Extract request ID if provided
	if id, ok := message["id"].(string); ok {
		request.ID = id
	}

	// Extract metadata if provided
	if metadata, ok := message["metadata"].(map[string]interface{}); ok {
		for k, v := range metadata {
			if str, ok := v.(string); ok {
				request.Metadata[k] = str
			}
		}
	}

	// Extract payload
	if payload, exists := message["payload"]; exists {
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			return r.sendError(ctx, event.RequestContext.ConnectionID,
				NewError(ErrCodeValidation, "Invalid payload format"))
		}
		request.Payload = payloadBytes
	}

	// Get handler for action
	r.mu.RLock()
	handler, exists := r.handlers[action]
	r.mu.RUnlock()

	if !exists {
		return r.sendError(ctx, event.RequestContext.ConnectionID,
			NewError(ErrCodeInvalidAction, fmt.Sprintf("Unknown action: %s", action)))
	}

	// Validate request
	if err := handler.Validate(request); err != nil {
		return r.sendError(ctx, event.RequestContext.ConnectionID,
			NewError(ErrCodeValidation, err.Error()))
	}

	// Check if request should be processed async
	if handler.EstimatedDuration() > r.asyncThreshold {
		// Queue for async processing
		if err := r.requestStore.Enqueue(ctx, request); err != nil {
			return r.sendError(ctx, event.RequestContext.ConnectionID,
				NewError(ErrCodeInternalError, "Failed to queue request"))
		}

		// Send acknowledgment
		ack := map[string]interface{}{
			"type":       "acknowledgment",
			"request_id": request.ID,
			"status":     "queued",
			"message":    "Request queued for async processing",
		}
		return r.connManager.Send(ctx, event.RequestContext.ConnectionID, ack)
	}

	// Process synchronously
	result, err := handler.Process(ctx, request)
	if err != nil {
		return r.sendError(ctx, event.RequestContext.ConnectionID,
			NewError(ErrCodeInternalError, err.Error()))
	}

	// Send response
	response := map[string]interface{}{
		"type":       "response",
		"request_id": request.ID,
		"success":    result.Success,
		"data":       result.Data,
	}
	if result.Error != nil {
		response["error"] = result.Error
	}

	return r.connManager.Send(ctx, event.RequestContext.ConnectionID, response)
}

// sendError sends an error response to the client
func (r *DefaultRouter) sendError(ctx context.Context, connectionID string, err *Error) error {
	response := map[string]interface{}{
		"type":  "error",
		"error": err,
	}
	return r.connManager.Send(ctx, connectionID, response)
}

// generateRequestID generates a unique request ID
func generateRequestID() string {
	// In production, use a proper UUID generator
	return fmt.Sprintf("req_%d", time.Now().UnixNano())
}

// LoggingMiddleware adds logging to handler execution
func LoggingMiddleware(logger func(format string, args ...interface{})) Middleware {
	return func(next Handler) Handler {
		return &loggingHandler{
			Handler: next,
			logger:  logger,
		}
	}
}

type loggingHandler struct {
	Handler
	logger func(format string, args ...interface{})
}

func (h *loggingHandler) Process(ctx context.Context, request *Request) (*Result, error) {
	start := time.Now()
	h.logger("Processing request: %s, action: %s", request.ID, request.Action)

	result, err := h.Handler.Process(ctx, request)

	duration := time.Since(start)
	if err != nil {
		h.logger("Request failed: %s, duration: %v, error: %v", request.ID, duration, err)
	} else {
		h.logger("Request completed: %s, duration: %v, success: %v", request.ID, duration, result.Success)
	}

	return result, err
}
