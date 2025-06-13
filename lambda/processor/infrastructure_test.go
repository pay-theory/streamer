package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamodbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/pay-theory/dynamorm/pkg/mocks"
	lifttesting "github.com/pay-theory/lift/pkg/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestTableManagementInfrastructure tests DynamoDB table management operations
// that aren't covered by DynamORM application-level abstractions
func TestTableManagementInfrastructure(t *testing.T) {
	mockClient := new(mocks.MockDynamoDBClient)
	mockWaiter := new(mocks.MockTableExistsWaiter)
	ctx := context.Background()

	t.Run("CreateConnectionsTable", func(t *testing.T) {
		// Test creating the connections table with proper schema
		expectedOutput := mocks.NewMockCreateTableOutput("connections")
		mockClient.On("CreateTable", ctx, mock.MatchedBy(func(input *dynamodb.CreateTableInput) bool {
			return *input.TableName == "connections" &&
				len(input.KeySchema) == 1 &&
				*input.KeySchema[0].AttributeName == "connection_id" &&
				input.KeySchema[0].KeyType == dynamodbtypes.KeyTypeHash
		}), mock.Anything).Return(expectedOutput, nil)

		// Simulate table creation
		_, err := mockClient.CreateTable(ctx, &dynamodb.CreateTableInput{
			TableName: aws.String("connections"),
			KeySchema: []dynamodbtypes.KeySchemaElement{
				{
					AttributeName: aws.String("connection_id"),
					KeyType:       dynamodbtypes.KeyTypeHash,
				},
			},
			AttributeDefinitions: []dynamodbtypes.AttributeDefinition{
				{
					AttributeName: aws.String("connection_id"),
					AttributeType: dynamodbtypes.ScalarAttributeTypeS,
				},
				{
					AttributeName: aws.String("user_id"),
					AttributeType: dynamodbtypes.ScalarAttributeTypeS,
				},
				{
					AttributeName: aws.String("tenant_id"),
					AttributeType: dynamodbtypes.ScalarAttributeTypeS,
				},
			},
			GlobalSecondaryIndexes: []dynamodbtypes.GlobalSecondaryIndex{
				{
					IndexName: aws.String("user-index"),
					KeySchema: []dynamodbtypes.KeySchemaElement{
						{
							AttributeName: aws.String("user_id"),
							KeyType:       dynamodbtypes.KeyTypeHash,
						},
					},
					Projection: &dynamodbtypes.Projection{
						ProjectionType: dynamodbtypes.ProjectionTypeAll,
					},
				},
				{
					IndexName: aws.String("tenant-index"),
					KeySchema: []dynamodbtypes.KeySchemaElement{
						{
							AttributeName: aws.String("tenant_id"),
							KeyType:       dynamodbtypes.KeyTypeHash,
						},
					},
					Projection: &dynamodbtypes.Projection{
						ProjectionType: dynamodbtypes.ProjectionTypeAll,
					},
				},
			},
			BillingMode: dynamodbtypes.BillingModePayPerRequest,
		})

		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
	})

	t.Run("WaitForTableActive", func(t *testing.T) {
		// Test waiting for table to become active
		mockWaiter.On("Wait", ctx, mock.MatchedBy(func(input *dynamodb.DescribeTableInput) bool {
			return *input.TableName == "connections"
		}), 5*time.Minute, mock.Anything).Return(nil)

		err := mockWaiter.Wait(ctx, &dynamodb.DescribeTableInput{
			TableName: aws.String("connections"),
		}, 5*time.Minute)

		assert.NoError(t, err)
		mockWaiter.AssertExpectations(t)
	})

	t.Run("EnableTTL", func(t *testing.T) {
		// Test enabling TTL for automatic cleanup of stale connections
		expectedOutput := mocks.NewMockUpdateTimeToLiveOutput("connections")
		mockClient.On("UpdateTimeToLive", ctx, mock.MatchedBy(func(input *dynamodb.UpdateTimeToLiveInput) bool {
			return *input.TableName == "connections" &&
				*input.TimeToLiveSpecification.AttributeName == "expires_at" &&
				*input.TimeToLiveSpecification.Enabled == true
		}), mock.Anything).Return(expectedOutput, nil)

		_, err := mockClient.UpdateTimeToLive(ctx, &dynamodb.UpdateTimeToLiveInput{
			TableName: aws.String("connections"),
			TimeToLiveSpecification: &dynamodbtypes.TimeToLiveSpecification{
				AttributeName: aws.String("expires_at"),
				Enabled:       aws.Bool(true),
			},
		})

		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
	})

	t.Run("CreateRequestQueueTable", func(t *testing.T) {
		// Test creating the request queue table
		expectedOutput := mocks.NewMockCreateTableOutput("request_queue")
		mockClient.On("CreateTable", ctx, mock.MatchedBy(func(input *dynamodb.CreateTableInput) bool {
			return *input.TableName == "request_queue"
		}), mock.Anything).Return(expectedOutput, nil)

		_, err := mockClient.CreateTable(ctx, &dynamodb.CreateTableInput{
			TableName: aws.String("request_queue"),
			KeySchema: []dynamodbtypes.KeySchemaElement{
				{
					AttributeName: aws.String("request_id"),
					KeyType:       dynamodbtypes.KeyTypeHash,
				},
			},
			AttributeDefinitions: []dynamodbtypes.AttributeDefinition{
				{
					AttributeName: aws.String("request_id"),
					AttributeType: dynamodbtypes.ScalarAttributeTypeS,
				},
				{
					AttributeName: aws.String("connection_id"),
					AttributeType: dynamodbtypes.ScalarAttributeTypeS,
				},
				{
					AttributeName: aws.String("status"),
					AttributeType: dynamodbtypes.ScalarAttributeTypeS,
				},
			},
			GlobalSecondaryIndexes: []dynamodbtypes.GlobalSecondaryIndex{
				{
					IndexName: aws.String("connection-status-index"),
					KeySchema: []dynamodbtypes.KeySchemaElement{
						{
							AttributeName: aws.String("connection_id"),
							KeyType:       dynamodbtypes.KeyTypeHash,
						},
						{
							AttributeName: aws.String("status"),
							KeyType:       dynamodbtypes.KeyTypeRange,
						},
					},
					Projection: &dynamodbtypes.Projection{
						ProjectionType: dynamodbtypes.ProjectionTypeAll,
					},
				},
			},
			BillingMode: dynamodbtypes.BillingModePayPerRequest,
		})

		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
	})

	t.Run("ErrorHandling_TableAlreadyExists", func(t *testing.T) {
		// Test handling table already exists error
		mockClient.ExpectedCalls = nil // Reset expectations

		expectedErr := &dynamodbtypes.ResourceInUseException{
			Message: aws.String("Table already exists: connections"),
		}
		mockClient.On("CreateTable", ctx, mock.AnythingOfType("*dynamodb.CreateTableInput"), mock.Anything).
			Return(nil, expectedErr)

		_, err := mockClient.CreateTable(ctx, &dynamodb.CreateTableInput{
			TableName: aws.String("connections"),
			KeySchema: []dynamodbtypes.KeySchemaElement{
				{
					AttributeName: aws.String("connection_id"),
					KeyType:       dynamodbtypes.KeyTypeHash,
				},
			},
			AttributeDefinitions: []dynamodbtypes.AttributeDefinition{
				{
					AttributeName: aws.String("connection_id"),
					AttributeType: dynamodbtypes.ScalarAttributeTypeS,
				},
			},
			BillingMode: dynamodbtypes.BillingModePayPerRequest,
		})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Table already exists")
		mockClient.AssertExpectations(t)
	})
}

