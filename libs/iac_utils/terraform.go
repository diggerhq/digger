package iac_utils

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/dineshba/tf-summarize/terraformstate"
	"github.com/dineshba/tf-summarize/writer"
	tfjson "github.com/hashicorp/terraform-json"
	"github.com/samber/lo"
	"regexp"
	"strings"
)

type TerraformUtils struct {
}

func parseTerraformPlanOutput(terraformJson string) (*tfjson.Plan, error) {
	var plan tfjson.Plan
	if err := json.Unmarshal([]byte(terraformJson), &plan); err != nil {
		return nil, fmt.Errorf("Unable to parse the plan file: %v", err)
	}

	return &plan, nil
}

func (tu TerraformUtils) GetSummaryFromPlanJson(planJson string) (bool, *IacSummary, error) {
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

	for _, change := range tfplan.OutputChanges {
		if len(change.Actions) != 1 || change.Actions[0] != "no-op" {
			isPlanEmpty = false
			break
		}
	}

	planSummary := IacSummary{}
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

func (tu TerraformUtils) GetSummaryFromApplyOutput(applyOutput string) (IacSummary, error) {
	scanner := bufio.NewScanner(strings.NewReader(applyOutput))
	var added, changed, destroyed uint = 0, 0, 0

	summaryRegex := regexp.MustCompile(`(\d+) added, (\d+) changed, (\d+) destroyed`)

	foundResourcesLine := false
	for scanner.Scan() {
		line := scanner.Text()
		if matches := summaryRegex.FindStringSubmatch(line); matches != nil {
			foundResourcesLine = true
			fmt.Sscanf(matches[1], "%d", &added)
			fmt.Sscanf(matches[2], "%d", &changed)
			fmt.Sscanf(matches[3], "%d", &destroyed)
		}
	}

	if !foundResourcesLine {
		return IacSummary{}, fmt.Errorf("could not find resources line in terraform apply output")
	}

	return IacSummary{
		ResourcesCreated: added,
		ResourcesUpdated: changed,
		ResourcesDeleted: destroyed,
	}, nil
}

func (tu TerraformUtils) GetPlanFootprint(planJson string) (*IacPlanFootprint, error) {
	tfplan, err := parseTerraformPlanOutput(planJson)
	if err != nil {
		return nil, err
	}
	planAddresses := lo.Map[*tfjson.ResourceChange, string](tfplan.ResourceChanges, func(change *tfjson.ResourceChange, idx int) string {
		return change.Address
	})
	footprint := IacPlanFootprint{
		Addresses: planAddresses,
	}
	return &footprint, nil
}

func (tu TerraformUtils) PerformPlanSimilarityCheck(footprint1 IacPlanFootprint, footprint2 IacPlanFootprint) (bool, error) {
	return footprint1.hash() == footprint2.hash(), nil
}

func (tu TerraformUtils) SimilarityCheck(footprints []IacPlanFootprint) (bool, error) {
	if len(footprints) < 2 {
		return true, nil
	}
	footprintHashes := lo.Map(footprints, func(footprint IacPlanFootprint, i int) string {
		return footprint.hash()
	})
	allSimilar := lo.EveryBy(footprintHashes, func(footprint string) bool {
		return footprint == footprintHashes[0]
	})
	return allSimilar, nil

}

func (tu TerraformUtils) GetSummarizePlan(planJson string) (string, error) {
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
