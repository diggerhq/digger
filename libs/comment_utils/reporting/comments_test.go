package reporting

import (
	"strings"
	"testing"
)

func TestGetTerraformOutputAsCollapsibleCommentPreservesPercentEncoding(t *testing.T) {
	summary := "Plan output (project%2Ftest)"
	comment := `resource "google_service_networking_connection" "psa_connection" {
	id = "projects%2Facme-production-network%2Fglobal%2Fnetworks%2Facme-production-network:servicenetworking.googleapis.com"
}`

	formatter := GetTerraformOutputAsCollapsibleComment(summary, true)
	result := formatter(comment)

	if strings.Contains(result, "%!(MISSING)") || strings.Contains(result, "%!!") {
		t.Errorf("Result contains corrupted percent specifier error: %s", result)
	}

	if !strings.Contains(result, "projects%2Facme-production-network%2Fglobal") {
		t.Errorf("Result missing original percent-encoded string: %s", result)
	}
}
