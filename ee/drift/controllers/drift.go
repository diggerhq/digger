package controllers

import (
	"fmt"
	"github.com/diggerhq/digger/backend/ci_backends"
	"github.com/diggerhq/digger/ee/drift/dbmodels"
	services2 "github.com/diggerhq/digger/ee/drift/services"
	"github.com/diggerhq/digger/ee/drift/utils"
	"github.com/diggerhq/digger/libs/ci/generic"
	dg_configuration "github.com/diggerhq/digger/libs/digger_config"
	"github.com/diggerhq/digger/libs/scheduler"
	"github.com/diggerhq/digger/libs/spec"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"log"
	"net/http"
	"os"
	"strconv"
)

type TriggerDriftRunRequest struct {
	ProjectId string `json:"project_id"`
}

func (mc MainController) TriggerDriftRunForProject(c *gin.Context) {
	var request TriggerDriftRunRequest
	err := c.BindJSON(&request)
	if err != nil {
		log.Printf("Error binding JSON: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error binding JSON"})
		return
	}
	projectId := request.ProjectId

	p := dbmodels.DB.Query.Project
	project, err := dbmodels.DB.Query.Project.Where(p.ID.Eq(projectId)).First()
	if err != nil {
		log.Printf("could not find project %v: %v", projectId, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "could not find project"})
		return
	}

	r := dbmodels.DB.Query.Repo
	repo, err := dbmodels.DB.Query.Repo.Where(r.ID.Eq(project.RepoID)).First()
	if err != nil {
		log.Printf("could not find repo: %v for project %v: %v", project.RepoID, project.ID, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "could not find repo"})
		return
	}

	orgId := repo.OrganisationID
	issueNumber := 0
	repoFullName := repo.RepoFullName
	repoOwner := repo.RepoOrganisation
	repoName := repo.RepoName
	githubAppId := repo.GithubAppID
	installationid := repo.GithubInstallationID
	installationid64, err := strconv.ParseInt(installationid, 10, 64)
	cloneUrl := repo.CloneURL
	branch := repo.DefaultBranch
	command := "digger plan"
	workflowFile := "digger_workflow.yml"

	if err != nil {
		log.Printf("could not convert installationID to int64 %v", installationid)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not prarse installation id"})
		return
	}

	_, _, config, _, err := utils.GetDiggerConfigForBranch(mc.GithubClientProvider, installationid64, repoFullName, repoOwner, repoName, cloneUrl, branch)
	if err != nil {
		log.Printf("Error loading digger config: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error loading digger config"})
		return
	}

	theProject := config.GetProject(project.Name)
	if theProject == nil {
		log.Printf("Could find project %v in digger yml", project.Name)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("could not find project %v in digger.yml", theProject)})
		return
	}
	projects := []dg_configuration.Project{*theProject}

	jobsForImpactedProjects, err := generic.CreateJobsForProjects(projects, command, "drift", repoFullName, "digger", config.Workflows, &issueNumber, nil, branch, branch)
	if err != nil {
		log.Printf("error converting digger project %v to job", project.Name, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("could not find project %v in digger.yml", theProject)})
		return
	}

	jobToken, err := dbmodels.DB.CreateDiggerJobToken(orgId)
	if err != nil {
		log.Printf("Error creating job token: %v %v", project.Name, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("error creating job token")})
		return
	}

	backendHostName := os.Getenv("DIGGER_HOSTNAME")
	jobSpec := scheduler.JobToJson(jobsForImpactedProjects[0], "plan", "digger", branch, "", jobToken.Value, backendHostName, *theProject)

	spec := spec.Spec{
		JobId:     uuid.NewString(),
		CommentId: "",
		Job:       jobSpec,
		Reporter: spec.ReporterSpec{
			ReportingStrategy: "noop",
		},
		Lock: spec.LockSpec{
			LockType: "noop",
		},
		Backend: spec.BackendSpec{
			BackendHostname:         jobSpec.BackendHostname,
			BackendOrganisationName: jobSpec.BackendOrganisationName,
			BackendJobToken:         jobSpec.BackendJobToken,
			BackendType:             "backend",
		},
		VCS: spec.VcsSpec{
			VcsType:      "noop",
			Actor:        "digger",
			RepoFullname: repoFullName,
			RepoOwner:    repoOwner,
			RepoName:     repoName,
			WorkflowFile: workflowFile,
		},
		Variables: make([]spec.VariableSpec, 0),
		Policy: spec.PolicySpec{
			PolicyType: "http",
		},
		CommentUpdater: spec.CommentUpdaterSpec{
			CommentUpdaterType: dg_configuration.CommentRenderModeBasic,
		},
	}

	runName, err := services2.GetRunNameFromJob(spec)
	if err != nil {
		log.Printf("Error creating ru name: %v %v", project.Name, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("error creating run name")})
		return
	}

	vcsToken, err := services2.GetVCSToken("github", repoFullName, repoOwner, repoName, installationid64, mc.GithubClientProvider)
	if err != nil {
		log.Printf("Error creating vcs token: %v %v", project.Name, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("error creating vcs token")})
		return
	}

	ciBackend, err := mc.CiBackendProvider.GetCiBackend(
		ci_backends.CiBackendOptions{
			GithubClientProvider: mc.GithubClientProvider,
			GithubInstallationId: installationid64,
			GithubAppId:          githubAppId,
			RepoName:             repoName,
			RepoOwner:            repoOwner,
			RepoFullName:         repoFullName,
		},
	)
	if err != nil {
		log.Printf("Error creating CI backend: %v %v", project.Name, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("error creating CI backend")})
		return

	}

	err = ciBackend.TriggerWorkflow(spec, *runName, *vcsToken)
	if err != nil {
		log.Printf("TriggerWorkflow err: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Trigger workflow error")})
		return
	}

	//job.Status = orchestrator_scheduler.DiggerJobTriggered
	//err = models.DB.UpdateDiggerJob(job)
	//if err != nil {
	//	log.Printf("failed to Update digger job state: %v\n", err)
	//	return err
	//}

}
