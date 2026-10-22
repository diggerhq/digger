package storage

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeJWT builds a structurally-valid JWT whose `scp` claim encodes the given
// Actions.Results scope. Production code only base64-decodes the payload and
// reads the claim — no signature verification — so the header and signature
// segments are fixed dummies.
func makeJWT(t *testing.T, runID, jobRunID string) string {
	t.Helper()
	return makeJWTWithScope(t, "Actions.Results:"+runID+":"+jobRunID)
}

func makeJWTWithScope(t *testing.T, scp string) string {
	t.Helper()
	payload, err := json.Marshal(map[string]string{"scp": scp})
	require.NoError(t, err)
	return makeJWTWithRawPayload(t, payload)
}

func makeJWTWithRawPayload(t *testing.T, payload []byte) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	body := base64.RawURLEncoding.EncodeToString(payload)
	signature := base64.RawURLEncoding.EncodeToString([]byte("sig"))
	return header + "." + body + "." + signature
}

func TestExtractArtifactBackendIDs_HappyPath(t *testing.T) {
	token := makeJWT(t, "abc123", "def456")

	runID, jobRunID, err := extractArtifactBackendIDs(token)

	require.NoError(t, err)
	assert.Equal(t, "abc123", runID)
	assert.Equal(t, "def456", jobRunID)
}

func TestExtractArtifactBackendIDs_PicksActionsResultsAmongOtherScopes(t *testing.T) {
	token := makeJWTWithScope(t, "Foo:1:2 Actions.Results:run-9:job-9 Bar:x:y")

	runID, jobRunID, err := extractArtifactBackendIDs(token)

	require.NoError(t, err)
	assert.Equal(t, "run-9", runID)
	assert.Equal(t, "job-9", jobRunID)
}

func TestExtractArtifactBackendIDs_Errors(t *testing.T) {
	cases := []struct {
		name           string
		token          string
		errorSubstring string
	}{
		{
			name:           "empty token",
			token:          "",
			errorSubstring: "3 JWT parts",
		},
		{
			name:           "single segment",
			token:          "onlyone",
			errorSubstring: "3 JWT parts",
		},
		{
			name:           "two segments",
			token:          "header.payload",
			errorSubstring: "3 JWT parts",
		},
		{
			name:           "four segments",
			token:          "a.b.c.d",
			errorSubstring: "3 JWT parts",
		},
		{
			name:           "invalid base64 payload",
			token:          "header.!!!notbase64!!!.sig",
			errorSubstring: "failed to base64-decode",
		},
		{
			name:           "payload not json",
			token:          makeJWTWithRawPayload(t, []byte("not-json")),
			errorSubstring: "failed to parse JWT claims",
		},
		{
			name:           "scp without Actions.Results scope",
			token:          makeJWTWithScope(t, "OtherScope:1:2 SomethingElse:3:4"),
			errorSubstring: "no Actions.Results scope",
		},
		{
			name:           "empty scp claim",
			token:          makeJWTWithScope(t, ""),
			errorSubstring: "no Actions.Results scope",
		},
		{
			name:           "scp claim absent from payload",
			token:          makeJWTWithRawPayload(t, []byte(`{"other":"value"}`)),
			errorSubstring: "no Actions.Results scope",
		},
		{
			name:           "Actions.Results scope without jobRunID",
			token:          makeJWTWithScope(t, "Actions.Results:onlyone"),
			errorSubstring: "no Actions.Results scope",
		},
		{
			name:           "Actions.Results with empty runID",
			token:          makeJWTWithScope(t, "Actions.Results::jobid"),
			errorSubstring: "no Actions.Results scope",
		},
		{
			name:           "Actions.Results with empty jobRunID",
			token:          makeJWTWithScope(t, "Actions.Results:runid:"),
			errorSubstring: "no Actions.Results scope",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runID, jobRunID, err := extractArtifactBackendIDs(tc.token)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errorSubstring)
			assert.Empty(t, runID)
			assert.Empty(t, jobRunID)
		})
	}
}

type recordedRequest struct {
	Method  string
	Path    string
	Headers http.Header
	Body    []byte
}

type capturedRequests struct {
	mu       sync.Mutex
	Create   []recordedRequest
	Upload   []recordedRequest
	Finalize []recordedRequest
}

func (c *capturedRequests) record(slot *[]recordedRequest, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	c.mu.Lock()
	defer c.mu.Unlock()
	*slot = append(*slot, recordedRequest{
		Method:  r.Method,
		Path:    r.URL.Path,
		Headers: r.Header.Clone(),
		Body:    body,
	})
}

type artifactServerOpts struct {
	CreateStatus   int
	CreateBody     string // if "", a valid CreateArtifact JSON pointing signed_upload_url at <server>/upload is used
	UploadStatus   int
	FinalizeStatus int
	FinalizeBody   string
}

const (
	createPath   = "/twirp/github.actions.results.api.v1.ArtifactService/CreateArtifact"
	finalizePath = "/twirp/github.actions.results.api.v1.ArtifactService/FinalizeArtifact"
	uploadPath   = "/upload"
)

