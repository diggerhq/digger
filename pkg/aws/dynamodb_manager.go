package aws

import "github.com/aws/aws-sdk-go/service/dynamodb"

type DynamoDBManager interface {
}

type DynamoDBManagerImpl struct {
	DescribeTableInput *dynamodb.DescribeTableInput
	CreateTableInput   *dynamodb.CreateTableInput
}
