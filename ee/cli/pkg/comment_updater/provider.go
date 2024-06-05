package comment_updater

import (
	"fmt"
	comment_updater "github.com/diggerhq/digger/libs/comment_utils/summary"
	"github.com/diggerhq/digger/libs/digger_config"
)

type CommentUpdaterProviderAdvanced struct {
}

func (c CommentUpdaterProviderAdvanced) Get(config digger_config.DiggerConfig) (comment_updater.CommentUpdater, error) {
	if config.CommentRenderMode == digger_config.CommentRenderModeBasic {
		return AdvancedCommentUpdater{}, nil
	} else if config.CommentRenderMode == digger_config.CommentRenderModeGroupByModule {
		commentUpdater := comment_updater.BasicCommentUpdater{}
		return commentUpdater, nil
	} else {
		return nil, fmt.Errorf("Unknown comment render mode found: %v", config.CommentRenderMode)
	}
}
