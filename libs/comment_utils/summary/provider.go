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
	switch renderMode {
	case digger_config.CommentRenderModeBasic:
		return BasicCommentUpdater{}, nil
	case digger_config.CommentRenderModeGroupByModule:
		commentUpdater := BasicCommentUpdater{}
		return commentUpdater, nil
	case "noop":
		return NoopCommentUpdater{}, nil
	default:
		return nil, fmt.Errorf("Unknown comment render mode found: %v", renderMode)
	}
}
