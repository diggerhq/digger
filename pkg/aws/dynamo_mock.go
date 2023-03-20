package aws

import "errors"

type MockDynamoDb struct {
	Records map[string]int
}

func (mockDynamoDb *MockDynamoDb) Lock(timeout int, transactionId int, resource string) (bool, error) {
	_, ok := mockDynamoDb.Records[resource]
	if ok {
		return false, errors.New("lock already exists")
	}
	mockDynamoDb.Records[resource] = transactionId
	return true, nil
}

func (mockDynamoDb *MockDynamoDb) Unlock(resource string) (bool, error) {
	delete(mockDynamoDb.Records, resource)
	return true, nil
}

func (mockDynamoDb *MockDynamoDb) GetLock(resource string) (*int, error) {
	res, ok := mockDynamoDb.Records[resource]
	if ok {
		return &res, nil
	} else {
		return nil, nil
	}
}
