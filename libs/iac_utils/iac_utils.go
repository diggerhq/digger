package iac_utils

import (
	"github.com/diggerhq/digger/libs/scheduler"
	"github.com/samber/lo"
	"sort"
)

type IacSummary struct {
	ResourcesCreated uint `json:"resources_created"`
	ResourcesUpdated uint `json:"resources_updated"`
	ResourcesDeleted uint `json:"resources_deleted"`
}

func (p *IacSummary) ToJson() map[string]interface{} {
	if p == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"resources_created": p.ResourcesCreated,
		"resources_updated": p.ResourcesUpdated,
		"resources_deleted": p.ResourcesDeleted,
	}
}

// IacPlanFootprint represents a derivation of a terraform plan json that has
// any sensitive data stripped out. Used for performing operations such
// as plan similarity check
type IacPlanFootprint struct {
	Addresses []string `json:"addresses"`
}

func (f *IacPlanFootprint) ToJson() map[string]interface{} {
	if f == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"addresses": f.Addresses,
	}
}

func (footprint IacPlanFootprint) hash() string {
	addresses := make([]string, len(footprint.Addresses))
	copy(addresses, footprint.Addresses)
	sort.Strings(addresses)
	// concatenate all the addresses after sorting to form the hash
	return lo.Reduce(addresses, func(a string, b string, i int) string {
		return a + b
	}, "")
}

type IacUtils interface {
	GetSummaryFromPlanJson(planJson string) (bool, *IacSummary, error)
	GetSummaryFromApplyOutput(applyOutput string) (IacSummary, error)
	GetPlanFootprint(planJson string) (*IacPlanFootprint, error)
	PerformPlanSimilarityCheck(footprint1 IacPlanFootprint, footprint2 IacPlanFootprint) (bool, error)
	SimilarityCheck(footprints []IacPlanFootprint) (bool, error)
	GetSummarizePlan(planJson string) (string, error)
}

func GetIacUtilsIacType(iacType scheduler.IacType) IacUtils {
	if iacType == scheduler.IacTypePulumi {
		return PulumiUtils{}
	} else {
		return TerraformUtils{}
	}
}
