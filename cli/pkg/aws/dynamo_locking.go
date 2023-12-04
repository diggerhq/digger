package aws

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
)

const (
	TABLE_NAME              = "DiggerDynamoDBLockTable"
	TableCreationInterval   = 1 * time.Second
	TableCreationRetryCount = 10
	TableLockTimeout        = 60 * 60 * 24 * 90 * time.Second
)

type DynamoDbLock struct {
	DynamoDb *dynamodb.DynamoDB
}

func isResourceNotFoundExceptionError(err error) bool {
	if err != nil {
		aerr, ok := err.(awserr.Error)
		if !ok {
			// not aws error
			return false
		}

		if aerr.Code() == dynamodb.ErrCodeResourceNotFoundException {
			// ErrCodeResourceNotFoundException code
			return true
		}
	}
	return false
}

func (dynamoDbLock *DynamoDbLock) waitUntilTableCreated() error {
	input := &dynamodb.DescribeTableInput{
		TableName: aws.String(TABLE_NAME),
	}
	status, err := dynamoDbLock.DynamoDb.DescribeTable(input)
	cnt := 0

	if err != nil {
		if !isResourceNotFoundExceptionError(err) {
			return err
		}
	}

	for *status.Table.TableStatus != "ACTIVE" {
		time.Sleep(TableCreationInterval)
		status, err = dynamoDbLock.DynamoDb.DescribeTable(input)
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
func (dynamoDbLock *DynamoDbLock) createTableIfNotExists() error {
	_, err := dynamoDbLock.DynamoDb.DescribeTable(&dynamodb.DescribeTableInput{
		TableName: aws.String(TABLE_NAME),
	})

	if err != nil {
		if !isResourceNotFoundExceptionError(err) {
			return err
		}
	}

	createtbl_input := &dynamodb.CreateTableInput{
		AttributeDefinitions: []*dynamodb.AttributeDefinition{
			{
				AttributeName: aws.String("PK"),
				AttributeType: aws.String("S"),
			},
			{
				AttributeName: aws.String("SK"),
				AttributeType: aws.String("S"),
			},
		},
		KeySchema: []*dynamodb.KeySchemaElement{
			{
				AttributeName: aws.String("PK"),
				KeyType:       aws.String("HASH"),
			},
			{
				AttributeName: aws.String("SK"),
				KeyType:       aws.String("RANGE"),
			},
		},
		BillingMode: aws.String("PAY_PER_REQUEST"),
		TableName:   aws.String(TABLE_NAME),
	}
	_, err = dynamoDbLock.DynamoDb.CreateTable(createtbl_input)
	if err != nil {
		if os.Getenv("DEBUG") != "" {
			log.Printf("%v\n", err)
		}
		return err
	}

	err = dynamoDbLock.waitUntilTableCreated()
	if err != nil {
		log.Printf("%v\n", err)
		return err
	}
	log.Printf("DynamoDB Table %v has been created\n", TABLE_NAME)
	return nil
}

func (dynamoDbLock *DynamoDbLock) Lock(transactionId int, resource string) (bool, error) {
	dynamoDbLock.createTableIfNotExists()
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
		TableName:                 aws.String(TABLE_NAME),
		Key:                       map[string]*dynamodb.AttributeValue{"PK": {S: aws.String("LOCK")}, "SK": {S: aws.String("RES#" + resource)}},
		ConditionExpression:       expr.Condition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		UpdateExpression:          expr.Update(),
	}

	_, err = dynamoDbLock.DynamoDb.UpdateItem(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			if aerr.Code() == dynamodb.ErrCodeConditionalCheckFailedException {
				return false, nil
			}
		}
		return false, err
	}

	return true, nil
}

func (dynamoDbLock *DynamoDbLock) Unlock(resource string) (bool, error) {
	dynamoDbLock.createTableIfNotExists()
	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(TABLE_NAME),
		Key:       map[string]*dynamodb.AttributeValue{"PK": {S: aws.String("LOCK")}, "SK": {S: aws.String("RES#" + resource)}},
	}

	_, err := dynamoDbLock.DynamoDb.DeleteItem(input)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (dynamoDbLock *DynamoDbLock) GetLock(lockId string) (*int, error) {
	dynamoDbLock.createTableIfNotExists()
	input := &dynamodb.GetItemInput{
		TableName: aws.String(TABLE_NAME),
		Key:       map[string]*dynamodb.AttributeValue{"PK": {S: aws.String("LOCK")}, "SK": {S: aws.String("RES#" + lockId)}},
	}

	result, err := dynamoDbLock.DynamoDb.GetItem(input)
	if err != nil {
		return nil, err
	}

	if result.Item != nil {
		transactionId := result.Item["transaction_id"].N
		res, err := strconv.Atoi(*transactionId)
		return &res, err
	}

	return nil, nil
}
