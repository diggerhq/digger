package digger

import (
	"errors"
	"html"
	"strings"
	"testing"
	"time"

	"github.com/diggerhq/digger/libs/comment_utils/reporting"
	"github.com/stretchr/testify/require"
)

type planURLStorage struct {
	url                string
	err                error
	calls              int
	storedPlanFilePath string
	validFor           time.Duration
}

func (s *planURLStorage) StorePlanFile(fileContents []byte, artifactName string, storedPlanFilePath string) error {
	return nil
}

func (s *planURLStorage) RetrievePlan(localPlanFilePath string, artifactName string, storedPlanFilePath string) (*string, error) {
	return nil, nil
}

func (s *planURLStorage) DeleteStoredPlan(artifactName string, storedPlanFilePath string) error {
	return nil
}

func (s *planURLStorage) PlanExists(artifactName string, storedPlanFilePath string) (bool, error) {
	return true, nil
}

func (s *planURLStorage) StoredPlanUrl(storedPlanFilePath string, validFor time.Duration) (string, error) {
	s.calls++
	s.storedPlanFilePath = storedPlanFilePath
	s.validFor = validFor
	return s.url, s.err
}

type formattingReporter struct {
	supportsMarkdown bool
	formattedReports []string
}

func (r *formattingReporter) Report(report string, formatter func(string) string) (string, string, error) {
	r.formattedReports = append(r.formattedReports, formatter(report))
	return "", "", nil
}

func (r *formattingReporter) Flush() (string, string, error) {
	return "", "", nil
}

func (r *formattingReporter) SupportsMarkdown() bool {
	return r.supportsMarkdown
}

func (r *formattingReporter) Suppress() error {
	return nil
}

func TestReportTerraformPlanOutputAddsEscapedPresignedLink(t *testing.T) {
	const signedURL = "https://plans.example.com/acme.tfplan.txt?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Expires=3600&X-Amz-Signature=abc123"
	storage := &planURLStorage{url: signedURL}
	reporter := &formattingReporter{supportsMarkdown: true}

	reportTerraformPlanOutput(reporter, "project", "terraform plan body", storage, "acme.tfplan")

	require.Equal(t, 1, storage.calls)
	require.Equal(t, "acme.tfplan.txt", storage.storedPlanFilePath)
	require.Equal(t, time.Hour, storage.validFor)
	require.Len(t, reporter.formattedReports, 1)

	report := reporter.formattedReports[0]
	escapedURL := html.EscapeString(signedURL)
	require.Contains(t, report, `href="`+escapedURL+`"`)
	require.Contains(t, report, "full plan — valid for up to 1 hour")
	require.Contains(t, report, "terraform plan body")
	require.NotContains(t, report, `href="`+signedURL+`"`)

	hrefStart := strings.Index(report, `href="`) + len(`href="`)
	require.GreaterOrEqual(t, hrefStart, len(`href="`))
	hrefEnd := strings.Index(report[hrefStart:], `"`)
	require.GreaterOrEqual(t, hrefEnd, 0)
	require.Equal(t, signedURL, html.UnescapeString(report[hrefStart:hrefStart+hrefEnd]))
}

func TestReportTerraformPlanOutputUsesPlainURLWithoutMarkdown(t *testing.T) {
	const signedURL = "https://plans.example.com/acme.tfplan.txt?X-Amz-Expires=3600&X-Amz-Signature=abc123"
	storage := &planURLStorage{url: signedURL}
	reporter := &formattingReporter{}

	reportTerraformPlanOutput(reporter, "project", "terraform plan body", storage, "acme.tfplan")

	require.Len(t, reporter.formattedReports, 1)
	require.Contains(t, reporter.formattedReports[0], "Plan output - full plan: "+signedURL)
	require.NotContains(t, reporter.formattedReports[0], "&amp;")
}

func TestReportTerraformPlanOutputOmitsFailedLink(t *testing.T) {
	storage := &planURLStorage{
		url: "https://plans.example.com/dead-link",
		err: errors.New("could not sign plan"),
	}
	reporter := &formattingReporter{supportsMarkdown: true}

	reportTerraformPlanOutput(reporter, "project", "terraform plan body", storage, "acme.tfplan")

	require.Equal(t, 1, storage.calls)
	require.Len(t, reporter.formattedReports, 1)
	require.Contains(t, reporter.formattedReports[0], "<summary>Plan output</summary>")
	require.Contains(t, reporter.formattedReports[0], "terraform plan body")
	require.NotContains(t, reporter.formattedReports[0], "dead-link")
	require.NotContains(t, reporter.formattedReports[0], "full plan")
}

func TestReportTerraformPlanOutputSignsWhenLazyReporterFlushes(t *testing.T) {
	storage := &planURLStorage{url: "https://plans.example.com/acme.tfplan.txt?X-Amz-Expires=3600"}
	ciService := &MockPRManager{}
	lazyReporter := reporting.NewCiReporterLazy(reporting.CiReporter{
		CiService:         ciService,
		PrNumber:          42,
		IsSupportMarkdown: true,
		ReportStrategy:    &reporting.MultipleCommentsStrategy{},
	})

	reportTerraformPlanOutput(lazyReporter, "project", "terraform plan body", storage, "acme.tfplan")
	require.Zero(t, storage.calls, "the URL must not be signed while the report is buffered")

	_, _, err := lazyReporter.Flush()
	require.NoError(t, err)
	require.Equal(t, 1, storage.calls)
	require.Equal(t, "acme.tfplan.txt", storage.storedPlanFilePath)
	require.Equal(t, fullPlanLinkTTL, storage.validFor)
	require.Len(t, ciService.Commands, 1)
	require.Equal(t, "PublishComment", ciService.Commands[0].Command)
	require.Contains(t, ciService.Commands[0].Params, "full plan — valid for up to 1 hour")
}
