package utils

import (
	"fmt"
	"log/slog"
	"sort"

	"github.com/diggerhq/digger/libs/ci"
	"github.com/diggerhq/digger/libs/scheduler"
)

// CommentReporterManager thin wrapper around CommentReporter that makes it "Lazy" so we dont comment anything when it is initialized
// and we can update comment at any time (initial update creates a new comment, future updates will update that comment)
type CommentReporterManager struct {
	CommentReporter *CommentReporter
	prService       ci.PullRequestService
	prNumber        int
}

func InitCommentReporterManager(prService ci.PullRequestService, prNumber int) CommentReporterManager {
	slog.Debug("Initializing comment reporter manager", "prNumber", prNumber)
	return CommentReporterManager{
		CommentReporter: nil,
		prService:       prService,
		prNumber:        prNumber,
	}
}

func (cm *CommentReporterManager) GetCommentReporter() (*CommentReporter, error) {
	if cm.CommentReporter != nil {
		slog.Debug("Using existing comment reporter", "prNumber", cm.prNumber, "commentId", cm.CommentReporter.CommentId)
		return cm.CommentReporter, nil
	} else {
		slog.Debug("Creating new comment reporter", "prNumber", cm.prNumber)
		cr, err := InitCommentReporter(cm.prService, cm.prNumber, "digger report")
		if err != nil {
			slog.Error("Failed to initialize comment reporter", "prNumber", cm.prNumber, "error", err)
			return nil, err
		}

		cm.CommentReporter = cr
		slog.Debug("Created new comment reporter", "prNumber", cm.prNumber, "commentId", cr.CommentId)
		return cr, err
	}
}

func (cm *CommentReporterManager) UpdateComment(commentMessage string) (*CommentReporter, error) {
	if cm.CommentReporter != nil {
		slog.Debug("Updating existing comment",
			"prNumber", cm.prNumber,
			"commentId", cm.CommentReporter.CommentId,
			"messageLength", len(commentMessage),
		)

		err := UpdateCRComment(cm.CommentReporter, commentMessage)
		if err != nil {
			slog.Error("Failed to update comment",
				"prNumber", cm.prNumber,
				"commentId", cm.CommentReporter.CommentId,
				"error", err,
			)
		}
		return cm.CommentReporter, err
	} else {
		slog.Debug("Creating new comment",
			"prNumber", cm.prNumber,
			"messageLength", len(commentMessage),
		)

		cr, err := InitCommentReporter(cm.prService, cm.prNumber, commentMessage)
		if err != nil {
			slog.Error("Failed to create comment", "prNumber", cm.prNumber, "error", err)
			return nil, err
		}

		cm.CommentReporter = cr
		slog.Debug("Created new comment", "prNumber", cm.prNumber, "commentId", cr.CommentId)
		return cr, err
	}
}

type CommentReporter struct {
	PrNumber  int
	PrService ci.PullRequestService
	CommentId string
}

func InitCommentReporter(prService ci.PullRequestService, prNumber int, commentMessage string) (*CommentReporter, error) {
	slog.Debug("Initializing comment reporter",
		"prNumber", prNumber,
		"messageLength", len(commentMessage),
	)

	comment, err := prService.PublishComment(prNumber, commentMessage)
	if err != nil {
		slog.Error("Failed to publish comment", "prNumber", prNumber, "error", err)
		return nil, fmt.Errorf("count not initialize comment reporter: %v", err)
	}

	//commentId, err := strconv.ParseInt(fmt.Sprintf("%v", comment.Id), 10, 64)
	if err != nil {
		slog.Error("Could not convert comment ID to int64", "commentId", comment.Id, "error", err)
		return nil, fmt.Errorf("could not convert to int64, %v", err)
	}

	slog.Info("Created new PR comment", "prNumber", prNumber, "commentId", comment.Id)
	return &CommentReporter{
		PrNumber:  prNumber,
		PrService: prService,
		CommentId: comment.Id,
	}, nil
}

