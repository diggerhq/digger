package reporting

import (
	"digger/pkg/core/utils"
	"fmt"
	orchestrator "github.com/diggerhq/lib-orchestrator"
	"log"
	"strings"
	"time"
)

type CiReporter struct {
	CiService                     orchestrator.PullRequestService
	PrNumber                      int
	IsSupportsCollapsibleComments bool
	ReportStrategy                ReportStrategy
}

func (ciReporter *CiReporter) Report(report string, reportFormatter func(report string) string) error {
	return ciReporter.ReportStrategy.Report(ciReporter.CiService, ciReporter.PrNumber, report, reportFormatter, ciReporter.SupportsCollapsibleComments())
}

func (ciReporter *CiReporter) SupportsCollapsibleComments() bool {
	return ciReporter.IsSupportsCollapsibleComments
}

type StdOutReporter struct{}

func (reporter *StdOutReporter) Report(report string, reportFormatter func(report string) string) error {
	log.Println(reportFormatter(report))
	return nil
}

func (reporter *StdOutReporter) SupportsCollapsibleComments() bool {
	return false
}

type ReportStrategy interface {
	Report(ciService orchestrator.PullRequestService, PrNumber int, report string, reportFormatter func(report string) string, supportsCollapsibleComment bool) error
}

type CommentPerRunStrategy struct {
	TimeOfRun time.Time
}

func (strategy *CommentPerRunStrategy) Report(ciService orchestrator.PullRequestService, PrNumber int, report string, reportFormatter func(report string) string, supportsCollapsibleComment bool) error {
	comments, err := ciService.GetComments(PrNumber)
	if err != nil {
		return fmt.Errorf("error getting comments: %v", err)
	}

	reportTitle := "Digger run report at " + strategy.TimeOfRun.Format("2006-01-02 15:04:05 (MST)")
	return upsertComment(ciService, PrNumber, report, reportFormatter, comments, reportTitle, supportsCollapsibleComment)
}

func upsertComment(ciService orchestrator.PullRequestService, PrNumber int, report string, reportFormatter func(report string) string, comments []orchestrator.Comment, reportTitle string, supportsCollapsible bool) error {
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
		var comment string
		if !supportsCollapsible {
			comment = utils.AsComment(reportTitle)(report)
		} else {
			comment = utils.AsCollapsibleComment(reportTitle)(report)
		}
		err := ciService.PublishComment(PrNumber, comment)
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

	var completeComment string
	if !supportsCollapsible {
		completeComment = utils.AsComment(reportTitle)(commentBody)
	} else {
		completeComment = utils.AsCollapsibleComment(reportTitle)(commentBody)
	}

	err := ciService.EditComment(PrNumber, commentIdForThisRun, completeComment)

	if err != nil {
		return fmt.Errorf("error editing comment: %v", err)
	}
	return nil
}

type LatestRunCommentStrategy struct {
	TimeOfRun time.Time
}

func (strategy *LatestRunCommentStrategy) Report(ciService orchestrator.PullRequestService, prNumber int, comment string, commentFormatting func(comment string) string, supportsCollapsibleComments bool) error {
	comments, err := ciService.GetComments(prNumber)
	if err != nil {
		return fmt.Errorf("error getting comments: %v", err)
	}

	reportTitle := "Digger latest run report"
	return upsertComment(ciService, prNumber, comment, commentFormatting, comments, reportTitle, supportsCollapsibleComments)
}

type MultipleCommentsStrategy struct{}

func (strategy *MultipleCommentsStrategy) Report(ciService orchestrator.PullRequestService, PrNumber int, report string, formatter func(string) string, supportsCollapsibleComments bool) error {
	return ciService.PublishComment(PrNumber, formatter(report))
}
