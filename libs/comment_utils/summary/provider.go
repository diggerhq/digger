package comment_updater

import (
	"fmt"
	"github.com/diggerhq/digger/libs/digger_config"
)

type CommentUpdaterProvider interface {
	Get(renderMode string) (CommentUpdater, error)
}

type CommentUpdaterProviderBasic struct{}

func (c CommentUpdaterProviderBasic) Get(renderMode string) (CommentUpdater, error) {
	// Always return NoopCommentUpdater to disable CLI comment updating
	// Real-time comment updating is now handled by the backend
	return NoopCommentUpdater{}, nil
}
