package utils

import (
	"fmt"
	"github.com/diggerhq/digger/libs/ci"
	"github.com/diggerhq/digger/libs/scheduler"
	"log"
)

type CommentReporter struct {
	PrNumber  int
	PrService ci.PullRequestService
	CommentId string
}

func InitCommentReporter(prService ci.PullRequestService, prNumber int, commentMessage string) (*CommentReporter, error) {
	comment, err := prService.PublishComment(prNumber, commentMessage)
	if err != nil {
		return nil, fmt.Errorf("count not initialize comment reporter: %v", err)
	}
	//commentId, err := strconv.ParseInt(fmt.Sprintf("%v", comment.Id), 10, 64)
	if err != nil {
		log.Printf("could not convert to int64, %v", err)
		return nil, fmt.Errorf("could not convert to int64, %v", err)
	}
	return &CommentReporter{
		PrNumber:  prNumber,
		PrService: prService,
		CommentId: comment.Id,
	}, nil
}

func ReportInitialJobsStatus(cr *CommentReporter, jobs []scheduler.Job) error {
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

func ReportNoProjectsImpacted(cr *CommentReporter, jobs []scheduler.Job) error {
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
