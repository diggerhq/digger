package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestReadRepositoryFile(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusNotFound, http.StatusForbidden} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v3/repos/owner/repo/contents/digger.yml" || r.URL.Query().Get("ref") != "feature/timeout" {
					t.Errorf("unexpected request: %s", r.URL)
				}
				if r.Header.Get("Authorization") != "Bearer test-token" {
					t.Error("missing authentication")
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				if status == http.StatusOK {
					fmt.Fprint(w, `{"type":"file","encoding":"base64","content":"Z2l0X3RpbWVvdXQ6IDEyMAo="}`)
				} else {
					fmt.Fprint(w, `{"message":"unavailable"}`)
				}
			}))
			defer server.Close()
			svc, err := NewServiceForCloneURL(server.URL+"/owner/repo.git", "test-token")
			if err != nil {
				t.Fatal(err)
			}
			data, err := svc.ReadRepositoryFile(context.Background(), "digger.yml", "feature/timeout")
			switch status {
			case http.StatusOK:
				if err != nil || string(data) != "git_timeout: 120\n" {
					t.Fatalf("got %q, %v", data, err)
				}
			case http.StatusNotFound:
				if !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("expected missing file, got %v", err)
				}
			case http.StatusForbidden:
				if err == nil || errors.Is(err, os.ErrNotExist) {
					t.Fatalf("expected API failure, got %v", err)
				}
			}
		})
	}
}

func TestNewServiceForCloneURL(t *testing.T) {
	svc, err := NewServiceForCloneURL("https://github.com/owner/repo.git", "test-token")
	if err != nil {
		t.Fatal(err)
	}
	if svc.Client.BaseURL.String() != "https://api.github.com/" || svc.Owner != "owner" || svc.RepoName != "repo" {
		t.Fatalf("unexpected service: %+v", svc)
	}
	for _, cloneURL := range []string{"", "file:///tmp/repo", "https://github.com/owner", "https://github.com/owner/repo/extra"} {
		if _, err := NewServiceForCloneURL(cloneURL, "test-token"); err == nil {
			t.Errorf("accepted invalid clone URL: %s", cloneURL)
		}
	}
}
