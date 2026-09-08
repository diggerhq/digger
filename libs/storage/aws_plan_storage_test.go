package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
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

type mockS3PresignClient struct {
	MockPresignGetObject func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
}

func (m *mockS3PresignClient) PresignGetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error) {
	return m.MockPresignGetObject(ctx, params, optFns...)
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

func TestStoredPlanUrl(t *testing.T) {
	const (
		bucket = "my-plans"
		key    = "nested/acme infra+network.tfplan.txt"
		url    = "https://my-plans.s3.eu-central-1.amazonaws.com/nested/acme%20infra%2Bnetwork.tfplan.txt?X-Amz-Expires=3600&X-Amz-Signature=signature"
	)

	client := &mockS3Client{
		MockHeadObject: func(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
			require.Equal(t, bucket, *params.Bucket)
			require.Equal(t, key, *params.Key)
			return &s3.HeadObjectOutput{}, nil
		},
	}
	presigner := &mockS3PresignClient{
		MockPresignGetObject: func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error) {
			require.Equal(t, bucket, *params.Bucket)
			require.Equal(t, key, *params.Key, "the SDK must receive the unencoded object key")
			require.Equal(t, "inline", *params.ResponseContentDisposition)
			require.Equal(t, "text/plain", *params.ResponseContentType)

			options := s3.PresignOptions{}
			for _, optFn := range optFns {
				optFn(&options)
			}
			require.Equal(t, time.Hour, options.Expires)
			return &v4.PresignedHTTPRequest{URL: url}, nil
		},
	}
	psa := &PlanStorageAWS{
		Client:    client,
		Presigner: presigner,
		Bucket:    bucket,
		Context:   context.Background(),
	}

	got, err := psa.StoredPlanUrl(key, time.Hour)
	require.NoError(t, err)
	require.Equal(t, url, got)
	require.NotContains(t, got, "console.aws.amazon.com")
	require.NotContains(t, got, "s3://")
}

func TestStoredPlanUrlErrors(t *testing.T) {
	tests := []struct {
		name             string
		headError        error
		presignError     error
		missingPresigner bool
		wantError        string
		wantPresignCalls int
	}{
		{
			name:      "readable object is missing",
			headError: &types.NotFound{},
			wantError: "readable plan does not exist",
		},
		{
			name:      "object check fails",
			headError: errors.New("head object failed"),
			wantError: "unable to verify readable plan",
		},
		{
			name:             "signing fails",
			presignError:     errors.New("signing failed"),
			wantError:        "unable to presign readable plan",
			wantPresignCalls: 1,
		},
		{
			name:             "presigner is missing",
			missingPresigner: true,
			wantError:        "S3 presigner is not configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &mockS3Client{
				MockHeadObject: func(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
					if tt.headError != nil {
						return nil, tt.headError
					}
					return &s3.HeadObjectOutput{}, nil
				},
			}

			presignCalls := 0
			var presigner S3PresignClient
			if !tt.missingPresigner {
				presigner = &mockS3PresignClient{
					MockPresignGetObject: func(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error) {
						presignCalls++
						if tt.presignError != nil {
							return nil, tt.presignError
						}
						return &v4.PresignedHTTPRequest{URL: "https://example.com/plan"}, nil
					},
				}
			}

			var s3Client S3Client = client
			if tt.missingPresigner {
				s3Client = nil
			}

			psa := &PlanStorageAWS{
				Client:    s3Client,
				Presigner: presigner,
				Bucket:    "my-plans",
				Context:   context.Background(),
			}

			got, err := psa.StoredPlanUrl("acme-infra-42-vpc.tfplan.txt", time.Hour)
			require.ErrorContains(t, err, tt.wantError)
			require.Empty(t, got)
			require.Equal(t, tt.wantPresignCalls, presignCalls)
		})
	}
}

var _ PlanUrlProvider = (*PlanStorageAWS)(nil)
