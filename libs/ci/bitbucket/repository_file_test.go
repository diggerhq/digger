package bitbucket

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

type repositoryFileTransport func(*http.Request) (*http.Response, error)

func (f repositoryFileTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestReadRepositoryFile(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusNotFound, http.StatusForbidden} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			svc := BitbucketAPI{
				RepoWorkspace: "owner", RepoName: "repo", AuthToken: "test-token",
				HttpClient: http.Client{Transport: repositoryFileTransport(func(r *http.Request) (*http.Response, error) {
					if r.URL.EscapedPath() != "/2.0/repositories/owner/repo/src/feature%2Ftimeout/digger.yml" {
						t.Errorf("unexpected request: %s", r.URL)
					}
					if r.Header.Get("Authorization") != "Bearer test-token" {
						t.Error("missing authentication")
					}
					return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("git_timeout: 120\n")), Header: make(http.Header)}, nil
				})},
			}
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