func setupArtifactServer(t *testing.T, opts artifactServerOpts) (*httptest.Server, *capturedRequests) {
	t.Helper()
	captured := &capturedRequests{}

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == createPath:
			captured.record(&captured.Create, r)
			status := opts.CreateStatus
			if status == 0 {
				status = http.StatusOK
			}
			w.WriteHeader(status)
			body := opts.CreateBody
			if body == "" {
				body = `{"signed_upload_url":"` + server.URL + uploadPath + `"}`
			}
			_, _ = w.Write([]byte(body))
		case r.URL.Path == uploadPath:
			captured.record(&captured.Upload, r)
			status := opts.UploadStatus
			if status == 0 {
				status = http.StatusOK
			}
			w.WriteHeader(status)
		case r.URL.Path == finalizePath:
			captured.record(&captured.Finalize, r)
			status := opts.FinalizeStatus
			if status == 0 {
				status = http.StatusOK
			}
			w.WriteHeader(status)
			body := opts.FinalizeBody
			if body == "" {
				body = "{}"
			}
			_, _ = w.Write([]byte(body))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server, captured
}

func TestStorePlanFile_HappyPath(t *testing.T) {
	server, captured := setupArtifactServer(t, artifactServerOpts{})
	token := makeJWT(t, "run-42", "job-99")
	t.Setenv("ACTIONS_RESULTS_URL", server.URL)
	t.Setenv("ACTIONS_RUNTIME_TOKEN", token)

	gps := &GithubPlanStorage{}
	contents := []byte("plan-file-bytes")

	err := gps.StorePlanFile(contents, "my-artifact", "plan.tfplan")
	require.NoError(t, err)

	require.Len(t, captured.Create, 1, "CreateArtifact should be called exactly once")
	require.Len(t, captured.Upload, 1, "Upload PUT should be called exactly once")
	require.Len(t, captured.Finalize, 1, "FinalizeArtifact should be called exactly once")

	createReq := captured.Create[0]
	assert.Equal(t, http.MethodPost, createReq.Method)
	assert.Equal(t, "Bearer "+token, createReq.Headers.Get("Authorization"))
	assert.Equal(t, "application/json", createReq.Headers.Get("Content-Type"))
	var createBody map[string]any
	require.NoError(t, json.Unmarshal(createReq.Body, &createBody))
	assert.Equal(t, "run-42", createBody["workflow_run_backend_id"])
	assert.Equal(t, "job-99", createBody["workflow_job_run_backend_id"])
	assert.Equal(t, "my-artifact", createBody["name"])
	assert.EqualValues(t, 4, createBody["version"])

	uploadReq := captured.Upload[0]
	assert.Equal(t, http.MethodPut, uploadReq.Method)
	assert.Equal(t, "BlockBlob", uploadReq.Headers.Get("x-ms-blob-type"))
	assert.Equal(t, "application/octet-stream", uploadReq.Headers.Get("x-ms-blob-content-type"))
	assert.Equal(t, "15", uploadReq.Headers.Get("Content-Length"))
	assert.Equal(t, contents, uploadReq.Body)

	finalizeReq := captured.Finalize[0]
	assert.Equal(t, http.MethodPost, finalizeReq.Method)
	assert.Equal(t, "Bearer "+token, finalizeReq.Headers.Get("Authorization"))
	var finalizeBody map[string]any
	require.NoError(t, json.Unmarshal(finalizeReq.Body, &finalizeBody))
	assert.Equal(t, "run-42", finalizeBody["workflow_run_backend_id"])
	assert.Equal(t, "job-99", finalizeBody["workflow_job_run_backend_id"])
	assert.Equal(t, "my-artifact", finalizeBody["name"])
	assert.Equal(t, "15", finalizeBody["size"])
}

func TestStorePlanFile_MissingResultsURL(t *testing.T) {
	t.Setenv("ACTIONS_RESULTS_URL", "")
	t.Setenv("ACTIONS_RUNTIME_TOKEN", makeJWT(t, "r", "j"))

	gps := &GithubPlanStorage{}
	err := gps.StorePlanFile([]byte("data"), "name", "path")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ACTIONS_RESULTS_URL is not set")
}

func TestStorePlanFile_MissingRuntimeToken(t *testing.T) {
	t.Setenv("ACTIONS_RESULTS_URL", "http://example.invalid")
	t.Setenv("ACTIONS_RUNTIME_TOKEN", "")

	gps := &GithubPlanStorage{}
	err := gps.StorePlanFile([]byte("data"), "name", "path")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ACTIONS_RUNTIME_TOKEN is not set")
}

