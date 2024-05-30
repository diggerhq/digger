package locking

import (
	"errors"
	"fmt"
	"github.com/diggerhq/digger/backend/models"
	"gorm.io/gorm"
	"log"
)

type BackendDBLock struct {
	OrgId uint
}

func (lock BackendDBLock) Lock(lockId int, resource string) (bool, error) {
	_, err := models.DB.CreateDiggerLock(resource, lockId, lock.OrgId)
	if err != nil {
		return false, fmt.Errorf("could not create lock record: %v", err)
	}
	return true, nil
}

func (lock BackendDBLock) Unlock(resource string) (bool, error) {
	theLock, err := models.DB.GetDiggerLock(resource)
	if err != nil {
		if err != nil {
			return false, fmt.Errorf("could not get lock record: %v", err)
		}
	}

	err = models.DB.DeleteDiggerLock(theLock)
	if err != nil {
		return false, fmt.Errorf("could not delete lock record: %v", err)
	}

	return true, nil
}

func (lock BackendDBLock) GetLock(resource string) (*int, error) {
	theLock, err := models.DB.GetDiggerLock(resource)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		log.Printf("its an error not found")
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("could not get lock record: %v", err)
	}
	return &theLock.LockId, nil
}
