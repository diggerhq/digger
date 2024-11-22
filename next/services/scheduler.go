package services

import (
	"encoding/json"
	"fmt"
	"github.com/diggerhq/digger/libs/ci/generic"
	dg_configuration "github.com/diggerhq/digger/libs/digger_config"
	orchestrator_scheduler "github.com/diggerhq/digger/libs/scheduler"
	"github.com/diggerhq/digger/next/ci_backends"
	"github.com/diggerhq/digger/next/dbmodels"
	"github.com/diggerhq/digger/next/model"
	"github.com/diggerhq/digger/next/utils"
	"github.com/dominikbraun/graph"
	"log"
	"os"
	"strconv"
)

func ScheduleJob(ciBackend ci_backends.CiBackend, repoFullname string, repoOwner string, repoName string, batchId string, job *model.DiggerJob, gh utils.GithubClientProvider) error {
	maxConcurrencyForBatch, err := strconv.Atoi(os.Getenv("MAX_DIGGER_CONCURRENCY_PER_BATCH"))
	if err != nil {
		log.Printf("WARN: could not get max concurrency for batch, setting it to 0: %v", err)
		maxConcurrencyForBatch = 0
	}
	if maxConcurrencyForBatch == 0 {
		// concurrency limits not set
		err := TriggerJob(gh, ciBackend, repoFullname, repoOwner, repoName, batchId, job)
		if err != nil {
			log.Printf("Could not trigger job: %v", err)
			return err
		}
	} else {
		// concurrency limits set
		log.Printf("Scheduling job with concurrency limit: %v per batch", maxConcurrencyForBatch)
		jobs, err := dbmodels.DB.GetDiggerJobsForBatchWithStatus(batchId, []orchestrator_scheduler.DiggerJobStatus{
			orchestrator_scheduler.DiggerJobTriggered,
			orchestrator_scheduler.DiggerJobStarted,
		})
		if err != nil {
			log.Printf("GetDiggerJobsForBatchWithStatus err: %v\n", err)
			return err
		}
		log.Printf("Length of jobs: %v", len(jobs))
		if len(jobs) >= maxConcurrencyForBatch {
			log.Printf("max concurrency for jobs reached: %v, queuing until more jobs succeed", len(jobs))
			job.Status = int16(orchestrator_scheduler.DiggerJobQueuedForRun)
			dbmodels.DB.UpdateDiggerJob(job)
			return nil
		} else {
			err := TriggerJob(gh, ciBackend, repoFullname, repoOwner, repoName, batchId, job)
			if err != nil {
				log.Printf("Could not trigger job: %v", err)
				return err
			}
		}
	}
	return nil
}

func TriggerJob(gh utils.GithubClientProvider, ciBackend ci_backends.CiBackend, repoFullname string, repoOwner string, repoName string, batchId string, job *model.DiggerJob) error {
	log.Printf("TriggerJob jobId: %v", job.DiggerJobID)

	if job.JobSpec == nil {
		log.Printf("Jobspec can't be nil")
		return fmt.Errorf("JobSpec is nil, skipping")
	}
	jobString := string(job.JobSpec)
	log.Printf("jobString: %v \n", jobString)

	runName, err := GetRunNameFromJob(*job)
	if err != nil {
		log.Printf("could not get run name: %v", err)
		return fmt.Errorf("could not get run name %v", err)
	}

	err = RefreshVariableSpecForJob(job)
	if err != nil {
		log.Printf("could not get variable spec from job: %v", err)
		return fmt.Errorf("could not get variable spec from job: %v", err)
	}

	err = dbmodels.DB.RefreshDiggerJobTokenExpiry(job)
	if err != nil {
		log.Printf("could not refresh job token expiry: %v", err)
		return fmt.Errorf("could not refresh job token from expiry: %v", err)
	}

	spec, err := GetSpecFromJob(*job)
	if err != nil {
		log.Printf("could not get spec: %v", err)
		return fmt.Errorf("could not get spec %v", err)
	}

	vcsToken, err := GetVCSTokenFromJob(*job, gh)
	if err != nil {
		log.Printf("could not get vcs token: %v", err)
		return fmt.Errorf("could not get vcs token: %v", err)
	}

	err = ciBackend.TriggerWorkflow(*spec, *runName, *vcsToken)
	if err != nil {
		log.Printf("TriggerJob err: %v\n", err)
		return err
	}

	job.Status = int16(orchestrator_scheduler.DiggerJobTriggered)
	err = dbmodels.DB.UpdateDiggerJob(job)
	if err != nil {
		log.Printf("failed to Update digger job state: %v\n", err)
		return err
	}

	return nil
}

