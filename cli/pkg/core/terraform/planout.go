package terraform

import (
	"encoding/json"
	"fmt"
)

type PlanSummary struct {
	ResourcesCreated int
	ResourcesUpdated int
	ResourcesDeleted int
}

type TerraformPlan struct {
	ResourceChanges []ResourceChange `json:"resource_changes"`
}

type ResourceChange struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Change     Change `json:"change"`
	ChangeType string `json:"change_type"`
}

type Change struct {
	Actions []string `json:"actions"`
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
		switch resourceChange.ChangeType {
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
