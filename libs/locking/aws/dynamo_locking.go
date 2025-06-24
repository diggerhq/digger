package aws

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/aws/smithy-go"
)

const (
	TABLE_NAME              = "DiggerDynamoDBLockTable"
	TableCreationInterval   = 1 * time.Second
	TableCreationRetryCount = 10
	TableLockTimeout        = 60 * 60 * 24 * 90 * time.Second
)

type DynamoDbLock struct {
	DynamoDb DynamoDBClient
}

type DynamoDBClient interface {
	DescribeTable(ctx context.Context, params *dynamodb.DescribeTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error)
	CreateTable(ctx context.Context, params *dynamodb.CreateTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.CreateTableOutput, error)
	UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
	DeleteItem(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error)
	GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
}

func isResourceNotFoundException(err error) bool {
	var apiError smithy.APIError
	if errors.As(err, &apiError) {
		switch apiError.(type) {
		case *types.ResourceNotFoundException:
			return true
		}
	}
	return false
}

func (dynamoDbLock *DynamoDbLock) waitUntilTableCreated(ctx context.Context) error {
	input := &dynamodb.DescribeTableInput{
		TableName: aws.String(TABLE_NAME),
	}
	status, err := dynamoDbLock.DynamoDb.DescribeTable(ctx, input)
	cnt := 0

	if err != nil {
		if !isResourceNotFoundException(err) {
			return err
		}
	}

	for status.Table.TableStatus != "ACTIVE" {
		slog.Debug("Waiting for DynamoDB table to become active",
			"tableName", TABLE_NAME,
			"currentStatus", status.Table.TableStatus,
			"retryCount", cnt+1)

		time.Sleep(TableCreationInterval)
		status, err = dynamoDbLock.DynamoDb.DescribeTable(ctx, input)
		if err != nil {
			if !isResourceNotFoundException(err) {
				return err
			}
		}
		cnt++
		if cnt > TableCreationRetryCount {
			msg := "DynamoDB table creation timed out"
			slog.Error(msg,
				"tableName", TABLE_NAME,
				"retryCount", cnt)
			return errors.New(msg)
		}
	}

	return nil
}

func (dynamoDbLock *DynamoDbLock) createTableIfNotExists(ctx context.Context) error {
	_, err := dynamoDbLock.DynamoDb.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(TABLE_NAME),
	})
	if err == nil { // Table exists
		slog.Debug("DynamoDB table already exists", "tableName", TABLE_NAME)
		return nil
	}
	if !isResourceNotFoundException(err) {
		slog.Info("Error describing DynamoDB table, proceeding to create", "tableName", TABLE_NAME, "error", err)
	}

	slog.Info("Creating DynamoDB table", "tableName", TABLE_NAME)

	createtbl_input := &dynamodb.CreateTableInput{
		AttributeDefinitions: []types.AttributeDefinition{
			{
				AttributeName: aws.String("PK"),
				AttributeType: types.ScalarAttributeTypeS,
			},
			{
				AttributeName: aws.String("SK"),
				AttributeType: types.ScalarAttributeTypeS,
			},
		},
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: aws.String("PK"),
				KeyType:       types.KeyTypeHash,
			},
			{
				AttributeName: aws.String("SK"),
				KeyType:       types.KeyTypeRange,
			},
		},
		BillingMode: types.BillingModePayPerRequest,
		TableName:   aws.String(TABLE_NAME),
	}
	_, err = dynamoDbLock.DynamoDb.CreateTable(ctx, createtbl_input)
	if err != nil {
		if os.Getenv("DEBUG") != "" {
			slog.Debug("DynamoDB table creation error", "error", err)
		}
		return err
	}

	err = dynamoDbLock.waitUntilTableCreated(ctx)
	if err != nil {
		slog.Error("Error waiting for DynamoDB table creation", "tableName", TABLE_NAME, "error", err)
		return err
	}
	slog.Info("DynamoDB table created successfully", "tableName", TABLE_NAME)
	return nil
}

