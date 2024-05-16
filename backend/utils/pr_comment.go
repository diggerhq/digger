package utils

import (
	"fmt"
	"github.com/diggerhq/digger/libs/orchestrator"
	github2 "github.com/diggerhq/digger/libs/orchestrator/github"
)

type CommentReporter struct {
	PrNumber  int
	PrService *github2.GithubService
	CommentId int64
}

func InitCommentReporter(prService *github2.GithubService, prNumber int, comment string) (*CommentReporter, error) {
	commentId, err := prService.PublishComment(prNumber, comment)
	if err != nil {
		return nil, fmt.Errorf("count not initialize comment reporter: %v", err)
	}
	return &CommentReporter{
		PrNumber:  prNumber,
		PrService: prService,
		CommentId: commentId,
	}, nil
}

func ReportInitialJobsStatus(cr *CommentReporter, jobs []orchestrator.Job) error {
	prNumber := cr.PrNumber
	prService := cr.PrService
	commentId := cr.CommentId
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
	err := prService.EditComment(prNumber, commentId, message)
	return err
}

func ReportNoProjectsImpacted(cr *CommentReporter, jobs []orchestrator.Job) error {
	prNumber := cr.PrNumber
	prService := cr.PrService
	commentId := cr.CommentId
	message := "" +
		":construction_worker: The following projects are impacted\n\n"
	for _, job := range jobs {
		message = message + fmt.Sprintf(""+
			"<!-- PROJECTHOLDER %v -->\n"+
			":clock11: **%v**: pending...\n"+
			"<!-- PROJECTHOLDEREND %v -->\n"+
			"", job.ProjectName, job.ProjectName, job.ProjectName)
	}
	err := prService.EditComment(prNumber, commentId, message)
	return err
}
