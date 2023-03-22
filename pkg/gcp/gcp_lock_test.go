package gcp

import (
	"cloud.google.com/go/storage"
	"context"
	"github.com/stretchr/testify/assert"
	"log"
	"math/rand"
	"testing"
	"time"
)

func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

	var seededRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))

	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

func GetGoogleStorageClient() (context.Context, *storage.Client) {
	ctx := context.Background()

	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatalf("Failed to create Google Storage client: %v", err)
	}
	return ctx, client
}

func TestGoogleStorageLock_NewLock(t *testing.T) {
	ctx, client := GetGoogleStorageClient()
	defer func(client *storage.Client) {
		err := client.Close()
		if err != nil {
			log.Fatalf("Failed to close Google Storage client: %v", err)
		}
	}(client)

	bucketName := "digger-lock-test"
	fileName := "digger-lock-" + randomString(16)

	bucket := client.Bucket(bucketName)
	lock := GoogleStorageLock{client, bucket, ctx}

	locked, err := lock.Lock(100, fileName)
	assert.NoError(t, err)
	assert.True(t, locked)
}

func TestGoogleStorageLock_LockLocked(t *testing.T) {
	ctx, client := GetGoogleStorageClient()
	defer func(client *storage.Client) {
		err := client.Close()
		if err != nil {
			log.Fatalf("Failed to close Google Storage client: %v", err)
		}
	}(client)

	bucketName := "digger-lock-test"
	fileName := "digger-lock-" + randomString(16)

	bucket := client.Bucket(bucketName)
	lock := GoogleStorageLock{client, bucket, ctx}

	locked, err := lock.Lock(100, fileName)
	assert.NoError(t, err)
	assert.True(t, locked)

	locked, err = lock.Lock(100, fileName)
	assert.NoError(t, err)
	assert.False(t, locked)
}

func TestGoogleStorageLock_UnlockLocked(t *testing.T) {
	ctx, client := GetGoogleStorageClient()
	defer func(client *storage.Client) {
		err := client.Close()
		if err != nil {
			log.Fatalf("Failed to close Google Storage client: %v", err)
		}
	}(client)

	bucketName := "digger-lock-test"
	fileName := "digger-lock-" + randomString(16)
	transactionId := 100

	bucket := client.Bucket(bucketName)
	lock := GoogleStorageLock{client, bucket, ctx}

	locked, err := lock.Lock(transactionId, fileName)
	assert.NoError(t, err)
	assert.True(t, locked)

	locked, err = lock.Unlock(transactionId, fileName)
	assert.NoError(t, err)
	assert.True(t, locked)

	lockTransactionId, err := lock.GetLock(fileName)
	assert.NoError(t, err)
	assert.Nil(t, lockTransactionId)
}

func TestGoogleStorageLock_UnlockLockedWithDifferentId(t *testing.T) {
	ctx, client := GetGoogleStorageClient()
	defer func(client *storage.Client) {
		err := client.Close()
		if err != nil {
			log.Fatalf("Failed to close Google Storage client: %v", err)
		}
	}(client)

	bucketName := "digger-lock-test"
	fileName := "digger-lock-" + randomString(16)
	transactionId := 100
	anotherTransactionId := 200

	bucket := client.Bucket(bucketName)
	lock := GoogleStorageLock{client, bucket, ctx}

	locked, err := lock.Lock(transactionId, fileName)
	assert.NoError(t, err)
	assert.True(t, locked)

	locked, err = lock.Unlock(anotherTransactionId, fileName)
	assert.NoError(t, err)
	assert.False(t, locked)
}

func TestGoogleStorageLock_GetExistingLock(t *testing.T) {
	ctx, client := GetGoogleStorageClient()
	defer func(client *storage.Client) {
		err := client.Close()
		if err != nil {
			log.Fatalf("Failed to close Google Storage client: %v", err)
		}
	}(client)

	bucketName := "digger-lock-test"
	fileName := "digger-lock-" + randomString(16)
	transactionId := 100

	bucket := client.Bucket(bucketName)
	lock := GoogleStorageLock{client, bucket, ctx}

	locked, err := lock.Lock(transactionId, fileName)
	assert.NoError(t, err)
	assert.True(t, locked)

	lockTransactionId, err := lock.GetLock(fileName)
	assert.NoError(t, err)
	assert.Equal(t, transactionId, *lockTransactionId)
}

func TestGoogleStorageLock_GetNotExistingLock(t *testing.T) {
	ctx, client := GetGoogleStorageClient()
	defer func(client *storage.Client) {
		err := client.Close()
		if err != nil {
			log.Fatalf("Failed to close Google Storage client: %v", err)
		}
	}(client)

	bucketName := "digger-lock-test"
	fileName := "digger-lock-" + randomString(16)

	bucket := client.Bucket(bucketName)
	lock := GoogleStorageLock{client, bucket, ctx}

	lockTransactionId, err := lock.GetLock(fileName)
	assert.NoError(t, err)
	assert.Nil(t, lockTransactionId)
}
