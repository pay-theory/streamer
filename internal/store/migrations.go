package store

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// TableDefinition holds the configuration for a DynamoDB table
type TableDefinition struct {
	TableName              string
	AttributeDefinitions   []types.AttributeDefinition
	KeySchema              []types.KeySchemaElement
	GlobalSecondaryIndexes []types.GlobalSecondaryIndex
	BillingMode            types.BillingMode
}

// GetTableDefinitions returns all table definitions for Streamer
func GetTableDefinitions() []TableDefinition {
	return []TableDefinition{
		getConnectionsTableDefinition(),
		getRequestsTableDefinition(),
		getSubscriptionsTableDefinition(),
	}
}

// getConnectionsTableDefinition returns the definition for the connections table
func getConnectionsTableDefinition() TableDefinition {
	return TableDefinition{
		TableName: ConnectionsTable,
		AttributeDefinitions: []types.AttributeDefinition{
			{
				AttributeName: aws.String("ConnectionID"),
				AttributeType: types.ScalarAttributeTypeS,
			},
			{
				AttributeName: aws.String("UserID"),
				AttributeType: types.ScalarAttributeTypeS,
			},
			{
				AttributeName: aws.String("TenantID"),
				AttributeType: types.ScalarAttributeTypeS,
			},
		},
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: aws.String("ConnectionID"),
				KeyType:       types.KeyTypeHash,
			},
		},
		GlobalSecondaryIndexes: []types.GlobalSecondaryIndex{
			{
				IndexName: aws.String("UserIndex"),
				KeySchema: []types.KeySchemaElement{
					{
						AttributeName: aws.String("UserID"),
						KeyType:       types.KeyTypeHash,
					},
				},
				Projection: &types.Projection{
					ProjectionType: types.ProjectionTypeAll,
				},
			},
			{
				IndexName: aws.String("TenantIndex"),
				KeySchema: []types.KeySchemaElement{
					{
						AttributeName: aws.String("TenantID"),
						KeyType:       types.KeyTypeHash,
					},
				},
				Projection: &types.Projection{
					ProjectionType: types.ProjectionTypeAll,
				},
			},
		},
		BillingMode: types.BillingModePayPerRequest,
	}
}

// getRequestsTableDefinition returns the definition for the requests table
func getRequestsTableDefinition() TableDefinition {
	return TableDefinition{
		TableName: RequestsTable,
		AttributeDefinitions: []types.AttributeDefinition{
			{
				AttributeName: aws.String("RequestID"),
				AttributeType: types.ScalarAttributeTypeS,
			},
			{
				AttributeName: aws.String("ConnectionID"),
				AttributeType: types.ScalarAttributeTypeS,
			},
			{
				AttributeName: aws.String("Status"),
				AttributeType: types.ScalarAttributeTypeS,
			},
			{
				AttributeName: aws.String("CreatedAt"),
				AttributeType: types.ScalarAttributeTypeS,
			},
		},
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: aws.String("RequestID"),
				KeyType:       types.KeyTypeHash,
			},
		},
		GlobalSecondaryIndexes: []types.GlobalSecondaryIndex{
			{
				IndexName: aws.String("ConnectionIndex"),
				KeySchema: []types.KeySchemaElement{
					{
						AttributeName: aws.String("ConnectionID"),
						KeyType:       types.KeyTypeHash,
					},
					{
						AttributeName: aws.String("CreatedAt"),
						KeyType:       types.KeyTypeRange,
					},
				},
				Projection: &types.Projection{
					ProjectionType: types.ProjectionTypeAll,
				},
			},
			{
				IndexName: aws.String("StatusIndex"),
				KeySchema: []types.KeySchemaElement{
					{
						AttributeName: aws.String("Status"),
						KeyType:       types.KeyTypeHash,
					},
					{
						AttributeName: aws.String("CreatedAt"),
						KeyType:       types.KeyTypeRange,
					},
				},
				Projection: &types.Projection{
					ProjectionType: types.ProjectionTypeAll,
				},
			},
		},
		BillingMode: types.BillingModePayPerRequest,
	}
}

