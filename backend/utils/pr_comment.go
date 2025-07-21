package utils

import (
	"fmt"
	"log/slog"

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

	// commentId, err := strconv.ParseInt(fmt.Sprintf("%v", comment.Id), 10, 64)
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
		message += ":construction_worker: No projects impacted"
	} else {
		message += "| Project | Status |\n"
		message += "|---------|--------|\n"
		for _, job := range jobs {
			message += fmt.Sprintf(""+
				"|:clock11: **%v**|pending...|\n", job.ProjectName)
		}
	}

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

func ReportNoProjectsImpacted(cr *CommentReporter, jobs []scheduler.Job) error {
	prNumber := cr.PrNumber
	prService := cr.PrService
	commentId := cr.CommentId

	slog.Info("Reporting no projects impacted",
		"prNumber", prNumber,
		"commentId", commentId,
		"jobCount", len(jobs),
	)

	message := "" +
		":construction_worker: The following projects are impacted\n\n"
	for _, job := range jobs {
		message += fmt.Sprintf(""+
			"<!-- PROJECTHOLDER %v -->\n"+
			":clock11: **%v**: pending...\n"+
			"<!-- PROJECTHOLDEREND %v -->\n"+
			"", job.ProjectName, job.ProjectName, job.ProjectName)

		slog.Debug("Added project placeholder to message", "projectName", job.ProjectName)
	}

	err := prService.EditComment(prNumber, commentId, message)
	if err != nil {
		slog.Error("Failed to update comment with no projects impacted message",
			"prNumber", prNumber,
			"commentId", commentId,
			"error", err,
		)
		return err
	}

	slog.Debug("Successfully reported no projects impacted", "prNumber", prNumber, "commentId", commentId)
	return nil
}
