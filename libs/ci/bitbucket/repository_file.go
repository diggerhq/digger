package bitbucket

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
)

func (b BitbucketAPI) ReadRepositoryFile(ctx context.Context, path, ref string) ([]byte, error) {
	if ref == "" {
		ref = "HEAD"
	}
	endpoint := fmt.Sprintf("%s/repositories/%s/%s/src/%s/%s", bitbucketBaseURL, url.PathEscape(b.RepoWorkspace), url.PathEscape(b.RepoName), url.PathEscape(ref), url.PathEscape(path))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+b.AuthToken)
	response, err := b.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%s: %w", path, os.ErrNotExist)
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("read %s: Bitbucket returned HTTP %d", path, response.StatusCode)
	}
	return io.ReadAll(response.Body)
}