// getSubscriptionsTableDefinition returns the definition for the subscriptions table
func getSubscriptionsTableDefinition() TableDefinition {
	return TableDefinition{
		TableName: SubscriptionsTable,
		AttributeDefinitions: []types.AttributeDefinition{
			{
				AttributeName: aws.String("SubscriptionID"),
				AttributeType: types.ScalarAttributeTypeS,
			},
			{
				AttributeName: aws.String("ConnectionID"),
				AttributeType: types.ScalarAttributeTypeS,
			},
			{
				AttributeName: aws.String("RequestID"),
				AttributeType: types.ScalarAttributeTypeS,
			},
		},
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: aws.String("SubscriptionID"),
				KeyType:       types.KeyTypeHash,
			},
		},
		GlobalSecondaryIndexes: []types.GlobalSecondaryIndex{
			{
				IndexName: aws.String("ConnectionIndex"),
				KeySchema: []types.KeySchemaElement{
					{
						AttributeName: aws.String("ConnectionID"),
						KeyType:       types.KeyTypeHash,
					},
				},
				Projection: &types.Projection{
					ProjectionType: types.ProjectionTypeAll,
				},
			},
			{
				IndexName: aws.String("RequestIndex"),
				KeySchema: []types.KeySchemaElement{
					{
						AttributeName: aws.String("RequestID"),
						KeyType:       types.KeyTypeHash,
					},
				},
				Projection: &types.Projection{
					ProjectionType: types.ProjectionTypeAll,
				},
			},
		},
		BillingMode: types.BillingModePayPerRequest,
	}
}

// CreateTables creates all required DynamoDB tables
func CreateTables(ctx context.Context, client *dynamodb.Client) error {
	definitions := GetTableDefinitions()

	for _, def := range definitions {
		if err := createTable(ctx, client, def); err != nil {
			return fmt.Errorf("failed to create table %s: %w", def.TableName, err)
		}
	}

	return nil
}

// createTable creates a single DynamoDB table
func createTable(ctx context.Context, client *dynamodb.Client, def TableDefinition) error {
	// Check if table already exists
	_, err := client.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(def.TableName),
	})
	if err == nil {
		// Table already exists
		fmt.Printf("Table %s already exists, skipping creation\n", def.TableName)
		return nil
	}

	input := &dynamodb.CreateTableInput{
		TableName:            aws.String(def.TableName),
		AttributeDefinitions: def.AttributeDefinitions,
		KeySchema:            def.KeySchema,
		BillingMode:          def.BillingMode,
	}

	// Add GSIs if any
	if len(def.GlobalSecondaryIndexes) > 0 {
		input.GlobalSecondaryIndexes = def.GlobalSecondaryIndexes
	}

	// Enable TTL after table creation
	_, err = client.CreateTable(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	fmt.Printf("Created table %s\n", def.TableName)

	// Wait for table to become active
	waiter := dynamodb.NewTableExistsWaiter(client)
	if err := waiter.Wait(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(def.TableName),
	}, 5*time.Minute); err != nil {
		return fmt.Errorf("failed waiting for table to become active: %w", err)
	}

	// Enable TTL on the tables
	if err := enableTTL(ctx, client, def.TableName); err != nil {
		// TTL enablement failure is not critical
		fmt.Printf("Warning: Failed to enable TTL on table %s: %v\n", def.TableName, err)
	}

	return nil
}

// enableTTL enables time-to-live on a DynamoDB table
func enableTTL(ctx context.Context, client *dynamodb.Client, tableName string) error {
	input := &dynamodb.UpdateTimeToLiveInput{
		TableName: aws.String(tableName),
		TimeToLiveSpecification: &types.TimeToLiveSpecification{
			AttributeName: aws.String("TTL"),
			Enabled:       aws.Bool(true),
		},
	}

	_, err := client.UpdateTimeToLive(ctx, input)
	return err
}

// DeleteTables deletes all Streamer tables (for testing)
func DeleteTables(ctx context.Context, client *dynamodb.Client) error {
	definitions := GetTableDefinitions()

	for _, def := range definitions {
		input := &dynamodb.DeleteTableInput{
			TableName: aws.String(def.TableName),
		}

		_, err := client.DeleteTable(ctx, input)
		if err != nil {
			// Ignore if table doesn't exist
			fmt.Printf("Warning: Failed to delete table %s: %v\n", def.TableName, err)
			continue
		}

		fmt.Printf("Deleted table %s\n", def.TableName)
	}

	return nil
}
