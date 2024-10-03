package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/diggerhq/digger/backend/ci_backends"
	"github.com/diggerhq/digger/ee/drift/dbmodels"
	services2 "github.com/diggerhq/digger/ee/drift/services"
	"github.com/diggerhq/digger/ee/drift/utils"
	"github.com/diggerhq/digger/libs/ci/generic"
	dg_configuration "github.com/diggerhq/digger/libs/digger_config"
	"github.com/diggerhq/digger/libs/scheduler"
	"github.com/diggerhq/digger/libs/spec"
	utils2 "github.com/diggerhq/digger/next/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
			ReportingStrategy:     "noop",
			ReportTerraformOutput: true,
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
			CommentUpdaterType: "noop",
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

	_, err = dbmodels.DB.CreateCiJobFromSpec(spec, *runName, project.ID)
	if err != nil {
		log.Printf("error creating the job: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error creating job entry")})
		return
	}

	err = ciBackend.TriggerWorkflow(spec, *runName, *vcsToken)
	if err != nil {
		log.Printf("TriggerWorkflow err: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Trigger workflow error")})
		return
	}

}

func (mc MainController) ProcessAllDrift(c *gin.Context) {
	diggerHostname := os.Getenv("DIGGER_HOSTNAME")
	webhookSecret := os.Getenv("DIGGER_WEBHOOK_SECRET")
	orgSettings, err := dbmodels.DB.Query.OrgSetting.Find()
	if err != nil {
		log.Printf("could not select all orgs: %v", err)
	}

	driftUrl, err := url.JoinPath(diggerHostname, "_internal/process_drift_for_org")
	log.Printf(driftUrl)
	if err != nil {
		log.Printf("could not form drift url: %v", err)
		c.JSON(500, gin.H{"error": "could not form drift url"})
		return
	}

	for _, orgSetting := range orgSettings {
		cron := orgSetting.Schedule
		matches, err := utils2.MatchesCrontab(cron, time.Now(), time.Hour)
		if err != nil {
			log.Printf("could not check matching crontab for org :%v", orgSetting.OrgID)
			continue
		}

		if matches {
			payload := DriftForOrgRequest{OrgId: orgSetting.OrgID}

			// Convert payload to JSON
			jsonPayload, err := json.Marshal(payload)
			if err != nil {
				fmt.Println("Process Drift: error marshaling JSON:", err)
				return
			}

			// Create a new request
			req, err := http.NewRequest("POST", driftUrl, bytes.NewBuffer(jsonPayload))
			if err != nil {
				fmt.Println("Process Drift: Error creating request:", err)
				return
			}

			// Set headers
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %v", webhookSecret))

			// Send the request
			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				fmt.Println("Error sending request:", err)
				return
			}
			defer resp.Body.Close()

			// Get the status code
			statusCode := resp.StatusCode
			if statusCode != 200 {
				log.Printf("got unexpected drift status for org: %v - status: %v", orgSetting.OrgID, statusCode)
			}
		}
	}

	c.String(200, "success")
}

type DriftForOrgRequest struct {
	OrgId string `json:"org_id"`
}

func (mc MainController) ProcessDriftForOrg(c *gin.Context) {
	diggerHostname := os.Getenv("DIGGER_HOSTNAME")
	webhookSecret := os.Getenv("DIGGER_WEBHOOK_SECRET")

	triggerDriftUrl, err := url.JoinPath(diggerHostname, "_internal/trigger_drift_for_project")
	if err != nil {
		log.Printf("could not form drift url: %v", err)
		c.JSON(500, gin.H{"error": "could not form drift url"})
		return
	}

	var request DriftForOrgRequest
	err = c.BindJSON(&request)
	if err != nil {
		log.Printf("Error binding JSON: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error binding JSON"})
		return
	}
	orgId := request.OrgId
	projects, err := dbmodels.DB.LoadProjectsForOrg(orgId)
	for _, project := range projects {
		if project.DriftEnabled {
			projectId := project.ID

			payload := TriggerDriftRunRequest{ProjectId: projectId}

			// Convert payload to JSON
			jsonPayload, err := json.Marshal(payload)
			if err != nil {
				fmt.Println("Process Drift: error marshaling JSON:", err)
				return
			}

			// Create a new request
			req, err := http.NewRequest("POST", triggerDriftUrl, bytes.NewBuffer(jsonPayload))
			if err != nil {
				fmt.Println("Process Drift: Error creating request:", err)
				return
			}

			// Set headers
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %v", webhookSecret))

			// Send the request
			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				fmt.Println("Error sending request:", err)
				return
			}
			defer resp.Body.Close()

			// Get the status code
			statusCode := resp.StatusCode
			if statusCode != 200 {
				log.Printf("got unexpected drift status for project: %v - status: %v", projectId, statusCode)
			}
		}

	}
	c.String(200, "success")
}
