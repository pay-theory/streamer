# Streamer Demo Client

This is a demo WebSocket client for testing the Streamer async processing system.

## Quick Start

### 1. Setup Demo Data

First, create a test connection in DynamoDB:

```bash
cd ../scripts/demo
go run main.go
```

This will output a connection ID like `demo-conn-12345678`. Save this for the next step.

### 2. Install Dependencies

```bash
npm install
```

### 3. Run the Demo Client

```bash
# Replace with your actual values
export WS_URL="wss://your-api-id.execute-api.us-east-1.amazonaws.com/prod"
export CONNECTION_ID="demo-conn-12345678"  # From step 1

npm start
```

## Available Commands

Once connected, you can use these commands:

- `echo <message>` - Test synchronous echo
- `echo_async <message>` - Test async echo with progress updates
- `report` - Generate an async report with real-time progress
- `data` - Process data asynchronously
- `exit` - Close the connection

## Demo Flow

1. **Test Sync Echo** (verify basic connectivity):
   ```
   > echo Hello World
   ```

2. **Test Async Echo** (verify progress updates):
   ```
   > echo_async Testing async
   ```
   You should see a progress bar updating in real-time.

3. **Generate Report** (full async flow):
   ```
   > report
   ```
   This simulates a long-running report generation with multiple stages.

## Troubleshooting

### Connection Issues
- Verify the WebSocket URL is correct
- Check that the Lambda functions are deployed
- Ensure the connection ID exists in DynamoDB

### No Progress Updates
- Check CloudWatch logs for the processor Lambda
- Verify the connection is still active in DynamoDB
- Look for "410 Gone" errors in the logs

### Debug Mode

To see more details, check the Lambda logs:

```bash
# Router logs
aws logs tail /aws/lambda/router --follow

# Processor logs
aws logs tail /aws/lambda/processor --follow
```

## Demo Talk Track

1. **Show Connection**: "First, we connect to the WebSocket endpoint..."
2. **Sync Request**: "For small requests, we process synchronously..."
3. **Async Request**: "For long-running operations, we queue them..."
4. **Progress Updates**: "Notice the real-time progress updates..."
5. **Completion**: "And the final result with the S3 link..." 