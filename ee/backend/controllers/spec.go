package controllers

import (
	"encoding/json"
	"fmt"
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/libs/ci/generic"
	"github.com/diggerhq/digger/libs/digger_config"
	"github.com/diggerhq/digger/libs/scheduler"
	"github.com/diggerhq/digger/libs/spec"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"os"
	"strings"
)

type GetSpecPayload struct {
	RepoFullName  string `json:"repo_full_name"`
	Actor         string `json:"actor"`
	DefaultBranch string `json:"default_branch"`
	PrBranch      string `json:"pr_branch"`
	DiggerConfig  string `json:"digger_config"`
	Project       string `json:"project"`
}

func (d DiggerEEController) GetSpec(c *gin.Context) {
	var payload GetSpecPayload

	// Bind the JSON payload to the struct
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	repoFullName := payload.RepoFullName
	repoOwner, repoName, _ := strings.Cut(repoFullName, "/")
	workflowFile := "digger_workflow.yml"
	actor := payload.Actor
	commitSha := ""
	defaultBranch := payload.DefaultBranch
	prBranch := payload.PrBranch

	config := digger_config.DiggerConfig{}
	err := json.Unmarshal([]byte(payload.DiggerConfig), &config)
	if err != nil {
		c.String(500, fmt.Sprintf("could not parse digger config from payload: %v", err))
		return
	}

	project := digger_config.Project{}
	err = json.Unmarshal([]byte(payload.Project), &project)
	if err != nil {
		c.String(500, fmt.Sprintf("could not parse project from payload: %v", err))
		return
	}

	jobs, err := generic.CreateJobsForProjects([]digger_config.Project{project}, "digger plan", "manual", repoFullName, actor, map[string]digger_config.Workflow{}, nil, &commitSha, defaultBranch, prBranch)
	if err != nil {
		c.String(500, fmt.Sprintf("ncould not create jobs based on project: %v", err))
		return
	}
	job := jobs[0]

	org := models.Organisation{}

	jobToken, err := models.DB.CreateDiggerJobToken(org.ID)
	if err != nil {
		log.Printf("Error creating job token: %v %v", project.Name, err)
		c.String(500, fmt.Sprintf("error creating job token: %v", err))
		return
	}
	backendHostName := os.Getenv("HOSTNAME")

	jobSpec := scheduler.JobToJson(job, scheduler.DiggerCommandPlan, org.Name, prBranch, commitSha, jobToken.Value, backendHostName, project)

	spec := spec.Spec{
		//JobId: diggerJob.DiggerJobID,
		//CommentId: "",
		Job: jobSpec,
		Reporter: spec.ReporterSpec{
			ReportingStrategy: "comments_per_run",
			ReporterType:      "lazy",
		},
		Lock: spec.LockSpec{
			LockType: "noop",
		},
		Backend: spec.BackendSpec{
			BackendType: "noop",
		},
		VCS: spec.VcsSpec{
			VcsType:      "github",
			Actor:        actor,
			RepoFullname: repoFullName,
			RepoOwner:    repoOwner,
			RepoName:     repoName,
			WorkflowFile: workflowFile,
		},
		Policy: spec.PolicySpec{
			PolicyType: "http",
		},
	}

	specBytes, err := json.Marshal(spec)

	c.String(200, string(specBytes))
	return

}
