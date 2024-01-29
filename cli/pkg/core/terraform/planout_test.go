package terraform

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPlanOutputEmpty(t *testing.T) {
	emptyTerraformPlanJson := "{\"format_version\":\"1.1\",\"terraform_version\":\"1.4.6\",\"planned_values\":{\"root_module\":{\"resources\":[{\"address\":\"null_resource.test\",\"mode\":\"managed\",\"type\":\"null_resource\",\"name\":\"test\",\"provider_name\":\"registry.terraform.io/hashicorp/null\",\"schema_version\":0,\"values\":{\"id\":\"7587790946951100994\",\"triggers\":null},\"sensitive_values\":{}}]}},\"resource_changes\":[{\"address\":\"null_resource.test\",\"mode\":\"managed\",\"type\":\"null_resource\",\"name\":\"test\",\"provider_name\":\"registry.terraform.io/hashicorp/null\",\"change\":{\"actions\":[\"no-op\"],\"before\":{\"id\":\"7587790946951100994\",\"triggers\":null},\"after\":{\"id\":\"7587790946951100994\",\"triggers\":null},\"after_unknown\":{},\"before_sensitive\":{},\"after_sensitive\":{}}}],\"prior_state\":{\"format_version\":\"1.0\",\"terraform_version\":\"1.4.6\",\"values\":{\"root_module\":{\"resources\":[{\"address\":\"null_resource.test\",\"mode\":\"managed\",\"type\":\"null_resource\",\"name\":\"test\",\"provider_name\":\"registry.terraform.io/hashicorp/null\",\"schema_version\":0,\"values\":{\"id\":\"7587790946951100994\",\"triggers\":null},\"sensitive_values\":{}}]}}},\"configuration\":{\"provider_config\":{\"null\":{\"name\":\"null\",\"full_name\":\"registry.terraform.io/hashicorp/null\"}},\"root_module\":{\"resources\":[{\"address\":\"null_resource.test\",\"mode\":\"managed\",\"type\":\"null_resource\",\"name\":\"test\",\"provider_config_key\":\"null\",\"schema_version\":0}]}}}\n"
	isEmpty, _, err := GetPlanSummary(emptyTerraformPlanJson)
	assert.Nil(t, err)
	assert.True(t, isEmpty)
}

func TestPlanOutputNonEmpty(t *testing.T) {
	nonEmptyTerraformPlanJson := "{\"format_version\":\"1.1\",\"terraform_version\":\"1.4.6\",\"planned_values\":{\"root_module\":{\"resources\":[{\"address\":\"null_resource.test\",\"mode\":\"managed\",\"type\":\"null_resource\",\"name\":\"test\",\"provider_name\":\"registry.terraform.io/hashicorp/null\",\"schema_version\":0,\"values\":{\"id\":\"7587790946951100994\",\"triggers\":null},\"sensitive_values\":{}},{\"address\":\"null_resource.testx\",\"mode\":\"managed\",\"type\":\"null_resource\",\"name\":\"testx\",\"provider_name\":\"registry.terraform.io/hashicorp/null\",\"schema_version\":0,\"values\":{\"triggers\":null},\"sensitive_values\":{}}]}},\"resource_changes\":[{\"address\":\"null_resource.test\",\"mode\":\"managed\",\"type\":\"null_resource\",\"name\":\"test\",\"provider_name\":\"registry.terraform.io/hashicorp/null\",\"change\":{\"actions\":[\"no-op\"],\"before\":{\"id\":\"7587790946951100994\",\"triggers\":null},\"after\":{\"id\":\"7587790946951100994\",\"triggers\":null},\"after_unknown\":{},\"before_sensitive\":{},\"after_sensitive\":{}}},{\"address\":\"null_resource.testx\",\"mode\":\"managed\",\"type\":\"null_resource\",\"name\":\"testx\",\"provider_name\":\"registry.terraform.io/hashicorp/null\",\"change\":{\"actions\":[\"create\"],\"before\":null,\"after\":{\"triggers\":null},\"after_unknown\":{\"id\":true},\"before_sensitive\":false,\"after_sensitive\":{}}}],\"prior_state\":{\"format_version\":\"1.0\",\"terraform_version\":\"1.4.6\",\"values\":{\"root_module\":{\"resources\":[{\"address\":\"null_resource.test\",\"mode\":\"managed\",\"type\":\"null_resource\",\"name\":\"test\",\"provider_name\":\"registry.terraform.io/hashicorp/null\",\"schema_version\":0,\"values\":{\"id\":\"7587790946951100994\",\"triggers\":null},\"sensitive_values\":{}}]}}},\"configuration\":{\"provider_config\":{\"null\":{\"name\":\"null\",\"full_name\":\"registry.terraform.io/hashicorp/null\"}},\"root_module\":{\"resources\":[{\"address\":\"null_resource.test\",\"mode\":\"managed\",\"type\":\"null_resource\",\"name\":\"test\",\"provider_config_key\":\"null\",\"schema_version\":0},{\"address\":\"null_resource.testx\",\"mode\":\"managed\",\"type\":\"null_resource\",\"name\":\"testx\",\"provider_config_key\":\"null\",\"schema_version\":0}]}}}\n"
	isEmpty, _, err := GetPlanSummary(nonEmptyTerraformPlanJson)
	assert.Nil(t, err)
	assert.False(t, isEmpty)
}

func TestPlanOutputInvalidJsonFailsGracefully(t *testing.T) {
	InvalidJson := "{\"format_version\":\" notsovalid"
	_, _, err := GetPlanSummary(InvalidJson)
	assert.NotNil(t, err)
}
