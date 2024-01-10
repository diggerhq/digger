package terraform

import (
	"encoding/json"
	"fmt"
)

type TerraformPlan struct {
	ResourceChanges []ResourceChange `json:"resource_changes"`
}

type ResourceChange struct {
	Change Change `json:"change"`
}

type Change struct {
	Actions []string `json:"actions"`
}

func parseTerraformPlanOutput(terraformJson string) (*TerraformPlan, error) {

	var plan TerraformPlan
	if err := json.Unmarshal([]byte(terraformJson), &plan); err != nil {
		return nil, fmt.Errorf("Unable to parse the plan file: %v", err)
	}

	return &plan, nil
}

func IsPlanJsonPlanEmpty(planJson string) (bool, error) {
	tfplan, err := parseTerraformPlanOutput(planJson)
	if err != nil {
		return false, fmt.Errorf("Error while parsing json file: %v", err)
	}
	isPlanEmpty := true
	for _, change := range tfplan.ResourceChanges {
		if len(change.Change.Actions) != 1 || change.Change.Actions[0] != "no-op" {
			isPlanEmpty = false
			break
		}
	}

	return isPlanEmpty, nil
}
