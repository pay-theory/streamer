package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
)

func main() {
	ctx := context.Background()

	// Load AWS config
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}

	// Create DynamoDB client
	svc := dynamodb.NewFromConfig(cfg)

	// Create test connection
	connectionID := "demo-conn-" + uuid.New().String()[:8]
	userID := "demo-user"
	tenantID := "demo-tenant"

	// Insert test connection
	connectionItem := map[string]types.AttributeValue{
		"connection_id": &types.AttributeValueMemberS{Value: connectionID},
		"user_id":       &types.AttributeValueMemberS{Value: userID},
		"tenant_id":     &types.AttributeValueMemberS{Value: tenantID},
		"created_at":    &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
		"last_ping":     &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
		"metadata": &types.AttributeValueMemberM{
			Value: map[string]types.AttributeValue{
				"demo": &types.AttributeValueMemberBOOL{Value: true},
			},
		},
	}

	_, err = svc.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String("streamer_connections"),
		Item:      connectionItem,
	})
	if err != nil {
		log.Fatalf("Failed to create test connection: %v", err)
	}

	fmt.Printf("Demo setup complete!\n")
	fmt.Printf("Connection ID: %s\n", connectionID)
	fmt.Printf("\nUse this connection ID in your WebSocket client\n")
}
