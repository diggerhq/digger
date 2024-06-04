package orchestrator

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"reflect"
	"slices"
	"testing"
)

func TestAllFieldsInJobJsonAreAlsoInJob(t *testing.T) {
	spec := JobJson{}
	job := Job{}

	specVal := reflect.Indirect(reflect.ValueOf(spec))
	nFieldsSpec := specVal.Type().NumField()

	jobVal := reflect.Indirect(reflect.ValueOf(job))
	nFieldsJob := jobVal.Type().NumField()
	jobFileds := make([]string, 0)

	for j := 0; j < nFieldsJob; j++ {
		jobFileds = append(jobFileds, jobVal.Type().Field(j).Name)
	}

	fieldsToIgnore := []string{"Commit", "Branch", "JobType", "AwsRoleRegion", "StateRoleName", "CommandRoleName", "BackendHostname", "BackendOrganisationName", "BackendJobToken"}
	for i := 0; i < nFieldsSpec; i++ {
		field := specVal.Type().Field(i).Name
		if slices.Contains(fieldsToIgnore, field) {
			continue
		}
		assert.True(t, slices.Contains(jobFileds, field),
			"IMPORTANT: Please ensure all fields are correctly passed in serialization")
	}
}

func TestAllFieldsInJobAreAlsoInJobJson(t *testing.T) {
	spec := JobJson{}
	job := Job{}

	jobVal := reflect.Indirect(reflect.ValueOf(job))
	nFieldsJob := jobVal.Type().NumField()

	specVal := reflect.Indirect(reflect.ValueOf(spec))
	nFieldsSpec := specVal.Type().NumField()
	specFields := make([]string, 0)
	for j := 0; j < nFieldsSpec; j++ {
		specFields = append(specFields, specVal.Type().Field(j).Name)
	}

	fmt.Printf("%v ::\n", specFields)
	fieldsToIgnore := []string{"ProjectWorkflow", "StateEnvProvider", "CommandEnvProvider"}
	for i := 0; i < nFieldsJob; i++ {
		field := jobVal.Type().Field(i).Name
		if slices.Contains(fieldsToIgnore, field) {
			continue
		}
		assert.True(t, slices.Contains(specFields, field),
			"IMPORTANT: Please ensure all fields are correctly passed in serialization")
	}
}

func TestIsPlanForDiggerPlanJobCorrect(t *testing.T) {
	j := JobJson{
		ProjectName:      "project.Name",
		ProjectDir:       "project.Dir",
		ProjectWorkspace: "workspace",
		Terragrunt:       false,
		Commands:         []string{"run echo 'hello", "digger plan"},
		EventName:        "issue_comment",
	}
	assert.True(t, j.IsPlan())
	assert.False(t, j.IsApply())
}

func TestIsApplyForDiggerApplyJobCorrect(t *testing.T) {
	j := JobJson{
		ProjectName:      "project.Name",
		ProjectDir:       "project.Dir",
		ProjectWorkspace: "workspace",
		Terragrunt:       false,
		Commands:         []string{"digger apply"},
		EventName:        "issue_comment",
	}
	assert.True(t, j.IsApply())
	assert.False(t, j.IsPlan())
}
