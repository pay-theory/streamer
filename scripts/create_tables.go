package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/pay-theory/streamer/internal/store"
)

func main() {
	var (
		endpoint = flag.String("endpoint", "http://localhost:8000", "DynamoDB endpoint")
		region   = flag.String("region", "us-east-1", "AWS region")
		profile  = flag.String("profile", "", "AWS profile to use")
		destroy  = flag.Bool("destroy", false, "Delete tables instead of creating them")
	)
	flag.Parse()

	// Create AWS config
	opts := []func(*config.LoadOptions) error{
		config.WithRegion(*region),
	}

	// Use local endpoint if specified
	if *endpoint != "" {
		opts = append(opts,
			config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
				func(service, region string, options ...interface{}) (aws.Endpoint, error) {
					return aws.Endpoint{URL: *endpoint}, nil
				})),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("dummy", "dummy", "")),
		)
	} else if *profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(*profile))
	}

	cfg, err := config.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}

	// Create DynamoDB client
	client := dynamodb.NewFromConfig(cfg)

	ctx := context.Background()

	if *destroy {
		fmt.Println("Deleting DynamoDB tables...")
		if err := store.DeleteTables(ctx, client); err != nil {
			log.Printf("Warning: Failed to delete some tables: %v", err)
		}
		fmt.Println("Tables deleted successfully")
		os.Exit(0)
	}

	// Create tables
	fmt.Println("Creating DynamoDB tables...")
	if err := store.CreateTables(ctx, client); err != nil {
		log.Fatalf("Failed to create tables: %v", err)
	}

	fmt.Println("✅ All tables created successfully!")
	fmt.Println("\nCreated tables:")
	fmt.Println("  - streamer_connections")
	fmt.Println("  - streamer_requests")
	fmt.Println("  - streamer_subscriptions")
	fmt.Println("\nYou can now run the application or tests.")
}
