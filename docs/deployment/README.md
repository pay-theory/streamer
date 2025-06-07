# Deployment Guide

## Prerequisites

- AWS Account with appropriate permissions
- Go 1.21+ installed
- Pulumi CLI installed
- AWS CLI configured
- Domain name for WebSocket endpoint (optional)

## Quick Deploy

### 1. Clone and Build

```bash
git clone https://github.com/pay-theory/streamer.git
cd streamer
make build
```

### 2. Configure Environment

Create `deployment/pulumi/Pulumi.prod.yaml`:

```yaml
config:
  aws:region: us-east-1
  streamer:environment: production
  streamer:domain: wss://api.example.com
  streamer:jwtPublicKey: |
    -----BEGIN PUBLIC KEY-----
    YOUR_PUBLIC_KEY_HERE
    -----END PUBLIC KEY-----
```

### 3. Deploy Infrastructure

```bash
cd deployment/pulumi
pulumi up -s production
```

This will create:
- DynamoDB tables with streams
- Lambda functions
- API Gateway WebSocket API
- IAM roles and policies
- CloudWatch log groups
- X-Ray tracing

## Detailed Setup

### DynamoDB Tables

The deployment creates three tables:

1. **Connections Table**
   ```bash
   aws dynamodb create-table \
     --table-name streamer-connections-prod \
     --attribute-definitions \
       AttributeName=ConnectionID,AttributeType=S \
       AttributeName=UserID,AttributeType=S \
       AttributeName=TenantID,AttributeType=S \
     --key-schema \
       AttributeName=ConnectionID,KeyType=HASH \
     --global-secondary-indexes \
       IndexName=UserIndex,Keys=[{AttributeName=UserID,KeyType=HASH}] \
       IndexName=TenantIndex,Keys=[{AttributeName=TenantID,KeyType=HASH}] \
     --billing-mode PAY_PER_REQUEST
   ```

2. **AsyncRequests Table**
   ```bash
   aws dynamodb create-table \
     --table-name streamer-requests-prod \
     --attribute-definitions \
       AttributeName=RequestID,AttributeType=S \
       AttributeName=ConnectionID,AttributeType=S \
       AttributeName=Status,AttributeType=S \
       AttributeName=CreatedAt,AttributeType=S \
     --key-schema \
       AttributeName=RequestID,KeyType=HASH \
     --stream-specification \
       StreamEnabled=true,StreamViewType=NEW_AND_OLD_IMAGES \
     --billing-mode PAY_PER_REQUEST
   ```

### Lambda Functions

Deploy each Lambda function:

```bash
# Build Lambda binaries
make lambda-build

# Deploy Connect Lambda
aws lambda create-function \
  --function-name streamer-connect-prod \
  --runtime provided.al2 \
  --role arn:aws:iam::ACCOUNT:role/streamer-lambda-role \
  --handler bootstrap \
  --zip-file fileb://bin/connect.zip \
  --environment Variables="{
    JWT_PUBLIC_KEY=$JWT_PUBLIC_KEY,
    CONNECTIONS_TABLE=streamer-connections-prod
  }"

# Deploy Disconnect Lambda
aws lambda create-function \
  --function-name streamer-disconnect-prod \
  --runtime provided.al2 \
  --role arn:aws:iam::ACCOUNT:role/streamer-lambda-role \
  --handler bootstrap \
  --zip-file fileb://bin/disconnect.zip

# Deploy Router Lambda
aws lambda create-function \
  --function-name streamer-router-prod \
  --runtime provided.al2 \
  --role arn:aws:iam::ACCOUNT:role/streamer-lambda-role \
  --handler bootstrap \
  --zip-file fileb://bin/router.zip \
  --environment Variables="{
    CONNECTIONS_TABLE=streamer-connections-prod,
    REQUESTS_TABLE=streamer-requests-prod,
    WEBSOCKET_ENDPOINT=$WEBSOCKET_ENDPOINT
  }"

# Deploy Processor Lambda
aws lambda create-function \
  --function-name streamer-processor-prod \
  --runtime provided.al2 \
  --role arn:aws:iam::ACCOUNT:role/streamer-lambda-role \
  --handler bootstrap \
  --zip-file fileb://bin/processor.zip \
  --timeout 900 \
  --memory-size 1024
```

### API Gateway Setup

Create WebSocket API:

```bash
# Create API
aws apigatewayv2 create-api \
  --name streamer-prod \
  --protocol-type WEBSOCKET \
  --route-selection-expression '$request.body.action'

# Add routes
aws apigatewayv2 create-route \
  --api-id $API_ID \
  --route-key '$connect' \
  --target integrations/$CONNECT_INTEGRATION_ID

aws apigatewayv2 create-route \
  --api-id $API_ID \
  --route-key '$disconnect' \
  --target integrations/$DISCONNECT_INTEGRATION_ID

aws apigatewayv2 create-route \
  --api-id $API_ID \
  --route-key '$default' \
  --target integrations/$ROUTER_INTEGRATION_ID

# Deploy
aws apigatewayv2 create-deployment \
  --api-id $API_ID \
  --stage-name prod
```

### DynamoDB Streams Trigger

```bash
# Get stream ARN
STREAM_ARN=$(aws dynamodb describe-table \
  --table-name streamer-requests-prod \
  --query 'Table.LatestStreamArn' --output text)

# Create trigger
aws lambda create-event-source-mapping \
  --function-name streamer-processor-prod \
  --event-source-arn $STREAM_ARN \
  --starting-position LATEST \
  --batch-size 10
```

## Environment Variables

