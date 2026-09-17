package alicloud

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
)

const (
	LockBucketEnv = "ALICLOUD_OSS_LOCK_BUCKET"

	lockIDMetadataKey    = "lockid"
	createdAtMetadataKey = "createdat"
)

// OSSLock stores one object per locked resource; the holding PR number lives in the object's user metadata.
// Acquisition relies on x-oss-forbid-overwrite so that concurrent PutObject calls cannot both succeed.
type OSSLock struct {
	Client  OSSClient
	Bucket  string
	Context context.Context
}

// NewOSSLock reads ALICLOUD_OSS_LOCK_BUCKET and the client configuration from the environment.
func NewOSSLock() (*OSSLock, error) {
	bucket := os.Getenv(LockBucketEnv)
	if bucket == "" {
		return nil, fmt.Errorf("%s is not set", LockBucketEnv)
	}

	client, err := NewOSSClient()
	if err != nil {
		return nil, err
	}

	return &OSSLock{
		Client:  client,
		Bucket:  bucket,
		Context: context.Background(),
	}, nil
}

func (l *OSSLock) Lock(transactionID int, resource string) (bool, error) {
	_, err := l.Client.PutObject(l.Context, &oss.PutObjectRequest{
		Bucket:          oss.Ptr(l.Bucket),
		Key:             oss.Ptr(resource),
		ContentType:     oss.Ptr("text/plain"),
		ForbidOverwrite: oss.Ptr("true"),
		Metadata: map[string]string{
			lockIDMetadataKey:    strconv.Itoa(transactionID),
			createdAtMetadataKey: time.Now().UTC().Format(time.RFC3339),
		},
	})
	if err != nil {
		if HasStatus(err, http.StatusConflict) {
			slog.Debug("Lock already held", "resource", resource, "bucket", l.Bucket)
			return false, nil
		}
		return false, fmt.Errorf("put lock object %q in bucket %q: %w", resource, l.Bucket, err)
	}

	slog.Info("Lock acquired",
		"resource", resource,
		"transactionId", transactionID,
		"bucket", l.Bucket)
	return true, nil
}

func (l *OSSLock) Unlock(resource string) (bool, error) {
	_, err := l.Client.DeleteObject(l.Context, &oss.DeleteObjectRequest{
		Bucket: oss.Ptr(l.Bucket),
		Key:    oss.Ptr(resource),
	})
	if err != nil {
		return false, fmt.Errorf("delete lock object %q from bucket %q: %w", resource, l.Bucket, err)
	}

	slog.Info("Lock released", "resource", resource, "bucket", l.Bucket)
	return true, nil
}

func (l *OSSLock) GetLock(resource string) (*int, error) {
	result, err := l.Client.HeadObject(l.Context, &oss.HeadObjectRequest{
		Bucket: oss.Ptr(l.Bucket),
		Key:    oss.Ptr(resource),
	})
	if err != nil {
		if HasStatus(err, http.StatusNotFound) {
			slog.Debug("No lock exists", "resource", resource, "bucket", l.Bucket)
			return nil, nil
		}
		return nil, fmt.Errorf("head lock object %q in bucket %q: %w", resource, l.Bucket, err)
	}

	raw, ok := result.Metadata[lockIDMetadataKey]
	if !ok {
		return nil, fmt.Errorf("lock object %q in bucket %q has no %s metadata", resource, l.Bucket, lockIDMetadataKey)
	}
	transactionID, err := strconv.Atoi(raw)
	if err != nil {
		return nil, fmt.Errorf("parse %s metadata %q on lock object %q: %w", lockIDMetadataKey, raw, resource, err)
	}

	slog.Debug("Lock found",
		"resource", resource,
		"transactionId", transactionID,
		"bucket", l.Bucket)
	return &transactionID, nil
}
