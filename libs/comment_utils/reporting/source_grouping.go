package reporting

import (
	"encoding/json"
	"fmt"
	"github.com/diggerhq/digger/libs/ci"
	"github.com/diggerhq/digger/libs/comment_utils/utils"
	"github.com/diggerhq/digger/libs/digger_config"
	"github.com/diggerhq/digger/libs/iac_utils"
	"github.com/diggerhq/digger/libs/scheduler"
	"github.com/samber/lo"
	"log"
)

type ProjectNameSourceDetail struct {
	ProjectName   string
	Source        string
	Job           scheduler.SerializedJob
	JobSpec       scheduler.JobJson
	PlanFootPrint iac_utils.IacPlanFootprint
}

type SourceGroupingReporter struct {
	Jobs      []scheduler.SerializedJob
	PrNumber  int
	PrService ci.PullRequestService
}

func (r SourceGroupingReporter) UpdateComment(sourceDetails []SourceDetails, location string, terraformOutputs map[string]string) error {

	sourceDetaiItem, found := lo.Find(sourceDetails, func(item SourceDetails) bool {
		return item.SourceLocation == location
	})

	if !found {
		log.Printf("location not found in sourcedetails list")
		return fmt.Errorf("location not found in sourcedetails list")
	}

	projectNameToJobMap, err := scheduler.JobsToProjectMap(r.Jobs)
	if err != nil {
		return fmt.Errorf("could not convert jobs to map: %v", err)
	}

	projectNameToFootPrintMap := make(map[string]iac_utils.IacPlanFootprint)
	for _, job := range r.Jobs {
		var footprint iac_utils.IacPlanFootprint
		if job.PlanFootprint != nil {
			err := json.Unmarshal(job.PlanFootprint, &footprint)
			if err != nil {
				log.Printf("could not unmarshal footprint: %v", err)
				return fmt.Errorf("could not unmarshal footprint: %v", err)
			}
		} else {
			footprint = iac_utils.IacPlanFootprint{}
		}
		projectNameToFootPrintMap[job.ProjectName] = footprint
	}

	// TODO: make it generic based on iac type
	iacUtils := iac_utils.TerraformUtils{}
	footprints := lo.FilterMap(sourceDetaiItem.Projects, func(project string, i int) (iac_utils.IacPlanFootprint, bool) {
		if projectNameToJobMap[project].Status == scheduler.DiggerJobSucceeded {
			return projectNameToFootPrintMap[project], true
		}
		return iac_utils.IacPlanFootprint{}, false
	})
	allSimilarInGroup, err := iacUtils.SimilarityCheck(footprints)
	if err != nil {
		return fmt.Errorf("error performing similar check: %v", err)
	}

	message := ""
	message = message + fmt.Sprintf("# Group: %v (similar: %v)\n", location, allSimilarInGroup)
	for i, project := range sourceDetaiItem.Projects {
		job := projectNameToJobMap[project]
		if job.Status != scheduler.DiggerJobSucceeded {
			continue
		}
		expanded := i == 0 || !allSimilarInGroup
		commenter := utils.GetTerraformOutputAsCollapsibleComment(fmt.Sprintf("Plan for %v", project), expanded)
		message = message + commenter(terraformOutputs[project]) + "\n"
	}

	CommentId := sourceDetaiItem.CommentId
	if err != nil {
		log.Printf("Could not convert commentId to int64: %v", err)
		return fmt.Errorf("could not convert commentId to int64: %v", err)
	}
	r.PrService.EditComment(r.PrNumber, CommentId, message)
	return nil
}

// returns a map inverting locations
func ImpactedSourcesMapToGroupMapping(impactedSources map[string]digger_config.ProjectToSourceMapping, jobMapping map[string]scheduler.SerializedJob, jobSpecMapping map[string]scheduler.JobJson, footprintsMap map[string]iac_utils.IacPlanFootprint) map[string][]ProjectNameSourceDetail {

	projectNameSourceList := make([]ProjectNameSourceDetail, 0)
	for projectName, locations := range impactedSources {
		for _, location := range locations.ImpactingLocations {
			projectNameSourceList = append(projectNameSourceList, ProjectNameSourceDetail{
				projectName,
				location,
				jobMapping[projectName],
				jobSpecMapping[projectName],
				footprintsMap[projectName],
			})
		}
	}
	res := lo.GroupBy(projectNameSourceList, func(t ProjectNameSourceDetail) string {
		return t.Source
	})
	return res
}
