# Lift Framework Integration Guide for Streamer
*Version: 1.0 | Date: 2025-06-13*
*Target: Streamer Team | Framework: Lift v1.0*

## 🎯 Overview

This guide provides comprehensive instructions for integrating the Streamer project with the Lift framework. Lift provides the HTTP/Lambda foundation while Streamer handles WebSocket-based async processing - a perfect architectural complement.

### **Integration Benefits** 🚀
- **Unified Request Handling**: Lift handles HTTP requests, Streamer handles WebSocket async processing
- **Shared Infrastructure**: Common observability, security, and compliance frameworks
- **Type Safety**: End-to-end type safety across both frameworks
- **Performance**: Maintain Lift's exceptional performance (2µs cold start) with Streamer's async capabilities
- **Enterprise Features**: Leverage Lift's compliance automation and testing frameworks

## 📋 Prerequisites

### **Lift Framework Status** ✅
- **Version**: v1.0 (Sprint 7 Complete)
- **Status**: Production-ready with enterprise compliance platform
- **Features**: Complete HTTP framework, security, observability, testing
- **Performance**: 2µs cold start, 2.5M req/sec throughput

### **Streamer Project Status** ✅
- **Version**: Current (100% Operational)
- **Status**: Production-ready async processing system
- **Features**: WebSocket infrastructure, JWT auth, progress tracking
- **Performance**: <50ms sync latency, 10K+ concurrent connections

## 🏗️ Integration Architecture

### **Recommended Architecture Pattern**
```
┌─────────────┐    HTTP     ┌─────────────┐    Async Queue    ┌─────────────┐
│   Client    │ ──────────> │    Lift     │ ───────────────> │  Streamer   │
│ Application │             │  Framework  │                   │  Processor  │
└─────────────┘             └─────────────┘                   └─────────────┘
                                   │                                 │
                                   │ <5s operations                  │ >5s operations
                                   │                                 │
                                   ▼                                 ▼
                            ┌─────────────┐                   ┌─────────────┐
                            │   Direct    │                   │  WebSocket  │
                            │  Response   │                   │  Progress   │
                            └─────────────┘                   └─────────────┘
```

### **Integration Points** 🔗
1. **HTTP to Async Handoff**: Lift determines sync vs async processing
2. **Shared Authentication**: JWT tokens validated by both frameworks
3. **Common Observability**: Shared CloudWatch, X-Ray, and metrics
4. **Unified Error Handling**: Consistent error responses across both systems
5. **Compliance Integration**: Shared audit trails and compliance monitoring

## 🚀 Implementation Guide

### **Step 1: Lift Handler Integration**

Create Lift handlers that integrate with Streamer for async operations:

