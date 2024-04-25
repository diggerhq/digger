package storage

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/stretchr/testify/require"
	"gotest.tools/v3/assert"
)

type mockS3Client struct {
	MockHeadObject   func(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	MockPutObject    func(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	MockGetObject    func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	MockDeleteObject func(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

func (m *mockS3Client) HeadObject(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	return m.MockHeadObject(ctx, params, optFns...)
}

func (m *mockS3Client) PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	return m.MockPutObject(ctx, params, optFns...)
}

func (m *mockS3Client) GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	return m.MockGetObject(ctx, params, optFns...)
}

func (m *mockS3Client) DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	return m.MockDeleteObject(ctx, params, optFns...)
}

type emulateS3Client struct {
	objects map[string][]byte
}

func (m *emulateS3Client) HeadObject(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	if _, ok := m.objects[*params.Key]; ok {
		return &s3.HeadObjectOutput{}, nil
	}
	return nil, &types.NotFound{}
}
func (m *emulateS3Client) PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	buf, err := io.ReadAll(params.Body)
	if err != nil {
		return nil, err
	}

	m.objects[*params.Key] = buf
	return &s3.PutObjectOutput{}, nil
}

func (m *emulateS3Client) GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	if buf, ok := m.objects[*params.Key]; ok {
		return &s3.GetObjectOutput{
			Body: io.NopCloser(bytes.NewReader(buf)),
		}, nil
	}
	return nil, &types.NotFound{}
}
func (m *emulateS3Client) DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	delete(m.objects, *params.Key)
	return &s3.DeleteObjectOutput{}, nil
}

func TestPlanStorageAWS_PlanExists(t *testing.T) {
	client := &mockS3Client{
		MockHeadObject: func(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
			return nil, &types.NotFound{}
		},
	}
	client.MockHeadObject = func(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
		return nil, &types.NotFound{}
	}
	psa := &PlanStorageAWS{
		Client: client,
		Bucket: "test-bucket",
	}
	exists, err := psa.PlanExists("not in use", "plan.txt")
	if err != nil {
		require.NoError(t, err)
	}
	assert.Equal(t, false, exists)
}

func TestPlanStorageAWS_E2E(t *testing.T) {

	client := &emulateS3Client{
		objects: make(map[string][]byte),
	}
	// Create a PlanStorageAWS instance with the mock S3 client
	psa := &PlanStorageAWS{
		Client: client,
		Bucket: "test-bucket",
	}

	planFilename := "plan.txt"

	exists, err := psa.PlanExists("not in use", planFilename)
	require.NoError(t, err)
	assert.Equal(t, false, exists)

	data := []byte("test")
	err = psa.StorePlanFile(data, "test", planFilename)
	if err != nil {
		require.NoError(t, err)
	}

	exists, err = psa.PlanExists("not in use", planFilename)
	if err != nil {
		require.NoError(t, err)
	}
	assert.Equal(t, true, exists)

	// Use memfs to create a new directory

	tmpDir := t.TempDir()
	if err != nil {
		require.NoError(t, err)
	}

	newFile, err := psa.RetrievePlan(filepath.Join(tmpDir, planFilename), "not in use", planFilename)
	if err != nil {
		require.NoError(t, err)
	}
	outData, err := os.ReadFile(*newFile)
	if err != nil {
		require.NoError(t, err)
	}
	if string(data) != string(outData) {
		t.Errorf("expected %s, got %s", string(data), string(outData))
	}

	err = psa.DeleteStoredPlan("not in use", planFilename)
	if err != nil {
		require.NoError(t, err)
	}

	exists, err = psa.PlanExists("not in use", planFilename)
	require.NoError(t, err)
	assert.Equal(t, false, exists)
}