func TestStorePlanFile_MalformedToken(t *testing.T) {
	server, captured := setupArtifactServer(t, artifactServerOpts{})
	t.Setenv("ACTIONS_RESULTS_URL", server.URL)
	t.Setenv("ACTIONS_RUNTIME_TOKEN", "not-a-jwt")

	gps := &GithubPlanStorage{}
	err := gps.StorePlanFile([]byte("data"), "name", "path")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "could not extract artifact backend IDs")
	assert.Empty(t, captured.Create, "no HTTP calls should be made when token is malformed")
	assert.Empty(t, captured.Upload)
	assert.Empty(t, captured.Finalize)
}

func TestStorePlanFile_ServerErrors(t *testing.T) {
	cases := []struct {
		name           string
		opts           artifactServerOpts
		errorSubstring string
		wantUpload     bool
		wantFinalize   bool
	}{
		{
			name:           "CreateArtifact returns 500",
			opts:           artifactServerOpts{CreateStatus: http.StatusInternalServerError},
			errorSubstring: "could not create artifact",
		},
		{
			name:           "CreateArtifact returns 400",
			opts:           artifactServerOpts{CreateStatus: http.StatusBadRequest},
			errorSubstring: "could not create artifact",
		},
		{
			name:           "CreateArtifact returns non-JSON body",
			opts:           artifactServerOpts{CreateBody: "not-json"},
			errorSubstring: "failed to parse CreateArtifact response",
		},
		{
			name:           "CreateArtifact response missing signed_upload_url",
			opts:           artifactServerOpts{CreateBody: `{"foo":"bar"}`},
			errorSubstring: "missing signed_upload_url",
		},
		{
			name:           "CreateArtifact response has empty signed_upload_url",
			opts:           artifactServerOpts{CreateBody: `{"signed_upload_url":""}`},
			errorSubstring: "missing signed_upload_url",
		},
		{
			name:           "CreateArtifact response has wrong-typed signed_upload_url",
			opts:           artifactServerOpts{CreateBody: `{"signed_upload_url":42}`},
			errorSubstring: "missing signed_upload_url",
		},
		{
			name:           "Upload PUT returns 403",
			opts:           artifactServerOpts{UploadStatus: http.StatusForbidden},
			errorSubstring: "could not upload artifact file",
			wantUpload:     true,
		},
		{
			name:           "FinalizeArtifact returns 500",
			opts:           artifactServerOpts{FinalizeStatus: http.StatusInternalServerError},
			errorSubstring: "could not finalize artifact upload",
			wantUpload:     true,
			wantFinalize:   true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server, captured := setupArtifactServer(t, tc.opts)
			t.Setenv("ACTIONS_RESULTS_URL", server.URL)
			t.Setenv("ACTIONS_RUNTIME_TOKEN", makeJWT(t, "r", "j"))

			gps := &GithubPlanStorage{}
			err := gps.StorePlanFile([]byte("payload"), "art", "path")

			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errorSubstring)
			assert.Len(t, captured.Create, 1, "CreateArtifact must always be attempted")
			if tc.wantUpload {
				assert.Len(t, captured.Upload, 1, "Upload should have been called")
			} else {
				assert.Empty(t, captured.Upload, "Upload must not be called when CreateArtifact already failed")
			}
			if tc.wantFinalize {
				assert.Len(t, captured.Finalize, 1, "Finalize should have been called")
			} else {
				assert.Empty(t, captured.Finalize, "Finalize must not be called when an earlier step failed")
			}
		})
	}
}

func TestStorePlanFile_TrailingSlashInResultsURL(t *testing.T) {
	server, captured := setupArtifactServer(t, artifactServerOpts{})
	t.Setenv("ACTIONS_RESULTS_URL", server.URL+"/")
	t.Setenv("ACTIONS_RUNTIME_TOKEN", makeJWT(t, "r", "j"))

	gps := &GithubPlanStorage{}
	err := gps.StorePlanFile([]byte("data"), "name", "path")
	require.NoError(t, err)

	require.Len(t, captured.Create, 1)
	assert.Equal(t, createPath, captured.Create[0].Path,
		"trailing slash in ACTIONS_RESULTS_URL must not produce a doubled slash in the request path")
	require.Len(t, captured.Finalize, 1)
	assert.Equal(t, finalizePath, captured.Finalize[0].Path)
}

func TestStorePlanFile_EmptyFileContents(t *testing.T) {
	server, captured := setupArtifactServer(t, artifactServerOpts{})
	t.Setenv("ACTIONS_RESULTS_URL", server.URL)
	t.Setenv("ACTIONS_RUNTIME_TOKEN", makeJWT(t, "r", "j"))

	gps := &GithubPlanStorage{}
	err := gps.StorePlanFile([]byte{}, "name", "path")
	require.NoError(t, err)

	require.Len(t, captured.Upload, 1)
	assert.Equal(t, "0", captured.Upload[0].Headers.Get("Content-Length"))
	assert.Empty(t, captured.Upload[0].Body)

	require.Len(t, captured.Finalize, 1)
	var finalizeBody map[string]any
	require.NoError(t, json.Unmarshal(captured.Finalize[0].Body, &finalizeBody))
	assert.Equal(t, "0", finalizeBody["size"])
}
