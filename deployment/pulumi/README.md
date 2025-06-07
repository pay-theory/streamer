# Streamer Infrastructure Deployment

This directory contains Pulumi infrastructure-as-code for deploying the Streamer WebSocket system.

## Prerequisites

1. Install Pulumi: https://www.pulumi.com/docs/get-started/install/
2. Configure AWS credentials
3. Create JWT public/private key pair for authentication

## Directory Structure

```
deployment/pulumi/
├── main.go           # Main Pulumi program
├── iam.go            # IAM roles and policies
├── lambda.go         # Lambda function definitions
├── apigateway.go     # API Gateway WebSocket configuration
├── alarms.go         # CloudWatch alarms
├── Pulumi.yaml       # Project configuration
├── Pulumi.dev.yaml   # Development environment
├── Pulumi.staging.yaml    # Staging environment
└── Pulumi.production.yaml # Production environment
```

## Configuration

Each environment has optimized settings:

### Development
- Lambda Memory: 3008 MB (2 vCPUs)
- Reserved Concurrency: 10
- Log Retention: 7 days
- Detailed logging and tracing

### Staging
- Lambda Memory: 3008 MB (2 vCPUs)
- Reserved Concurrency: 50
- Log Retention: 30 days
- Production-like configuration

### Production
- Lambda Memory: 3008 MB (2 vCPUs)
- Reserved Concurrency: 100
- Log Retention: 90 days
- Optimized for performance

## Deployment Steps

### 1. Initialize Pulumi Stack

```bash
# For development
pulumi stack init dev
pulumi config set aws:region us-east-1

# For staging
pulumi stack init staging

# For production
pulumi stack init production
```

### 2. Set JWT Keys

```bash
# Generate keys if needed
openssl genrsa -out private.pem 2048
openssl rsa -in private.pem -pubout -out public.pem

# Set in AWS Secrets Manager (via Pulumi)
pulumi config set --secret jwtPublicKey "$(cat public.pem)"
pulumi config set --secret jwtPrivateKey "$(cat private.pem)"
```

### 3. Build Lambda Functions

```bash
# From project root
make build-lambdas
```

### 4. Deploy Infrastructure

```bash
# Preview changes
pulumi preview

# Deploy
pulumi up

# Get outputs
pulumi stack output
```

## Outputs

After deployment, Pulumi will output:

- `apiEndpoint`: WebSocket API endpoint URL
- `connectFunctionArn`: Connect Lambda ARN
- `disconnectFunctionArn`: Disconnect Lambda ARN
- `routerFunctionArn`: Router Lambda ARN
- `processorFunctionArn`: Processor Lambda ARN
- `connectionsTableName`: DynamoDB connections table
- `subscriptionsTableName`: DynamoDB subscriptions table
- `requestsTableName`: DynamoDB requests table

## Security Features

1. **Encryption at Rest**
   - KMS encryption for DynamoDB
   - KMS encryption for CloudWatch Logs
   - Encrypted Secrets Manager

2. **Least Privilege IAM**
   - Function-specific roles
   - Minimal permissions per function
   - No wildcard resources

3. **Network Security**
   - API Gateway throttling
   - Reserved concurrency limits
   - DDoS protection via AWS Shield

4. **Monitoring**
   - X-Ray tracing enabled
   - CloudWatch metrics
   - Automated alarms

## Cost Optimization

1. **Lambda Memory: 3008 MB**
   - Provides 2 vCPUs
   - Faster execution = lower duration costs
   - Prevents memory errors

2. **DynamoDB: On-Demand**
   - Pay per request
   - Auto-scaling
   - No idle costs

3. **Log Retention**
   - Environment-specific
   - Automatic cleanup

## Monitoring

### CloudWatch Dashboards

Create a dashboard with:
- Connection metrics
- Message throughput
- Error rates
- Latency percentiles

### Alarms

Configured alarms:
- High authentication failures
- High message failures
- High processing latency
- Lambda errors/throttles
- High concurrent executions

### X-Ray Service Map

View the complete request flow:
1. API Gateway → Connect Lambda → DynamoDB
2. API Gateway → Router Lambda → DynamoDB/SQS
3. API Gateway → Disconnect Lambda → DynamoDB

## Troubleshooting

### Connection Issues
1. Check CloudWatch logs in `/aws/lambda/streamer-connect-{env}`
2. Verify JWT token in X-Ray traces
3. Check DynamoDB for connection record

### Performance Issues
1. Review X-Ray traces for bottlenecks
2. Check Lambda concurrent executions
3. Monitor DynamoDB throttling

### Cost Issues
1. Review Lambda duration metrics
2. Check DynamoDB consumed capacity
3. Analyze log volume

## Cleanup

To remove all resources:

```bash
pulumi destroy
pulumi stack rm
```

## CI/CD Integration

Example GitHub Actions workflow:

```yaml
- name: Deploy to Staging
  run: |
    pulumi stack select staging
    pulumi up --yes
``` 