### Required Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `JWT_PUBLIC_KEY` | Public key for JWT validation | RSA public key |
| `CONNECTIONS_TABLE` | DynamoDB table for connections | `streamer-connections-prod` |
| `REQUESTS_TABLE` | DynamoDB table for requests | `streamer-requests-prod` |
| `WEBSOCKET_ENDPOINT` | API Gateway endpoint | `https://xxx.execute-api.region.amazonaws.com/prod` |
| `AWS_REGION` | AWS region | `us-east-1` |

### Optional Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `LOG_LEVEL` | Logging verbosity | `INFO` |
| `CONNECTION_TTL_HOURS` | Connection expiry | `24` |
| `REQUEST_TTL_DAYS` | Request expiry | `7` |
| `MAX_CONNECTIONS_PER_USER` | Connection limit | `10` |

## Monitoring Setup

### CloudWatch Dashboards

Create a dashboard with key metrics:

```json
{
  "widgets": [
    {
      "type": "metric",
      "properties": {
        "metrics": [
          ["AWS/Lambda", "Invocations", {"stat": "Sum"}],
          [".", "Errors", {"stat": "Sum"}],
          [".", "Duration", {"stat": "Average"}],
          [".", "ConcurrentExecutions", {"stat": "Maximum"}]
        ],
        "period": 300,
        "stat": "Average",
        "region": "us-east-1",
        "title": "Lambda Performance"
      }
    }
  ]
}
```

### Alarms

Set up critical alarms:

```bash
# High error rate
aws cloudwatch put-metric-alarm \
  --alarm-name streamer-high-errors \
  --alarm-description "High Lambda error rate" \
  --metric-name Errors \
  --namespace AWS/Lambda \
  --statistic Sum \
  --period 300 \
  --evaluation-periods 2 \
  --threshold 10 \
  --comparison-operator GreaterThanThreshold

# Connection limit
aws cloudwatch put-metric-alarm \
  --alarm-name streamer-connection-limit \
  --alarm-description "Approaching connection limit" \
  --metric-name ActiveConnections \
  --namespace Streamer \
  --statistic Maximum \
  --period 300 \
  --evaluation-periods 1 \
  --threshold 9000 \
  --comparison-operator GreaterThanThreshold
```

## Security Considerations

### IAM Roles

Minimal Lambda execution role:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "dynamodb:PutItem",
        "dynamodb:GetItem",
        "dynamodb:DeleteItem",
        "dynamodb:Query",
        "dynamodb:UpdateItem"
      ],
      "Resource": [
        "arn:aws:dynamodb:*:*:table/streamer-*",
        "arn:aws:dynamodb:*:*:table/streamer-*/index/*"
      ]
    },
    {
      "Effect": "Allow",
      "Action": [
        "execute-api:ManageConnections"
      ],
      "Resource": "arn:aws:execute-api:*:*:*/prod/*"
    },
    {
      "Effect": "Allow",
      "Action": [
        "logs:CreateLogGroup",
        "logs:CreateLogStream",
        "logs:PutLogEvents"
      ],
      "Resource": "arn:aws:logs:*:*:*"
    },
    {
      "Effect": "Allow",
      "Action": [
        "xray:PutTraceSegments",
        "xray:PutTelemetryRecords"
      ],
      "Resource": "*"
    }
  ]
}
```

### Network Security

- Enable AWS WAF on API Gateway
- Configure rate limiting
- Use VPC endpoints for DynamoDB
- Enable API Gateway access logging

## Cost Optimization

### Estimated Monthly Costs

| Service | Usage | Cost |
|---------|-------|------|
| Lambda | 1M invocations | $2 |
| API Gateway | 1M messages | $1 |
| DynamoDB | 1GB storage, 1M R/W | $5 |
| CloudWatch | Logs & metrics | $10 |
| **Total** | | **~$18/month** |

### Cost Saving Tips

1. Use DynamoDB on-demand pricing
2. Set appropriate Lambda memory (512MB is usually enough)
3. Enable log expiration (30 days)
4. Use X-Ray sampling (10%)
5. Archive completed requests to S3

## Scaling Considerations

### Connection Limits
- API Gateway: 10,000 concurrent connections per API
- Lambda: 1,000 concurrent executions (soft limit)
- DynamoDB: No practical limit with on-demand

### Performance Tuning
1. Enable Lambda SnapStart for Java/Kotlin
2. Use provisioned concurrency for consistent performance
3. Optimize Lambda memory based on profiling
4. Consider DynamoDB auto-scaling for predictable workloads

## Troubleshooting

### Common Issues

1. **Lambda Cold Starts**
   - Solution: Provisioned concurrency or SnapStart
   - Monitor: X-Ray traces

2. **DynamoDB Throttling**
   - Solution: Switch to on-demand or increase capacity
   - Monitor: CloudWatch metrics

3. **WebSocket Disconnections**
   - Check: API Gateway logs
   - Verify: IAM permissions
   - Test: Connection TTL settings

### Debug Commands

```bash
# Check Lambda logs
aws logs tail /aws/lambda/streamer-router-prod --follow

# Test WebSocket connection
wscat -c wss://api.example.com/prod?Authorization=TOKEN

# Query DynamoDB
aws dynamodb query \
  --table-name streamer-connections-prod \
  --index-name UserIndex \
  --key-condition-expression "UserID = :uid" \
  --expression-attribute-values '{":uid":{"S":"user-123"}}'
```

## Maintenance

### Regular Tasks

- **Daily**: Check CloudWatch alarms
- **Weekly**: Review error logs and metrics
- **Monthly**: Analyze costs and optimize
- **Quarterly**: Update dependencies and runtime

### Backup Strategy

- DynamoDB point-in-time recovery enabled
- Lambda function versions preserved
- Infrastructure as code in Git
- Regular testing of restore procedures

## Support

For issues or questions:
1. Check CloudWatch logs
2. Review X-Ray traces
3. Consult documentation
4. Open GitHub issue
5. Contact support team 