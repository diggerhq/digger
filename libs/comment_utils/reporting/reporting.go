package reporting

import (
	"fmt"
	"github.com/diggerhq/digger/libs/ci"
	"github.com/diggerhq/digger/libs/comment_utils/utils"
	"log"
	"strings"
	"time"
)

type CiReporter struct {
	CiService         ci.PullRequestService
	PrNumber          int
	IsSupportMarkdown bool
	ReportStrategy    ReportStrategy
}

func (ciReporter CiReporter) Report(report string, reportFormatting func(report string) string) error {
	_, _, err := ciReporter.ReportStrategy.Report("", ciReporter.CiService, ciReporter.PrNumber, report, reportFormatting, ciReporter.SupportsMarkdown())
	return err
}

func (ciReporter CiReporter) Flush() (string, string, error) {
	return "", "", nil
}

func (ciReporter CiReporter) Suppress() error {
	return nil
}

func (ciReporter CiReporter) SupportsMarkdown() bool {
	return ciReporter.IsSupportMarkdown
}

type StdOutReporter struct{}

func (reporter StdOutReporter) Report(report string, reportFormatting func(report string) string) error {
	log.Printf("Info: %v", report)
	return nil
}

func (reporter StdOutReporter) Flush() (string, string, error) {
	return "", "", nil
}

func (reporter StdOutReporter) SupportsMarkdown() bool {
	return false
}

func (reporter StdOutReporter) Suppress() error {
	return nil
}

type ReportStrategy interface {
	Report(projectName string, report string, reportFormatter func(report string) string) (error error)
	Flush(ciService ci.PullRequestService, PrNumber int, supportsCollapsibleComment bool) (commentId string, commentUrl string, error error)
}

type ReportFormat struct {
	ReportTitle     string
	ReportFormatter func(report string) string
}

type SingleCommentStrategy struct {
	Title      string
	TimeOfRun  time.Time
	Formatters map[string]ReportFormat
}

func (strategy SingleCommentStrategy) Report(projectName string, report string, reportFormatter func(report string) string, supportsCollapsibleComment bool) error {
	strategy.Formatters[projectName] = ReportFormat{
		ReportTitle:     report,
		ReportFormatter: reportFormatter,
	}
	return nil
}

func (strategy SingleCommentStrategy) Flush(ciService ci.PullRequestService, PrNumber int, supportsCollapsibleComment bool) (string, string, error) {
	comments, err := ciService.GetComments(PrNumber)
	if err != nil {
		return "", "", fmt.Errorf("error getting comments: %v", err)
	}

	var reportTitle string
	if strategy.Title != "" {
		reportTitle = strategy.Title + " " + strategy.TimeOfRun.Format("2006-01-02 15:04:05 (MST)")
	} else {
		reportTitle = "Digger run report at " + strategy.TimeOfRun.Format("2006-01-02 15:04:05 (MST)")
	}
	commentId, commentUrl, err := upsertComment(ciService, PrNumber, report, reportFormatter, comments, reportTitle, supportsCollapsibleComment)
	return commentId, commentUrl, err
	return "", "", nil
}

func upsertComment(ciService ci.PullRequestService, PrNumber int, report string, reportFormatter func(report string) string, comments []ci.Comment, reportTitle string, supportsCollapsible bool) (string, string, error) {
	report = reportFormatter(report)
	commentIdForThisRun := ""
	var commentBody string
	var commentUrl string
	for _, comment := range comments {
		if strings.Contains(*comment.Body, reportTitle) {
			commentIdForThisRun = comment.Id
			commentBody = *comment.Body
			commentUrl = comment.Url
			break
		}
	}

	if commentIdForThisRun == "" {
		var commentMessage string
		if !supportsCollapsible {
			commentMessage = utils.AsComment(reportTitle)(report)
		} else {
			commentMessage = utils.AsCollapsibleComment(reportTitle, false)(report)
		}
		comment, err := ciService.PublishComment(PrNumber, commentMessage)
		if err != nil {
			return "", "", fmt.Errorf("error publishing comment: %v", err)
		}
		return fmt.Sprintf("%v", comment.Id), comment.Url, nil
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
		completeComment = utils.AsCollapsibleComment(reportTitle, false)(commentBody)
	}

	err := ciService.EditComment(PrNumber, commentIdForThisRun, completeComment)

	if err != nil {
		return "", "", fmt.Errorf("error editing comment: %v", err)
	}
	return fmt.Sprintf("%v", commentIdForThisRun), commentUrl, nil
}

type MultipleCommentsStrategy struct {
	Formatters map[string]ReportFormat
}

func (strategy MultipleCommentsStrategy) Report(projectName string, report string, reportFormatter func(report string) string) error {
	strategy.Formatters[projectName] = ReportFormat{
		ReportTitle:     report,
		ReportFormatter: reportFormatter,
	}
	return nil
}

func (strategy MultipleCommentsStrategy) Flush(ciService ci.PullRequestService, PrNumber int, supportsCollapsibleComment bool) (string, string, error) {
	return "", "", nil
}