```go
// handlers/async_handler.go
package handlers

import (
    "context"
    "time"
    
    "github.com/pay-theory/lift/pkg/lift"
    "github.com/pay-theory/streamer/pkg/streamer"
    "github.com/pay-theory/streamer/internal/store"
)

// AsyncHandler integrates Lift with Streamer for async processing
type AsyncHandler struct {
    streamerRouter *streamer.Router
    requestQueue   store.RequestQueue
    syncThreshold  time.Duration // Operations under this threshold run sync
}

// NewAsyncHandler creates a new async handler
func NewAsyncHandler(router *streamer.Router, queue store.RequestQueue) *AsyncHandler {
    return &AsyncHandler{
        streamerRouter: router,
        requestQueue:   queue,
        syncThreshold:  5 * time.Second, // API Gateway timeout consideration
    }
}

// ProcessRequest handles both sync and async operations
func (h *AsyncHandler) ProcessRequest(ctx *lift.Context) error {
    var req AsyncRequest
    if err := ctx.ParseRequest(&req); err != nil {
        return ctx.BadRequest("Invalid request format", err)
    }
    
    // Get handler for the action
    handler, exists := h.streamerRouter.GetHandler(req.Action)
    if !exists {
        return ctx.NotFound("Action not found", nil)
    }
    
    // Determine if operation should be sync or async
    estimatedDuration := handler.EstimatedDuration()
    
    if estimatedDuration < h.syncThreshold {
        // Process synchronously with Lift
        return h.processSyncWithLift(ctx, &req, handler)
    } else {
        // Queue for async processing with Streamer
        return h.queueAsyncWithStreamer(ctx, &req)
    }
}

// processSyncWithLift handles fast operations directly in Lift
func (h *AsyncHandler) processSyncWithLift(ctx *lift.Context, req *AsyncRequest, handler streamer.Handler) error {
    // Convert Lift context to Streamer request
    streamerReq := &streamer.Request{
        ID:           req.ID,
        ConnectionID: ctx.GetHeader("Connection-ID"), // From WebSocket if available
        Action:       req.Action,
        Payload:      req.Payload,
        Metadata:     extractMetadata(ctx),
        CreatedAt:    time.Now(),
    }
    
    // Validate request
    if err := handler.Validate(streamerReq); err != nil {
        return ctx.BadRequest("Validation failed", err)
    }
    
    // Process with timeout
    processCtx, cancel := context.WithTimeout(ctx.Request.Context(), h.syncThreshold)
    defer cancel()
    
    result, err := handler.Process(processCtx, streamerReq)
    if err != nil {
        return ctx.InternalError("Processing failed", err)
    }
    
    // Return result
    return ctx.JSON(SyncResponse{
        RequestID: result.RequestID,
        Success:   result.Success,
        Data:      result.Data,
        ProcessedSync: true,
    })
}

// queueAsyncWithStreamer queues long operations for async processing
func (h *AsyncHandler) queueAsyncWithStreamer(ctx *lift.Context, req *AsyncRequest) error {
    // Create async request for Streamer
    asyncReq := &store.AsyncRequest{
        RequestID:    req.ID,
        ConnectionID: ctx.GetHeader("Connection-ID"),
        Action:       req.Action,
        Status:       store.StatusPending,
        Payload:      convertPayload(req.Payload),
        CreatedAt:    time.Now(),
        UserID:       ctx.GetUserID(),
        TenantID:     ctx.GetTenantID(),
        TTL:          time.Now().Add(24 * time.Hour).Unix(),
    }
    
    // Queue for async processing
    if err := h.requestQueue.Enqueue(ctx.Request.Context(), asyncReq); err != nil {
        return ctx.InternalError("Failed to queue request", err)
    }
    
    // Return async response with WebSocket connection info
    return ctx.Accepted(AsyncResponse{
        RequestID: req.ID,
        Status:    "queued",
        Message:   "Request queued for async processing",
        WebSocketURL: buildWebSocketURL(ctx),
        EstimatedDuration: "5-30 minutes",
    })
}

// extractMetadata extracts relevant metadata from Lift context
func extractMetadata(ctx *lift.Context) map[string]string {
    metadata := make(map[string]string)
    
    // Extract user information
    if userID := ctx.GetUserID(); userID != "" {
        metadata["user_id"] = userID
    }
    if tenantID := ctx.GetTenantID(); tenantID != "" {
        metadata["tenant_id"] = tenantID
    }
    
    // Extract request metadata
    metadata["source_ip"] = ctx.GetClientIP()
    metadata["user_agent"] = ctx.GetHeader("User-Agent")
    metadata["request_id"] = ctx.GetRequestID()
    
    return metadata
}
```

### **Step 2: Shared Authentication Integration**

Integrate JWT authentication between Lift and Streamer:

```go
// middleware/shared_auth.go
package middleware

import (
    "github.com/pay-theory/lift/pkg/lift"
    "github.com/pay-theory/lift/pkg/middleware"
    "github.com/golang-jwt/jwt/v5"
)

// SharedJWTAuth creates JWT middleware compatible with Streamer
func SharedJWTAuth(config JWTConfig) lift.Middleware {
    return middleware.JWT(middleware.JWTConfig{
        SigningKey:    config.SigningKey,
        TokenLookup:   "header:Authorization,query:token,cookie:jwt",
        AuthScheme:    "Bearer",
        ContextKey:    "user",
        
        // Custom claims validation for Streamer compatibility
        Claims: func() jwt.Claims {
            return &StreamerClaims{}
        },
        
        // Success handler that sets Streamer-compatible context
        SuccessHandler: func(ctx *lift.Context) {
            claims := ctx.Get("user").(*StreamerClaims)
            
            // Set context values that Streamer expects
            ctx.Set("user_id", claims.UserID)
            ctx.Set("tenant_id", claims.TenantID)
            ctx.Set("connection_id", claims.ConnectionID)
            
            // Set headers for downstream services
            ctx.SetHeader("X-User-ID", claims.UserID)
            ctx.SetHeader("X-Tenant-ID", claims.TenantID)
        },
    })
}

// StreamerClaims defines JWT claims compatible with Streamer
type StreamerClaims struct {
    UserID       string `json:"user_id"`
    TenantID     string `json:"tenant_id"`
    ConnectionID string `json:"connection_id,omitempty"`
    Permissions  []string `json:"permissions"`
    jwt.RegisteredClaims
}
```

### **Step 3: Observability Integration**

Integrate Lift's observability with Streamer's monitoring:

```go
// observability/integration.go
package observability

import (
    "github.com/pay-theory/lift/pkg/observability"
    "github.com/pay-theory/lift/pkg/middleware"
    "github.com/pay-theory/streamer/pkg/connection"
)

// IntegratedObservability sets up unified monitoring
func IntegratedObservability() lift.Middleware {
    return middleware.Compose(
        // Lift's observability middleware
        observability.CloudWatch(observability.Config{
            Namespace: "PayTheory/Streamer",
            Metrics: []string{
                "request_count",
                "request_duration",
                "async_queue_size",
                "websocket_connections",
            },
        }),
        
        // X-Ray tracing with Streamer context
        observability.XRay(observability.XRayConfig{
            ServiceName: "streamer-api",
            Annotations: map[string]string{
                "component": "lift-streamer-integration",
            },
        }),
        
        // Custom metrics for async operations
        func(next lift.Handler) lift.Handler {
            return lift.HandlerFunc(func(ctx *lift.Context) error {
                // Track async vs sync processing
                if ctx.Get("processing_mode") == "async" {
                    observability.IncrementCounter("async_requests_queued")
                } else {
                    observability.IncrementCounter("sync_requests_processed")
                }
                
                return next.Handle(ctx)
            })
        },
    )
}
```

### **Step 4: Error Handling Integration**

Create unified error handling across both frameworks:

```go
// errors/integration.go
package errors

import (
    "github.com/pay-theory/lift/pkg/lift"
    "github.com/pay-theory/streamer/pkg/streamer"
)

// UnifiedErrorHandler handles errors consistently across Lift and Streamer
func UnifiedErrorHandler() lift.Middleware {
    return func(next lift.Handler) lift.Handler {
        return lift.HandlerFunc(func(ctx *lift.Context) error {
            err := next.Handle(ctx)
            if err == nil {
                return nil
            }
            
            // Convert Streamer errors to Lift responses
            if streamerErr, ok := err.(*streamer.Error); ok {
                return handleStreamerError(ctx, streamerErr)
            }
            
            // Handle other errors normally
            return err
        })
    }
}

// handleStreamerError converts Streamer errors to appropriate Lift responses
func handleStreamerError(ctx *lift.Context, err *streamer.Error) error {
    switch err.Code {
    case streamer.ErrCodeValidation:
        return ctx.BadRequest(err.Message, err.Details)
    case streamer.ErrCodeNotFound:
        return ctx.NotFound(err.Message, err.Details)
    case streamer.ErrCodeUnauthorized:
        return ctx.Unauthorized(err.Message, err.Details)
    case streamer.ErrCodeRateLimited:
        return ctx.TooManyRequests(err.Message, err.Details)
    case streamer.ErrCodeTimeout:
        return ctx.RequestTimeout(err.Message, err.Details)
    default:
        return ctx.InternalError(err.Message, err.Details)
    }
}
```

