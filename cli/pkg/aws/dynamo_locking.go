package aws

import (
	"context"
	"errors"
	"log"
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
	DynamoDb *dynamodb.Client
}

func isResourceNotFoundExceptionError(err error) bool {
	if err != nil {
		var apiError smithy.APIError
		if errors.As(err, &apiError) {
			switch apiError.(type) {
			case *types.ResourceNotFoundException:
				return true
			}
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
		if !isResourceNotFoundExceptionError(err) {
			return err
		}
	}

	for status.Table.TableStatus != "ACTIVE" {
		time.Sleep(TableCreationInterval)
		status, err = dynamoDbLock.DynamoDb.DescribeTable(ctx, input)
		if err != nil {
			if !isResourceNotFoundExceptionError(err) {
				return err
			}
		}
		cnt++
		if cnt > TableCreationRetryCount {
			log.Printf("DynamoDB failed to create, timed out during creation.\n" +
				"Rerunning the action may cause creation to succeed\n")
			os.Exit(1)
		}
	}

	return nil
}

// TODO: refactor func to return actual error and fail on callers
func (dynamoDbLock *DynamoDbLock) createTableIfNotExists(ctx context.Context) error {
	_, err := dynamoDbLock.DynamoDb.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: aws.String(TABLE_NAME),
	})

	if err != nil {
		if !isResourceNotFoundExceptionError(err) {
			return err
		}
	}

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
			log.Printf("%v\n", err)
		}
		return err
	}

	err = dynamoDbLock.waitUntilTableCreated(ctx)
	if err != nil {
		log.Printf("%v\n", err)
		return err
	}
	log.Printf("DynamoDB Table %v has been created\n", TABLE_NAME)
	return nil
}

func (dynamoDbLock *DynamoDbLock) Lock(transactionId int, resource string) (bool, error) {
	ctx := context.Background()
	dynamoDbLock.createTableIfNotExists(ctx)
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
				return false, nil
			}
		}
		return false, err
	}

	return true, nil
}

func (dynamoDbLock *DynamoDbLock) Unlock(resource string) (bool, error) {
	ctx := context.Background()
	dynamoDbLock.createTableIfNotExists(ctx)
	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(TABLE_NAME),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: "LOCK"},
			"SK": &types.AttributeValueMemberS{Value: "RES#" + resource},
		},
	}

	_, err := dynamoDbLock.DynamoDb.DeleteItem(ctx, input)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (dynamoDbLock *DynamoDbLock) GetLock(lockId string) (*int, error) {
	ctx := context.Background()
	dynamoDbLock.createTableIfNotExists(ctx)
	input := &dynamodb.GetItemInput{
		TableName: aws.String(TABLE_NAME),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: "LOCK"},
			"SK": &types.AttributeValueMemberS{Value: "RES#" + lockId},
		},
	}

	result, err := dynamoDbLock.DynamoDb.GetItem(ctx, input)
	if err != nil {
		return nil, err
	}

	type TransactionLock struct {
		TransactionID int `dynamodbav:"transaction_id"`
	}

	var t TransactionLock
	err = attributevalue.UnmarshalMap(result.Item, &t)
	if err != nil {
		return nil, err
	}
	if t.TransactionID != 0 {
		return &t.TransactionID, nil
	}
	return nil, nil
}
