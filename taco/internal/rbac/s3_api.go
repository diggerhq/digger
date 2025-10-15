package rbac

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3API defines the minimal S3 operations needed by s3RBACStore.
// This interface allows us to use mock S3 clients in tests while
// production code uses the real AWS S3 client.
type S3API interface {
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
}

// Ensure *s3.Client implements S3API (compile-time check)
var _ S3API = (*s3.Client)(nil)