// TestCloudWatchMetricsInfrastructure tests CloudWatch metrics operations
// using Lift 1.0.19 AWS SDK-compatible CloudWatch mocks
func TestCloudWatchMetricsInfrastructure(t *testing.T) {
	mockClient := lifttesting.NewMockCloudWatchClient()
	ctx := context.Background()

	t.Run("PublishConnectionMetrics", func(t *testing.T) {
		// Test publishing WebSocket connection metrics
		expectedOutput := lifttesting.NewMockPutMetricDataOutput()
		mockClient.On("PutMetricData", ctx, mock.MatchedBy(func(input *cloudwatch.PutMetricDataInput) bool {
			return *input.Namespace == "PayTheory/Streamer" &&
				len(input.MetricData) == 3 &&
				*input.MetricData[0].MetricName == "ActiveConnections" &&
				*input.MetricData[1].MetricName == "MessageLatency" &&
				*input.MetricData[2].MetricName == "ErrorRate"
		}), mock.Anything).Return(expectedOutput, nil)

		// Simulate publishing metrics
		_, err := mockClient.PutMetricData(ctx, &cloudwatch.PutMetricDataInput{
			Namespace: aws.String("PayTheory/Streamer"),
			MetricData: []types.MetricDatum{
				{
					MetricName: aws.String("ActiveConnections"),
					Value:      aws.Float64(25.0),
					Unit:       types.StandardUnitCount,
					Dimensions: []types.Dimension{
						{
							Name:  aws.String("Service"),
							Value: aws.String("Streamer"),
						},
						{
							Name:  aws.String("Environment"),
							Value: aws.String("production"),
						},
					},
					Timestamp: aws.Time(time.Now()),
				},
				{
					MetricName: aws.String("MessageLatency"),
					Value:      aws.Float64(45.2),
					Unit:       types.StandardUnitMilliseconds,
					Dimensions: []types.Dimension{
						{
							Name:  aws.String("Service"),
							Value: aws.String("Streamer"),
						},
					},
					Timestamp: aws.Time(time.Now()),
				},
				{
					MetricName: aws.String("ErrorRate"),
					Value:      aws.Float64(0.02),
					Unit:       types.StandardUnitPercent,
					Dimensions: []types.Dimension{
						{
							Name:  aws.String("Service"),
							Value: aws.String("Streamer"),
						},
					},
					Timestamp: aws.Time(time.Now()),
				},
			},
		})

		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
	})

	t.Run("GetMetricStatistics", func(t *testing.T) {
		// Test retrieving metric statistics
		datapoints := []types.Datapoint{
			{
				Timestamp: aws.Time(time.Now().Add(-5 * time.Minute)),
				Average:   aws.Float64(23.5),
				Maximum:   aws.Float64(30.0),
				Minimum:   aws.Float64(20.0),
				Sum:       aws.Float64(235.0),
				Unit:      types.StandardUnitCount,
			},
			{
				Timestamp: aws.Time(time.Now()),
				Average:   aws.Float64(27.2),
				Maximum:   aws.Float64(35.0),
				Minimum:   aws.Float64(22.0),
				Sum:       aws.Float64(272.0),
				Unit:      types.StandardUnitCount,
			},
		}
		expectedOutput := lifttesting.NewMockGetMetricStatisticsOutput(datapoints)

		mockClient.On("GetMetricStatistics", ctx, mock.MatchedBy(func(input *cloudwatch.GetMetricStatisticsInput) bool {
			return *input.MetricName == "ActiveConnections" &&
				*input.Namespace == "PayTheory/Streamer"
		}), mock.Anything).Return(expectedOutput, nil)

		now := time.Now()
		result, err := mockClient.GetMetricStatistics(ctx, &cloudwatch.GetMetricStatisticsInput{
			Namespace:  aws.String("PayTheory/Streamer"),
			MetricName: aws.String("ActiveConnections"),
			Dimensions: []types.Dimension{
				{
					Name:  aws.String("Service"),
					Value: aws.String("Streamer"),
				},
			},
			StartTime:  aws.Time(now.Add(-1 * time.Hour)),
			EndTime:    aws.Time(now),
			Period:     aws.Int32(300),
			Statistics: []types.Statistic{types.StatisticAverage, types.StatisticMaximum, types.StatisticMinimum},
		})

		assert.NoError(t, err)
		assert.Len(t, result.Datapoints, 2)
		assert.Equal(t, 23.5, *result.Datapoints[0].Average)
		assert.Equal(t, 27.2, *result.Datapoints[1].Average)
		mockClient.AssertExpectations(t)
	})

	t.Run("ErrorHandling_ServiceUnavailable", func(t *testing.T) {
		// Test handling CloudWatch service errors
		mockClient.ExpectedCalls = nil // Reset expectations

		expectedErr := fmt.Errorf("service unavailable")
		mockClient.On("PutMetricData", ctx, mock.AnythingOfType("*cloudwatch.PutMetricDataInput"), mock.Anything).
			Return((*cloudwatch.PutMetricDataOutput)(nil), expectedErr)

		_, err := mockClient.PutMetricData(ctx, &cloudwatch.PutMetricDataInput{
			Namespace: aws.String("PayTheory/Streamer"),
			MetricData: []types.MetricDatum{
				{
					MetricName: aws.String("TestMetric"),
					Value:      aws.Float64(1.0),
					Unit:       types.StandardUnitCount,
				},
			},
		})

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "service unavailable")
		mockClient.AssertExpectations(t)
	})
}

