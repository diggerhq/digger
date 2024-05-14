package comment_updater

import (
	"encoding/json"
	"fmt"
	"github.com/diggerhq/digger/libs/digger_config"
	"github.com/diggerhq/digger/libs/orchestrator"
	"github.com/diggerhq/digger/libs/orchestrator/scheduler"
	"github.com/diggerhq/digger/libs/terraform_utils"
	"github.com/samber/lo"
)

type ProjectNameSourceDetail struct {
	ProjectName   string
	Source        string
	Job           scheduler.SerializedJob
	JobSpec       orchestrator.JobJson
	PlanFootPrint terraform_utils.TerraformPlanFootprint
}

type ModuleGroupingCommentUpdater struct {
}

func (b ModuleGroupingCommentUpdater) UpdateComment(jobs []scheduler.SerializedJob, prNumber int, prService orchestrator.PullRequestService, prCommentId int64) error {
	jobSpecs, err := scheduler.GetJobSpecs(jobs)
	if err != nil {
		return fmt.Errorf("could not get job specs: %v", err)
	}

	impactedSources := jobSpecs[0].ImpactedSources
	projectNameToJobMap, err := scheduler.JobsToProjectMap(jobs)
	if err != nil {
		return fmt.Errorf("could not convert jobs to map: %v", err)
	}

	projectNameToJobSpecMap, err := orchestrator.JobsSpecsToProjectMap(jobSpecs)
	if err != nil {
		return fmt.Errorf("could not convert jobs to map: %v", err)
	}

	projectNameToFootPrintMap := make(map[string]terraform_utils.TerraformPlanFootprint)
	for _, job := range jobs {
		var footprint terraform_utils.TerraformPlanFootprint
		err := json.Unmarshal(job.PlanFootprint, &footprint)
		if err != nil {
			return fmt.Errorf("Could not unmarshal footprint: %v", err)
		}
		projectNameToFootPrintMap[job.ProjectName] = footprint
	}

	groupsToProjectMap := ImpactedSourcesMapToGroupMapping(impactedSources, projectNameToJobMap, projectNameToJobSpecMap, projectNameToFootPrintMap)

	message := ":construction_worker: Jobs status:\n\n"
	for sourceLocation, projectSourceDetailList := range groupsToProjectMap {
		footprints := lo.Map(projectSourceDetailList, func(detail ProjectNameSourceDetail, i int) terraform_utils.TerraformPlanFootprint {
			return detail.PlanFootPrint
		})
		allSimilarInGroup, err := terraform_utils.SimilarityCheck(footprints)
		if err != nil {
			return fmt.Errorf("error performing similar check: %v", err)
		}

		message = message + fmt.Sprintf("# Group: %v (similar: %v)", sourceLocation, allSimilarInGroup)
		for _, projectSourceDetail := range projectSourceDetailList {
			job := projectSourceDetail.Job
			jobSpec := projectSourceDetail.JobSpec
			isPlan := jobSpec.IsPlan()
			message = message + fmt.Sprintf("<!-- PROJECTHOLDER %v -->\n", job.ProjectName)
			message = message + fmt.Sprintf("%v **%v** <a href='%v'>%v</a>%v\n", job.Status.ToEmoji(), job.ProjectName, *job.WorkflowRunUrl, job.Status.ToString(), job.ResourcesSummaryString(isPlan))
			message = message + fmt.Sprintf("<!-- PROJECTHOLDEREND %v -->\n", job.ProjectName)
		}

	}

	prService.EditComment(prNumber, prCommentId, message)
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
