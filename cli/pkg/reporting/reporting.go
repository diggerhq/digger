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
	CiService         orchestrator.PullRequestService
	PrNumber          int
	IsSupportMarkdown bool
	ReportStrategy    ReportStrategy
}

func (ciReporter *CiReporter) Report(report string, reportFormatter func(report string) string) error {
	return ciReporter.ReportStrategy.Report(ciReporter.CiService, ciReporter.PrNumber, report, reportFormatter, ciReporter.SupportsMarkdown())
}

func (ciReporter *CiReporter) Flush() error {
	return nil
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

func (lazyReporter *CiReporterLazy) Report(report string, reportFormatter func(report string) string) error {
	lazyReporter.reports = append(lazyReporter.reports, report)
	lazyReporter.formatters = append(lazyReporter.formatters, reportFormatter)
	return nil
}

func (lazyReporter *CiReporterLazy) Flush() error {
	if lazyReporter.isSuppressed {
		log.Printf("Reporter is suprresed, ignoring messages ...")
		return nil
	}
	for i, _ := range lazyReporter.formatters {
		err := lazyReporter.CiReporter.ReportStrategy.Report(lazyReporter.CiReporter.CiService, lazyReporter.CiReporter.PrNumber, lazyReporter.reports[i], lazyReporter.formatters[i], lazyReporter.SupportsMarkdown())
		if err != nil {
			log.Printf("failed to report strategy: ")
			return err
		}
	}
	return nil
}

func (lazyReporter *CiReporterLazy) Suppress() error {
	lazyReporter.isSuppressed = true
	return nil
}

func (lazyReporter *CiReporterLazy) SupportsMarkdown() bool {
	return lazyReporter.CiReporter.IsSupportMarkdown
}

type StdOutReporter struct{}

func (reporter *StdOutReporter) Report(report string, reportFormatter func(report string) string) error {
	log.Printf("Info: %v", report)
	return nil
}

func (reporter *StdOutReporter) Flush() error {
	return nil
}

func (reporter *StdOutReporter) SupportsMarkdown() bool {
	return false
}

func (reporter *StdOutReporter) Suppress() error {
	return nil
}

type ReportStrategy interface {
	Report(ciService orchestrator.PullRequestService, PrNumber int, report string, reportFormatter func(report string) string, supportsCollapsibleComment bool) error
}

type CommentPerRunStrategy struct {
	IsPlan    bool
	Project   string
	TimeOfRun time.Time
}

func (strategy *CommentPerRunStrategy) Report(ciService orchestrator.PullRequestService, PrNumber int, report string, reportFormatter func(report string) string, supportsCollapsibleComment bool) error {
	comments, err := ciService.GetComments(PrNumber)
	if err != nil {
		return fmt.Errorf("error getting comments: %v", err)
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
			comment = utils.AsCollapsibleComment(reportTitle, false)(report)
		}
		_, err := ciService.PublishComment(PrNumber, comment)
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
		completeComment = utils.AsCollapsibleComment(reportTitle, false)(commentBody)
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

func (strategy *LatestRunCommentStrategy) Report(ciService orchestrator.PullRequestService, prNumber int, comment string, commentFormatting func(comment string) string, supportsMarkdown bool) error {
	comments, err := ciService.GetComments(prNumber)
	if err != nil {
		return fmt.Errorf("error getting comments: %v", err)
	}

	reportTitle := "Digger latest run report"
	return upsertComment(ciService, prNumber, comment, commentFormatting, comments, reportTitle, supportsMarkdown)
}

type MultipleCommentsStrategy struct{}

func (strategy *MultipleCommentsStrategy) Report(ciService orchestrator.PullRequestService, PrNumber int, report string, formatter func(string) string, supportsMarkdown bool) error {
	_, err := ciService.PublishComment(PrNumber, formatter(report))
	return err
}
