package terraform_utils

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strings"

	"github.com/dineshba/tf-summarize/terraformstate"
	"github.com/dineshba/tf-summarize/writer"
	tfjson "github.com/hashicorp/terraform-json"

	"github.com/samber/lo"
)

type TerraformSummary struct {
	ResourcesCreated uint `json:"resources_created"`
	ResourcesUpdated uint `json:"resources_updated"`
	ResourcesDeleted uint `json:"resources_deleted"`
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

func (f *TerraformPlanFootprint) ToJson() map[string]interface{} {
	if f == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"addresses": f.Addresses,
	}
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

func (p *TerraformSummary) ToJson() map[string]interface{} {
	if p == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"resources_created": p.ResourcesCreated,
		"resources_updated": p.ResourcesUpdated,
		"resources_deleted": p.ResourcesDeleted,
	}
}
func parseTerraformPlanOutput(terraformJson string) (*tfjson.Plan, error) {
	var plan tfjson.Plan
	if err := json.Unmarshal([]byte(terraformJson), &plan); err != nil {
		return nil, fmt.Errorf("Unable to parse the plan file: %v", err)
	}

	return &plan, nil
}

func GetSummaryFromPlanJson(planJson string) (bool, *TerraformSummary, error) {
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

	if tfplan.OutputChanges != nil {
		isPlanEmpty = false
	}

	planSummary := TerraformSummary{}
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

func GetSummaryFromTerraformApplyOutput(applyOutput string) (TerraformSummary, error) {
	scanner := bufio.NewScanner(strings.NewReader(applyOutput))
	var added, changed, destroyed uint = 0, 0, 0

	summaryRegex := regexp.MustCompile(`(\d+) added, (\d+) changed, (\d+) destroyed`)

	foundResourcesLine := false
	for scanner.Scan() {
		line := scanner.Text()
		if matches := summaryRegex.FindStringSubmatch(line); matches != nil {
			log.Printf("matches found: %v", matches)
			foundResourcesLine = true
			fmt.Sscanf(matches[1], "%d", &added)
			fmt.Sscanf(matches[2], "%d", &changed)
			fmt.Sscanf(matches[3], "%d", &destroyed)
		}
	}

	if !foundResourcesLine {
		return TerraformSummary{}, fmt.Errorf("could not find resources line in terraform apply output")
	}

	log.Printf("finished scan of terraform output: %v", applyOutput)
	log.Printf("values found: %v %v %v", added, changed, destroyed)
	return TerraformSummary{
		ResourcesCreated: added,
		ResourcesUpdated: changed,
		ResourcesDeleted: destroyed,
	}, nil
}

func GetPlanFootprint(planJson string) (*TerraformPlanFootprint, error) {
	tfplan, err := parseTerraformPlanOutput(planJson)
	if err != nil {
		return nil, err
	}
	planAddresses := lo.Map[*tfjson.ResourceChange, string](tfplan.ResourceChanges, func(change *tfjson.ResourceChange, idx int) string {
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

func GetTfSummarizePlan(planJson string) (string, error) {
	plan := tfjson.Plan{}
	err := json.Unmarshal([]byte(planJson), &plan)
	if err != nil {
		return "", err
	}

	buf := new(bytes.Buffer)
	w := writer.NewTableWriter(terraformstate.GetAllResourceChanges(plan), terraformstate.GetAllOutputChanges(plan), true)
	w.Write(buf)

	return buf.String(), nil
}
