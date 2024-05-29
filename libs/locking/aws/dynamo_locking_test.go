package aws

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type mockDynamoDbClient struct {
	table             map[string]map[string]types.AttributeValue
	Options           dynamodb.Options
	MockDescribeTable func(ctx context.Context, params dynamodb.DescribeTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error)
	MockUpdateItem    func(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error)
	MockGetItem       func(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	MockDeleteItem    func(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error)
}

func (m *mockDynamoDbClient) DescribeTable(ctx context.Context, params *dynamodb.DescribeTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
	if m.table == nil || m.table[aws.ToString(params.TableName)] == nil {
		return nil, &types.TableNotFoundException{}
	}
	if m.table[aws.ToString(params.TableName)] != nil {
		return &dynamodb.DescribeTableOutput{Table: &types.TableDescription{TableName: params.TableName}}, nil
	}
	return nil, nil
}

func (m *mockDynamoDbClient) CreateTable(ctx context.Context, params *dynamodb.CreateTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.CreateTableOutput, error) {
	m.table[aws.ToString(params.TableName)] = make(map[string]types.AttributeValue)
	return nil, nil
}

func (m *mockDynamoDbClient) UpdateItem(ctx context.Context, params *dynamodb.UpdateItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	// TODO: Implement this
	return &dynamodb.UpdateItemOutput{}, nil
}

func (m *mockDynamoDbClient) GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	return &dynamodb.GetItemOutput{
		Item: map[string]types.AttributeValue{
			"PK":             &types.AttributeValueMemberS{Value: "LOCK"},
			"SK":             &types.AttributeValueMemberS{Value: "RES#example-resource"},
			"transaction_id": &types.AttributeValueMemberN{Value: "123"},
			"timeout":        &types.AttributeValueMemberS{Value: "2024-04-01T00:00:00Z"},
		},
	}, nil
}

func (m *mockDynamoDbClient) DeleteItem(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
	m.table[aws.ToString(params.TableName)][aws.ToString(&params.Key["SK"].(*types.AttributeValueMemberS).Value)] = nil
	return &dynamodb.DeleteItemOutput{}, nil
}

func TestDynamoDbLock_Lock(t *testing.T) {
	client := mockDynamoDbClient{table: make(map[string]map[string]types.AttributeValue)}
	dynamodbLock := DynamoDbLock{
		DynamoDb: &client,
	}
	dynamodbLock.DynamoDb.CreateTable(context.Background(), &dynamodb.CreateTableInput{TableName: aws.String(TABLE_NAME)})

	// Set up the input parameters for the Lock method
	transactionId := 123
	resource := "example-resource"

	locked, err := dynamodbLock.Lock(transactionId, resource)
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	if !locked {
		t.Fatalf("Expected true, got %v", locked)
	}
}
func TestDynamoDbLock_GetLock(t *testing.T) {
	// Create a mock DynamoDB client
	client := mockDynamoDbClient{table: make(map[string]map[string]types.AttributeValue)}
	dynamodbLock := DynamoDbLock{
		DynamoDb: &client,
	}
	dynamodbLock.DynamoDb.CreateTable(context.Background(), &dynamodb.CreateTableInput{TableName: aws.String(TABLE_NAME)})

	id, err := dynamodbLock.GetLock("example-resource")
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	if *id != 123 {
		t.Fatalf("Expected 123, got %v", id)
	}
}
