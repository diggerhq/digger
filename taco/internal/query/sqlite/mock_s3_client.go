package sqlite

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

// mockS3Client implements an in-memory S3 client for testing.
// This allows us to use the production rbac.s3RBACStore code in tests,
// ensuring we test actual production behavior including:
// - Optimistic locking (version conflicts)
// - Retry logic
// - Subject sanitization
// - S3-specific error handling
type mockS3Client struct {
	mu      sync.RWMutex
	objects map[string][]byte // key -> data
}

func newMockS3Client() *mockS3Client {
	return &mockS3Client{
		objects: make(map[string][]byte),
	}
}

// GetObject retrieves an object from the mock S3 store
func (m *mockS3Client) GetObject(ctx context.Context, input *s3.GetObjectInput, opts ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := aws.ToString(input.Key)
	data, exists := m.objects[key]
	if !exists {
		// Return S3-style NoSuchKey error (production uses this)
		return nil, &smithy.OperationError{
			ServiceID:     "S3",
			OperationName: "GetObject",
			Err: &smithy.GenericAPIError{
				Code:    "NoSuchKey",
				Message: fmt.Sprintf("The specified key does not exist: %s", key),
			},
		}
	}

	return &s3.GetObjectOutput{
		Body: io.NopCloser(bytes.NewReader(data)),
	}, nil
}

// PutObject stores an object in the mock S3 store
func (m *mockS3Client) PutObject(ctx context.Context, input *s3.PutObjectInput, opts ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := aws.ToString(input.Key)
	data, err := io.ReadAll(input.Body)
	if err != nil {
		return nil, err
	}

	m.objects[key] = data
	return &s3.PutObjectOutput{}, nil
}

// DeleteObject removes an object from the mock S3 store
func (m *mockS3Client) DeleteObject(ctx context.Context, input *s3.DeleteObjectInput, opts ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := aws.ToString(input.Key)
	delete(m.objects, key)
	return &s3.DeleteObjectOutput{}, nil
}

// ListObjectsV2 lists objects with a given prefix
func (m *mockS3Client) ListObjectsV2(ctx context.Context, input *s3.ListObjectsV2Input, opts ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	prefix := ""
	if input.Prefix != nil {
		prefix = aws.ToString(input.Prefix)
	}

	var contents []types.Object
	for key := range m.objects {
		if strings.HasPrefix(key, prefix) {
			size := int64(len(m.objects[key]))
			keyCopy := key
			contents = append(contents, types.Object{
				Key:  &keyCopy,
				Size: &size,
			})
		}
	}

	// Simple implementation without pagination
	// Production handles pagination, but for tests this is sufficient
	return &s3.ListObjectsV2Output{
		Contents:    contents,
		IsTruncated: aws.Bool(false),
	}, nil
}

