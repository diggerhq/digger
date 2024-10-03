package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/diggerhq/digger/libs/scheduler"
	"github.com/diggerhq/digger/next/ci_backends"
	"github.com/diggerhq/digger/next/dbmodels"
	"github.com/diggerhq/digger/next/services"
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

	orgId := repo.OrganizationID
	repoFullName := repo.RepoFullName
	repoOwner := repo.RepoOrganisation
	repoName := repo.RepoName

	appInstallation, err := dbmodels.DB.GetGithubAppInstallationByOrgAndRepo(orgId, repo.RepoFullName, dbmodels.GithubAppInstallActive)
	if err != nil {
		log.Printf("error retriving app installation")
		c.JSON(http.StatusBadRequest, gin.H{"error": "error retriving app installation"})
		return
	}
	installationId := appInstallation.GithubInstallationID

	ghService, _, err := utils.GetGithubService(d.GithubClientProvider, installationId, repoFullName, repoOwner, repoName)
	if err != nil {
		log.Printf("Error creating github service: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "error creating github service"})
		return

	}

	batchId, _, err := services.CreateJobAndBatchForProjectFromBranch(d.GithubClientProvider, projectId, "digger plan", dbmodels.DiggerBatchDriftEvent, scheduler.DiggerCommandPlan)

	ciBackend, err := d.CiBackendProvider.GetCiBackend(
		ci_backends.CiBackendOptions{
			GithubClientProvider: d.GithubClientProvider,
			GithubInstallationId: installationId,
			RepoName:             repoName,
			RepoOwner:            repoOwner,
			RepoFullName:         repoFullName,
		},
	)

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
		matches, err := utils.MatchesCrontab(proj.DriftCrontab, time.Now(), time.Minute)
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
