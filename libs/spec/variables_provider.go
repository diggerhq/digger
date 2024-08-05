package spec

import (
	"fmt"
	digger_crypto "github.com/diggerhq/digger/libs/crypto"
	"github.com/samber/lo"
	"os"
)

type VariablesProvider struct{}

func (p VariablesProvider) GetVariables(variables []VariableSpec) (map[string]string, error) {
	private_key := os.Getenv("DIGGER_PRIVATE_KEY")
	secrets := lo.Filter(variables, func(variable VariableSpec, i int) bool {
		return variable.IsSecret
	})
	if len(secrets) > 0 && private_key == "" {
		return nil, fmt.Errorf("digger private key not supplied, unable to decrypt secrets")
	}

	res := make(map[string]string)

	for _, v := range variables {
		if v.IsSecret {
			value, err := digger_crypto.DecryptValueUsingPrivateKey(v.Value, private_key)
			if err != nil {
				return nil, fmt.Errorf("could not decrypt value using private key: %v", err)
			}
			res[v.Name] = string(value)
		} else {
			res[v.Name] = v.Value
		}
	}

	return res, nil
}
