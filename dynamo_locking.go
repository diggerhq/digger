package main

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
	"time"
)

const TABLE_NAME = "DiggerDynamoDBLockTable"

type DynamoDbLock struct {
	DynamoDb dynamodb.DynamoDB
}

type Lock interface {
	Lock(timeout int, transactionId string, resource string) (bool, error)
	Unlock(resource string) (bool, error)
	GetLock(resource string) (*string, error)
}

func (dynamoDbLock *DynamoDbLock) Lock(timeout int, transactionId string, resource string) (bool, error) {
	now := time.Now().Format(time.RFC3339)
	newTimeout := time.Now().Add(time.Duration(timeout) * time.Second).Format(time.RFC3339)

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
		if err.Error() == dynamodb.ErrCodeConditionalCheckFailedException {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (dynamoDbLock *DynamoDbLock) Unlock(resource string) (bool, error) {
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

func (dynamoDbLock *DynamoDbLock) GetLock(resource string) (*string, error) {
	input := &dynamodb.GetItemInput{
		TableName: aws.String(TABLE_NAME),
		Key:       map[string]*dynamodb.AttributeValue{"PK": {S: aws.String("LOCK")}, "SK": {S: aws.String("RES#" + resource)}},
	}

	result, err := dynamoDbLock.DynamoDb.GetItem(input)
	if err != nil {
		return nil, err
	}

	if result.Item != nil {
		return result.Item["transaction_id"].S, nil
	} else {
		return nil, nil
	}

}