## 🔧 Configuration & Setup

### **Step 5: Application Setup**

Create a unified application that uses both Lift and Streamer:

```go
// main.go
package main

import (
    "log"
    "time"
    
    "github.com/pay-theory/lift/pkg/lift"
    "github.com/pay-theory/lift/pkg/middleware"
    "github.com/pay-theory/streamer/pkg/streamer"
    "github.com/pay-theory/streamer/internal/store"
)

func main() {
    // Initialize Lift app
    app := lift.New()
    
    // Add Lift middleware stack
    app.Use(
        middleware.CORS(),
        middleware.RequestID(),
        SharedJWTAuth(jwtConfig),
        IntegratedObservability(),
        UnifiedErrorHandler(),
        middleware.RateLimiting(rateLimitConfig),
    )
    
    // Initialize Streamer components
    streamerRouter := streamer.NewRouter()
    requestQueue := store.NewDynamoDBQueue(dynamoConfig)
    
    // Register Streamer handlers
    registerStreamerHandlers(streamerRouter)
    
    // Create integrated async handler
    asyncHandler := NewAsyncHandler(streamerRouter, requestQueue)
    
    // Define API routes
    api := app.Group("/api/v1")
    
    // Sync/Async processing endpoint
    api.POST("/process", asyncHandler.ProcessRequest)
    
    // Status and monitoring endpoints
    api.GET("/status/:request_id", asyncHandler.GetRequestStatus)
    api.GET("/health", healthCheck)
    
    // WebSocket info endpoint
    api.GET("/websocket/info", getWebSocketInfo)
    
    // Start the application
    app.Start()
}

// registerStreamerHandlers registers all Streamer handlers
func registerStreamerHandlers(router *streamer.Router) {
    // Register your Streamer handlers here
    router.Register("generate_report", NewReportHandler())
    router.Register("process_data", NewDataProcessingHandler())
    router.Register("export_data", NewExportHandler())
}
```

### **Step 6: Testing Integration**

Create comprehensive tests for the integration:

```go
// integration_test.go
package main

import (
    "testing"
    "time"
    
    "github.com/pay-theory/lift/pkg/testing"
    "github.com/stretchr/testify/assert"
)

func TestLiftStreamerIntegration(t *testing.T) {
    // Setup test app
    app := setupTestApp()
    testApp := testing.NewTestApp(app)
    
    t.Run("Sync Processing", func(t *testing.T) {
        // Test fast operations processed synchronously
        resp := testApp.POST("/api/v1/process").
            WithJSON(map[string]interface{}{
                "id":     "test-123",
                "action": "echo",
                "payload": map[string]string{"message": "hello"},
            }).
            Expect()
        
        resp.Status(200)
        resp.JSON().Path("$.processed_sync").Equal(true)
        resp.JSON().Path("$.data.message").Equal("hello")
    })
    
    t.Run("Async Processing", func(t *testing.T) {
        // Test slow operations queued for async processing
        resp := testApp.POST("/api/v1/process").
            WithJSON(map[string]interface{}{
                "id":     "test-456",
                "action": "long_operation",
                "payload": map[string]string{"duration": "10s"},
            }).
            Expect()
        
        resp.Status(202)
        resp.JSON().Path("$.status").Equal("queued")
        resp.JSON().Path("$.websocket_url").NotNull()
    })
    
    t.Run("Error Handling", func(t *testing.T) {
        // Test error handling integration
        resp := testApp.POST("/api/v1/process").
            WithJSON(map[string]interface{}{
                "id":     "test-789",
                "action": "invalid_action",
            }).
            Expect()
        
        resp.Status(404)
        resp.JSON().Path("$.error.message").Equal("Action not found")
    })
}
```

## 📊 Performance Considerations

### **Optimization Guidelines** ⚡