func CreateJobAndBatchForProjectFromBranch(gh utils.GithubClientProvider, projectId string, command string, event dbmodels.BatchEventType, batchType orchestrator_scheduler.DiggerCommand) (*string, *string, error) {
	p := dbmodels.DB.Query.Project
	project, err := dbmodels.DB.Query.Project.Where(p.ID.Eq(projectId)).First()
	if err != nil {
		log.Printf("could not find project %v: %v", projectId, err)
		return nil, nil, fmt.Errorf("could not find project: %v", err)
	}

	branch := project.Branch

	r := dbmodels.DB.Query.Repo
	repo, err := dbmodels.DB.Query.Repo.Where(r.ID.Eq(project.RepoID)).First()
	if err != nil {
		log.Printf("could not find repo: %v for project %v: %v", project.RepoID, project.ID, err)
		return nil, nil, fmt.Errorf("could not find repo: %v for project %v: %v", project.RepoID, project.ID, err)
	}

	orgId := repo.OrganizationID
	repoFullName := repo.RepoFullName
	repoOwner := repo.RepoOrganisation
	repoName := repo.RepoName

	appInstallation, err := dbmodels.DB.GetGithubAppInstallationByOrgAndRepo(orgId, repo.RepoFullName, dbmodels.GithubAppInstallActive)
	if err != nil {
		log.Printf("error retrieving app installation")
		return nil, nil, fmt.Errorf("error retrieving app installation %v", err)
	}
	installationId := appInstallation.GithubInstallationID
	log.Printf("installation id is: %v", installationId)

	var dgprojects = []dg_configuration.Project{dbmodels.ToDiggerProject(project)}
	projectsGraph, err := dg_configuration.CreateProjectDependencyGraph(dgprojects)
	workflows, err := GetWorkflowsForRepoAndBranch(gh, repo.ID, branch, "")
	if err != nil {
		log.Printf("error retrieving digger.yml workflows: %v ", err)
		return nil, nil, fmt.Errorf("error retrieving digger.yml workflows:  %v", err)

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

	issueNumber := 0

	jobs, err := generic.CreateJobsForProjects(dgprojects, command, string(event), repoFullName, "digger", config.Workflows, &issueNumber, nil, branch, branch)
	if err != nil {
		log.Printf("Error creating jobs: %v", err)
		return nil, nil, fmt.Errorf("error creating jobs: %v", err)
	}

	impactedProjectsMap := make(map[string]dg_configuration.Project)
	for _, p := range dgprojects {
		impactedProjectsMap[p.Name] = p
	}

	impactedJobsMap := make(map[string]orchestrator_scheduler.Job)
	for _, j := range jobs {
		impactedJobsMap[j.ProjectName] = j
	}

	ghService, _, err := utils.GetGithubService(gh, installationId, repoFullName, repoOwner, repoName)
	if err != nil {
		log.Printf("Error creating github service: %v", err)
		return nil, nil, fmt.Errorf("error creating github service: %v", err)
	}

	commitSha, _, err := ghService.GetHeadCommitFromBranch(branch)

	batchId, _, err := ConvertJobsToDiggerJobs(batchType, dbmodels.DiggerVCSGithub, orgId, impactedJobsMap, impactedProjectsMap, projectsGraph, installationId, project.Branch, 0, repoOwner, repoName, repoFullName, commitSha, 0, "", 0, event)
	if err != nil {
		log.Printf("ConvertJobsToDiggerJobs error: %v", err)
		return nil, nil, fmt.Errorf("ConvertJobsToDiggerJobs error: %v", err)
	}

	return batchId, &commitSha, nil

}

func ConvertJobsToDiggerJobs(jobType orchestrator_scheduler.DiggerCommand, vcsType dbmodels.DiggerVCSType, organisationId string, jobsMap map[string]orchestrator_scheduler.Job, projectMap map[string]dg_configuration.Project, projectsGraph graph.Graph[string, dg_configuration.Project], githubInstallationId int64, branch string, prNumber int, repoOwner string, repoName string, repoFullName string, commitSha string, commentId int64, diggerConfigStr string, gitlabProjectId int, batchEventType dbmodels.BatchEventType) (*string, []*model.DiggerJob, error) {
	result := make([]*model.DiggerJob, 0)
	organisation, err := dbmodels.DB.GetOrganisationById(organisationId)
	if err != nil {
		log.Printf("Error getting organisation: %v %v", organisationId, err)
		return nil, nil, fmt.Errorf("error retrieving organisation")
	}
	organisationName := organisation.Title

	backendHostName := os.Getenv("DIGGER_HOSTNAME")

	log.Printf("Number of Jobs: %v\n", len(jobsMap))
	marshalledJobsMap := map[string][]byte{}
	for projectName, job := range jobsMap {
		jobToken, err := dbmodels.DB.CreateDiggerJobToken(organisationId)
		if err != nil {
			log.Printf("Error creating job token: %v %v", projectName, err)
			return nil, nil, fmt.Errorf("error creating job token")
		}

		marshalled, err := json.Marshal(orchestrator_scheduler.JobToJson(job, jobType, organisationName, branch, commitSha, jobToken.Value, backendHostName, projectMap[projectName]))
		if err != nil {
			return nil, nil, err
		}
		marshalledJobsMap[job.ProjectName] = marshalled
	}

	log.Printf("marshalledJobsMap: %v\n", marshalledJobsMap)

	batch, err := dbmodels.DB.CreateDiggerBatch(organisationId, vcsType, githubInstallationId, repoOwner, repoName, repoFullName, prNumber, diggerConfigStr, branch, jobType, &commentId, gitlabProjectId, batchEventType)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create batch: %v", err)
	}
	for pname, _ := range marshalledJobsMap {
		_, err := dbmodels.DB.CreateDiggerJob(batch.ID, marshalledJobsMap[pname], projectMap[pname].WorkflowFile)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create job: %v %v", pname, err)
		}
	}

	if err != nil {
		return nil, nil, err
	}

	return &batch.ID, result, nil
}
