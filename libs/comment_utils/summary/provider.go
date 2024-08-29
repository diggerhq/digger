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
	if renderMode == digger_config.CommentRenderModeBasic {
		return BasicCommentUpdater{}, nil
	} else if renderMode == digger_config.CommentRenderModeGroupByModule {
		commentUpdater := BasicCommentUpdater{}
		return commentUpdater, nil
	} else if renderMode == "noop" {
		return NoopCommentUpdater{}, nil
	} else {
		return nil, fmt.Errorf("Unknown comment render mode found: %v", renderMode)
	}
}
