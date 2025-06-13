.PHONY: build test clean build-lift test-lift compare

# Default target
all: build test

# Build the original handler
build:
	GOOS=linux GOARCH=amd64 go build -o bootstrap .
	zip disconnect-handler.zip bootstrap

# Build the Lift-based handler
build-lift:
	GOOS=linux GOARCH=amd64 go build -tags lift -o bootstrap-lift .
	zip disconnect-handler-lift.zip bootstrap-lift

# Test the original handler
test:
	go test -v -cover -run "Test[^L][^i][^f][^t]" ./...

# Test the Lift-based handler
test-lift:
	go test -v -cover -run "TestDisconnectHandlerLift" ./...

# Run all tests
test-all: test test-lift

# Clean build artifacts
clean:
	rm -f bootstrap bootstrap-lift *.zip

# Compare sizes
compare: build build-lift
	@echo "=== Size Comparison ==="
	@echo "Original handler:"
	@ls -lh disconnect-handler.zip
	@echo ""
	@echo "Lift-based handler:"
	@ls -lh disconnect-handler-lift.zip
	@echo ""
	@echo "=== Code Metrics ==="
	@echo "Original handler lines:"
	@wc -l handler.go main.go | tail -1
	@echo ""
	@echo "Lift handler lines:"
	@wc -l handler_lift.go main_lift.go | tail -1 