1. **Sync/Async Threshold**: Set appropriate threshold (recommended: 5 seconds)
2. **Connection Pooling**: Reuse DynamoDB connections between Lift and Streamer
3. **Memory Management**: Share memory pools where possible
4. **Caching**: Use Lift's caching middleware for frequently accessed data
5. **Monitoring**: Track both sync and async operation metrics

### **Expected Performance** 📈
- **Sync Operations**: Maintain Lift's 2µs cold start performance
- **Async Handoff**: <10ms to queue async operations
- **WebSocket Delivery**: <100ms for progress updates
- **Memory Usage**: <50MB combined overhead
- **Throughput**: 2.5M sync req/sec, 10K+ concurrent async operations

## 🛡️ Security & Compliance

### **Shared Security Framework** 🔒

Leverage Lift's enterprise security features:

```go
// security/integration.go
package security

import (
    "github.com/pay-theory/lift/pkg/security"
    "github.com/pay-theory/lift/pkg/middleware"
)

// IntegratedSecurity applies Lift's security framework to Streamer operations
func IntegratedSecurity() lift.Middleware {
    return middleware.Compose(
        // OWASP security controls
        security.OWASP(security.OWASPConfig{
            EnableAll: true,
        }),
        
        // Compliance automation
        security.Compliance(security.ComplianceConfig{
            Frameworks: []string{"SOC2", "GDPR", "PCI-DSS"},
            AuditLevel: "FULL",
        }),
        
        // Data protection
        security.DataProtection(security.DataProtectionConfig{
            Classification: security.CONFIDENTIAL,
            Encryption:    true,
        }),
    )
}
```

## 📚 Best Practices

### **Development Guidelines** 📋

1. **Handler Design**: Keep Streamer handlers focused on business logic
2. **Error Propagation**: Use consistent error codes across both frameworks
3. **Logging**: Use structured logging with correlation IDs
4. **Testing**: Test both sync and async paths thoroughly
5. **Monitoring**: Monitor queue depth and processing times
6. **Documentation**: Document sync/async decision criteria

### **Deployment Guidelines** 🚀

1. **Environment Parity**: Use same configuration for both frameworks
2. **Scaling**: Configure auto-scaling for both HTTP and WebSocket traffic
3. **Monitoring**: Set up alerts for queue depth and processing failures
4. **Rollback**: Plan rollback strategy for both components
5. **Health Checks**: Implement health checks for both frameworks

## 🔍 Troubleshooting

### **Common Issues** ⚠️

1. **Authentication Mismatch**: Ensure JWT claims are compatible
2. **Timeout Issues**: Verify sync/async threshold configuration
3. **Queue Backlog**: Monitor DynamoDB capacity and scaling
4. **WebSocket Disconnections**: Implement reconnection logic
5. **Memory Leaks**: Monitor memory usage in long-running operations

### **Debugging Tools** 🔧

1. **Lift Dashboard**: Monitor HTTP request metrics
2. **Streamer Monitoring**: Track WebSocket connections and queue depth
3. **CloudWatch Logs**: Unified logging across both frameworks
4. **X-Ray Tracing**: End-to-end request tracing
5. **Performance Profiling**: Use Lift's built-in profiling tools

## 📞 Support & Resources

### **Documentation** 📖
- [Lift Framework Documentation](../../../lift/docs/)
- [Streamer API Reference](../api/)
- [Integration Examples](../../examples/)

### **Team Contacts** 👥
- **Lift Framework Team**: Core framework support
- **Streamer Team**: WebSocket and async processing
- **Platform Team**: Infrastructure and deployment

### **Next Steps** ➡️
1. Review this integration guide
2. Set up development environment
3. Implement basic integration
4. Add comprehensive testing
5. Deploy to staging environment
6. Monitor and optimize performance

---

**Integration Status**: Ready for Implementation  
**Framework Compatibility**: Lift v1.0 + Streamer v1.0  
**Last Updated**: 2025-06-13 