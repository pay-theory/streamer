.PHONY: build clean deploy test

# Variables
STACK_NAME ?= streamer-websocket
SAM_BUCKET ?= your-sam-deployment-bucket
REGION ?= us-east-1

# Build all Lambda functions
build:
	@echo "Building Lambda functions..."
	@for dir in connect disconnect router processor; do \
		if [ -d "$$dir" ]; then \
			echo "Building $$dir..."; \
			cd $$dir && GOOS=linux GOARCH=amd64 go build -o main . && cd ..; \
		fi \
	done
	@echo "Build complete!"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@for dir in connect disconnect router processor; do \
		if [ -f "$$dir/main" ]; then \
			rm -f "$$dir/main"; \
		fi \
	done
	@rm -rf .aws-sam
	@echo "Clean complete!"

# Run tests
test:
	@echo "Running tests..."
	go test ../pkg/connection/... -v
	go test ./shared/... -v

# Deploy using SAM
deploy: build
	@echo "Deploying to AWS..."
	sam deploy \
		--template-file template.yaml \
		--stack-name $(STACK_NAME) \
		--s3-bucket $(SAM_BUCKET) \
		--capabilities CAPABILITY_IAM \
		--region $(REGION) \
		--parameter-overrides \
			JWTSecret=$(JWT_SECRET) \
			TablePrefix=$(TABLE_PREFIX)

# Deploy with guided prompts
deploy-guided: build
	sam deploy --guided

# Validate SAM template
validate:
	sam validate --template template.yaml

# Local testing with SAM
local-start:
	sam local start-lambda

# Package for deployment
package: build
	sam package \
		--template-file template.yaml \
		--s3-bucket $(SAM_BUCKET) \
		--output-template-file packaged.yaml

# Show stack outputs
outputs:
	@aws cloudformation describe-stacks \
		--stack-name $(STACK_NAME) \
		--query 'Stacks[0].Outputs' \
		--output table

# Tail Lambda logs
logs-connect:
	sam logs -n ConnectFunction --stack-name $(STACK_NAME) --tail

logs-disconnect:
	sam logs -n DisconnectFunction --stack-name $(STACK_NAME) --tail

logs-router:
	sam logs -n RouterFunction --stack-name $(STACK_NAME) --tail

# Delete stack
delete:
	@echo "Deleting stack $(STACK_NAME)..."
	aws cloudformation delete-stack --stack-name $(STACK_NAME)
	@echo "Stack deletion initiated"

# Help
help:
	@echo "Available targets:"
	@echo "  build         - Build all Lambda functions"
	@echo "  clean         - Clean build artifacts"
	@echo "  test          - Run tests"
	@echo "  deploy        - Deploy to AWS using SAM"
	@echo "  deploy-guided - Deploy with interactive prompts"
	@echo "  validate      - Validate SAM template"
	@echo "  local-start   - Start local Lambda runtime"
	@echo "  package       - Package for deployment"
	@echo "  outputs       - Show stack outputs"
	@echo "  logs-*        - Tail logs for specific function"
	@echo "  delete        - Delete the stack"
	@echo ""
	@echo "Environment variables:"
	@echo "  STACK_NAME    - CloudFormation stack name (default: streamer-websocket)"
	@echo "  SAM_BUCKET    - S3 bucket for SAM deployments"
	@echo "  REGION        - AWS region (default: us-east-1)"
	@echo "  JWT_SECRET    - JWT secret for authentication"
	@echo "  TABLE_PREFIX  - DynamoDB table prefix" 