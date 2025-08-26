package reporting

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/diggerhq/digger/libs/ci"
	"github.com/diggerhq/digger/libs/digger_config"
	"github.com/diggerhq/digger/libs/iac_utils"
	"github.com/diggerhq/digger/libs/scheduler"
	"github.com/samber/lo"
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
		slog.Error("location not found in sourcedetails list", "location", location)
		return fmt.Errorf("location not found in sourcedetails list")
	}

	projectNameToJobMap, err := scheduler.JobsToProjectMap(r.Jobs)
	if err != nil {
		slog.Error("could not convert jobs to map", "error", err)
		return fmt.Errorf("could not convert jobs to map: %v", err)
	}

	projectNameToFootPrintMap := make(map[string]iac_utils.IacPlanFootprint)
	for _, job := range r.Jobs {
		var footprint iac_utils.IacPlanFootprint
		if job.PlanFootprint != nil {
			err := json.Unmarshal(job.PlanFootprint, &footprint)
			if err != nil {
				slog.Error("could not unmarshal footprint",
					"error", err,
					"projectName", job.ProjectName)
				return fmt.Errorf("could not unmarshal footprint: %v", err)
			}
		} else {
			footprint = iac_utils.IacPlanFootprint{}
		}
		projectKey := scheduler.GetProjectAlias(job) // Use alias as key
		projectNameToFootPrintMap[projectKey] = footprint
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
		slog.Error("error performing similarity check",
			"error", err,
			"location", location)
		return fmt.Errorf("error performing similar check: %v", err)
	}

	message := ""
	message = message + fmt.Sprintf("# Group: %v (similar: %v)\n", location, allSimilarInGroup)

	slog.Info("generating comment for source location",
		"location", location,
		"projectCount", len(sourceDetaiItem.Projects),
		"allSimilarInGroup", allSimilarInGroup)

	for i, project := range sourceDetaiItem.Projects {
		job := projectNameToJobMap[project]
		if job.Status != scheduler.DiggerJobSucceeded {
			slog.Debug("skipping project with unsuccessful job",
				"project", project,
				"status", job.Status)
			continue
		}
		expanded := i == 0 || !allSimilarInGroup

		// Use alias for display but ensure we have the output
		projectAlias := scheduler.GetProjectAlias(job)
		commenter := GetTerraformOutputAsCollapsibleComment(fmt.Sprintf("Plan for %v", projectAlias), expanded)

		// Try to get output using alias first, fall back to project name if needed
		output, exists := terraformOutputs[projectAlias]
		if !exists {
			output, exists = terraformOutputs[job.ProjectName]
			if !exists {
				slog.Warn("No terraform output found for project",
					"projectAlias", projectAlias,
					"projectName", job.ProjectName)
				output = "No output available"
			} else {
				slog.Debug("Used project name fallback for terraform output",
					"projectAlias", projectAlias,
					"projectName", job.ProjectName)
			}
		}

		message = message + commenter(output) + "\n"
	}

	CommentId := sourceDetaiItem.CommentId
	if err != nil {
		slog.Error("could not convert commentId to int64",
			"error", err,
			"commentId", CommentId)
		return fmt.Errorf("could not convert commentId to int64: %v", err)
	}

	slog.Info("updating comment with plan details",
		"commentId", CommentId,
		"prNumber", r.PrNumber,
		"location", location)

	err = r.PrService.EditComment(r.PrNumber, CommentId, message)
	if err != nil {
		slog.Error("failed to edit comment",
			"error", err,
			"commentId", CommentId,
			"prNumber", r.PrNumber)
		return fmt.Errorf("failed to edit comment: %v", err)
	}

	return nil
}

// returns a map inverting locations
func ImpactedSourcesMapToGroupMapping(impactedSources map[string]digger_config.ProjectToSourceMapping, jobMapping map[string]scheduler.SerializedJob, jobSpecMapping map[string]scheduler.JobJson, footprintsMap map[string]iac_utils.IacPlanFootprint) map[string][]ProjectNameSourceDetail {
	slog.Debug("converting impacted sources to group mapping",
		"projectCount", len(impactedSources))

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

	slog.Debug("grouped sources by location",
		"originalCount", len(projectNameSourceList),
		"groupCount", len(res))

	return res
}
