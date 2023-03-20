package main

import "errors"

type MockDynamoDb struct {
	records map[string]int
}

func (mockDynamoDb *MockDynamoDb) Lock(timeout int, transactionId int, resource string) (bool, error) {
	_, ok := mockDynamoDb.records[resource]
	if ok {
		return false, errors.New("lock already exists")
	}
	mockDynamoDb.records[resource] = transactionId
	return true, nil
}

func (mockDynamoDb *MockDynamoDb) Unlock(resource string) (bool, error) {
	delete(mockDynamoDb.records, resource)
	return true, nil
}

func (mockDynamoDb *MockDynamoDb) GetLock(resource string) (*int, error) {
	res, ok := mockDynamoDb.records[resource]
	if ok {
		return &res, nil
	} else {
		return nil, nil
	}
}
