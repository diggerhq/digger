package ci_backends

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/diggerhq/digger/backend/config"
	"github.com/diggerhq/digger/libs/spec"
	"github.com/google/go-github/v61/github"
)

// Helper to create a github.Client that talks to an httptest server
func newTestGithubClient(ts *httptest.Server) *github.Client {
	client := github.NewClient(ts.Client())
	base, _ := url.Parse(ts.URL + "/")
	client.BaseURL = base
	client.UploadURL, _ = url.Parse(ts.URL + "/")
	return client
}

// setupTestClientAndSpec centralizes common test setup: set the feature flag,
// create a client and GithubActionCi configured to point to the test server,
// and build a basic Spec with the given branch. Tests should use this helper
// to keep setups consistent and small.
func setupTestClientAndSpec(ts *httptest.Server, forceDefault bool, branch string) (GithubActionCi, spec.Spec) {
	config.DiggerConfig.Set("force_trigger_from_default_branch", forceDefault)
	client := newTestGithubClient(ts)
	ga := GithubActionCi{Client: client}

	s := spec.Spec{}
	s.VCS.RepoOwner = "owner"
	s.VCS.RepoName = "repo"
	s.VCS.WorkflowFile = "workflow.yml"
	s.Job.Branch = branch

	return ga, s
}

func TestTriggerWorkflow_UsesJobBranchWhenNotForced(t *testing.T) {
	// server returns error if repo default branch is requested (shouldn't be called)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			// Expect the ref to be the job branch
			bodyBytes, _ := io.ReadAll(r.Body)
			defer r.Body.Close()
			var payload map[string]interface{}
			_ = json.Unmarshal(bodyBytes, &payload)
			if payload["ref"] != "feature/abc" {
				t.Fatalf("expected ref 'feature/abc', got %v", payload["ref"])
			}
			w.WriteHeader(http.StatusCreated)
			return
		}
		t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
	}))
	defer ts.Close()

	// Ensure flag is false
	config.DiggerConfig.Set("force_trigger_from_default_branch", false)

	ga, s := setupTestClientAndSpec(ts, false, "feature/abc")

	if err := ga.TriggerWorkflow(s, "run", "token"); err != nil {
		t.Fatalf("TriggerWorkflow failed: %v", err)
	}
}

func TestTriggerWorkflow_UsesRepoDefaultBranchWhenForced(t *testing.T) {
	// Server returns repo info and accept the dispatch
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// repos/{owner}/{repo}
			resp := map[string]string{"default_branch": "main"}
			_ = json.NewEncoder(w).Encode(resp)
			return
		case http.MethodPost:
			// Check dispatched ref == main
			bodyBytes, _ := io.ReadAll(r.Body)
			defer r.Body.Close()
			var payload map[string]interface{}
			_ = json.Unmarshal(bodyBytes, &payload)
			if payload["ref"] != "main" {
				t.Fatalf("expected ref 'main' when forced, got %v", payload["ref"])
			}

			// Accept the dispatch — assertion for the spec contents is handled
			// in a separate dedicated test below.
			w.WriteHeader(http.StatusCreated)
			return
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer ts.Close()

	// Enable the flag and prepare client + spec
	ga, s := setupTestClientAndSpec(ts, true, "feature/abc")

	if err := ga.TriggerWorkflow(s, "run", "token"); err != nil {
		t.Fatalf("TriggerWorkflow failed: %v", err)
	}
}

func TestTriggerWorkflow_SpecStillContainsJobBranchWhenForced(t *testing.T) {
	// Server returns repo info and accept the dispatch
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// repos/{owner}/{repo}
			resp := map[string]string{"default_branch": "main"}
			_ = json.NewEncoder(w).Encode(resp)
			return
		case http.MethodPost:
			// Check inputs.spec still contains the original PR branch
			bodyBytes, _ := io.ReadAll(r.Body)
			defer r.Body.Close()
			var payload map[string]interface{}
			_ = json.Unmarshal(bodyBytes, &payload)

			inputs, ok := payload["inputs"].(map[string]interface{})
			if !ok {
				t.Fatalf("expected inputs to be map, got %T", payload["inputs"])
			}
			specStr, ok := inputs["spec"].(string)
			if !ok {
				t.Fatalf("expected inputs.spec to be string, got %T", inputs["spec"])
			}

			var decoded spec.Spec
			if err := json.Unmarshal([]byte(specStr), &decoded); err != nil {
				t.Fatalf("failed to unmarshal spec from inputs: %v", err)
			}
			if decoded.Job.Branch != "feature/abc" {
				t.Fatalf("expected spec.job.branch to still be feature/abc, got %v", decoded.Job.Branch)
			}

			w.WriteHeader(http.StatusCreated)
			return
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer ts.Close()

	// Enable flag and create test client/spec
	ga, s := setupTestClientAndSpec(ts, true, "feature/abc")

	if err := ga.TriggerWorkflow(s, "run", "token"); err != nil {
		t.Fatalf("TriggerWorkflow failed: %v", err)
	}
}

func TestResolveWorkflowRef_NotForcedReturnsJobBranch(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("no requests expected when flag is disabled, got %s %s", r.Method, r.URL.Path)
	}))
	defer ts.Close()

	config.DiggerConfig.Set("force_trigger_from_default_branch", false)

	client := newTestGithubClient(ts)
	ga := GithubActionCi{Client: client}

	s := spec.Spec{}
	s.Job.Branch = "feature/xyz"

	ref, err := ga.resolveWorkflowRef(context.Background(), s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref != "feature/xyz" {
		t.Fatalf("expected feature/xyz branch, got %v", ref)
	}
}

func TestResolveWorkflowRef_ForcedWithNoDefaultBranchFallsBackToMain(t *testing.T) {
	// server returns repo info without default_branch
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			// repos/{owner}/{repo} -> respond with empty object
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{}`)
			return
		}
		t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
	}))
	defer ts.Close()

	config.DiggerConfig.Set("force_trigger_from_default_branch", true)

	client := newTestGithubClient(ts)
	ga := GithubActionCi{Client: client}

	s := spec.Spec{}
	s.VCS.RepoOwner = "owner"
	s.VCS.RepoName = "repo"
	s.Job.Branch = "feature/xyz"

	ref, err := ga.resolveWorkflowRef(context.Background(), s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref != "main" {
		t.Fatalf("expected fallback main, got %v", ref)
	}
}
