package alicloud

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
	openapicred "github.com/aliyun/credentials-go/credentials"
)

const (
	RegionEnv   = "ALICLOUD_OSS_REGION"
	EndpointEnv = "ALICLOUD_OSS_ENDPOINT"

	ossAccessKeyIDEnv        = "OSS_ACCESS_KEY_ID"
	alibabaAccessKeyIDEnv    = "ALIBABA_CLOUD_ACCESS_KEY_ID"
	alicloudAccessKeyEnv     = "ALICLOUD_ACCESS_KEY"
	alicloudSecretKeyEnv     = "ALICLOUD_SECRET_KEY"
	alicloudSecurityTokenEnv = "ALICLOUD_SECURITY_TOKEN"
)

// OSSClient is the subset of *oss.Client that digger uses, so tests can substitute a fake.
type OSSClient interface {
	PutObject(ctx context.Context, request *oss.PutObjectRequest, optFns ...func(*oss.Options)) (*oss.PutObjectResult, error)
	HeadObject(ctx context.Context, request *oss.HeadObjectRequest, optFns ...func(*oss.Options)) (*oss.HeadObjectResult, error)
	GetObject(ctx context.Context, request *oss.GetObjectRequest, optFns ...func(*oss.Options)) (*oss.GetObjectResult, error)
	DeleteObject(ctx context.Context, request *oss.DeleteObjectRequest, optFns ...func(*oss.Options)) (*oss.DeleteObjectResult, error)
	GetBucketVersioning(ctx context.Context, request *oss.GetBucketVersioningRequest, optFns ...func(*oss.Options)) (*oss.GetBucketVersioningResult, error)
}

// NewOSSClient builds an OSS client from ALICLOUD_OSS_REGION (falling back to the region variables
// the Terraform alicloud provider reads) and the Alibaba Cloud credentials found in the environment.
func NewOSSClient() (*oss.Client, error) {
	region := cmp.Or(os.Getenv(RegionEnv), os.Getenv("ALICLOUD_REGION"), os.Getenv("ALIBABA_CLOUD_REGION_ID"))
	if region == "" {
		return nil, fmt.Errorf("%s is not set", RegionEnv)
	}

	provider, err := credentialsProvider()
	if err != nil {
		return nil, err
	}

	cfg := oss.LoadDefaultConfig().
		WithRegion(region).
		WithCredentialsProvider(provider)

	endpoint := os.Getenv(EndpointEnv)
	if endpoint != "" {
		cfg = cfg.WithEndpoint(endpoint)
	}

	slog.Info("Alibaba Cloud OSS client created", "region", region, "endpoint", endpoint)
	return oss.NewClient(cfg), nil
}

// credentialsProvider order: OSS_ACCESS_KEY_*, then ALICLOUD_ACCESS_KEY/ALICLOUD_SECRET_KEY when ALIBABA_CLOUD_ACCESS_KEY_ID
// is unset, otherwise the Alibaba Cloud default chain (ALIBABA_CLOUD_*, OIDC role, ECS RAM role).
func credentialsProvider() (credentials.CredentialsProvider, error) {
	if os.Getenv(ossAccessKeyIDEnv) != "" {
		slog.Debug("Using OSS access key credentials from environment")
		return credentials.NewEnvironmentVariableCredentialsProvider(), nil
	}

	if os.Getenv(alibabaAccessKeyIDEnv) == "" && os.Getenv(alicloudAccessKeyEnv) != "" {
		slog.Debug("Using ALICLOUD_* access key credentials from environment")
		return credentials.NewStaticCredentialsProvider(
			os.Getenv(alicloudAccessKeyEnv),
			os.Getenv(alicloudSecretKeyEnv),
			os.Getenv(alicloudSecurityTokenEnv),
		), nil
	}

	slog.Debug("Using Alibaba Cloud default credentials chain")
	cred, err := openapicred.NewCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("initialise Alibaba Cloud credentials chain: %w", err)
	}

	return credentials.CredentialsProviderFunc(func(context.Context) (credentials.Credentials, error) {
		model, err := cred.GetCredential()
		if err != nil {
			return credentials.Credentials{}, fmt.Errorf("resolve Alibaba Cloud credentials: %w", err)
		}
		return credentials.Credentials{
			AccessKeyID:     oss.ToString(model.AccessKeyId),
			AccessKeySecret: oss.ToString(model.AccessKeySecret),
			SecurityToken:   oss.ToString(model.SecurityToken),
		}, nil
	}), nil
}

// HasStatus reports whether err is an OSS service error carrying the given HTTP status code.
func HasStatus(err error, status int) bool {
	var serviceErr *oss.ServiceError
	return errors.As(err, &serviceErr) && serviceErr.StatusCode == status
}

// IsNoSuchKey reports whether err is OSS's NoSuchKey error; a missing bucket is a 404 too but must surface as an error.
func IsNoSuchKey(err error) bool {
	var serviceErr *oss.ServiceError
	return errors.As(err, &serviceErr) && serviceErr.Code == "NoSuchKey"
}
