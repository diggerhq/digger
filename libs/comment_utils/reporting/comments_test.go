package reporting

import (
	"strings"
	"testing"
)

func TestGetTerraformOutputAsCollapsibleComment_PercentInComment(t *testing.T) {
	// MySQL wildcard host 10.% in resource addresses should not be mangled
	formatter := GetTerraformOutputAsCollapsibleComment("Plan output", true)
	comment := `mysql_user.app["10.%"]: Refreshing state...`

	result := formatter(comment)

	if !strings.Contains(result, `10.%`) {
		t.Errorf("percent in comment was mangled: %s", result)
	}
	if strings.Contains(result, `MISSING`) || strings.Contains(result, `BADWIDTH`) {
		t.Errorf("fmt.Sprintf interpreted percent as format verb: %s", result)
	}
}

func TestGetTerraformOutputAsCollapsibleComment_PercentInSummary(t *testing.T) {
	formatter := GetTerraformOutputAsCollapsibleComment("Plan for project_100%_done", false)
	result := formatter("no changes")

	if !strings.Contains(result, "100%_done") {
		t.Errorf("percent in summary was mangled: %s", result)
	}
}

func TestGetTerraformOutputAsCollapsibleComment_OpenTag(t *testing.T) {
	formatter := GetTerraformOutputAsCollapsibleComment("Plan output", true)
	result := formatter("some plan")

	if !strings.Contains(result, `open="true"`) {
		t.Errorf("expected open tag in result: %s", result)
	}
	if !strings.Contains(result, "<summary>Plan output</summary>") {
		t.Errorf("expected summary in result: %s", result)
	}
	if !strings.Contains(result, "some plan") {
		t.Errorf("expected comment body in result: %s", result)
	}
}

func TestGetTerraformOutputAsCollapsibleComment_ClosedTag(t *testing.T) {
	formatter := GetTerraformOutputAsCollapsibleComment("Plan output", false)
	result := formatter("some plan")

	if strings.Contains(result, `open="true"`) {
		t.Errorf("did not expect open tag in result: %s", result)
	}
}

func TestAsCollapsibleComment_PercentInComment(t *testing.T) {
	formatter := AsCollapsibleComment("Instructions", false)
	comment := `override_special = "~!#%^&*()"`

	result := formatter(comment)

	if !strings.Contains(result, `#%^&`) {
		t.Errorf("percent in comment was mangled: %s", result)
	}
	if strings.Contains(result, `MISSING`) || strings.Contains(result, `BADWIDTH`) {
		t.Errorf("fmt.Sprintf interpreted percent as format verb: %s", result)
	}
}

func TestAsCollapsibleComment_PercentInSummary(t *testing.T) {
	formatter := AsCollapsibleComment("100% complete", true)
	result := formatter("details here")

	if !strings.Contains(result, "100% complete") {
		t.Errorf("percent in summary was mangled: %s", result)
	}
}

func TestAsCollapsibleComment_OpenTag(t *testing.T) {
	formatter := AsCollapsibleComment("Title", true)
	result := formatter("body")

	if !strings.Contains(result, `open="true"`) {
		t.Errorf("expected open tag in result: %s", result)
	}
	if !strings.Contains(result, "<summary>Title</summary>") {
		t.Errorf("expected summary in result: %s", result)
	}
	if !strings.Contains(result, "body") {
		t.Errorf("expected comment body in result: %s", result)
	}
}