// TestCloudWatchAlarmsInfrastructure tests CloudWatch alarms operations
func TestCloudWatchAlarmsInfrastructure(t *testing.T) {
	mockClient := lifttesting.NewMockCloudWatchClient()
	ctx := context.Background()

	t.Run("CreateConnectionAlarm", func(t *testing.T) {
		// Test creating CloudWatch alarm for high connection count
		expectedOutput := lifttesting.NewMockPutMetricAlarmOutput()
		mockClient.On("PutMetricAlarm", ctx, mock.MatchedBy(func(input *cloudwatch.PutMetricAlarmInput) bool {
			return *input.AlarmName == "HighConnectionCount" &&
				*input.MetricName == "ActiveConnections" &&
				*input.Namespace == "PayTheory/Streamer" &&
				*input.Threshold == 100.0 &&
				input.ComparisonOperator == types.ComparisonOperatorGreaterThanThreshold
		}), mock.Anything).Return(expectedOutput, nil)

		_, err := mockClient.PutMetricAlarm(ctx, &cloudwatch.PutMetricAlarmInput{
			AlarmName:        aws.String("HighConnectionCount"),
			AlarmDescription: aws.String("Alert when WebSocket connections exceed threshold"),
			MetricName:       aws.String("ActiveConnections"),
			Namespace:        aws.String("PayTheory/Streamer"),
			Statistic:        types.StatisticAverage,
			Dimensions: []types.Dimension{
				{
					Name:  aws.String("Service"),
					Value: aws.String("Streamer"),
				},
			},
			Period:             aws.Int32(300), // 5 minutes
			EvaluationPeriods:  aws.Int32(1),
			Threshold:          aws.Float64(100.0),
			ComparisonOperator: types.ComparisonOperatorGreaterThanThreshold,
			TreatMissingData:   aws.String("notBreaching"),
		})

		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
	})

	t.Run("CreateLatencyAlarm", func(t *testing.T) {
		// Test creating CloudWatch alarm for high latency
		expectedOutput := lifttesting.NewMockPutMetricAlarmOutput()
		mockClient.On("PutMetricAlarm", ctx, mock.MatchedBy(func(input *cloudwatch.PutMetricAlarmInput) bool {
			return *input.AlarmName == "HighMessageLatency" &&
				*input.MetricName == "MessageLatency" &&
				*input.Threshold == 1000.0 // 1 second
		}), mock.Anything).Return(expectedOutput, nil)

		_, err := mockClient.PutMetricAlarm(ctx, &cloudwatch.PutMetricAlarmInput{
			AlarmName:        aws.String("HighMessageLatency"),
			AlarmDescription: aws.String("Alert when message latency is too high"),
			MetricName:       aws.String("MessageLatency"),
			Namespace:        aws.String("PayTheory/Streamer"),
			Statistic:        types.StatisticAverage,
			Dimensions: []types.Dimension{
				{
					Name:  aws.String("Service"),
					Value: aws.String("Streamer"),
				},
			},
			Period:             aws.Int32(300),
			EvaluationPeriods:  aws.Int32(2),
			Threshold:          aws.Float64(1000.0),
			ComparisonOperator: types.ComparisonOperatorGreaterThanThreshold,
			TreatMissingData:   aws.String("breaching"),
		})

		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
	})

	t.Run("DescribeAlarms", func(t *testing.T) {
		// Test describing existing alarms
		alarms := []types.MetricAlarm{
			{
				AlarmName:          aws.String("HighConnectionCount"),
				MetricName:         aws.String("ActiveConnections"),
				Namespace:          aws.String("PayTheory/Streamer"),
				Threshold:          aws.Float64(100.0),
				ComparisonOperator: types.ComparisonOperatorGreaterThanThreshold,
				StateValue:         types.StateValueOk,
			},
		}
		expectedOutput := lifttesting.NewMockDescribeAlarmsOutput(alarms)

		mockClient.On("DescribeAlarms", ctx, mock.MatchedBy(func(input *cloudwatch.DescribeAlarmsInput) bool {
			return len(input.AlarmNames) == 1 && input.AlarmNames[0] == "HighConnectionCount"
		}), mock.Anything).Return(expectedOutput, nil)

		result, err := mockClient.DescribeAlarms(ctx, &cloudwatch.DescribeAlarmsInput{
			AlarmNames: []string{"HighConnectionCount"},
		})

		assert.NoError(t, err)
		assert.Len(t, result.MetricAlarms, 1)
		assert.Equal(t, "HighConnectionCount", *result.MetricAlarms[0].AlarmName)
		assert.Equal(t, types.StateValueOk, result.MetricAlarms[0].StateValue)
		mockClient.AssertExpectations(t)
	})
}

