package forgejo

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestService(t *testing.T, handler http.HandlerFunc) ForgejoService {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "token test-token", r.Header.Get("Authorization"))
		if r.URL.Path == "/api/v1/version" {
			fmt.Fprint(w, `{"version":"16.0.0"}`)
			return
		}
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	svc, err := ForgejoServiceProviderBasic{}.NewService(server.URL, "test-token", "repo", "org")
	require.NoError(t, err)
	return svc
}

func decodeBody(t *testing.T, r *http.Request) map[string]interface{} {
	t.Helper()
	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
	return body
}

func TestGetChangedFilesPaginates(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/repos/org/repo/pulls/7/files", r.URL.Path)
		switch r.URL.Query().Get("page") {
		case "1":
			w.Header().Set("Link", `<`+r.URL.Path+`?page=2>; rel="next"`)
			fmt.Fprint(w, `[{"filename":"dev/main.tf"},{"filename":"dev/new.tf","previous_filename":"dev/old.tf"}]`)
		case "2":
			fmt.Fprint(w, `[{"filename":"prod/main.tf"}]`)
		default:
			t.Errorf("unexpected page %q", r.URL.Query().Get("page"))
		}
	})
	files, err := svc.GetChangedFiles(7)
	require.NoError(t, err)
	assert.Equal(t, []string{"dev/main.tf", "dev/new.tf", "dev/old.tf", "prod/main.tf"}, files)
}

func TestGetApprovalsUsesLatestReviewPerUser(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/repos/org/repo/pulls/7/reviews", r.URL.Path)
		fmt.Fprint(w, `[
			{"user":{"login":"alice"},"state":"APPROVED"},
			{"user":{"login":"alice"},"state":"REQUEST_CHANGES"},
			{"user":{"login":"bob"},"state":"APPROVED"},
			{"user":{"login":"bob"},"state":"COMMENT"},
			{"user":{"login":"bob"},"state":"PENDING"},
			{"user":{"login":"carol"},"state":"APPROVED","dismissed":true}
		]`)
	})
	approvals, err := svc.GetApprovals(7)
	require.NoError(t, err)
	assert.Equal(t, []string{"bob"}, approvals)
}

