package gcp

import (
	"context"
	"fmt"
	"log/slog"
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
		slog.Error("Failed to get bucket attributes", "error", err)
	}
	bucketName := bucketAttrs.Name

	if err := wc.Close(); err != nil {
		slog.Error("Unable to close bucket file",
			"bucket", bucketName,
			"file", fileName,
			"error", err)
	}

	slog.Debug("Lock acquired",
		"resource", resource,
		"transactionId", transactionId,
		"bucket", bucketName)
	return true, nil
}

func (googleLock *GoogleStorageLock) Unlock(resource string) (bool, error) {
	fileName := resource

	existingLockTransactionId, err := googleLock.GetLock(resource)
	if err != nil {
		slog.Error("Failed to get lock", "resource", resource, "error", err)
	}
	if existingLockTransactionId == nil {
		slog.Debug("No lock exists to unlock", "resource", resource)
		return false, nil
	}

	fileObject := googleLock.Bucket.Object(fileName)
	err = fileObject.Delete(googleLock.Context)
	if err != nil {
		return false, fmt.Errorf("failed to delete lock file: %v", err)
	}

	slog.Debug("Lock released",
		"resource", resource,
		"transactionId", *existingLockTransactionId)
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
		slog.Error("Failed to parse LockId in object's metadata",
			"resource", resource,
			"rawLockId", lockIdStr,
			"error", err)
	}

	slog.Debug("Retrieved lock information", "resource", resource, "transactionId", transactionId)
	return &transactionId, nil
}

func GetGoogleStorageClient() (context.Context, *storage.Client) {
	ctx := context.Background()

	client, err := storage.NewClient(ctx)
	if err != nil {
		slog.Error("Failed to create Google Storage client", "error", err)
		// Since the original used log.Fatalf which exits, we'll maintain that behavior
		panic(fmt.Sprintf("Failed to create Google Storage client: %v", err))
	}

	slog.Info("Google Storage client created successfully")
	return ctx, client
}