func (dynamoDbLock *DynamoDbLock) Lock(transactionId int, resource string) (bool, error) {
	ctx := context.Background()

	slog.Debug("Attempting to acquire lock",
		"resource", resource,
		"transactionId", transactionId)

	err := dynamoDbLock.createTableIfNotExists(ctx)
	if err != nil {
		slog.Error("Error creating DynamoDB table", "error", err)
		return false, err
	}

	// TODO: remove timeout completely
	now := time.Now().Format(time.RFC3339)
	newTimeout := time.Now().Add(TableLockTimeout).Format(time.RFC3339)

	expr, err := expression.NewBuilder().
		WithCondition(
			expression.Or(
				expression.AttributeNotExists(expression.Name("SK")),
				expression.LessThan(expression.Name("timeout"), expression.Value(now)),
			),
		).
		WithUpdate(
			expression.Set(
				expression.Name("transaction_id"), expression.Value(transactionId),
			).Set(expression.Name("timeout"), expression.Value(newTimeout)),
		).
		Build()
	if err != nil {
		slog.Error("Failed to build DynamoDB expression", "error", err)
		return false, err
	}

	input := &dynamodb.UpdateItemInput{
		TableName: aws.String(TABLE_NAME),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: "LOCK"},
			"SK": &types.AttributeValueMemberS{Value: "RES#" + resource},
		},
		ConditionExpression:       expr.Condition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		UpdateExpression:          expr.Update(),
	}

	_, err = dynamoDbLock.DynamoDb.UpdateItem(ctx, input)
	if err != nil {
		var apiError smithy.APIError
		if errors.As(err, &apiError) {
			switch apiError.(type) {
			case *types.ConditionalCheckFailedException:
				slog.Debug("Lock already exists or not expired", "resource", resource)
				return false, nil
			}
		}
		slog.Error("Error updating DynamoDB item for lock",
			"resource", resource,
			"error", err)
		return false, err
	}

	slog.Info("Lock acquired successfully",
		"resource", resource,
		"transactionId", transactionId,
		"timeout", newTimeout)
	return true, nil
}

func (dynamoDbLock *DynamoDbLock) Unlock(resource string) (bool, error) {
	ctx := context.Background()

	slog.Debug("Attempting to release lock", "resource", resource)

	err := dynamoDbLock.createTableIfNotExists(ctx)
	if err != nil {
		slog.Error("Error creating DynamoDB table", "error", err)
		return false, err
	}

	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(TABLE_NAME),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: "LOCK"},
			"SK": &types.AttributeValueMemberS{Value: "RES#" + resource},
		},
	}

	_, err = dynamoDbLock.DynamoDb.DeleteItem(ctx, input)
	if err != nil {
		slog.Error("Failed to delete DynamoDB item for lock", "resource", resource, "error", err)
		return false, err
	}

	slog.Info("Lock released successfully", "resource", resource)
	return true, nil
}

func (dynamoDbLock *DynamoDbLock) GetLock(lockId string) (*int, error) {
	ctx := context.Background()

	slog.Debug("Getting lock information", "lockId", lockId)

	err := dynamoDbLock.createTableIfNotExists(ctx)
	if err != nil {
		slog.Error("Error creating DynamoDB table", "error", err)
		return nil, err
	}

	input := &dynamodb.GetItemInput{
		TableName: aws.String(TABLE_NAME),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: "LOCK"},
			"SK": &types.AttributeValueMemberS{Value: "RES#" + lockId},
		},
		ConsistentRead: aws.Bool(true),
	}

	result, err := dynamoDbLock.DynamoDb.GetItem(ctx, input)
	if err != nil {
		slog.Error("Failed to get DynamoDB item for lock", "lockId", lockId, "error", err)
		return nil, err
	}

	type TransactionLock struct {
		TransactionID int    `dynamodbav:"transaction_id"`
		Timeout       string `dynamodbav:"timeout"`
	}

	var t TransactionLock
	err = attributevalue.UnmarshalMap(result.Item, &t)
	if err != nil {
		slog.Error("Failed to unmarshal DynamoDB item", "error", err)
		return nil, err
	}
	if t.TransactionID != 0 {
		slog.Debug("Lock found",
			"lockId", lockId,
			"transactionId", t.TransactionID,
			"timeout", t.Timeout)
		return &t.TransactionID, nil
	}

	slog.Debug("No lock exists", "lockId", lockId)
	return nil, nil
}
