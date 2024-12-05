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

	// also checking for uninitialized case which is going to be the scenario when not specified in yaml file
	var ip []string
	var ep []string
	result = MatchIncludeExcludePatternsToFile("/projects/dev/test1", ip, ep)
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

func TestFilterPathsOutsideOfProjectPath(t *testing.T) {
	projectDir := "staging/aws/us-east-1/k8s"
	includePatterns := []string{"staging/aws/us-east-1/k8s/*.hcl", "staging/terragrunt-root.hcl vpc/*.tf*", "staging/aws/us-east-1/aws_region.tfvars", "staging/aws/aws_assume_role_arn.tfvars", "staging/aws/us-east-1/k8s/*.tf*"}
	res, _ := FilterPathsOutsideOfProjectPath(projectDir, includePatterns)
	assert.Equal(t, 2, len(res))
	assert.Equal(t, "staging/aws/us-east-1/k8s/*.hcl", res[0])
	assert.Equal(t, "staging/aws/us-east-1/k8s/*.tf*", res[1])

}
