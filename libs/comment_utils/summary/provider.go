package comment_updater

import (
	"fmt"
	"github.com/diggerhq/digger/libs/digger_config"
)

type CommentUpdaterProvider interface {
	Get(config digger_config.DiggerConfig) (CommentUpdater, error)
}

type CommentUpdaterProviderBasic struct{}

func (c CommentUpdaterProviderBasic) Get(config digger_config.DiggerConfig) (CommentUpdater, error) {
	if config.CommentRenderMode == digger_config.CommentRenderModeBasic {
		return BasicCommentUpdater{}, nil
	} else if config.CommentRenderMode == digger_config.CommentRenderModeGroupByModule {

		commentUpdater := BasicCommentUpdater{}
		return commentUpdater, nil
	} else {
		return nil, fmt.Errorf("Unknown comment render mode found: %v", config.CommentRenderMode)
	}
}
