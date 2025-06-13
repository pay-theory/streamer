.PHONY: build test clean build-lift test-lift compare

# Default target
all: build test

# Build the original handler
build:
	GOOS=linux GOARCH=amd64 go build -o bootstrap .
	zip router-handler.zip bootstrap

# Build the Lift-based handler
build-lift:
	GOOS=linux GOARCH=amd64 go build -tags lift -o bootstrap-lift .
	zip router-handler-lift.zip bootstrap-lift

# Test the original handler
test:
	go test -v -cover -run "Test[^L][^i][^f][^t]" ./...

# Test the Lift-based handler
test-lift:
	go test -v -cover -run "TestRouterHandlerLift" ./...

# Run all tests
test-all: test test-lift

# Clean build artifacts
clean:
	rm -f bootstrap bootstrap-lift *.zip coverage.out router

# Compare sizes
compare: build build-lift
	@echo "=== Size Comparison ==="
	@echo "Original handler:"
	@ls -lh router-handler.zip
	@echo ""
	@echo "Lift-based handler:"
	@ls -lh router-handler-lift.zip
	@echo ""
	@echo "=== Code Metrics ==="
	@echo "Original handler lines:"
	@wc -l main.go handlers.go | tail -1
	@echo ""
	@echo "Lift handler lines:"
	@wc -l main_lift.go handler_lift.go | tail -1 