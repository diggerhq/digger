package gcp

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"cloud.google.com/go/storage"
)

type GoogleStorageLock struct {
	Client  *storage.Client
	Bucket  *storage.BucketHandle
	Context context.Context
}

func (googleLock *GoogleStorageLock) Lock(transactionId int, resource string) (bool, error) {
	now := time.Now().Format(time.RFC3339)
	fileName := resource

	wc := googleLock.Bucket.Object(fileName).NewWriter(googleLock.Context)
	wc.ContentType = "text/plain"
	wc.Metadata = map[string]string{
		"LockId":    strconv.Itoa(transactionId),
		"CreatedAt": now,
	}

	bucketAttrs, err := googleLock.Bucket.Attrs(googleLock.Context)
	if err != nil {
		log.Printf("failed to get bucket attributes: %v\n", err)
	}
	bucketName := bucketAttrs.Name

	if err := wc.Close(); err != nil {
		log.Printf("createFile: unable to close bucket %q, file %q: %v\n", bucketName, fileName, err)
	}
	return true, nil
}

func (googleLock *GoogleStorageLock) Unlock(resource string) (bool, error) {
	fileName := resource

	existingLockTransactionId, err := googleLock.GetLock(resource)
	if err != nil {
		log.Printf("failed to get lock: %v\n", err)
	}
	if existingLockTransactionId == nil {
		return false, nil
	}

	fileObject := googleLock.Bucket.Object(fileName)
	err = fileObject.Delete(googleLock.Context)
	if err != nil {
		return false, fmt.Errorf("failed to delete lock file: %v\n", err)
	}

	return true, nil
}

func (googleLock *GoogleStorageLock) GetLock(resource string) (*int, error) {
	fileName := resource
	fileObject := googleLock.Bucket.Object(fileName)
	fileAttrs, err := fileObject.Attrs(googleLock.Context)
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
		log.Printf("failed to parse LockId in object's metadata: %v\n", err)
	}
	return &transactionId, nil
}

func GetGoogleStorageClient() (context.Context, *storage.Client) {
	ctx := context.Background()

	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatalf("Failed to create Google Storage client: %v", err)
	}
	return ctx, client
}
