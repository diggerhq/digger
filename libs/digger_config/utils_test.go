package digger_config

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMatchIncludeExcludePatternsToFile(t *testing.T) {
	includePatterns := []string{"projects/dev/**/*"}
	excludePatterns := []string{"projects/dev/project"}
	result := MatchIncludeExcludePatternsToFile("/projects/dev/test1", includePatterns, excludePatterns)
	assert.Equal(t, true, result)

	result = MatchIncludeExcludePatternsToFile("/projects/dev/test/test1", includePatterns, excludePatterns)
	assert.Equal(t, true, result)

	result = MatchIncludeExcludePatternsToFile("/dev/test1", includePatterns, excludePatterns)
	assert.Equal(t, false, result)

	result = MatchIncludeExcludePatternsToFile("projects/dev/project", includePatterns, excludePatterns)
	assert.Equal(t, false, result)
}

func TestGetPatternsRelativeToRepo(t *testing.T) {
	projectDir := "myProject/terraform/environments/devel"
	includePatterns := []string{"../../*.tf*"}
	res, _ := GetPatternsRelativeToRepo(projectDir, includePatterns)
	assert.Equal(t, "myProject/terraform/*.tf*", res[0])

	projectDir = "myProject/terraform/environments/devel"
	includePatterns = []string{"*.tf"}
	res, _ = GetPatternsRelativeToRepo(projectDir, includePatterns)
	assert.Equal(t, "myProject/terraform/environments/devel/*.tf", res[0])

	projectDir = "myProject/terraform/environments/devel"
	includePatterns = []string{"*.hcl"}
	res, _ = GetPatternsRelativeToRepo(projectDir, includePatterns)
	assert.Equal(t, "myProject/terraform/environments/devel/*.hcl", res[0])

}
