package iac_utils

import (
	"bufio"
	"encoding/json"
	"fmt"
	tfjson "github.com/hashicorp/terraform-json"
	"github.com/samber/lo"
	"log"
	"regexp"
	"strings"
)

// PulumiPreview represents the root structure of Pulumi preview JSON
type PulumiPreview struct {
	ChangeSummary ChangeSummary `json:"changeSummary"`
}

// ChangeSummary contains the summary of all changes
type ChangeSummary struct {
	Create int `json:"create"`
	Same   int `json:"same"`
	Update int `json:"update"`
	Delete int `json:"delete"`
}

// PreviewStats holds the statistics of resource changes
type PreviewStats struct {
	Created int
	Updated int
	Deleted int
	Total   int
}

type PulumiUtils struct{}

func parsePulumiPlanOutput(terraformJson string) (*tfjson.Plan, error) {
	var plan tfjson.Plan
	if err := json.Unmarshal([]byte(terraformJson), &plan); err != nil {
		return nil, fmt.Errorf("Unable to parse the plan file: %v", err)
	}

	return &plan, nil
}

func (tu PulumiUtils) GetSummaryFromPlanJson(planJson string) (bool, *IacSummary, error) {
	log.Printf("checking plan for json: %v", planJson)
	var preview PulumiPreview
	if err := json.Unmarshal([]byte(planJson), &preview); err != nil {
		return false, &IacSummary{}, err
	}

	summary := IacSummary{
		ResourcesCreated: uint(preview.ChangeSummary.Create),
		ResourcesUpdated: uint(preview.ChangeSummary.Update),
		ResourcesDeleted: uint(preview.ChangeSummary.Delete),
	}

	total := summary.ResourcesCreated + summary.ResourcesUpdated + summary.ResourcesDeleted
	isPlanEmpty := total == 0
	return isPlanEmpty, &summary, nil
}

func (tu PulumiUtils) GetSummaryFromApplyOutput(applyOutput string) (IacSummary, error) {
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

func (tu PulumiUtils) GetPlanFootprint(planJson string) (*IacPlanFootprint, error) {
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

func (tu PulumiUtils) PerformPlanSimilarityCheck(footprint1 IacPlanFootprint, footprint2 IacPlanFootprint) (bool, error) {
	return footprint1.hash() == footprint2.hash(), nil
}

func (tu PulumiUtils) SimilarityCheck(footprints []IacPlanFootprint) (bool, error) {
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

func (tu PulumiUtils) GetSummarizePlan(planJson string) (string, error) {
	// TODO: Implement me (equivalent of tfsummarize for pulumi)
	return "", nil
}
