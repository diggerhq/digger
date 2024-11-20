package utils

import (
	"fmt"
	"github.com/diggerhq/digger/libs/ci"
	"github.com/diggerhq/digger/libs/scheduler"
	"log"
)

// CommentReporterManager thin wrapper around CommentReporter that makes it "Lazy" so we dont comment anything when it is initialized
// and we can update comment at any time (intial update creates a new comment, future updates will update that comment)
type CommentReporterManager struct {
	CommentReporter *CommentReporter
	prService       ci.PullRequestService
	prNumber        int
}

func InitCommentReporterManager(prService ci.PullRequestService, prNumber int) CommentReporterManager {
	return CommentReporterManager{
		CommentReporter: nil,
		prService:       prService,
		prNumber:        prNumber,
	}
}

func (cm *CommentReporterManager) GetCommentReporter() (*CommentReporter, error) {
	if cm.CommentReporter != nil {
		return cm.CommentReporter, nil
	} else {
		cr, err := InitCommentReporter(cm.prService, cm.prNumber, "digger report")
		cm.CommentReporter = cr
		return cr, err
	}
}

func (cm *CommentReporterManager) UpdateComment(commentMessage string) (*CommentReporter, error) {
	if cm.CommentReporter != nil {
		err := UpdateCRComment(cm.CommentReporter, commentMessage)
		return cm.CommentReporter, err
	} else {
		cr, err := InitCommentReporter(cm.prService, cm.prNumber, commentMessage)
		cm.CommentReporter = cr
		return cr, err

	}
}

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

func UpdateCRComment(cr *CommentReporter, comment string) error {
	commentId := cr.CommentId
	prNumber := cr.PrNumber
	prService := cr.PrService
	err := prService.EditComment(prNumber, commentId, comment)
	return err

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

	const GithubCommentMaxLength = 65536

	if len(message) > GithubCommentMaxLength {
		// TODO: Handle the case where message is too long by trimming
		log.Printf("WARN: message is too long, trimming")
		log.Printf(message)
		const footer = "[trimmed]"
		trimLength := len(message) - GithubCommentMaxLength + len(footer)
		message = message[:len(message)-trimLength] + footer
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
