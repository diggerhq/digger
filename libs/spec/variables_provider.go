package spec

import (
	"fmt"
	digger_crypto "github.com/diggerhq/digger/libs/crypto"
	"github.com/samber/lo"
	"os"
)

type VariablesProvider struct{}

func (p VariablesProvider) GetVariables(variables []VariableSpec) (map[string]map[string]string, error) {
	private_key := os.Getenv("DIGGER_PRIVATE_KEY")

	// Group variables by their stage
	stagedVariables := lo.GroupBy(variables, func(variable VariableSpec) string {
		return variable.Stage
	})

	result := make(map[string]map[string]string)

	for stage, vars := range stagedVariables {
		stageResult := make(map[string]string)

		// Filter variables into three categories
		secrets := lo.Filter(vars, func(variable VariableSpec, i int) bool {
			return variable.IsSecret
		})
		interpolated := lo.Filter(vars, func(variable VariableSpec, i int) bool {
			return variable.IsInterpolated
		})
		plain := lo.Filter(vars, func(variable VariableSpec, i int) bool {
			return !variable.IsSecret && !variable.IsInterpolated
		})

		// Check if private key is required for secrets
		if len(secrets) > 0 && private_key == "" {
			return nil, fmt.Errorf("digger private key not supplied, unable to decrypt secrets")
		}

		// Process secret variables
		for _, v := range secrets {
			value, err := digger_crypto.DecryptValueUsingPrivateKey(v.Value, private_key)
			if err != nil {
				return nil, fmt.Errorf("could not decrypt value using private key: %v", err)
			}
			stageResult[v.Name] = string(value)
		}

		// Process interpolated variables
		for _, v := range interpolated {
			stageResult[v.Name] = os.Getenv(v.Value)
		}

		// Process plain variables
		for _, v := range plain {
			stageResult[v.Name] = v.Value
		}

		// Add the processed variables for the current stage to the result
		result[stage] = stageResult
	}

	return result, nil
}
