package comment_updater

import (
	"fmt"
	comment_updater "github.com/diggerhq/digger/libs/comment_utils/summary"
	"github.com/diggerhq/digger/libs/digger_config"
)

type CommentUpdaterProviderAdvanced struct {
}

func (c CommentUpdaterProviderAdvanced) Get(renderMode string) (comment_updater.CommentUpdater, error) {
	if renderMode == digger_config.CommentRenderModeBasic {
		return comment_updater.BasicCommentUpdater{}, nil
	} else if renderMode == digger_config.CommentRenderModeGroupByModule {
		commentUpdater := comment_updater.BasicCommentUpdater{}
		return commentUpdater, nil
	} else {
		return nil, fmt.Errorf("Unknown comment render mode found: %v", renderMode)
	}
}