// TestBasicDynamoDBOperations tests basic DynamoDB operations
func TestBasicDynamoDBOperations(t *testing.T) {
	mockClient := new(mocks.MockDynamoDBClient)
	ctx := context.Background()

	t.Run("PutItem", func(t *testing.T) {
		// Test putting an item
		expectedOutput := &dynamodb.PutItemOutput{}
		mockClient.On("PutItem", ctx, mock.MatchedBy(func(input *dynamodb.PutItemInput) bool {
			return *input.TableName == "connections"
		}), mock.Anything).Return(expectedOutput, nil)

		_, err := mockClient.PutItem(ctx, &dynamodb.PutItemInput{
			TableName: aws.String("connections"),
			Item: map[string]dynamodbtypes.AttributeValue{
				"connection_id": &dynamodbtypes.AttributeValueMemberS{Value: "conn-123"},
				"user_id":       &dynamodbtypes.AttributeValueMemberS{Value: "user-456"},
				"tenant_id":     &dynamodbtypes.AttributeValueMemberS{Value: "tenant-789"},
				"created_at":    &dynamodbtypes.AttributeValueMemberS{Value: "2023-01-01T00:00:00Z"},
			},
		})

		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
	})

	t.Run("GetItem", func(t *testing.T) {
		// Test getting an item
		expectedOutput := &dynamodb.GetItemOutput{
			Item: map[string]dynamodbtypes.AttributeValue{
				"connection_id": &dynamodbtypes.AttributeValueMemberS{Value: "conn-123"},
				"user_id":       &dynamodbtypes.AttributeValueMemberS{Value: "user-456"},
				"tenant_id":     &dynamodbtypes.AttributeValueMemberS{Value: "tenant-789"},
				"created_at":    &dynamodbtypes.AttributeValueMemberS{Value: "2023-01-01T00:00:00Z"},
			},
		}

		mockClient.On("GetItem", ctx, mock.MatchedBy(func(input *dynamodb.GetItemInput) bool {
			return *input.TableName == "connections"
		}), mock.Anything).Return(expectedOutput, nil)

		result, err := mockClient.GetItem(ctx, &dynamodb.GetItemInput{
			TableName: aws.String("connections"),
			Key: map[string]dynamodbtypes.AttributeValue{
				"connection_id": &dynamodbtypes.AttributeValueMemberS{Value: "conn-123"},
			},
		})

		assert.NoError(t, err)
		assert.NotNil(t, result.Item)
		assert.Equal(t, "conn-123", result.Item["connection_id"].(*dynamodbtypes.AttributeValueMemberS).Value)
		mockClient.AssertExpectations(t)
	})

	t.Run("UpdateItem", func(t *testing.T) {
		// Test update operations
		expectedOutput := &dynamodb.UpdateItemOutput{}

		mockClient.On("UpdateItem", ctx, mock.MatchedBy(func(input *dynamodb.UpdateItemInput) bool {
			return *input.TableName == "connections" &&
				*input.UpdateExpression == "SET #status = :status"
		}), mock.Anything).Return(expectedOutput, nil)

		_, err := mockClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
			TableName: aws.String("connections"),
			Key: map[string]dynamodbtypes.AttributeValue{
				"connection_id": &dynamodbtypes.AttributeValueMemberS{Value: "conn-1"},
			},
			UpdateExpression: aws.String("SET #status = :status"),
			ExpressionAttributeNames: map[string]string{
				"#status": "status",
			},
			ExpressionAttributeValues: map[string]dynamodbtypes.AttributeValue{
				":status": &dynamodbtypes.AttributeValueMemberS{Value: "active"},
			},
		})

		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
	})

	t.Run("DeleteItem", func(t *testing.T) {
		// Test deleting an item
		expectedOutput := &dynamodb.DeleteItemOutput{}
		mockClient.On("DeleteItem", ctx, mock.MatchedBy(func(input *dynamodb.DeleteItemInput) bool {
			return *input.TableName == "connections"
		}), mock.Anything).Return(expectedOutput, nil)

		_, err := mockClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
			TableName: aws.String("connections"),
			Key: map[string]dynamodbtypes.AttributeValue{
				"connection_id": &dynamodbtypes.AttributeValueMemberS{Value: "conn-123"},
			},
		})

		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
	})
}

