package dbmodels

import (
	"github.com/diggerhq/digger/libs/spec"
	"github.com/diggerhq/digger/next/model"
)

func ToVariableSpec(v model.EnvVar) spec.VariableSpec {
	return spec.VariableSpec{
		Name:     v.Name,
		Value:    v.Value,
		IsSecret: v.IsSecret,
	}
}
