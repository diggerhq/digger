// filepath: digger/backend/services/spec_test.go
package services

import (
	"os"
	"testing"

	"github.com/diggerhq/digger/libs/spec"
	"github.com/stretchr/testify/assert"
)

func TestFindDuplicatesInStage_NoDuplicates(t *testing.T) {
	variablesSpec := []spec.VariableSpec{
		{Name: "VAR1"},
		{Name: "VAR2"},
		{Name: "VAR3"},
	}
	err := findDuplicatesInStage(variablesSpec, "test-stage", "")
	assert.Nil(t, err, "Expected no error when there are no duplicates")
}

func TestFindDuplicatesInStage_WithDuplicates(t *testing.T) {
	variablesSpec := []spec.VariableSpec{
		{Name: "VAR1"},
		{Name: "VAR2"},
		{Name: "VAR1"},
	}
	err := findDuplicatesInStage(variablesSpec, "test-stage", "")
	assert.NotNil(t, err, "Expected an error when duplicates are present")
	assert.Contains(t, err.Error(), "duplicate variable names found in 'test-stage' stage: VAR1")
}

func TestFindDuplicatesInStage_EmptyVariablesSpec(t *testing.T) {
	variablesSpec := []spec.VariableSpec{}
	err := findDuplicatesInStage(variablesSpec, "test-stage", "")
	assert.Nil(t, err, "Expected no error when variablesSpec is empty")
}

func TestFindDuplicatesInStage_SingleVariable(t *testing.T) {
	variablesSpec := []spec.VariableSpec{
		{Name: "VAR1"},
	}
	err := findDuplicatesInStage(variablesSpec, "test-stage", "")
	assert.Nil(t, err, "Expected no error when there is only one variable")
}

// TestGetVariables_Success tests the GetVariables function for successful variable retrieval.
func TestGetVariables_Success(t *testing.T) {
	variablesSpec := []spec.VariableSpec{
		{Name: "VAR1", Value: "value1", Stage: "stage1"},
		{Name: "VAR2", Value: "value2", Stage: "stage1"},
		{Name: "VAR3", Value: "value3", Stage: "stage2"},
	}

	variablesProvider := spec.VariablesProvider{}
	variablesMap, err := variablesProvider.GetVariables(variablesSpec)

	assert.Nil(t, err, "Expected no error when retrieving variables")
	assert.Equal(t, "value1", variablesMap["stage1"]["VAR1"])
	assert.Equal(t, "value2", variablesMap["stage1"]["VAR2"])
	assert.Equal(t, "value3", variablesMap["stage2"]["VAR3"])
}

// TestGetVariables_MissingPrivateKey tests the GetVariables function when a private key is required but missing.
func TestGetVariables_MissingPrivateKey(t *testing.T) {
	variablesSpec := []spec.VariableSpec{
		{Name: "VAR1", Value: "encryptedValue", IsSecret: true, Stage: "stage1"},
	}

	variablesProvider := spec.VariablesProvider{}
	_, err := variablesProvider.GetVariables(variablesSpec)

	assert.NotNil(t, err, "Expected an error when private key is missing")
	assert.Contains(t, err.Error(), "digger private key not supplied, unable to decrypt secrets")
}

// TestGetVariables_DecryptError tests the GetVariables function when decryption fails.
func TestGetVariables_DecryptError(t *testing.T) {
	os.Setenv("DIGGER_PRIVATE_KEY", "invalidPrivateKey")
	defer os.Unsetenv("DIGGER_PRIVATE_KEY")

	variablesSpec := []spec.VariableSpec{
		{Name: "VAR1", Value: "encryptedValue", IsSecret: true, Stage: "stage1"},
	}

	variablesProvider := spec.VariablesProvider{}
	_, err := variablesProvider.GetVariables(variablesSpec)

	assert.NotNil(t, err, "Expected an error when decryption fails")
	assert.Contains(t, err.Error(), "could not decrypt value using private key")
}

// TestGetVariables_InterpolatedVariables tests the GetVariables function for interpolated variables.
func TestGetVariables_InterpolatedVariables(t *testing.T) {
	os.Setenv("INTERPOLATED_VAR", "interpolatedValue")
	defer os.Unsetenv("INTERPOLATED_VAR")

	variablesSpec := []spec.VariableSpec{
		{Name: "VAR1", Value: "INTERPOLATED_VAR", IsInterpolated: true, Stage: "stage1"},
	}

	variablesProvider := spec.VariablesProvider{}
	variablesMap, err := variablesProvider.GetVariables(variablesSpec)

	assert.Nil(t, err, "Expected no error when retrieving interpolated variables")
	assert.Equal(t, "interpolatedValue", variablesMap["stage1"]["VAR1"])
}

// TestGetVariables_EmptyVariables tests the GetVariables function with an empty variables spec.
func TestGetVariables_EmptyVariables(t *testing.T) {
	variablesSpec := []spec.VariableSpec{}

	variablesProvider := spec.VariablesProvider{}
	variablesMap, err := variablesProvider.GetVariables(variablesSpec)

	assert.Nil(t, err, "Expected no error when variablesSpec is empty")
	assert.Empty(t, variablesMap, "Expected variablesMap to be empty")
}
