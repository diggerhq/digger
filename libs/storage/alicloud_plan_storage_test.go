package storage

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type emulateOSSClient struct {
	objects map[string][]byte
	headErr error
}

func newEmulateOSSClient() *emulateOSSClient {
	return &emulateOSSClient{objects: make(map[string][]byte)}
}

func (m *emulateOSSClient) PutObject(_ context.Context, request *oss.PutObjectRequest, _ ...func(*oss.Options)) (*oss.PutObjectResult, error) {
	data, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, err
	}
	m.objects[oss.ToString(request.Key)] = data
	return &oss.PutObjectResult{}, nil
}

func (m *emulateOSSClient) HeadObject(_ context.Context, request *oss.HeadObjectRequest, _ ...func(*oss.Options)) (*oss.HeadObjectResult, error) {
	if m.headErr != nil {
		return nil, m.headErr
	}
	if _, ok := m.objects[oss.ToString(request.Key)]; !ok {
		return nil, &oss.ServiceError{StatusCode: http.StatusNotFound, Code: "NoSuchKey"}
	}
	return &oss.HeadObjectResult{}, nil
}

func (m *emulateOSSClient) GetObject(_ context.Context, request *oss.GetObjectRequest, _ ...func(*oss.Options)) (*oss.GetObjectResult, error) {
	data, ok := m.objects[oss.ToString(request.Key)]
	if !ok {
		return nil, &oss.ServiceError{StatusCode: http.StatusNotFound, Code: "NoSuchKey"}
	}
	return &oss.GetObjectResult{Body: io.NopCloser(bytes.NewReader(data))}, nil
}

func (m *emulateOSSClient) DeleteObject(_ context.Context, request *oss.DeleteObjectRequest, _ ...func(*oss.Options)) (*oss.DeleteObjectResult, error) {
	delete(m.objects, oss.ToString(request.Key))
	return &oss.DeleteObjectResult{}, nil
}

func newTestAlicloudStorage(client *emulateOSSClient) *PlanStorageAlicloud {
	return &PlanStorageAlicloud{Client: client, Bucket: "digger-plans", Context: context.Background()}
}

func TestPlanStorageAlicloud_RoundTrip(t *testing.T) {
	const key = "123-dev.tfplan"
	contents := []byte("terraform plan output")
	client := newEmulateOSSClient()
	storage := newTestAlicloudStorage(client)

	exists, err := storage.PlanExists("artifact", key)
	require.NoError(t, err)
	assert.False(t, exists)

	require.NoError(t, storage.StorePlanFile(contents, "artifact", key))
	assert.Equal(t, contents, client.objects[key])

	exists, err = storage.PlanExists("artifact", key)
	require.NoError(t, err)
	assert.True(t, exists)

	localPath := filepath.Join(t.TempDir(), "plan.tfplan")
	retrieved, err := storage.RetrievePlan(localPath, "artifact", key)
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.True(t, filepath.IsAbs(*retrieved))
	got, err := os.ReadFile(*retrieved)
	require.NoError(t, err)
	assert.Equal(t, contents, got)

	require.NoError(t, storage.DeleteStoredPlan("artifact", key))
	exists, err = storage.PlanExists("artifact", key)
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestPlanStorageAlicloud_PlanExists_PropagatesNon404(t *testing.T) {
	client := newEmulateOSSClient()
	client.headErr = &oss.ServiceError{StatusCode: http.StatusForbidden, Code: "AccessDenied"}
	storage := newTestAlicloudStorage(client)

	exists, err := storage.PlanExists("artifact", "missing.tfplan")
	require.Error(t, err)
	assert.False(t, exists)
	assert.Contains(t, err.Error(), "AccessDenied")
}

func TestPlanStorageAlicloud_RetrievePlan_MissingObject(t *testing.T) {
	storage := newTestAlicloudStorage(newEmulateOSSClient())

	retrieved, err := storage.RetrievePlan(filepath.Join(t.TempDir(), "plan.tfplan"), "artifact", "missing.tfplan")
	require.Error(t, err)
	assert.Nil(t, retrieved)
}
