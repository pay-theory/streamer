package dynamorm

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/pay-theory/dynamorm"
	"github.com/pay-theory/dynamorm/pkg/session"
	"github.com/pay-theory/streamer/internal/store"
)

// StoreFactory creates all the necessary stores using DynamORM
type StoreFactory struct {
	db                *dynamorm.DB
	connectionStore   store.ConnectionStore
	requestQueue      store.RequestQueue
	subscriptionStore store.SubscriptionStore
}

// NewStoreFactory creates a new DynamORM store factory
func NewStoreFactory(config session.Config) (*StoreFactory, error) {
	// Initialize DynamORM
	db, err := dynamorm.New(config)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize DynamORM: %w", err)
	}

	// Type assert to get the correct DB type
	dynamormDB, ok := db.(*dynamorm.DB)
	if !ok {
		return nil, fmt.Errorf("failed to get DynamORM DB instance")
	}

	// Create stores
	factory := &StoreFactory{
		db:              dynamormDB,
		connectionStore: NewConnectionStore(dynamormDB),
		requestQueue:    NewRequestQueue(dynamormDB),
		// TODO: Implement subscription store
		// subscriptionStore: NewSubscriptionStore(dynamormDB),
	}

	return factory, nil
}

// NewStoreFactoryFromClient creates a new factory from an existing DynamoDB client
// This is useful for Lambda functions that already have a client configured
func NewStoreFactoryFromClient(client *dynamodb.Client, region string) (*StoreFactory, error) {
	// For now, we'll create a new DynamORM instance with the region
	// In production, you might want to reuse the existing client configuration
	config := session.Config{
		Region: region,
	}

	return NewStoreFactory(config)
}

// ConnectionStore returns the connection store
func (f *StoreFactory) ConnectionStore() store.ConnectionStore {
	return f.connectionStore
}

// RequestQueue returns the request queue
func (f *StoreFactory) RequestQueue() store.RequestQueue {
	return f.requestQueue
}

// SubscriptionStore returns the subscription store
func (f *StoreFactory) SubscriptionStore() store.SubscriptionStore {
	return f.subscriptionStore
}

// DB returns the underlying DynamORM database instance
func (f *StoreFactory) DB() *dynamorm.DB {
	return f.db
}

// EnsureTables creates the required DynamoDB tables if they don't exist
// This should only be used in development/testing environments
func (f *StoreFactory) EnsureTables(ctx context.Context) error {
	// Connection table
	connTable := &Connection{}
	if err := f.db.AutoMigrate(connTable); err != nil {
		return fmt.Errorf("failed to ensure connections table: %w", err)
	}

	// Request table
	reqTable := &AsyncRequest{}
	if err := f.db.AutoMigrate(reqTable); err != nil {
		return fmt.Errorf("failed to ensure requests table: %w", err)
	}

	// Subscription table
	subTable := &Subscription{}
	if err := f.db.AutoMigrate(subTable); err != nil {
		return fmt.Errorf("failed to ensure subscriptions table: %w", err)
	}

	return nil
}
