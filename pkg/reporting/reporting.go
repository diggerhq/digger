package reporting

import (
	"digger/pkg/ci"
	"digger/pkg/core/utils"
	"fmt"
	"strings"
	"time"
)

type CiReporter struct {
	CiService      ci.CIService
	PrNumber       int
	ReportStrategy ReportStrategy
}

type ReportStrategy interface {
	Report(ciService ci.CIService, PrNumber int, report string, reportFormatter func(report string) string) error
}

type CommentPerRunStrategy struct {
	TimeOfRun time.Time
}

func (strategy *CommentPerRunStrategy) Report(ciService ci.CIService, PrNumber int, report string, reportFormatter func(report string) string) error {
	comments, err := ciService.GetComments(PrNumber)
	if err != nil {
		return fmt.Errorf("error getting comments: %v", err)
	}

	reportTitle := "Digger run report at " + strategy.TimeOfRun.Format("2006-01-02 15:04:05")
	report = reportFormatter(report)
	var commentIdForThisRun interface{}
	var commentBody string
	for _, comment := range comments {
		if strings.Contains(*comment.Body, reportTitle) {
			commentIdForThisRun = comment.Id
			commentBody = *comment.Body
			break
		}
	}

	if commentIdForThisRun == nil {
		collapsibleComment := utils.AsCollapsibleComment(reportTitle)(report)
		err := ciService.PublishComment(PrNumber, collapsibleComment)
		if err != nil {
			return fmt.Errorf("error publishing comment: %v", err)
		}
		return nil
	}

	// strip first and last lines
	lines := strings.Split(commentBody, "\n")
	lines = lines[1 : len(lines)-1]
	commentBody = strings.Join(lines, "\n")

	commentBody = commentBody + "\n\n" + report + "\n"

	completeComment := utils.AsCollapsibleComment(reportTitle)(commentBody)

	err = ciService.EditComment(commentIdForThisRun, completeComment)

	if err != nil {
		return fmt.Errorf("error editing comment: %v", err)
	}
	return nil
}

type LatestRunCommentStrategy struct {
	TimeOfRun time.Time
}

func (strategy *LatestRunCommentStrategy) Report(ciService ci.CIService, prNumber int, comment string, commentFormatting func(comment string) string) error {
	comments, err := ciService.GetComments(prNumber)
	if err != nil {
		return fmt.Errorf("error getting comments: %v", err)
	}

	reportTitle := "Digger latest run report"
	comment = commentFormatting(comment)
	var commentIdForThisRun interface{}
	for _, comment := range comments {
		if strings.Contains(*comment.Body, reportTitle) {
			commentIdForThisRun = comment.Id
			break
		}
	}

	if commentIdForThisRun == nil {
		collapsibleComment := utils.AsCollapsibleComment(reportTitle)(comment)
		err := ciService.PublishComment(prNumber, collapsibleComment)
		if err != nil {
			return fmt.Errorf("error publishing comment: %v", err)
		}
		return nil
	}

	completeComment := utils.AsCollapsibleComment(reportTitle)(comment)

	err = ciService.EditComment(commentIdForThisRun, completeComment)

	if err != nil {
		return fmt.Errorf("error editing comment: %v", err)
	}
	return nil
}

type MultipleCommentsStrategy struct{}

func (strategy *MultipleCommentsStrategy) Report(ciService ci.CIService, PrNumber int, report string, formatter func(string) string) error {
	return ciService.PublishComment(PrNumber, formatter(report))
}

func (ciReporter *CiReporter) Report(report string, reportFormatter func(report string) string) error {
	return ciReporter.ReportStrategy.Report(ciReporter.CiService, ciReporter.PrNumber, report, reportFormatter)
}
