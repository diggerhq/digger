package controllers

import (
	"encoding/json"
	"fmt"
	"github.com/dchest/uniuri"
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

func (d DiggerEEController) GetSpec(c *gin.Context) {
	var payload spec.GetSpecPayload

	// Bind the JSON payload to the struct
	if err := c.ShouldBindJSON(&payload); err != nil {
		log.Printf("could not bind json: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	repoFullName := payload.RepoFullName
	repoOwner, repoName, _ := strings.Cut(repoFullName, "/")
	command := payload.Command

	actor := payload.Actor
	commitSha := ""
	//defaultBranch := payload.DefaultBranch
	//prBranch := payload.PrBranch
	issueNumber := 000

	config := digger_config.DiggerConfig{}
	err := json.Unmarshal([]byte(payload.DiggerConfig), &config)
	if err != nil {
		log.Printf("could not parse digger config from payload: %v", err)
		c.String(500, fmt.Sprintf("could not parse digger config from payload: %v", err))
		return
	}

	project := digger_config.Project{}
	err = json.Unmarshal([]byte(payload.Project), &project)
	if err != nil {
		log.Printf("could not parse project from payload: %v", err)
		c.String(500, fmt.Sprintf("could not parse project from payload: %v", err))
		return
	}

	workflowFile := project.WorkflowFile

	jobs, err := generic.CreateJobsForProjects([]digger_config.Project{project}, command, "manual", repoFullName, actor, config.Workflows, &issueNumber, &commitSha, "", "")
	if err != nil {
		log.Printf("could not create jobs based on project: %v", err)
		c.String(500, fmt.Sprintf("could not create jobs based on project: %v", err))
		return
	}
	job := jobs[0]

	//temp  to get orgID TODO: fetch from db
	org, err := models.DB.GetOrganisation(models.DEFAULT_ORG_NAME)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to get default organisation")
		return
	}

	jobToken, err := models.DB.CreateDiggerJobToken(org.ID)
	if err != nil {
		log.Printf("Error creating job token: %v %v", project.Name, err)
		c.String(500, fmt.Sprintf("error creating job token: %v", err))
		return
	}
	backendHostName := os.Getenv("HOSTNAME")

	jobSpec := scheduler.JobToJson(job, scheduler.DiggerCommandPlan, org.Name, "", commitSha, jobToken.Value, backendHostName, project)

	spec := spec.Spec{
		SpecType: spec.SpecTypeManualJob,
		JobId:    uniuri.New(),
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
			BackendType:             "backend",
			BackendHostname:         jobSpec.BackendHostname,
			BackendJobToken:         jobSpec.BackendJobToken,
			BackendOrganisationName: jobSpec.BackendOrganisationName,
		},
		VCS: spec.VcsSpec{
			VcsType:                  "github",
			Actor:                    actor,
			RepoFullname:             repoFullName,
			RepoOwner:                repoOwner,
			RepoName:                 repoName,
			WorkflowFile:             workflowFile,
			GithubEnterpriseHostname: os.Getenv("DIGGER_GITHUB_HOSTNAME"),
		},
		Policy: spec.PolicySpec{
			PolicyType: "http",
		},
	}

	specBytes, err := json.Marshal(spec)

	log.Printf("specBytes: %v", spec)
	c.String(200, string(specBytes))
	return

}
