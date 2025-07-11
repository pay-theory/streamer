# Streamer Demo Client

**🔥 Updated for DynamORM Architecture**

This is a demo WebSocket client for testing the Streamer async processing system using DynamORM single-model architecture.

## Quick Start

### 1. Generate JWT Token

First, create a JWT token for authentication:

```bash
cd ../scripts/demo
go run generate_jwt.go
```

This will output a JWT token. Save this for authentication.

### 2. Setup Demo Data

Create test data in DynamoDB using DynamORM models:

```bash
cd ../scripts/demo
go run main.go
```

This will output a connection ID like `demo-conn-12345678`. Save this for the next step.

### 3. Install Dependencies

```bash
npm install
```

### 4. Run the Demo Client

```bash
# Replace with your actual values
export WS_URL="wss://your-api-id.execute-api.us-east-1.amazonaws.com/prod"
export CONNECTION_ID="demo-conn-12345678"  # From step 2
export JWT_TOKEN="your-jwt-token-here"     # From step 1

npm start
```

## Available Commands

Once connected, you can use these commands:

- `ping` - Fast sync ping (< 5s) 
- `echo <message>` - Test synchronous echo
- `echo_async <message>` - Test async echo with progress updates
- `report` - Generate an async report with real-time progress
- `knowledge <query>` - Query knowledge base async
- `data` - Process data asynchronously
- `exit` - Close the connection

## Demo Flow

1. **Test Basic Connectivity** (fast sync operation):
   ```
   > ping
   ```

2. **Test Sync Echo** (verify basic connectivity):
   ```
   > echo Hello World
   ```

3. **Test Async Echo** (verify progress updates):
   ```
   > echo_async Testing async
   ```
   You should see a progress bar updating in real-time.

4. **Generate Report** (full async flow):
   ```
   > report
   ```
   This simulates a long-running report generation with multiple stages.

5. **Query Knowledge Base** (async with progress):
   ```
   > knowledge What is async processing?
   ```

## Troubleshooting

### Connection Issues
- Verify the WebSocket URL is correct
- Check that JWT token is not expired
- Ensure the Lambda functions are deployed
- Verify the connection ID exists in DynamoDB using DynamORM models

### Authentication Issues
- Ensure JWT_TOKEN environment variable is set
- Check token contains required claims (sub, tenant_id)
- Verify token is not expired

### No Progress Updates
- Check CloudWatch logs for the processor Lambda
- Verify the connection is still active in DynamoDB
- Look for "410 Gone" errors in the logs
- Ensure DynamoDB Streams is enabled for async processing

### Debug Mode

To see more details, check the Lambda logs:

```bash
# Router logs (WebSocket and sync processing)
aws logs tail /aws/lambda/streamer-router-prod --follow

# Processor logs (async processing with DynamORM)
aws logs tail /aws/lambda/streamer-processor-prod --follow
```

## Demo Talk Track

1. **Show Connection**: "First, we connect to the WebSocket endpoint using JWT authentication..."
2. **Ping Test**: "Let's start with a fast ping to verify connectivity..."
3. **Sync Request**: "For small requests under 5 seconds, we process synchronously..."
4. **Async Request**: "For long-running operations, we queue them using DynamORM..."
5. **Progress Updates**: "Notice the real-time progress updates via WebSocket..."
6. **Completion**: "And the final result with links to generated files..."
7. **Architecture**: "This demonstrates DynamORM's single-model approach for async processing..." 