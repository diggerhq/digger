package terraform_utils

import (
	"encoding/json"
	"fmt"
	"github.com/samber/lo"
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

func (footprint TerraformPlanFootprint) hash() string {
	addresses := make([]string, len(footprint.Addresses))
	copy(addresses, footprint.Addresses)
	sort.Strings(addresses)
	// concatenate all the addreses after sorting to form the hash
	return lo.Reduce(addresses, func(a string, b string, i int) string {
		return a + b
	}, "")
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
	return footprint1.hash() == footprint2.hash(), nil
}

func SimilarityCheck(footprints []TerraformPlanFootprint) (bool, error) {
	if len(footprints) < 2 {
		return true, nil
	}
	footprintHashes := lo.Map(footprints, func(footprint TerraformPlanFootprint, i int) string {
		return footprint.hash()
	})
	allSimilar := lo.EveryBy(footprintHashes, func(footprint string) bool {
		return footprint == footprintHashes[0]
	})
	return allSimilar, nil

}