// TestCompleteInfrastructureWorkflow tests a complete infrastructure setup
func TestCompleteInfrastructureWorkflow(t *testing.T) {
	// Setup all mocks
	dynamoClient := new(mocks.MockDynamoDBClient)
	tableWaiter := new(mocks.MockTableExistsWaiter)
	cloudwatchClient := lifttesting.NewMockCloudWatchClient()
	ctx := context.Background()

	// Test complete infrastructure setup workflow
	t.Run("CompleteSetup", func(t *testing.T) {
		// 1. Create connections table
		createTableOutput := mocks.NewMockCreateTableOutput("connections")
		dynamoClient.On("CreateTable", ctx, mock.AnythingOfType("*dynamodb.CreateTableInput"), mock.Anything).
			Return(createTableOutput, nil).Once()

		// 2. Wait for table to be active
		tableWaiter.On("Wait", ctx, mock.AnythingOfType("*dynamodb.DescribeTableInput"), 5*time.Minute, mock.Anything).
			Return(nil).Once()

		// 3. Enable TTL
		ttlOutput := mocks.NewMockUpdateTimeToLiveOutput("connections")
		dynamoClient.On("UpdateTimeToLive", ctx, mock.AnythingOfType("*dynamodb.UpdateTimeToLiveInput"), mock.Anything).
			Return(ttlOutput, nil).Once()

		// 4. Create CloudWatch alarms
		alarmOutput := &cloudwatch.PutMetricAlarmOutput{}
		cloudwatchClient.On("PutMetricAlarm", ctx, mock.AnythingOfType("*cloudwatch.PutMetricAlarmInput"), mock.Anything).
			Return(alarmOutput, nil).Times(2) // Two alarms

		// Execute workflow
		// Create table
		_, err := dynamoClient.CreateTable(ctx, &dynamodb.CreateTableInput{
			TableName: aws.String("connections"),
			KeySchema: []dynamodbtypes.KeySchemaElement{
				{AttributeName: aws.String("connection_id"), KeyType: dynamodbtypes.KeyTypeHash},
			},
			AttributeDefinitions: []dynamodbtypes.AttributeDefinition{
				{AttributeName: aws.String("connection_id"), AttributeType: dynamodbtypes.ScalarAttributeTypeS},
			},
			BillingMode: dynamodbtypes.BillingModePayPerRequest,
		})
		assert.NoError(t, err)

		// Wait for table
		err = tableWaiter.Wait(ctx, &dynamodb.DescribeTableInput{
			TableName: aws.String("connections"),
		}, 5*time.Minute)
		assert.NoError(t, err)

		// Enable TTL
		_, err = dynamoClient.UpdateTimeToLive(ctx, &dynamodb.UpdateTimeToLiveInput{
			TableName: aws.String("connections"),
			TimeToLiveSpecification: &dynamodbtypes.TimeToLiveSpecification{
				AttributeName: aws.String("expires_at"),
				Enabled:       aws.Bool(true),
			},
		})
		assert.NoError(t, err)

		// Create alarms
		_, err = cloudwatchClient.PutMetricAlarm(ctx, &cloudwatch.PutMetricAlarmInput{
			AlarmName:          aws.String("HighConnectionCount"),
			MetricName:         aws.String("ActiveConnections"),
			Namespace:          aws.String("PayTheory/Streamer"),
			Threshold:          aws.Float64(100.0),
			ComparisonOperator: types.ComparisonOperatorGreaterThanThreshold,
		})
		assert.NoError(t, err)

		_, err = cloudwatchClient.PutMetricAlarm(ctx, &cloudwatch.PutMetricAlarmInput{
			AlarmName:          aws.String("HighLatency"),
			MetricName:         aws.String("MessageLatency"),
			Namespace:          aws.String("PayTheory/Streamer"),
			Threshold:          aws.Float64(1000.0),
			ComparisonOperator: types.ComparisonOperatorGreaterThanThreshold,
		})
		assert.NoError(t, err)

		// Verify all expectations
		dynamoClient.AssertExpectations(t)
		tableWaiter.AssertExpectations(t)
		cloudwatchClient.AssertExpectations(t)
	})
}
