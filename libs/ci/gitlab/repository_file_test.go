package gitlab

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	gitlab "github.com/xanzy/go-gitlab"
)

func TestReadRepositoryFile(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusNotFound, http.StatusForbidden} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v4/projects/42/repository/files/digger.yml/raw" || r.URL.Query().Get("ref") != "feature/timeout" {
					t.Errorf("unexpected request: %s", r.URL)
				}
				if r.Header.Get("PRIVATE-TOKEN") != "test-token" {
					t.Error("missing authentication")
				}
				w.WriteHeader(status)
				fmt.Fprint(w, "git_timeout: 120\n")
			}))
			defer server.Close()
			client, err := gitlab.NewClient("test-token", gitlab.WithBaseURL(server.URL))
			if err != nil {
				t.Fatal(err)
			}
			projectID := 42
			svc := GitLabService{Client: client, Context: &GitLabContext{ProjectId: &projectID}}
			data, err := svc.ReadRepositoryFile(context.Background(), "digger.yml", "feature/timeout")
			if status == http.StatusOK {
				if err != nil || string(data) != "git_timeout: 120\n" {
					t.Fatalf("got %q, %v", data, err)
				}
			} else if err == nil || errors.Is(err, os.ErrNotExist) != (status == http.StatusNotFound) {
				t.Fatalf("unexpected error for HTTP %d: %v", status, err)
			}
		})
	}
}
