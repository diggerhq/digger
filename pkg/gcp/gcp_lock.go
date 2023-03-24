package gcp

import (
	"cloud.google.com/go/storage"
	"context"
	"fmt"
	"strconv"
	"time"
)

type GoogleStorageLock struct {
	Client *storage.Client
	Bucket *storage.BucketHandle
	ctx    context.Context
}

func (googleLock *GoogleStorageLock) Lock(transactionId int, resource string) (bool, error) {
	existingLockTransactionId, err := googleLock.GetLock(resource)
	if err != nil {
		fmt.Printf("failed to get lock: %v\n", err)
	}
	if existingLockTransactionId != nil {
		return false, nil
	}

	now := time.Now().Format(time.RFC3339)
	fileName := resource

	wc := googleLock.Bucket.Object(fileName).NewWriter(googleLock.ctx)
	wc.ContentType = "text/plain"
	wc.Metadata = map[string]string{
		"LockId":    strconv.Itoa(transactionId),
		"CreatedAt": now,
	}

	bucketAttrs, err := googleLock.Bucket.Attrs(googleLock.ctx)
	if err != nil {
		fmt.Printf("failed to get bucket attributes: %v\n", err)
	}
	bucketName := bucketAttrs.Name

	if err := wc.Close(); err != nil {
		fmt.Printf("createFile: unable to close bucket %q, file %q: %v\n", bucketName, fileName, err)
	}
	return true, nil
}

func (googleLock *GoogleStorageLock) Unlock(transactionId int, resource string) (bool, error) {
	fileName := resource

	existingLockTransactionId, err := googleLock.GetLock(resource)
	if err != nil {
		fmt.Printf("failed to get lock: %v\n", err)
	}
	if existingLockTransactionId == nil {
		return false, nil
	}

	if transactionId != *existingLockTransactionId {
		fmt.Printf("failed to Unlock, transactionId doesn't match: %d != %d\n", transactionId, *existingLockTransactionId)
		return false, nil
	}

	// lock exist, transactionId matches, we can delete the lock

	bucketAttrs, err := googleLock.Bucket.Attrs(googleLock.ctx)
	if err != nil {
		fmt.Printf("failed to get bucket attributes: %v\n", err)
	}
	bucketName := bucketAttrs.Name
	print("bucketname: ")
	println(bucketName)

	fileObject := googleLock.Bucket.Object(fileName)
	err = fileObject.Delete(googleLock.ctx)
	if err != nil {
		return false, fmt.Errorf("failed to delete lock file: %v\n", err)
	}

	return true, nil
}

func (googleLock *GoogleStorageLock) GetLock(resource string) (*int, error) {
	fileName := resource

	bucketAttrs, err := googleLock.Bucket.Attrs(googleLock.ctx)
	if err != nil {
		fmt.Printf("failed to get bucket attributes: %v\n", err)
	}
	bucketName := bucketAttrs.Name
	print("bucketname: ")
	println(bucketName)

	fileObject := googleLock.Bucket.Object(fileName)
	fileAttrs, err := fileObject.Attrs(googleLock.ctx)
	if err != nil {
		// TODO: not sure if it's the best way to handle it
		if err.Error() == "storage: object doesn't exist" {
			return nil, nil
		}
		return nil, err
	}
	fileMetadata := fileAttrs.Metadata
	lockIdStr := fileMetadata["LockId"]
	transactionId, err := strconv.Atoi(lockIdStr)
	if err != nil {
		fmt.Printf("failed to parse LockId in object's metadata: %v\n", err)
	}
	return &transactionId, nil
}
