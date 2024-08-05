package dbmodels

import (
	"github.com/diggerhq/digger/libs/spec"
	"github.com/diggerhq/digger/next/model"
)

func ToVariableSpec(v model.EncryptedEnvVar) spec.VariableSpec {
	return spec.VariableSpec{
		Name:     v.Name,
		Value:    v.EncryptedValue,
		IsSecret: v.IsSecret,
	}
}
