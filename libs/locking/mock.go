package locking

type MockLock struct {
	MapLock map[string]int
}

func (lock *MockLock) Lock(transactionId int, resource string) (bool, error) {
	if lock.MapLock == nil {
		lock.MapLock = make(map[string]int)
	}
	lock.MapLock[resource] = transactionId
	return true, nil
}

func (lock *MockLock) Unlock(resource string) (bool, error) {
	delete(lock.MapLock, resource)
	return true, nil
}

func (lock *MockLock) GetLock(resource string) (*int, error) {
	result, ok := lock.MapLock[resource]
	if ok {
		return &result, nil
	}
	return nil, nil
}
