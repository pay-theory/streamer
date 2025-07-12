// Example: Simple External Processor using public models
// This shows how external teams can build processors using Streamer's public models
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/pay-theory/dynamorm"
	"github.com/pay-theory/streamer/pkg/models"
)

var logger *log.Logger

func init() {
	logger = log.New(os.Stdout, "[PROCESSOR-EXAMPLE] ", log.LstdFlags)
}

func handler(ctx context.Context, event events.DynamoDBEvent) error {
	logger.Printf("Processing %d stream records", len(event.Records))

	for _, record := range event.Records {
		// Only process INSERT and MODIFY events for async requests
		if record.EventName != "INSERT" && record.EventName != "MODIFY" {
			continue
		}

		// Parse the AsyncRequest from DynamoDB stream
		asyncReq, err := parseAsyncRequest(record)
		if err != nil {
			logger.Printf("Failed to parse AsyncRequest: %v", err)
			continue
		}

		if asyncReq == nil {
			continue
		}

		// Only process pending requests
		if asyncReq.Status != models.StatusPending {
			logger.Printf("Skipping request %s with status %s", asyncReq.RequestID, asyncReq.Status)
			continue
		}

		// Process the request
		logger.Printf("Processing request %s with action %s", asyncReq.RequestID, asyncReq.Action)
		
		// Your processing logic would go here
		err = processRequest(ctx, asyncReq)
		if err != nil {
			logger.Printf("Failed to process request %s: %v", asyncReq.RequestID, err)
		}
	}

	return nil
}

// parseAsyncRequest converts a DynamoDB stream record to an AsyncRequest using DynamORM's stream support
func parseAsyncRequest(record events.DynamoDBEventRecord) (*models.AsyncRequest, error) {
	// For INSERT events, use NewImage; for MODIFY events, use NewImage as well
	image := record.Change.NewImage
	if image == nil {
		return nil, nil
	}

	// Use DynamORM's native stream support
	var asyncReq models.AsyncRequest
	if err := dynamorm.UnmarshalStreamImage(image, &asyncReq); err != nil {
		return nil, fmt.Errorf("failed to unmarshal AsyncRequest: %w", err)
	}

	return &asyncReq, nil
}

// processRequest demonstrates basic request processing
func processRequest(ctx context.Context, req *models.AsyncRequest) error {
	logger.Printf("Processing action '%s' for request %s", req.Action, req.RequestID)
	
	// Example processing based on action
	switch req.Action {
	case "delay":
		logger.Printf("Simulating delay processing for request %s", req.RequestID)
		// In a real implementation, you'd do actual work here
		return nil
	case "echo":
		logger.Printf("Echo processing for request %s with payload: %+v", req.RequestID, req.Payload)
		return nil
	case "knowledge_query":
		logger.Printf("Knowledge base query for request %s", req.RequestID)
		// In a real implementation, you'd query your knowledge base
		return nil
	default:
		return fmt.Errorf("unknown action: %s", req.Action)
	}
}

func main() {
	lambda.Start(handler)
}