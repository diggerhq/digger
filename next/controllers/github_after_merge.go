package controllers

import (
	"encoding/json"
	"fmt"
	"github.com/diggerhq/digger/libs/ci/generic"
	dg_github "github.com/diggerhq/digger/libs/ci/github"
	dg_configuration "github.com/diggerhq/digger/libs/digger_config"
	"github.com/diggerhq/digger/libs/scheduler"
	"github.com/diggerhq/digger/next/dbmodels"
	"github.com/diggerhq/digger/next/services"
	nextutils "github.com/diggerhq/digger/next/utils"
	"github.com/google/go-github/v61/github"
	"log"
	"os"
	"strings"
)

func handlePushEventApplyAfterMerge(gh nextutils.GithubClientProvider, payload *github.PushEvent) error {
	installationId := *payload.Installation.ID
	repoName := *payload.Repo.Name
	repoFullName := *payload.Repo.FullName
	repoOwner := *payload.Repo.Owner.Login
	commitId := *payload.After
	requestedBy := *payload.Sender.Login
	ref := *payload.Ref
	targetBranch := strings.ReplaceAll(ref, "refs/heads/", "")
	backendHostName := os.Getenv("DIGGER_HOSTNAME")

	link, err := dbmodels.DB.GetGithubAppInstallationLink(installationId)
	if err != nil {
		log.Printf("Error getting GetGithubAppInstallationLink: %v", err)
		return fmt.Errorf("error getting github app link")
	}

	orgId := link.OrganizationID
	organization, err := dbmodels.DB.GetOrganisationById(orgId)

	orgName := organization.Title
	diggerRepoName := strings.ReplaceAll(repoFullName, "/", "-")
	repo, err := dbmodels.DB.GetRepo(orgId, diggerRepoName)
	if err != nil {
		log.Printf("Error getting Repo: %v", err)
		return fmt.Errorf("error getting github app link")
	}
	if repo == nil {
		log.Printf("Repo not found: Org: %v | repo: %v", orgId, diggerRepoName)
		return fmt.Errorf("Repo not found: Org: %v | repo: %v", orgId, diggerRepoName)
	}

	ghService, _, err := nextutils.GetGithubService(gh, installationId, repoFullName, repoOwner, repoName)
	if err != nil {
		log.Printf("Error getting github service: %v", err)
		return fmt.Errorf("error getting github service")
	}

	// ==== starting apply after merge part  =======

	p := dbmodels.DB.Query.Project
	projects, err := dbmodels.DB.Query.Project.Where(p.RepoID.Eq(repo.ID)).Find()

	var dgprojects = []dg_configuration.Project{}
	for _, proj := range projects {
		projectBranch := proj.Branch
		if targetBranch == projectBranch {
			dgprojects = append(dgprojects, dbmodels.ToDiggerProject(proj))
		}
	}
	projectsGraph, err := dg_configuration.CreateProjectDependencyGraph(dgprojects)
	workflows, err := services.GetWorkflowsForRepoAndBranch(gh, repo.ID, targetBranch, commitId)
	if err != nil {
		log.Printf("error getting workflows from config: %v", err)
		return fmt.Errorf("error getting workflows from config")
	}
	var config *dg_configuration.DiggerConfig = &dg_configuration.DiggerConfig{
		ApplyAfterMerge:   true,
		AllowDraftPRs:     false,
		CommentRenderMode: "",
		DependencyConfiguration: dg_configuration.DependencyConfiguration{
			Mode: dg_configuration.DependencyConfigurationHard,
		},
		PrLocks:                    false,
		Projects:                   dgprojects,
		AutoMerge:                  false,
		Telemetry:                  false,
		Workflows:                  workflows,
		MentionDriftedProjectsInPR: false,
		TraverseToNestedProjects:   false,
	}

	impactedProjects, _, _, _, err := dg_github.ProcessGitHubPushEvent(payload, config, projectsGraph, ghService)
	if err != nil {
		log.Printf("Error processing event: %v", err)
		return fmt.Errorf("error processing event")
	}
	log.Printf("GitHub IssueComment event processed successfully\n")

	// create 2 jobspecs (digger plan, digger apply) using commitID
	// TODO: find a way to get issue number from github api PushEvent
	// TODO: find a way to set the right PR branch
	issueNumber := 0

	planJobs, err := generic.CreateJobsForProjects(impactedProjects, "digger plan", "push", repoFullName, requestedBy, config.Workflows, &issueNumber, &commitId, targetBranch, targetBranch)
	if err != nil {
		log.Printf("Error creating jobs: %v", err)
		return fmt.Errorf("error creating jobs")
	}

	applyJobs, err := generic.CreateJobsForProjects(impactedProjects, "digger apply", "push", repoFullName, requestedBy, config.Workflows, &issueNumber, &commitId, targetBranch, targetBranch)
	if err != nil {
		log.Printf("Error creating jobs: %v", err)
		return fmt.Errorf("error creating jobs")
	}

	if len(planJobs) == 0 || len(applyJobs) == 0 {
		log.Printf("no projects impacated, succeeding")
		return nil
	}

	impactedProjectsMap := make(map[string]dg_configuration.Project)
	for _, p := range impactedProjects {
		impactedProjectsMap[p.Name] = p
	}

	impactedProjectsJobMap := make(map[string]scheduler.Job)
	for _, j := range planJobs {
		impactedProjectsJobMap[j.ProjectName] = j
	}

	for i, _ := range planJobs {
		planJob := planJobs[i]
		applyJob := applyJobs[i]
		projectName := planJob.ProjectName
		project, err := dbmodels.DB.GetProjectByName(orgId, repo, projectName)
		if err != nil {
			log.Printf("Error getting project: %v", err)
			return fmt.Errorf("error getting project")
		}

		planJobToken, err := dbmodels.DB.CreateDiggerJobToken(orgId)
		if err != nil {
			log.Printf("Error creating job token: %v %v", projectName, err)
			return fmt.Errorf("error creating job token")
		}

		planJobSpec, err := json.Marshal(scheduler.JobToJson(planJob, scheduler.DiggerCommandPlan, orgName, targetBranch, commitId, planJobToken.Value, backendHostName, impactedProjects[i]))
		if err != nil {
			log.Printf("Error creating jobspec: %v %v", projectName, err)
			return fmt.Errorf("error creating jobspec")

		}

		applyJobToken, err := dbmodels.DB.CreateDiggerJobToken(orgId)
		if err != nil {
			log.Printf("Error creating job token: %v %v", projectName, err)
			return fmt.Errorf("error creating job token")
		}

		applyJobSpec, err := json.Marshal(scheduler.JobToJson(applyJob, scheduler.DiggerCommandApply, orgName, targetBranch, commitId, applyJobToken.Value, backendHostName, impactedProjects[i]))
		if err != nil {
			log.Printf("Error creating jobs: %v %v", projectName, err)
			return fmt.Errorf("error creating jobs")
		}

		// create batches
		var commentId int64 = 0
		planBatch, err := dbmodels.DB.CreateDiggerBatch(orgId, dbmodels.DiggerVCSGithub, installationId, repoOwner, repoName, repoFullName, issueNumber, "", targetBranch, scheduler.DiggerCommandPlan, &commentId, 0, dbmodels.DiggerBatchMergeEvent)
		if err != nil {
			log.Printf("Error creating batch: %v", err)
			return fmt.Errorf("error creating batch")
		}

		applyBatch, err := dbmodels.DB.CreateDiggerBatch(orgId, dbmodels.DiggerVCSGithub, installationId, repoOwner, repoName, repoFullName, issueNumber, "", targetBranch, scheduler.DiggerCommandApply, &commentId, 0, dbmodels.DiggerBatchMergeEvent)
		if err != nil {
			log.Printf("Error creating batch: %v", err)
			return fmt.Errorf("error creating batch")
		}

		// create jobs
		_, err = dbmodels.DB.CreateDiggerJob(planBatch.ID, planJobSpec, impactedProjects[i].WorkflowFile)
		if err != nil {
			log.Printf("Error creating digger job: %v", err)
			return fmt.Errorf("error creating digger job")
		}

		_, err = dbmodels.DB.CreateDiggerJob(applyBatch.ID, applyJobSpec, impactedProjects[i].WorkflowFile)
		if err != nil {
			log.Printf("Error creating digger job: %v", err)
			return fmt.Errorf("error creating digger job")
		}

		// creating run stages
		planStage, err := dbmodels.DB.CreateDiggerRunStage(planBatch.ID)
		if err != nil {
			log.Printf("Error creating digger run stage: %v", err)
			return fmt.Errorf("error creating digger run stage")
		}

		applyStage, err := dbmodels.DB.CreateDiggerRunStage(applyBatch.ID)
		if err != nil {
			log.Printf("Error creating digger run stage: %v", err)
			return fmt.Errorf("error creating digger run stage")
		}

		diggerRun, err := dbmodels.DB.CreateDiggerRun("push", 0, dbmodels.RunQueued, commitId, "", installationId, repo.ID, project.ID, projectName, dbmodels.PlanAndApply, planStage.ID, applyStage.ID, nil)
		if err != nil {
			log.Printf("Error creating digger run: %v", err)
			return fmt.Errorf("error creating digger run")
		}

		dbmodels.DB.CreateDiggerRunQueueItem(diggerRun.ID, project.ID)

	}

	return nil
}
