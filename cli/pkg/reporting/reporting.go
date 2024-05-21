package reporting

import (
	"fmt"
	"github.com/diggerhq/digger/cli/pkg/core/utils"
	"github.com/diggerhq/digger/libs/orchestrator"
	"log"
	"strings"
	"time"
)

type CiReporter struct {
	CommentId         *int64
	CiService         orchestrator.PullRequestService
	PrNumber          int
	IsSupportMarkdown bool
	ReportStrategy    ReportStrategy
}

func (ciReporter *CiReporter) Report(report string, reportFormatting func(report string) string) (string, string, error) {
	commentId, commentUrl, err := ciReporter.ReportStrategy.Report(ciReporter.CiService, ciReporter.PrNumber, report, reportFormatting, ciReporter.SupportsMarkdown())
	return commentId, commentUrl, err
}

func (ciReporter *CiReporter) Flush() (string, string, error) {
	return "", "", nil
}

func (ciReporter *CiReporter) Suppress() error {
	return nil
}

func (ciReporter *CiReporter) SupportsMarkdown() bool {
	return ciReporter.IsSupportMarkdown
}

type CiReporterLazy struct {
	CiReporter   CiReporter
	isSuppressed bool
	reports      []string
	formatters   []func(report string) string
}

func NewCiReporterLazy(ciReporter CiReporter) *CiReporterLazy {
	return &CiReporterLazy{
		CiReporter:   ciReporter,
		isSuppressed: false,
		reports:      []string{},
		formatters:   []func(report string) string{},
	}
}

func (lazyReporter *CiReporterLazy) Report(report string, reportFormatting func(report string) string) (string, string, error) {
	lazyReporter.reports = append(lazyReporter.reports, report)
	lazyReporter.formatters = append(lazyReporter.formatters, reportFormatting)
	return "", "", nil
}

func (lazyReporter *CiReporterLazy) Flush() (string, string, error) {
	if lazyReporter.isSuppressed {
		log.Printf("Reporter is suprresed, ignoring messages ...")
		return "", "", nil
	}
	var commentId, commentUrl string
	for i, _ := range lazyReporter.formatters {
		var err error
		commentId, commentUrl, err = lazyReporter.CiReporter.ReportStrategy.Report(lazyReporter.CiReporter.CiService, lazyReporter.CiReporter.PrNumber, lazyReporter.reports[i], lazyReporter.formatters[i], lazyReporter.SupportsMarkdown())
		if err != nil {
			log.Printf("failed to report strategy: ")
			return "", "", err
		}
	}
	// clear the buffers
	lazyReporter.formatters = []func(comment string) string{}
	lazyReporter.reports = []string{}
	return commentId, commentUrl, nil
}

func (lazyReporter *CiReporterLazy) Suppress() error {
	lazyReporter.isSuppressed = true
	return nil
}

func (lazyReporter *CiReporterLazy) SupportsMarkdown() bool {
	return lazyReporter.CiReporter.IsSupportMarkdown
}

type StdOutReporter struct{}

func (reporter *StdOutReporter) Report(report string, reportFormatting func(report string) string) (string, string, error) {
	log.Printf("Info: %v", report)
	return "", "", nil
}

func (reporter *StdOutReporter) Flush() (string, string, error) {
	return "", "", nil
}

func (reporter *StdOutReporter) SupportsMarkdown() bool {
	return false
}

func (reporter *StdOutReporter) Suppress() error {
	return nil
}

type ReportStrategy interface {
	Report(ciService orchestrator.PullRequestService, PrNumber int, report string, reportFormatter func(report string) string, supportsCollapsibleComment bool) (commentId string, commentUrl string, error error)
}

type CommentPerRunStrategy struct {
	IsPlan    bool
	Project   string
	TimeOfRun time.Time
}

func (strategy *CommentPerRunStrategy) Report(ciService orchestrator.PullRequestService, PrNumber int, report string, reportFormatter func(report string) string, supportsCollapsibleComment bool) (string, string, error) {
	comments, err := ciService.GetComments(PrNumber)
	if err != nil {
		return "", "", fmt.Errorf("error getting comments: %v", err)
	}

	var reportTitle string
	if strategy.Project != "" {
		if strategy.IsPlan {
			reportTitle = fmt.Sprintf("Plan for %v (%v)", strategy.Project, strategy.TimeOfRun.Format("2006-01-02 15:04:05 (MST)"))
		} else {
			reportTitle = fmt.Sprintf("Apply for %v (%v)", strategy.Project, strategy.TimeOfRun.Format("2006-01-02 15:04:05 (MST)"))
		}
	} else {
		reportTitle = "Digger run report at " + strategy.TimeOfRun.Format("2006-01-02 15:04:05 (MST)")
	}
	commentId, commentUrl, err := upsertComment(ciService, PrNumber, report, reportFormatter, comments, reportTitle, supportsCollapsibleComment)
	return commentId, commentUrl, err
}

func upsertComment(ciService orchestrator.PullRequestService, PrNumber int, report string, reportFormatter func(report string) string, comments []orchestrator.Comment, reportTitle string, supportsCollapsible bool) (string, string, error) {
	report = reportFormatter(report)
	var commentIdForThisRun interface{}
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

	if commentIdForThisRun == nil {
		var comment string
		if !supportsCollapsible {
			comment = utils.AsComment(reportTitle)(report)
		} else {
			comment = utils.AsCollapsibleComment(reportTitle, false)(report)
		}
		_, err := ciService.PublishComment(PrNumber, comment)
		if err != nil {
			return "", "", fmt.Errorf("error publishing comment: %v", err)
		}
		return "", "", nil
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

type LatestRunCommentStrategy struct {
	TimeOfRun time.Time
}

func (strategy *LatestRunCommentStrategy) Report(ciService orchestrator.PullRequestService, PrNumber int, report string, reportFormatter func(report string) string, supportsCollapsibleComment bool) (string, string, error) {
	comments, err := ciService.GetComments(PrNumber)
	if err != nil {
		return "", "", fmt.Errorf("error getting comments: %v", err)
	}

	reportTitle := "Digger latest run report"
	commentId, commentUrl, err := upsertComment(ciService, PrNumber, report, reportFormatter, comments, reportTitle, supportsCollapsibleComment)
	return commentId, commentUrl, err
}

type MultipleCommentsStrategy struct{}

func (strategy *MultipleCommentsStrategy) Report(ciService orchestrator.PullRequestService, PrNumber int, report string, reportFormatter func(report string) string, supportsCollapsibleComment bool) (string, string, error) {
	_, err := ciService.PublishComment(PrNumber, reportFormatter(report))
	return "", "", err
}
