package reporting

import (
	"fmt"
	"github.com/diggerhq/digger/libs/ci"
	"github.com/diggerhq/digger/libs/comment_utils/utils"
	"log"
	"time"
)

type CiReporter struct {
	CiService         ci.PullRequestService
	PrNumber          int
	IsSupportMarkdown bool
	IsSuppressed      bool
	ReportStrategy    ReportStrategy
}

func (ciReporter CiReporter) Report(projectName string, report string, reportFormatting func(report string) string) error {
	err := ciReporter.ReportStrategy.Report(projectName, report, reportFormatting)
	return err
}

func (ciReporter CiReporter) Flush() ([]string, []string, error) {
	commentIds, commentUrls, err := ciReporter.ReportStrategy.Flush(ciReporter.CiService, ciReporter.PrNumber, ciReporter.IsSupportMarkdown)
	return commentIds, commentUrls, err
}

func (ciReporter CiReporter) Suppress(projectName string) error {
	return ciReporter.ReportStrategy.Suppress(projectName)
}

func (ciReporter CiReporter) SupportsMarkdown() bool {
	return ciReporter.IsSupportMarkdown
}

type StdOutReporter struct{}

func (reporter StdOutReporter) Report(projectName string, report string, reportFormatting func(report string) string) error {
	log.Printf("Info: %v", report)
	return nil
}

func (reporter StdOutReporter) Flush() ([]string, []string, error) {
	return []string{}, []string{}, nil
}

func (reporter StdOutReporter) SupportsMarkdown() bool {
	return false
}

func (reporter StdOutReporter) Suppress(string) error {
	return nil
}

type ReportStrategy interface {
	Report(projectName string, report string, reportFormatter func(report string) string) (error error)
	Flush(ciService ci.PullRequestService, PrNumber int, supportsCollapsibleComment bool) (commentId []string, commentUrl []string, error error)
	Suppress(projectName string) error
}

type ReportFormat struct {
	Report          string
	ReportFormatter func(report string) string
}

type SingleCommentStrategy struct {
	TimeOfRun  time.Time
	formatters map[string][]ReportFormat
}

func NewSingleCommentStrategy() SingleCommentStrategy {
	return SingleCommentStrategy{
		TimeOfRun:  time.Now(),
		formatters: make(map[string][]ReportFormat),
	}
}

func (strategy SingleCommentStrategy) Report(projectName string, report string, reportFormatter func(report string) string) error {
	if _, exists := strategy.formatters[projectName]; !exists {
		strategy.formatters[projectName] = []ReportFormat{}
	}
	strategy.formatters[projectName] = append(strategy.formatters[projectName], ReportFormat{
		Report:          report,
		ReportFormatter: reportFormatter,
	})

	return nil
}

func (strategy SingleCommentStrategy) Flush(ciService ci.PullRequestService, PrNumber int, supportsCollapsibleComment bool) ([]string, []string, error) {
	var completeComment = ""
	for projectName, projectFormatters := range strategy.formatters {
		projectTitle := "report for " + projectName + " " + strategy.TimeOfRun.Format("2006-01-02 15:04:05 (MST)")
		var projectComment = ""
		for _, f := range projectFormatters {
			report := f.ReportFormatter(f.Report)
			projectComment = projectComment + "\n" + report
		}
		if !supportsCollapsibleComment {
			projectComment = utils.AsComment(projectTitle)(projectComment)
		} else {
			projectComment = utils.AsCollapsibleComment(projectTitle, false)(projectComment)
		}
		completeComment = completeComment + "\n" + projectComment
	}

	c, err := ciService.PublishComment(PrNumber, completeComment)
	if err != nil {
		log.Printf("error while publishing reporter comment: %v", err)
		return nil, nil, fmt.Errorf("error while publishing reporter comment: %v", err)
	}
	return []string{c.Id}, []string{c.Url}, nil
}

func (strategy SingleCommentStrategy) Suppress(projectName string) error {
	// TODO: figure out how to suppress a particular project (probably pop it from the formatters map?)
	return nil
}

type MultipleCommentsStrategy struct {
	formatters map[string][]ReportFormat
	TimeOfRun  time.Time
}

func NewMultipleCommentsStrategy() MultipleCommentsStrategy {
	return MultipleCommentsStrategy{
		TimeOfRun:  time.Now(),
		formatters: make(map[string][]ReportFormat),
	}
}

func (strategy MultipleCommentsStrategy) Report(projectName string, report string, reportFormatter func(report string) string) error {
	if _, exists := strategy.formatters[projectName]; !exists {
		strategy.formatters[projectName] = []ReportFormat{}
	}
	strategy.formatters[projectName] = append(strategy.formatters[projectName], ReportFormat{
		Report:          report,
		ReportFormatter: reportFormatter,
	})

	return nil
}

func (strategy MultipleCommentsStrategy) Flush(ciService ci.PullRequestService, PrNumber int, supportsCollapsibleComment bool) ([]string, []string, error) {
	hasError := false
	var latestError error = nil
	commentIds := make([]string, 0)
	commentUrls := make([]string, 0)
	for projectName, projectFormatters := range strategy.formatters {
		projectTitle := "Digger run report for " + projectName + " " + strategy.TimeOfRun.Format("2006-01-02 15:04:05 (MST)")
		var projectComment = ""
		for _, f := range projectFormatters {
			report := f.ReportFormatter(f.Report)
			projectComment = projectComment + "\n" + report
		}
		if !supportsCollapsibleComment {
			projectComment = utils.AsComment(projectTitle)(projectComment)
		} else {
			projectComment = utils.AsCollapsibleComment(projectTitle, false)(projectComment)
		}
		c, err := ciService.PublishComment(PrNumber, projectComment)
		if err != nil {
			log.Printf("error while publishing reporter comment: %v", err)
			hasError = true
			latestError = err
			// append placeholders
			commentIds = append(commentIds, "0")
			commentUrls = append(commentUrls, "")

		} else {
			commentIds = append(commentIds, c.Id)
			commentUrls = append(commentUrls, c.Url)
		}
	}

	if hasError {
		log.Printf("could not publish all comments")
		return commentIds, commentUrls, latestError
	}

	return commentIds, commentUrls, nil
}

func (strategy MultipleCommentsStrategy) Suppress(projectName string) error {
	// TODO: figure out how to suppress a particular project (probably pop it from the formatters map?)
	return nil
}
