package github

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/google/go-github/v61/github"
)

// NewServiceForCloneURL creates a client for workers that only have a clone URL and token.
func NewServiceForCloneURL(cloneURL, token string) (*GithubService, error) {
	u, err := url.Parse(cloneURL)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return nil, fmt.Errorf("expected an HTTP(S) GitHub clone URL")
	}
	parts := strings.Split(strings.Trim(strings.TrimSuffix(u.Path, ".git"), "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("expected owner/repository in GitHub clone URL")
	}
	client := github.NewClient(nil).WithAuthToken(token)
	if u.Host != "github.com" {
		baseURL := u.Scheme + "://" + u.Host + "/"
		client, err = client.WithEnterpriseURLs(baseURL, baseURL)
		if err != nil {
			return nil, err
		}
	}
	return &GithubService{Client: client, Owner: parts[0], RepoName: parts[1]}, nil
}

func (svc GithubService) ReadRepositoryFile(ctx context.Context, path, ref string) ([]byte, error) {
	file, _, response, err := svc.Client.Repositories.GetContents(ctx, svc.Owner, svc.RepoName, path, &github.RepositoryContentGetOptions{Ref: ref})
	if response != nil && response.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%s: %w", path, os.ErrNotExist)
	}
	if err != nil {
		return nil, err
	}
	if file == nil {
		return nil, fmt.Errorf("%s is not a file", path)
	}
	contents, err := file.GetContent()
	return []byte(contents), err
}
