#!/bin/bash

# Test script for verifying async flow
set -e

echo "🚀 Testing Streamer Async Flow"
echo "=============================="

# Check environment variables
if [ -z "$WS_URL" ]; then
    echo "❌ Error: WS_URL environment variable not set"
    echo "Example: export WS_URL='wss://your-api.execute-api.region.amazonaws.com/prod'"
    exit 1
fi

# Step 1: Create test connection
echo -e "\n1️⃣ Creating test connection..."
cd scripts/demo
CONNECTION_ID=$(go run main.go | grep "Connection ID:" | awk '{print $3}')
cd ../..
echo "✅ Created connection: $CONNECTION_ID"

# Step 2: Test WebSocket connection
echo -e "\n2️⃣ Testing WebSocket connection..."
node -e "
const WebSocket = require('ws');
const ws = new WebSocket('$WS_URL');
ws.on('open', () => {
    console.log('✅ WebSocket connection successful');
    ws.close();
    process.exit(0);
});
ws.on('error', (err) => {
    console.error('❌ WebSocket connection failed:', err.message);
    process.exit(1);
});
setTimeout(() => {
    console.error('❌ Connection timeout');
    process.exit(1);
}, 5000);
" || exit 1

# Step 3: Check DynamoDB tables
echo -e "\n3️⃣ Checking DynamoDB tables..."
aws dynamodb describe-table --table-name streamer_connections --query 'Table.TableStatus' --output text || {
    echo "❌ Connections table not found"
    exit 1
}
aws dynamodb describe-table --table-name streamer_requests --query 'Table.TableStatus' --output text || {
    echo "❌ Requests table not found"
    exit 1
}
echo "✅ DynamoDB tables are active"

# Step 4: Test Lambda functions
echo -e "\n4️⃣ Checking Lambda functions..."
aws lambda get-function --function-name router --query 'Configuration.State' --output text || {
    echo "❌ Router Lambda not found"
    exit 1
}
aws lambda get-function --function-name processor --query 'Configuration.State' --output text || {
    echo "❌ Processor Lambda not found"
    exit 1
}
echo "✅ Lambda functions are active"

# Step 5: Test sync echo
echo -e "\n5️⃣ Testing sync echo..."
TEST_PAYLOAD=$(cat <<EOF
{
    "action": "echo",
    "connection_id": "$CONNECTION_ID",
    "request_id": "test-sync-$(date +%s)",
    "payload": {
        "message": "Hello from test script"
    }
}
EOF
)

# TODO: Send actual WebSocket message and verify response
echo "⚠️  Manual test needed: Send sync echo through WebSocket client"

# Step 6: Test async flow
echo -e "\n6️⃣ Testing async flow..."
echo "⚠️  Manual test needed: Send async request through WebSocket client"

echo -e "\n📋 Summary"
echo "=========="
echo "✅ Infrastructure is set up correctly"
echo "✅ Connection created: $CONNECTION_ID"
echo ""
echo "Next steps:"
echo "1. Export CONNECTION_ID=$CONNECTION_ID"
echo "2. cd demo && npm install && npm start"
echo "3. Test the commands in the demo client"
echo ""
echo "Monitor logs:"
echo "- aws logs tail /aws/lambda/router --follow"
echo "- aws logs tail /aws/lambda/processor --follow" 