func TestSetStatusTargetsHeadSha(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/repos/org/repo/pulls/7":
			fmt.Fprint(w, `{"number":7,"head":{"ref":"feature","sha":"abc123"},"base":{"ref":"main","sha":"def456"}}`)
		case "/api/v1/repos/org/repo/statuses/abc123":
			assert.Equal(t, http.MethodPost, r.Method)
			body := decodeBody(t, r)
			assert.Equal(t, "failure", body["state"])
			assert.Equal(t, "digger/plan", body["context"])
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"id":1}`)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
		}
	})
	require.NoError(t, svc.SetStatus(7, "failure", "digger/plan"))
}

func TestPullRequestState(t *testing.T) {
	tests := []struct {
		name      string
		pr        string
		mergeable bool
		merged    bool
		closed    bool
	}{
		{"open and mergeable", `{"state":"open","mergeable":true,"base":{"ref":"main"},"head":{"sha":"abc123"}}`, true, false, false},
		{"open with conflicts", `{"state":"open","mergeable":false,"base":{"ref":"main"},"head":{"sha":"abc123"}}`, false, false, false},
		{"draft", `{"state":"open","mergeable":true,"draft":true,"base":{"ref":"main"},"head":{"sha":"abc123"}}`, false, false, false},
		{"merged", `{"state":"closed","mergeable":true,"merged":true,"base":{"ref":"main"},"head":{"sha":"abc123"}}`, false, true, true},
		{"closed without merge", `{"state":"closed","mergeable":true,"base":{"ref":"main"},"head":{"sha":"abc123"}}`, false, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/v1/repos/org/repo/pulls/7":
					fmt.Fprint(w, tt.pr)
				case "/api/v1/repos/org/repo/branches/main":
					fmt.Fprint(w, `{"name":"main","protected":false}`)
				default:
					t.Errorf("unexpected request %s", r.URL.Path)
				}
			})
			mergeable, err := svc.IsMergeable(7)
			require.NoError(t, err)
			merged, err := svc.IsMerged(7)
			require.NoError(t, err)
			closed, err := svc.IsClosed(7)
			require.NoError(t, err)
			assert.Equal(t, tt.mergeable, mergeable)
			assert.Equal(t, tt.merged, merged)
			assert.Equal(t, tt.closed, closed)
		})
	}
}

func TestIsMergeableHonoursBranchProtection(t *testing.T) {
	tests := []struct {
		name      string
		branch    string
		reviews   string
		statuses  string
		mergeable bool
	}{
		{"approval missing", `{"protected":true,"required_approvals":1}`, `[]`, `[]`, false},
		{"official approval present", `{"protected":true,"required_approvals":1}`, `[{"user":{"login":"alice"},"state":"APPROVED","official":true}]`, `[]`, true},
		{"only unofficial approval", `{"protected":true,"required_approvals":1}`, `[{"user":{"login":"alice"},"state":"APPROVED"}]`, `[]`, false},
		{"required status failing", `{"protected":true,"enable_status_check":true,"status_check_contexts":["ci/test"]}`, `[]`, `[{"context":"ci/test","status":"failure"}]`, false},
		{"required status missing", `{"protected":true,"enable_status_check":true,"status_check_contexts":["ci/test"]}`, `[]`, `[]`, false},
		{"required status passing", `{"protected":true,"enable_status_check":true,"status_check_contexts":["ci/test"]}`, `[]`, `[{"context":"ci/test","status":"success"}]`, true},
		{"glob pattern passing", `{"protected":true,"enable_status_check":true,"status_check_contexts":["ci/*"]}`, `[]`, `[{"context":"ci/test","status":"success"},{"context":"other","status":"failure"}]`, true},
		{"glob covering digger apply", `{"protected":true,"enable_status_check":true,"status_check_contexts":["digger/*"]}`, `[]`, `[{"context":"digger/plan","status":"success"}]`, true},
		{"only digger apply pending", `{"protected":true,"enable_status_check":true,"status_check_contexts":["digger/apply"]}`, `[]`, `[]`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/v1/repos/org/repo/pulls/7":
					fmt.Fprint(w, `{"state":"open","mergeable":true,"base":{"ref":"main"},"head":{"ref":"feature","sha":"abc123"}}`)
				case "/api/v1/repos/org/repo/branches/main":
					fmt.Fprint(w, tt.branch)
				case "/api/v1/repos/org/repo/pulls/7/reviews":
					fmt.Fprint(w, tt.reviews)
				case "/api/v1/repos/org/repo/commits/abc123/status":
					fmt.Fprintf(w, `{"state":"pending","sha":"abc123","statuses":%s}`, tt.statuses)
				default:
					t.Errorf("unexpected request %s", r.URL.Path)
				}
			})
			mergeable, err := svc.IsMergeable(7)
			require.NoError(t, err)
			assert.Equal(t, tt.mergeable, mergeable)
		})
	}
}

func TestGetBranchName(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/repos/org/repo/pulls/7", r.URL.Path)
		fmt.Fprint(w, `{"number":7,"head":{"ref":"feature","sha":"abc123"},"base":{"ref":"main","sha":"def456"}}`)
	})
	head, headSha, base, baseSha, err := svc.GetBranchName(7)
	require.NoError(t, err)
	assert.Equal(t, []string{"feature", "abc123", "main", "def456"}, []string{head, headSha, base, baseSha})
}

func TestMergePullRequestStyle(t *testing.T) {
	for strategy, expected := range map[string]string{"squash": "squash", "rebase": "rebase", "": "merge"} {
		t.Run(strategy, func(t *testing.T) {
			svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/api/v1/repos/org/repo/pulls/7/merge", r.URL.Path)
				assert.Equal(t, expected, decodeBody(t, r)["Do"])
				w.WriteHeader(http.StatusOK)
			})
			require.NoError(t, svc.MergePullRequest(7, strategy))
		})
	}
}

func TestMergePullRequestRejectedByServer(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprint(w, `{"message":"Please try again later"}`)
	})
	assert.ErrorContains(t, svc.MergePullRequest(7, "merge"), "was not merged")
}

func TestGetUserTeamsSkipsNonMemberTeams(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/orgs/org/teams":
			fmt.Fprint(w, `[{"id":1,"name":"admins"},{"id":2,"name":"devs"},{"id":3,"name":"hidden"}]`)
		case "/api/v1/teams/1/members/alice":
			fmt.Fprint(w, `{"login":"alice"}`)
		case "/api/v1/teams/2/members/alice":
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"message":"not found"}`)
		case "/api/v1/teams/3/members/alice":
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, `{"message":"forbidden"}`)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
		}
	})
	teams, err := svc.GetUserTeams("org", "alice")
	require.NoError(t, err)
	assert.Equal(t, []string{"admins"}, teams)
}

