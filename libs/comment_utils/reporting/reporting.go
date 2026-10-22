package reporting

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/diggerhq/digger/libs/ci"
)

type CiReporter struct {
	CiService         ci.PullRequestService
	PrNumber          int
	IsSupportMarkdown bool
	ReportStrategy    ReportStrategy
}

func (ciReporter CiReporter) Report(report string, reportFormatting func(report string) string) (string, string, error) {
	commentId, commentUrl, err := ciReporter.ReportStrategy.Report(ciReporter.CiService, ciReporter.PrNumber, report, reportFormatting, ciReporter.SupportsMarkdown())
	return commentId, commentUrl, err
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
		slog.Info("reporter is suppressed, ignoring messages")
		return "", "", nil
	}
	var commentId, commentUrl string
	for i := range lazyReporter.formatters {
		var err error
		commentId, commentUrl, err = lazyReporter.CiReporter.ReportStrategy.Report(lazyReporter.CiReporter.CiService, lazyReporter.CiReporter.PrNumber, lazyReporter.reports[i], lazyReporter.formatters[i], lazyReporter.SupportsMarkdown())
		if err != nil {
			slog.Error("failed to report strategy", "error", err)
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

func (reporter StdOutReporter) Report(report string, reportFormatting func(report string) string) (string, string, error) {
	slog.Info("report", "content", report)
	return "", "", nil
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
	Report(ciService ci.PullRequestService, PrNumber int, report string, reportFormatter func(report string) string, supportsCollapsibleComment bool) (commentId string, commentUrl string, error error)
}

type CommentPerRunStrategy struct {
	Title     string
	TimeOfRun time.Time
}

func (strategy CommentPerRunStrategy) Report(ciService ci.PullRequestService, PrNumber int, report string, reportFormatter func(report string) string, supportsCollapsibleComment bool) (string, string, error) {
	comments, err := ciService.GetComments(PrNumber)
	if err != nil {
		slog.Error("error getting comments", "error", err, "prNumber", PrNumber)
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
}

// CommentMaxSize returns the comment body size limit for the VCS behind
// ciService, falling back to GitHub's limit when the service does not say.
func CommentMaxSize(ciService ci.PullRequestService) int {
	if provider, ok := ciService.(ci.CommentMaxLengthProvider); ok {
		return provider.CommentMaxLength()
	}
	return defaultCommentMaxSize
}

// publishWrappedChunks splits report so that each chunk still fits the size
// limit after being wrapped with the report title, then publishes one comment
// per chunk. Returns the id and url of the last published comment, which
// holds the tail of the report (plan summary, warnings, errors).
func publishWrappedChunks(ciService ci.PullRequestService, PrNumber int, report string, wrap func(string) string, maxSize int) (string, string, error) {
	chunks := SplitComment(report, reserveUrlRoom(maxSize)-len(wrap("")), maxCommentsPerReport)
	var lastComment *ci.Comment
	for i, chunk := range chunks {
		if i > 0 {
			prevUrl := ""
			if lastComment != nil {
				prevUrl = lastComment.Url
			}
			chunk = fillPreviousCommentUrl(chunk, prevUrl)
		}
		comment, err := ciService.PublishComment(PrNumber, wrap(chunk))
		if err != nil {
			slog.Error("error publishing comment", "error", err, "prNumber", PrNumber)
			return "", "", fmt.Errorf("error publishing comment: %v", err)
		}
		lastComment = comment
	}
	// some PullRequestService implementations do not return the created comment
	if lastComment == nil {
		return "", "", nil
	}
	return fmt.Sprintf("%v", lastComment.Id), lastComment.Url, nil
}

func upsertComment(ciService ci.PullRequestService, PrNumber int, report string, reportFormatter func(report string) string, comments []ci.Comment, reportTitle string, supportsCollapsible bool) (string, string, error) {
	report = reportFormatter(report)
	maxSize := CommentMaxSize(ciService)

	var wrap func(string) string
	if !supportsCollapsible {
		wrap = AsComment(reportTitle)
	} else {
		wrap = AsCollapsibleComment(reportTitle, false)
	}

	// target the newest comment carrying the report title, so that once a
	// comment overflows and a continuation is created, subsequent reports
	// append to the continuation rather than the full older comment
	commentIdForThisRun := ""
	var commentBody string
	var commentUrl string
	for _, comment := range comments {
		if strings.Contains(*comment.Body, reportTitle) {
			commentIdForThisRun = comment.Id
			commentBody = *comment.Body
			commentUrl = comment.Url
		}
	}

	if commentIdForThisRun == "" {
		return publishWrappedChunks(ciService, PrNumber, report, wrap, maxSize)
	}

	// strip first and last lines
	lines := strings.Split(commentBody, "\n")
	lines = lines[1 : len(lines)-1]
	commentBody = strings.Join(lines, "\n")

	commentBody = commentBody + "\n\n" + report + "\n"

	completeComment := wrap(commentBody)

	if len(completeComment) > maxSize {
		// the merged comment would exceed the VCS limit: leave the existing
		// comment untouched and publish the new report as continuation
		// comment(s) under the same title
		slog.Info("comment size limit reached, publishing report as new comment",
			"commentId", commentIdForThisRun, "prNumber", PrNumber, "size", len(completeComment), "maxSize", maxSize)
		return publishWrappedChunks(ciService, PrNumber, report, wrap, maxSize)
	}

	err := ciService.EditComment(PrNumber, commentIdForThisRun, completeComment)

	if err != nil {
		slog.Error("error editing comment", "error", err, "commentId", commentIdForThisRun, "prNumber", PrNumber)
		return "", "", fmt.Errorf("error editing comment: %v", err)
	}
	return fmt.Sprintf("%v", commentIdForThisRun), commentUrl, nil
}

type LatestRunCommentStrategy struct {
	TimeOfRun time.Time
}

func (strategy LatestRunCommentStrategy) Report(ciService ci.PullRequestService, PrNumber int, report string, reportFormatter func(report string) string, supportsCollapsibleComment bool) (string, string, error) {
	comments, err := ciService.GetComments(PrNumber)
	if err != nil {
		slog.Error("error getting comments", "error", err, "prNumber", PrNumber)
		return "", "", fmt.Errorf("error getting comments: %v", err)
	}

	reportTitle := "Digger latest run report"
	commentId, commentUrl, err := upsertComment(ciService, PrNumber, report, reportFormatter, comments, reportTitle, supportsCollapsibleComment)
	return commentId, commentUrl, err
}

type MultipleCommentsStrategy struct{}

func (strategy MultipleCommentsStrategy) Report(ciService ci.PullRequestService, PrNumber int, report string, reportFormatter func(report string) string, supportsCollapsibleComment bool) (string, string, error) {
	chunks := SplitComment(reportFormatter(report), reserveUrlRoom(CommentMaxSize(ciService)), maxCommentsPerReport)
	var lastComment *ci.Comment
	for i, chunk := range chunks {
		if i > 0 {
			prevUrl := ""
			if lastComment != nil {
				prevUrl = lastComment.Url
			}
			chunk = fillPreviousCommentUrl(chunk, prevUrl)
		}
		comment, err := ciService.PublishComment(PrNumber, chunk)
		if err != nil {
			slog.Error("error publishing comment", "error", err, "prNumber", PrNumber)
			return "", "", err
		}
		lastComment = comment
	}
	// some PullRequestService implementations do not return the created comment
	if lastComment == nil {
		return "", "", nil
	}
	return lastComment.Id, lastComment.Url, nil
}
