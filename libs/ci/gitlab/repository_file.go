package gitlab

import (
	"context"
	"fmt"
	"net/http"
	"os"

	gitlab "github.com/xanzy/go-gitlab"
)

func (svc GitLabService) ReadRepositoryFile(ctx context.Context, path, ref string) ([]byte, error) {
	contents, response, err := svc.Client.RepositoryFiles.GetRawFile(*svc.Context.ProjectId, path, &gitlab.GetRawFileOptions{Ref: &ref}, gitlab.WithContext(ctx))
	if response != nil && response.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%s: %w", path, os.ErrNotExist)
	}
	return contents, err
}
