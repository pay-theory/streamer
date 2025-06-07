# Sprint Kickoff Prompts

> **Note**: Week 1 is complete! See [PROGRESS_REVIEW_WEEK1.md](./PROGRESS_REVIEW_WEEK1.md) for results.
> 
> **Current Sprint**: Week 2 - See [WEEK2_SPRINT_PROMPTS.md](./WEEK2_SPRINT_PROMPTS.md) for active prompts.

## ✅ COMPLETED - Team 1 - Sprint 1: Storage Layer Implementation

### Prompt
"You are building the storage layer for Streamer, an async request processing system for AWS Lambda. Using DynamORM (a DynamoDB ORM), implement the following:

1. **Connection Model**
   ```go
   type Connection struct {
       ConnectionID string            `dynamorm:"pk"`
       UserID       string            `dynamorm:"index:user-connections,pk"`
       TenantID     string            `dynamorm:"index:tenant-connections,pk"`
       Endpoint     string            
       ConnectedAt  time.Time         
       LastPing     time.Time         
       Metadata     map[string]string `dynamorm:"metadata,json"`
       TTL          int64             `dynamorm:"ttl"`
   }
   ```

2. **AsyncRequest Model**
   ```go
   type AsyncRequest struct {
       RequestID    string                 `dynamorm:"pk"`
       ConnectionID string                 `dynamorm:"index:connection-requests,pk"`
       Status       string                 `dynamorm:"index:status-time,pk"`
       CreatedAt    time.Time              `dynamorm:"index:status-time,sk"`
       Action       string                 
       Payload      map[string]interface{} `dynamorm:"payload,json"`
       Result       map[string]interface{} `dynamorm:"result,json"`
       TTL          int64                  `dynamorm:"ttl"`
   }
   ```

3. **Storage Interfaces**
   - ConnectionStore: Save, Get, Delete, ListByUser, ListByTenant, UpdateLastPing
   - RequestQueue: Enqueue, Dequeue, UpdateStatus, GetByConnection, GetByStatus

4. **Requirements**
   - Use DynamoDB best practices (single table design if appropriate)
   - Implement efficient queries using GSIs
   - Add proper error handling with custom error types
   - Include comprehensive unit tests with mocked DynamoDB
   - Set up table initialization scripts

Create the storage layer in `internal/store/` with proper separation of concerns and 90%+ test coverage."

---

## 🏗️ IN PROGRESS - Team 2 - Sprint 3: Request Router Implementation (80% Complete)

### Prompt
"You are implementing the request router for Streamer, which receives WebSocket messages and routes them to appropriate handlers. Build the following:

1. **Router Interface**
   ```go
   type Router interface {
       Handle(action string, handler Handler) error
       Route(ctx context.Context, event APIGatewayWebsocketProxyRequest) error
       SetAsyncThreshold(duration time.Duration)
   }
   ```

2. **Handler Interface**
   ```go
   type Handler interface {
       Validate(request *Request) error
       EstimatedDuration() time.Duration
       Process(ctx context.Context, request *Request) (*Result, error)
   }
   ```

3. **Core Features**
   - Message parsing from WebSocket events
   - Handler registry with action mapping
   - Sync vs async decision logic (based on EstimatedDuration)
   - Request validation framework
   - For async requests: queue to DynamoDB and return immediate acknowledgment
   - For sync requests: process immediately and return result

4. **Request/Response Types**
   ```go
   type Request struct {
       ID           string
       ConnectionID string
       Action       string
       Payload      json.RawMessage
       Metadata     map[string]string
   }

   type Result struct {
       Success bool
       Data    interface{}
       Error   *Error
   }
   ```

5. **Requirements**
   - Type-safe handler registration
   - Comprehensive error handling with WebSocket error responses
   - Request validation with clear error messages
   - Middleware support for auth, logging, metrics
   - Unit tests with mocked dependencies

Create the router in `pkg/streamer/` with clean API design focusing on developer experience."

---

## Team 1 - Week 2 Quick Start: Lambda Functions

### Prompt
"Create the AWS Lambda function handlers for WebSocket connection management:

1. **Connect Handler** (`lambda/connect/`)
   - Authenticate JWT tokens from query parameters
   - Extract user/tenant information
   - Save connection to DynamoDB using the storage layer
   - Return appropriate status codes

2. **Disconnect Handler** (`lambda/disconnect/`)
   - Clean up connection from DynamoDB
   - Cancel any pending subscriptions
   - Log disconnection metrics

3. **Requirements**
   - Optimize for cold starts (minimal dependencies)
   - Use AWS Lambda Powertools for logging/tracing
   - Handle API Gateway WebSocket events properly
   - Include deployment configuration (SAM/CDK)

Focus on production-readiness with proper error handling and observability."

---

## Team 2 - Week 4 Quick Start: Async Processor

### Prompt
"Build the DynamoDB Streams processor that handles async requests:

1. **Stream Processor** (`lambda/processor/`)
   - Process DynamoDB Stream events for new AsyncRequests
   - Look up and execute the appropriate handler
   - Implement progress reporting during execution
   - Update request status and results in DynamoDB
   - Send real-time updates via WebSocket

2. **Progress Reporter**
   ```go
   type ProgressReporter interface {
       Report(percentage float64, message string) error
       SetMetadata(key string, value interface{}) error
   }
   ```

3. **Requirements**
   - Handle batch processing from streams efficiently
   - Implement retry logic with exponential backoff
   - Add timeout handling for long-running operations
   - Dead letter queue for failed requests
   - Comprehensive error handling and recovery

Create a robust async processing system that maintains progress visibility throughout the operation lifecycle." 