package terraform_utils

import (
	"encoding/json"
	"fmt"
	"github.com/samber/lo"
	"slices"
	"sort"
)

type PlanSummary struct {
	ResourcesCreated uint `json:"resources_created"`
	ResourcesUpdated uint `json:"resources_updated"`
	ResourcesDeleted uint `json:"resources_deleted"`
}

type TerraformPlan struct {
	ResourceChanges []ResourceChange `json:"resource_changes"`
}

type ResourceChange struct {
	Name       string `json:"name"`
	Address    string `json:"address"`
	Type       string `json:"type"`
	Change     Change `json:"change"`
	ChangeType string `json:"change_type"`
}

type Change struct {
	Actions []string `json:"actions"`
}

// TerraformPlanFootprint represents a derivation of a terraform plan json that has
// any sensitive data stripped out. Used for performing operations such
// as plan similarity check
type TerraformPlanFootprint struct {
	Addresses []string `json:"addresses"`
}

func (p *PlanSummary) ToJson() map[string]interface{} {
	if p == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"resources_created": p.ResourcesCreated,
		"resources_updated": p.ResourcesUpdated,
		"resources_deleted": p.ResourcesDeleted,
	}
}
func parseTerraformPlanOutput(terraformJson string) (*TerraformPlan, error) {
	var plan TerraformPlan
	if err := json.Unmarshal([]byte(terraformJson), &plan); err != nil {
		return nil, fmt.Errorf("Unable to parse the plan file: %v", err)
	}

	return &plan, nil
}

func GetPlanSummary(planJson string) (bool, *PlanSummary, error) {
	tfplan, err := parseTerraformPlanOutput(planJson)
	if err != nil {
		return false, nil, fmt.Errorf("Error while parsing json file: %v", err)
	}
	isPlanEmpty := true
	for _, change := range tfplan.ResourceChanges {
		if len(change.Change.Actions) != 1 || change.Change.Actions[0] != "no-op" {
			isPlanEmpty = false
			break
		}
	}

	planSummary := PlanSummary{}
	for _, resourceChange := range tfplan.ResourceChanges {
		switch resourceChange.Change.Actions[0] {
		case "create":
			planSummary.ResourcesCreated++
		case "delete":
			planSummary.ResourcesDeleted++
		case "update":
			planSummary.ResourcesUpdated++
		}
	}
	return isPlanEmpty, &planSummary, nil
}

func GetPlanFootprint(planJson string) (*TerraformPlanFootprint, error) {
	tfplan, err := parseTerraformPlanOutput(planJson)
	if err != nil {
		return nil, err
	}
	planAddresses := lo.Map[ResourceChange, string](tfplan.ResourceChanges, func(change ResourceChange, idx int) string {
		return change.Address
	})
	footprint := TerraformPlanFootprint{
		Addresses: planAddresses,
	}
	return &footprint, nil
}

func PerformPlanSimilarityCheck(footprint1 TerraformPlanFootprint, footprint2 TerraformPlanFootprint) (bool, error) {
	sort.Strings(footprint1.Addresses)
	sort.Strings(footprint2.Addresses)
	isSimilar := slices.Equal(footprint1.Addresses, footprint2.Addresses)
	return isSimilar, nil
}