func UpdateCRComment(cr *CommentReporter, comment string) error {
	commentId := cr.CommentId
	prNumber := cr.PrNumber
	prService := cr.PrService

	slog.Debug("Updating PR comment",
		"prNumber", prNumber,
		"commentId", commentId,
		"messageLength", len(comment),
	)

	err := prService.EditComment(prNumber, commentId, comment)
	if err != nil {
		slog.Error("Failed to edit comment",
			"prNumber", prNumber,
			"commentId", commentId,
			"error", err,
		)
		return err
	}

	slog.Debug("Successfully updated PR comment", "prNumber", prNumber, "commentId", commentId)
	return nil
}

func trimMessageIfExceedsMaxLength(message string) string {

	const GithubCommentMaxLength = 65536

	if len(message) > GithubCommentMaxLength {
		slog.Warn("Comment message is too long, trimming",
			"originalLength", len(message),
			"maxLength", GithubCommentMaxLength,
		)

		const footer = "[trimmed]"
		trimLength := len(message) - GithubCommentMaxLength + len(footer)
		message = message[:len(message)-trimLength] + footer

		slog.Debug("Trimmed comment message",
			"newLength", len(message),
			"trimmedBytes", trimLength,
		)
	}
	return message
}

func ReportInitialJobsStatus(cr *CommentReporter, jobs []scheduler.Job) error {
	prNumber := cr.PrNumber
	prService := cr.PrService
	commentId := cr.CommentId

	slog.Info("Reporting initial jobs status",
		"prNumber", prNumber,
		"commentId", commentId,
		"jobCount", len(jobs),
	)

	message := ""
	if len(jobs) == 0 {
		message = message + ":construction_worker: No projects impacted"
	} else {
		message = message + fmt.Sprintf("| Project | Status |\n")
		message = message + fmt.Sprintf("|---------|--------|\n")
		for _, job := range jobs {
			message = message + fmt.Sprintf(""+
				"|:clock11: **%v**|pending...|\n", job.ProjectName)
		}
	}

	message = trimMessageIfExceedsMaxLength(message)
	err := prService.EditComment(prNumber, commentId, message)
	if err != nil {
		slog.Error("Failed to update comment with initial jobs status",
			"prNumber", prNumber,
			"commentId", commentId,
			"error", err,
		)
		return err
	}

	slog.Debug("Successfully reported initial jobs status", "prNumber", prNumber, "commentId", commentId)
	return nil
}

func ReportLayersTableForJobs(cr *CommentReporter, jobs []scheduler.Job) error {
	prNumber := cr.PrNumber
	prService := cr.PrService
	commentId := cr.CommentId

	// sort jobs by layer for better display (sort by name too)
	sort.Slice(jobs, func(i, j int) bool {
		if jobs[i].Layer == jobs[j].Layer {
			return jobs[i].ProjectName < jobs[j].ProjectName
		}
		return jobs[i].Layer < jobs[j].Layer
	})

	slog.Info("Reporting initial jobs status",
		"prNumber", prNumber,
		"commentId", commentId,
		"jobCount", len(jobs),
	)

	message := ""
	if len(jobs) == 0 {
		message = message + ":construction_worker: No projects impacted"
	} else {
		message = message + fmt.Sprintf("| Project | Layer |\n")
		message = message + fmt.Sprintf("|---------|--------|\n")
		for _, job := range jobs {
			message = message + fmt.Sprintf(""+
				"|:clock11: **%v**|%v|\n", job.ProjectName, job.Layer)
		}
	}

	message += "----------------\n\n"
	message += `
<details>
  <summary>Instructions</summary>

Since you enabled layers in your configuration, you can proceed to perform a layer-by-layer deployment.
To start planning the first layer you can run "digger plan --layer 0". To apply the first layer, run "digger apply --layer 0".

To deploy the next layer, run "digger plan --layer 1". To apply the next layer, run "digger apply --layer 1".

And so on. A new commit on the branch will restart this deployment process.
</details>
`
	message = trimMessageIfExceedsMaxLength(message)
	err := prService.EditComment(prNumber, commentId, message)
	if err != nil {
		slog.Error("Failed to update comment with initial jobs status",
			"prNumber", prNumber,
			"commentId", commentId,
			"error", err,
		)
		return err
	}

	slog.Debug("Successfully reported initial jobs status", "prNumber", prNumber, "commentId", commentId)
	return nil
}
