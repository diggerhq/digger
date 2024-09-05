package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/diggerhq/digger/libs/ci/generic"
	dg_configuration "github.com/diggerhq/digger/libs/digger_config"
	orchestrator_scheduler "github.com/diggerhq/digger/libs/scheduler"
	"github.com/diggerhq/digger/next/ci_backends"
	"github.com/diggerhq/digger/next/dbmodels"
	"github.com/diggerhq/digger/next/utils"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"
)

type TriggerDriftRequest struct {
	ProjectId string `json:"project_id"`
}

func (d DiggerController) TriggerDriftDetectionForProject(c *gin.Context) {
	var request TriggerDriftRequest

	err := c.BindJSON(&request)

	if err != nil {
		log.Printf("Error binding JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload received"})
		return
	}
	projectId := request.ProjectId
	log.Printf("Drift requests for project: %v", projectId)

	p := dbmodels.DB.Query.Project
	project, err := dbmodels.DB.Query.Project.Where(p.ID.Eq(projectId)).First()
	if err != nil {
		log.Printf("could not find project %v: %v", projectId, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Could not find project"})
		return
	}

	r := dbmodels.DB.Query.Repo
	repo, err := dbmodels.DB.Query.Repo.Where(r.ID.Eq(project.RepoID)).First()
	if err != nil {
		log.Printf("could not find repo: %v for project %v: %v", project.RepoID, project.ID, err)
		c.JSON(500, gin.H{"error": fmt.Sprintf("could not find repo for project: %v: %v", project.ID, err)})
		return
	}

	orgId := repo.OrganizationID
	repoFullName := repo.RepoFullName
	repoOwner := repo.RepoOrganisation
	repoName := repo.RepoName

	appInstallation, err := dbmodels.DB.GetGithubAppInstallationByOrgAndRepo(orgId, repo.RepoFullName, dbmodels.GithubAppInstallActive)
	if err != nil {
		log.Printf("error retriving app installation")
		c.JSON(500, gin.H{"error": "app installation retrieval failed"})
		return
	}
	installationId := appInstallation.GithubInstallationID
	log.Printf("installation id is: %v", installationId)

	var dgprojects = []dg_configuration.Project{dbmodels.ToDiggerProject(project)}

	projectsGraph, err := dg_configuration.CreateProjectDependencyGraph(dgprojects)
	var config *dg_configuration.DiggerConfig = &dg_configuration.DiggerConfig{
		ApplyAfterMerge:   true,
		AllowDraftPRs:     false,
		CommentRenderMode: "",
		DependencyConfiguration: dg_configuration.DependencyConfiguration{
			Mode: dg_configuration.DependencyConfigurationHard,
		},
		PrLocks:   false,
		Projects:  dgprojects,
		AutoMerge: false,
		Telemetry: false,
		Workflows: map[string]dg_configuration.Workflow{
			"default": dg_configuration.Workflow{
				EnvVars: nil,
				Plan:    nil,
				Apply:   nil,
				Configuration: &dg_configuration.WorkflowConfiguration{
					OnPullRequestPushed:           []string{"digger plan"},
					OnPullRequestClosed:           []string{},
					OnPullRequestConvertedToDraft: []string{},
					OnCommitToDefault:             []string{},
				},
			},
		},
		MentionDriftedProjectsInPR: false,
		TraverseToNestedProjects:   false,
	}

	branch := project.Branch

	issueNumber := 0

	jobs, err := generic.CreateJobsForProjects(dgprojects, "digger apply", "drift-detection", repoFullName, "digger", config.Workflows, &issueNumber, nil, branch, branch)
	if err != nil {
		log.Printf("Error creating jobs: %v", err)
		c.JSON(500, gin.H{"error": "error creating jobs"})
		return
	}

	impactedProjectsMap := make(map[string]dg_configuration.Project)
	for _, p := range dgprojects {
		impactedProjectsMap[p.Name] = p
	}

	impactedJobsMap := make(map[string]orchestrator_scheduler.Job)
	for _, j := range jobs {
		impactedJobsMap[j.ProjectName] = j
	}

	batchId, _, err := ConvertJobsToDiggerJobs("digger plan", dbmodels.DiggerVCSGithub, orgId, impactedJobsMap, impactedProjectsMap, projectsGraph, installationId, project.Branch, 0, repoOwner, repoName, repoFullName, "", 0, "", 0, dbmodels.DiggerBatchDriftEvent)
	if err != nil {
		log.Printf("ConvertJobsToDiggerJobs error: %v", err)
		c.JSON(500, gin.H{"error": "could not convert digger jobs"})
		return
	}

	ciBackend, err := d.CiBackendProvider.GetCiBackend(
		ci_backends.CiBackendOptions{
			GithubClientProvider: d.GithubClientProvider,
			GithubInstallationId: installationId,
			RepoName:             repo.RepoName,
			RepoOwner:            repo.RepoOrganisation,
			RepoFullName:         repo.RepoFullName,
		},
	)
	if err != nil {
		log.Printf("GetCiBackend error: %v", err)
		c.JSON(500, gin.H{"error": "could not get CI backend"})
		return
	}

	ghService, _, err := utils.GetGithubService(d.GithubClientProvider, installationId, repoFullName, repoOwner, repoName)
	if err != nil {
		log.Printf("GetGithubService error: %v", err)
		c.JSON(500, gin.H{"error": "could not initialize github client"})
		return
	}

	err = TriggerDiggerJobs(ciBackend, repoFullName, repoOwner, repoName, *batchId, 0, ghService, d.GithubClientProvider)
	if err != nil {
		log.Printf("TriggerDiggerJobs error: %v", err)
		c.JSON(500, gin.H{"error": "could not trigger jobs"})
		return
	}

	c.JSON(200, gin.H{
		"status":     "successful",
		"project_id": projectId,
	})
	return

}

func (d DiggerController) TriggerCronForMatchingProjects(c *gin.Context) {
	webhookSecret := os.Getenv("DIGGER_WEBHOOK_SECRET")
	diggerHostName := os.Getenv("DIGGER_HOSTNAME")

	driftUrl, err := url.JoinPath(diggerHostName, "_internal/trigger_drift")
	if err != nil {
		log.Printf("could not form drift url: %v", err)
		c.JSON(500, gin.H{"error": "could not form drift url"})
		return
	}

	p := dbmodels.DB.Query.Project
	driftEnabledProjects, err := dbmodels.DB.Query.Project.Where(p.IsDriftDetectionEnabled.Is(true)).Find()
	if err != nil {
		log.Printf("could not fetch drift enabled projects: %v", err)
		c.JSON(500, gin.H{"error": "could not fetch drift enabled projects"})
		return
	}

	// TODO: think about pubsub pattern or parallelised calls
	for _, proj := range driftEnabledProjects {
		matches, err := utils.MatchesCrontab(proj.DriftCrontab, time.Now())
		if err != nil {
			log.Printf("could not check for matching crontab for project %v, %v", proj.ID, err)
			// TODO: send metrics here
			continue
		}

		if matches {
			payload := TriggerDriftRequest{ProjectId: proj.ID}

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
				log.Printf("got unexpected drift status for project: %v - status: %v", proj.ID, statusCode)
			}
		}
	}
}
