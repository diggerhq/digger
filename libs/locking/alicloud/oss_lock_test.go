package alicloud

import (
	"context"
	"errors"
	"maps"
	"net/http"
	"testing"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeOSSClient struct {
	objects       map[string]map[string]string
	putErr        error
	headErr       error
	versioning    *string
	versioningErr error
}

func newFakeOSSClient() *fakeOSSClient {
	return &fakeOSSClient{objects: make(map[string]map[string]string)}
}

func (f *fakeOSSClient) PutObject(_ context.Context, request *oss.PutObjectRequest, _ ...func(*oss.Options)) (*oss.PutObjectResult, error) {
	if f.putErr != nil {
		return nil, f.putErr
	}
	key := oss.ToString(request.Key)
	if _, exists := f.objects[key]; exists && oss.ToString(request.ForbidOverwrite) == "true" {
		return nil, &oss.ServiceError{StatusCode: http.StatusConflict, Code: "FileAlreadyExists"}
	}
	f.objects[key] = maps.Clone(request.Metadata)
	return &oss.PutObjectResult{}, nil
}

func (f *fakeOSSClient) HeadObject(_ context.Context, request *oss.HeadObjectRequest, _ ...func(*oss.Options)) (*oss.HeadObjectResult, error) {
	if f.headErr != nil {
		return nil, f.headErr
	}
	metadata, ok := f.objects[oss.ToString(request.Key)]
	if !ok {
		return nil, &oss.ServiceError{StatusCode: http.StatusNotFound, Code: "NoSuchKey"}
	}
	return &oss.HeadObjectResult{Metadata: maps.Clone(metadata)}, nil
}

func (f *fakeOSSClient) GetObject(_ context.Context, _ *oss.GetObjectRequest, _ ...func(*oss.Options)) (*oss.GetObjectResult, error) {
	return nil, errors.New("not used by OSSLock")
}

func (f *fakeOSSClient) DeleteObject(_ context.Context, request *oss.DeleteObjectRequest, _ ...func(*oss.Options)) (*oss.DeleteObjectResult, error) {
	delete(f.objects, oss.ToString(request.Key))
	return &oss.DeleteObjectResult{}, nil
}

func (f *fakeOSSClient) GetBucketVersioning(_ context.Context, _ *oss.GetBucketVersioningRequest, _ ...func(*oss.Options)) (*oss.GetBucketVersioningResult, error) {
	if f.versioningErr != nil {
		return nil, f.versioningErr
	}
	return &oss.GetBucketVersioningResult{VersionStatus: f.versioning}, nil
}

func newTestLock(client OSSClient) *OSSLock {
	return &OSSLock{Client: client, Bucket: "digger-locks", Context: context.Background()}
}

func TestOSSLock_Lock(t *testing.T) {
	const resource = "org/repo#dev"

	t.Run("acquires a free lock and records the transaction id", func(t *testing.T) {
		client := newFakeOSSClient()
		lock := newTestLock(client)

		acquired, err := lock.Lock(42, resource)
		require.NoError(t, err)
		assert.True(t, acquired)
		assert.Equal(t, "42", client.objects[resource][lockIDMetadataKey])
		assert.NotEmpty(t, client.objects[resource][createdAtMetadataKey])
	})

	t.Run("does not overwrite a lock held by another transaction", func(t *testing.T) {
		client := newFakeOSSClient()
		lock := newTestLock(client)
		_, err := lock.Lock(42, resource)
		require.NoError(t, err)

		acquired, err := lock.Lock(43, resource)
		require.NoError(t, err)
		assert.False(t, acquired)
		assert.Equal(t, "42", client.objects[resource][lockIDMetadataKey])
	})

	t.Run("surfaces non-conflict service errors", func(t *testing.T) {
		client := newFakeOSSClient()
		client.putErr = &oss.ServiceError{StatusCode: http.StatusForbidden, Code: "AccessDenied"}
		lock := newTestLock(client)

		acquired, err := lock.Lock(42, resource)
		require.Error(t, err)
		assert.False(t, acquired)
		assert.True(t, HasStatus(err, http.StatusForbidden))
	})
}

func TestOSSLock_GetLock(t *testing.T) {
	const resource = "org/repo#dev"

	tests := []struct {
		name     string
		objects  map[string]map[string]string
		headErr  error
		want     *int
		wantErr  bool
		errMatch string
	}{
		{
			name:    "no lock object returns nil",
			objects: map[string]map[string]string{},
			want:    nil,
		},
		{
			name:    "existing lock returns holder",
			objects: map[string]map[string]string{resource: {lockIDMetadataKey: "7"}},
			want:    oss.Ptr(7),
		},
		{
			name:     "missing metadata is an error",
			objects:  map[string]map[string]string{resource: {}},
			wantErr:  true,
			errMatch: "has no lockid metadata",
		},
		{
			name:     "malformed metadata is an error",
			objects:  map[string]map[string]string{resource: {lockIDMetadataKey: "abc"}},
			wantErr:  true,
			errMatch: "parse lockid metadata",
		},
		{
			name:     "missing bucket is an error, not an absent lock",
			objects:  map[string]map[string]string{},
			headErr:  &oss.ServiceError{StatusCode: http.StatusNotFound, Code: "NoSuchBucket"},
			wantErr:  true,
			errMatch: "NoSuchBucket",
		},
		{
			name:     "non-404 service error is propagated",
			objects:  map[string]map[string]string{},
			headErr:  &oss.ServiceError{StatusCode: http.StatusForbidden, Code: "AccessDenied"},
			wantErr:  true,
			errMatch: "head lock object",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newFakeOSSClient()
			client.objects = tt.objects
			client.headErr = tt.headErr
			lock := newTestLock(client)

			got, err := lock.GetLock(resource)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMatch)
				assert.Nil(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestOSSLock_Unlock(t *testing.T) {
	const resource = "org/repo#dev"

	t.Run("removes the lock object", func(t *testing.T) {
		client := newFakeOSSClient()
		lock := newTestLock(client)
		_, err := lock.Lock(42, resource)
		require.NoError(t, err)

		released, err := lock.Unlock(resource)
		require.NoError(t, err)
		assert.True(t, released)

		got, err := lock.GetLock(resource)
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("unlocking a free resource is idempotent", func(t *testing.T) {
		lock := newTestLock(newFakeOSSClient())

		released, err := lock.Unlock(resource)
		require.NoError(t, err)
		assert.True(t, released)
	})
}

func TestOSSLock_EnsureVersioningDisabled(t *testing.T) {
	tests := []struct {
		name          string
		versioning    *string
		versioningErr error
		wantErr       string
	}{
		{name: "never enabled is accepted"},
		{name: "enabled is rejected", versioning: oss.Ptr("Enabled"), wantErr: "versioning Enabled"},
		{name: "suspended is rejected", versioning: oss.Ptr("Suspended"), wantErr: "versioning Suspended"},
		{name: "check failure only warns", versioningErr: &oss.ServiceError{StatusCode: http.StatusForbidden, Code: "AccessDenied"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newFakeOSSClient()
			client.versioning = tt.versioning
			client.versioningErr = tt.versioningErr

			err := newTestLock(client).ensureVersioningDisabled()
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestCredentialsProvider_EnvironmentPrecedence(t *testing.T) {
	clear := func(t *testing.T) {
		for _, v := range []string{
			"OSS_ACCESS_KEY_ID", "OSS_ACCESS_KEY_SECRET", "OSS_SESSION_TOKEN",
			"ALIBABA_CLOUD_ACCESS_KEY_ID", "ALIBABA_CLOUD_ACCESS_KEY_SECRET", "ALIBABA_CLOUD_SECURITY_TOKEN",
			"ALICLOUD_ACCESS_KEY", "ALICLOUD_SECRET_KEY", "ALICLOUD_SECURITY_TOKEN",
		} {
			t.Setenv(v, "")
		}
	}
	resolve := func(t *testing.T) string {
		provider, err := credentialsProvider()
		require.NoError(t, err)
		creds, err := provider.GetCredentials(context.Background())
		require.NoError(t, err)
		return creds.AccessKeyID
	}

	t.Run("ALICLOUD_* is used when ALIBABA_CLOUD_ACCESS_KEY_ID is unset", func(t *testing.T) {
		clear(t)
		t.Setenv("ALICLOUD_ACCESS_KEY", "ak-alicloud")
		t.Setenv("ALICLOUD_SECRET_KEY", "sk-alicloud")
		t.Setenv("ALICLOUD_SECURITY_TOKEN", "token-alicloud")

		provider, err := credentialsProvider()
		require.NoError(t, err)
		creds, err := provider.GetCredentials(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "ak-alicloud", creds.AccessKeyID)
		assert.Equal(t, "sk-alicloud", creds.AccessKeySecret)
		assert.Equal(t, "token-alicloud", creds.SecurityToken)
	})

	t.Run("ALIBABA_CLOUD_* wins over ALICLOUD_*", func(t *testing.T) {
		clear(t)
		t.Setenv("ALIBABA_CLOUD_ACCESS_KEY_ID", "ak-alibaba")
		t.Setenv("ALIBABA_CLOUD_ACCESS_KEY_SECRET", "sk-alibaba")
		t.Setenv("ALICLOUD_ACCESS_KEY", "ak-alicloud")
		t.Setenv("ALICLOUD_SECRET_KEY", "sk-alicloud")
		assert.Equal(t, "ak-alibaba", resolve(t))
	})

	t.Run("OSS_* wins over everything", func(t *testing.T) {
		clear(t)
		t.Setenv("OSS_ACCESS_KEY_ID", "ak-oss")
		t.Setenv("OSS_ACCESS_KEY_SECRET", "sk-oss")
		t.Setenv("ALIBABA_CLOUD_ACCESS_KEY_ID", "ak-alibaba")
		t.Setenv("ALIBABA_CLOUD_ACCESS_KEY_SECRET", "sk-alibaba")
		assert.Equal(t, "ak-oss", resolve(t))
	})
}

func TestNewOSSLock_RequiresBucket(t *testing.T) {
	t.Setenv(LockBucketEnv, "")

	lock, err := NewOSSLock()
	require.Error(t, err)
	assert.Nil(t, lock)
	assert.Contains(t, err.Error(), LockBucketEnv)
}

func TestNewOSSClient_RequiresRegion(t *testing.T) {
	t.Setenv(RegionEnv, "")
	t.Setenv("ALICLOUD_REGION", "")
	t.Setenv("ALIBABA_CLOUD_REGION_ID", "")

	client, err := NewOSSClient()
	require.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), RegionEnv)
}
