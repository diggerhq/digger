package reporting

import (
	"encoding/json"
	"fmt"
	"github.com/diggerhq/digger/libs/comment_utils/utils"
	"github.com/diggerhq/digger/libs/digger_config"
	"github.com/diggerhq/digger/libs/orchestrator"
	"github.com/diggerhq/digger/libs/orchestrator/scheduler"
	"github.com/diggerhq/digger/libs/terraform_utils"
	"github.com/samber/lo"
	"log"
	"strconv"
)

type ProjectNameSourceDetail struct {
	ProjectName   string
	Source        string
	Job           scheduler.SerializedJob
	JobSpec       orchestrator.JobJson
	PlanFootPrint terraform_utils.TerraformPlanFootprint
}

type SourceGroupingReporter struct {
	Jobs      []scheduler.SerializedJob
	PrNumber  int
	PrService orchestrator.PullRequestService
}

func (r SourceGroupingReporter) UpdateComment(sourceDetails []SourceDetails, location string, terraformOutputs map[string]string) error {
	jobSpecs, err := scheduler.GetJobSpecs(r.Jobs)
	if err != nil {
		return fmt.Errorf("could not get job specs: %v", err)
	}

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

	projectNameToJobSpecMap, err := orchestrator.JobsSpecsToProjectMap(jobSpecs)
	if err != nil {
		return fmt.Errorf("could not convert jobs to map: %v", err)
	}

	projectNameToFootPrintMap := make(map[string]terraform_utils.TerraformPlanFootprint)
	for _, job := range r.Jobs {
		var footprint terraform_utils.TerraformPlanFootprint
		if job.PlanFootprint != nil {
			err := json.Unmarshal(job.PlanFootprint, &footprint)
			if err != nil {
				log.Printf("could not unmarshal footprint: %v", err)
				return fmt.Errorf("could not unmarshal footprint: %v", err)
			}
		} else {
			footprint = terraform_utils.TerraformPlanFootprint{}
		}
		projectNameToFootPrintMap[job.ProjectName] = footprint
	}

	footprints := lo.FilterMap(sourceDetaiItem.Projects, func(project string, i int) (terraform_utils.TerraformPlanFootprint, bool) {
		if projectNameToJobMap[project].Status == scheduler.DiggerJobSucceeded {
			return projectNameToFootPrintMap[project], true
		}
		return terraform_utils.TerraformPlanFootprint{}, false
	})
	allSimilarInGroup, err := terraform_utils.SimilarityCheck(footprints)
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
		jobSpec := projectNameToJobSpecMap[project]
		isPlan := jobSpec.JobType == orchestrator.DiggerCommandPlan
		expanded := i == 0 || !allSimilarInGroup
		var commenter func(terraformOutput string) string
		if isPlan {
			commenter = utils.GetTerraformOutputAsCollapsibleComment(fmt.Sprintf("Plan for %v", project), expanded)
		} else {
			commenter = utils.GetTerraformOutputAsCollapsibleComment(fmt.Sprintf("Apply for %v", project), false)
		}
		message = message + commenter(terraformOutputs[project]) + "\n"
	}

	CommentId, err := strconv.ParseInt(sourceDetaiItem.CommentId, 10, 64)
	if err != nil {
		log.Printf("Could not convert commentId to int64: %v", err)
		return fmt.Errorf("could not convert commentId to int64: %v", err)
	}
	r.PrService.EditComment(r.PrNumber, CommentId, message)
	return nil
}

// returns a map inverting locations
func ImpactedSourcesMapToGroupMapping(impactedSources map[string]digger_config.ProjectToSourceMapping, jobMapping map[string]scheduler.SerializedJob, jobSpecMapping map[string]orchestrator.JobJson, footprintsMap map[string]terraform_utils.TerraformPlanFootprint) map[string][]ProjectNameSourceDetail {

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
