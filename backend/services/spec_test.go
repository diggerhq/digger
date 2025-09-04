// filepath: digger/backend/services/spec_test.go
package services

import (
	// "fmt"
	"github.com/diggerhq/digger/libs/spec"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestFindDuplicatesInStage_NoDuplicates(t *testing.T) {
	variablesSpec := []spec.VariableSpec{
		{Name: "VAR1"},
		{Name: "VAR2"},
		{Name: "VAR3"},
	}
	err := findDuplicatesInStage(variablesSpec, "test-stage")
	assert.Nil(t, err, "Expected no error when there are no duplicates")
}

func TestFindDuplicatesInStage_WithDuplicates(t *testing.T) {
	variablesSpec := []spec.VariableSpec{
		{Name: "VAR1"},
		{Name: "VAR2"},
		{Name: "VAR1"},
	}
	err := findDuplicatesInStage(variablesSpec, "test-stage")
	assert.NotNil(t, err, "Expected an error when duplicates are present")
	assert.Contains(t, err.Error(), "duplicate variable names found in 'test-stage' stage: VAR1")
}

func TestFindDuplicatesInStage_EmptyVariablesSpec(t *testing.T) {
	variablesSpec := []spec.VariableSpec{}
	err := findDuplicatesInStage(variablesSpec, "test-stage")
	assert.Nil(t, err, "Expected no error when variablesSpec is empty")
}

func TestFindDuplicatesInStage_SingleVariable(t *testing.T) {
	variablesSpec := []spec.VariableSpec{
		{Name: "VAR1"},
	}
	err := findDuplicatesInStage(variablesSpec, "test-stage")
	assert.Nil(t, err, "Expected no error when there is only one variable")
}
