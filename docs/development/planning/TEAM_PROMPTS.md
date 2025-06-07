# Streamer Project Team Prompts

## Team 1: Infrastructure & Core Systems

### Mission
Build the foundational infrastructure, storage layer, and AWS Lambda integration for Streamer. Focus on reliability, scalability, and production-readiness.

### Primary Objectives

1. **Storage Layer & Models (Sprint 1)**
   - Implement DynamORM-based models for Connection, AsyncRequest, and Subscription
   - Create storage interfaces (ConnectionStore, RequestQueue) with full CRUD operations
   - Design efficient DynamoDB indexes for querying by user, tenant, and status
   - Ensure TTL-based cleanup for connections and requests
   - Achieve 90%+ test coverage for the storage layer

2. **Connection Management System (Sprint 2)**
   - Build WebSocket connection lifecycle handlers ($connect, $disconnect)
   - Implement JWT-based authentication and authorization
   - Create connection activity tracking and heartbeat mechanism
   - Design multi-tenant connection isolation
   - Build connection querying capabilities (by user, tenant, status)

3. **AWS Lambda Functions**
   - Develop optimized Lambda functions for connection, router, and processor
   - Implement cold start optimization strategies
   - Configure DynamoDB Streams integration for async processing
   - Set up IAM roles and permissions for cross-service communication
   - Create Lambda deployment scripts and infrastructure as code

4. **Production Features (Sprint 6)**
   - Implement comprehensive CloudWatch metrics and alarms
   - Add AWS X-Ray tracing for distributed debugging
   - Build rate limiting and request throttling
   - Create circuit breaker for resilience
   - Implement dead letter queue for failed requests
   - Design health check endpoints

### Technical Requirements
- Use Go 1.21+ with AWS SDK v2
- Leverage DynamORM for all database operations
- Follow AWS Well-Architected Framework principles
- Implement structured logging with correlation IDs
- Use context propagation for request tracking

### Deliverables
- Production-ready storage layer with migrations
- Lambda functions with < 100ms cold start
- Monitoring dashboard with key metrics
- Infrastructure as Code (CDK/Terraform)
- Operational runbooks

### Success Metrics
- Cold start latency < 100ms
- DynamoDB query performance < 10ms p99
- 99.9% availability for connection management
- Zero data loss for async requests
- Cost per connection-hour < $0.001

---

## Team 2: Application Layer & Developer Experience

### Mission
Build the request routing system, async processing engine, real-time updates, and create an exceptional developer experience through SDKs and documentation.

### Primary Objectives

1. **Request Router System (Sprint 3)**
   - Design and implement the Router interface with handler registry
   - Build message parsing and validation framework
   - Create sync/async decision logic based on expected duration
   - Implement request queuing mechanism for async operations
   - Design type-safe handler interfaces with generics

2. **Async Processing Engine (Sprint 4)**
   - Build DynamoDB Streams event processor
   - Implement progress reporting system with percentage and messages
   - Create retry logic with exponential backoff
   - Design timeout handling and cancellation
   - Build error handling and recovery mechanisms

3. **Real-time Update System (Sprint 5)**
   - Implement WebSocket notification service
   - Build subscription management for progress updates
   - Create efficient batch notification system
   - Design connection state synchronization
   - Implement progress aggregation for multiple operations

4. **Client SDKs & Developer Tools (Sprint 7)**
   - Create Go client library with fluent API
   - Build TypeScript/JavaScript SDK with TypeScript definitions
   - Implement React hooks for easy integration
   - Design reconnection logic with exponential backoff
   - Create request builders and response handlers

5. **Examples & Documentation (Sprint 8)**
   - Build comprehensive examples (chat app, report generation, ETL pipeline)
   - Create getting started guide and tutorials
   - Write API reference documentation
   - Design architecture diagrams and sequence flows
   - Create video tutorials for common use cases

### Technical Requirements
- Design APIs following REST/WebSocket best practices
- Implement progress reporting as event streams
- Use Protocol Buffers or JSON Schema for message validation
- Follow semantic versioning for SDKs
- Create comprehensive test suites with mocks

### Deliverables
- Type-safe request router with < 50ms latency
- Async processor handling 10K+ requests/second
- Real-time updates with < 100ms delivery
- Production-ready SDKs in Go and TypeScript
- Complete documentation site with examples

### Success Metrics
- Time to first successful request < 5 minutes
- Message processing latency < 50ms p99
- SDK adoption rate > 80% of users
- Documentation satisfaction score > 4.5/5
- Support ticket reduction > 70%

---

## Collaboration Points

### Shared Interfaces
- Storage layer APIs (Team 1 provides, Team 2 consumes)
- Lambda function contracts (Team 1 implements, Team 2 defines)
- Monitoring metrics (Both teams contribute)
- Error codes and types (Jointly defined)

### Integration Milestones
- Week 2: Storage layer ready for router integration
- Week 4: Lambda functions ready for async processor
- Week 5: WebSocket infrastructure ready for notifications
- Week 7: End-to-end testing with both teams

### Communication Protocol
- Daily standup for blockers and dependencies
- Weekly architecture review
- Shared Slack channel for real-time coordination
- Pull request reviews across teams
- Joint debugging sessions for integration issues

### Code Standards
- Shared linting and formatting rules
- Common error handling patterns
- Consistent logging format
- Unified testing approach
- Coordinated versioning strategy 