func TestIsDivergedFromBranch(t *testing.T) {
	tests := []struct {
		name     string
		ahead    int
		behind   int
		diverged bool
	}{
		{"both sides have commits", 2, 1, true},
		{"only ahead", 2, 0, false},
		{"up to date", 0, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/v1/repos/org/repo/compare/main...feature":
					fmt.Fprintf(w, `{"total_commits":%d}`, tt.ahead)
				case "/api/v1/repos/org/repo/compare/feature...main":
					fmt.Fprintf(w, `{"total_commits":%d}`, tt.behind)
				default:
					t.Errorf("unexpected request %s", r.URL.Path)
				}
			})
			diverged, err := svc.IsDivergedFromBranch("feature", "main")
			require.NoError(t, err)
			assert.Equal(t, tt.diverged, diverged)
		})
	}
}

func TestCommentsRoundTrip(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "POST /api/v1/repos/org/repo/issues/7/comments":
			assert.Equal(t, "hello", decodeBody(t, r)["body"])
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"id":99,"body":"hello","html_url":"https://forgejo.example/org/repo/pulls/7#issuecomment-99"}`)
		case "GET /api/v1/repos/org/repo/issues/7/comments":
			fmt.Fprint(w, `[{"id":99,"body":"hello"}]`)
		case "PATCH /api/v1/repos/org/repo/issues/comments/99":
			assert.Equal(t, "edited", decodeBody(t, r)["body"])
			fmt.Fprint(w, `{"id":99,"body":"edited"}`)
		case "POST /api/v1/repos/org/repo/issues/comments/99/reactions":
			assert.Equal(t, "+1", decodeBody(t, r)["content"])
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"content":"+1"}`)
		case "DELETE /api/v1/repos/org/repo/issues/comments/99":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	})
	comment, err := svc.PublishComment(7, "hello")
	require.NoError(t, err)
	assert.Equal(t, "99", comment.Id)
	assert.Equal(t, "https://forgejo.example/org/repo/pulls/7#issuecomment-99", comment.Url)

	comments, err := svc.GetComments(7)
	require.NoError(t, err)
	require.Len(t, comments, 1)
	assert.Equal(t, "hello", *comments[0].Body)

	require.NoError(t, svc.EditComment(7, "99", "edited"))
	require.NoError(t, svc.CreateCommentReaction("99", "+1"))
	require.NoError(t, svc.DeleteComment("99"))
	assert.Error(t, svc.EditComment(7, "not-a-number", "edited"))
}

func TestGetCombinedPullRequestStatus(t *testing.T) {
	svc := newTestService(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/repos/org/repo/pulls/7":
			fmt.Fprint(w, `{"number":7,"head":{"ref":"feature","sha":"abc123"},"base":{"ref":"main","sha":"def456"}}`)
		case "/api/v1/repos/org/repo/commits/abc123/status":
			fmt.Fprint(w, `{"state":"success","sha":"abc123","total_count":1}`)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
		}
	})
	status, err := svc.GetCombinedPullRequestStatus(7)
	require.NoError(t, err)
	assert.Equal(t, "success", status)
}
