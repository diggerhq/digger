package gcp

import (
	"log/slog"
	"math/rand"
	"os"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"github.com/stretchr/testify/assert"
)

func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))

	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

func SkipCI(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping testing in CI environment")
	}
}

func TestGoogleStorageLock_NewLock(t *testing.T) {
	SkipCI(t)
	ctx, client := GetGoogleStorageClient()
	defer func(client *storage.Client) {
		err := client.Close()
		if err != nil {
			slog.Error("Failed to close Google Storage client", "error", err)
			// Keep the fatal behavior from the original
			t.Fatalf("Failed to close Google Storage client: %v", err)
		}
	}(client)

	bucketName := "digger-lock-test"
	fileName := "digger-lock-" + randomString(16)

	slog.Info("Testing new lock creation", "fileName", fileName, "bucketName", bucketName)

	bucket := client.Bucket(bucketName)
	lock := GoogleStorageLock{client, bucket, ctx}

	locked, err := lock.Lock(100, fileName)
	assert.NoError(t, err)
	assert.True(t, locked)
}

func TestGoogleStorageLock_LockLocked(t *testing.T) {
	SkipCI(t)
	ctx, client := GetGoogleStorageClient()
	defer func(client *storage.Client) {
		err := client.Close()
		if err != nil {
			slog.Error("Failed to close Google Storage client", "error", err)
			t.Fatalf("Failed to close Google Storage client: %v", err)
		}
	}(client)

	bucketName := "digger-lock-test"
	fileName := "digger-lock-" + randomString(16)

	slog.Info("Testing locking already locked resource", "fileName", fileName, "bucketName", bucketName)

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
	SkipCI(t)
	ctx, client := GetGoogleStorageClient()
	defer func(client *storage.Client) {
		err := client.Close()
		if err != nil {
			slog.Error("Failed to close Google Storage client", "error", err)
			t.Fatalf("Failed to close Google Storage client: %v", err)
		}
	}(client)

	bucketName := "digger-lock-test"
	fileName := "digger-lock-" + randomString(16)
	transactionId := 100

	slog.Info("Testing unlocking locked resource",
		"fileName", fileName,
		"bucketName", bucketName,
		"transactionId", transactionId)

	bucket := client.Bucket(bucketName)
	lock := GoogleStorageLock{client, bucket, ctx}

	locked, err := lock.Lock(transactionId, fileName)
	assert.NoError(t, err)
	assert.True(t, locked)

	unlocked, err := lock.Unlock(fileName)
	assert.NoError(t, err)
	assert.True(t, unlocked)

	lockTransactionId, err := lock.GetLock(fileName)
	assert.NoError(t, err)
	assert.Nil(t, lockTransactionId)
}

func TestGoogleStorageLock_UnlockLockedWithDifferentId(t *testing.T) {
	SkipCI(t)
	ctx, client := GetGoogleStorageClient()
	defer func(client *storage.Client) {
		err := client.Close()
		if err != nil {
			slog.Error("Failed to close Google Storage client", "error", err)
			t.Fatalf("Failed to close Google Storage client: %v", err)
		}
	}(client)

	bucketName := "digger-lock-test"
	fileName := "digger-lock-" + randomString(16)
	transactionId := 100
	// anotherTransactionId := 200

	slog.Info("Testing unlocking with different transaction ID",
		"fileName", fileName,
		"bucketName", bucketName,
		"transactionId", transactionId)

	bucket := client.Bucket(bucketName)
	lock := GoogleStorageLock{client, bucket, ctx}

	locked, err := lock.Lock(transactionId, fileName)
	assert.NoError(t, err)
	assert.True(t, locked)

	unlocked, err := lock.Unlock(fileName)
	assert.NoError(t, err)
	assert.True(t, unlocked)
}

func TestGoogleStorageLock_GetExistingLock(t *testing.T) {
	SkipCI(t)
	ctx, client := GetGoogleStorageClient()
	defer func(client *storage.Client) {
		err := client.Close()
		if err != nil {
			slog.Error("Failed to close Google Storage client", "error", err)
			t.Fatalf("Failed to close Google Storage client: %v", err)
		}
	}(client)

	bucketName := "digger-lock-test"
	fileName := "digger-lock-" + randomString(16)
	transactionId := 100

	slog.Info("Testing getting existing lock",
		"fileName", fileName,
		"bucketName", bucketName,
		"transactionId", transactionId)

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
	SkipCI(t)
	ctx, client := GetGoogleStorageClient()
	defer func(client *storage.Client) {
		err := client.Close()
		if err != nil {
			slog.Error("Failed to close Google Storage client", "error", err)
			t.Fatalf("Failed to close Google Storage client: %v", err)
		}
	}(client)

	bucketName := "digger-lock-test"
	fileName := "digger-lock-" + randomString(16)

	slog.Info("Testing getting non-existing lock",
		"fileName", fileName,
		"bucketName", bucketName)

	bucket := client.Bucket(bucketName)
	lock := GoogleStorageLock{client, bucket, ctx}

	lockTransactionId, err := lock.GetLock(fileName)
	assert.NoError(t, err)
	assert.Nil(t, lockTransactionId)
}
