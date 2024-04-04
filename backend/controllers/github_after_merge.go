package controllers

import (
	"encoding/json"
	"fmt"
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/utils"
	dg_configuration "github.com/diggerhq/digger/libs/digger_config"
	"github.com/diggerhq/digger/libs/orchestrator"
	dg_github "github.com/diggerhq/digger/libs/orchestrator/github"
	"github.com/diggerhq/digger/libs/orchestrator/scheduler"
	"github.com/gin-gonic/gin"
	"github.com/google/go-github/v58/github"
	"log"
	"net/http"
	"os"
	"path"
	"reflect"
	"strings"
)

func GithubAppWebHookAfterMerge(c *gin.Context) {
	c.Header("Content-Type", "application/json")
	gh := &utils.DiggerGithubRealClientProvider{}
	log.Printf("GithubAppWebHook")

	payload, err := github.ValidatePayload(c.Request, []byte(os.Getenv("GITHUB_WEBHOOK_SECRET")))
	if err != nil {
		log.Printf("Error validating github app webhook's payload: %v", err)
		c.String(http.StatusBadRequest, "Error validating github app webhook's payload")
		return
	}

	webhookType := github.WebHookType(c.Request)
	event, err := github.ParseWebHook(webhookType, payload)
	if err != nil {
		log.Printf("Failed to parse Github Event. :%v\n", err)
		c.String(http.StatusInternalServerError, "Failed to parse Github Event")
		return
	}

	log.Printf("github event type: %v\n", reflect.TypeOf(event))

	switch event := event.(type) {
	case *github.InstallationEvent:
		log.Printf("InstallationEvent, action: %v\n", *event.Action)
		if *event.Action == "created" {
			err := handleInstallationCreatedEvent(event)
			if err != nil {
				c.String(http.StatusInternalServerError, "Failed to handle webhook event.")
				return
			}
		}

		if *event.Action == "deleted" {
			err := handleInstallationDeletedEvent(event)
			if err != nil {
				c.String(http.StatusInternalServerError, "Failed to handle webhook event.")
				return
			}
		}
	case *github.InstallationRepositoriesEvent:
		log.Printf("InstallationRepositoriesEvent, action: %v\n", *event.Action)
		if *event.Action == "added" {
			err := handleInstallationRepositoriesAddedEvent(gh, event)
			if err != nil {
				c.String(http.StatusInternalServerError, "Failed to handle installation repo added event.")
			}
		}
		if *event.Action == "removed" {
			err := handleInstallationRepositoriesDeletedEvent(event)
			if err != nil {
				c.String(http.StatusInternalServerError, "Failed to handle installation repo deleted event.")
			}
		}

	case *github.IssueCommentEvent:
		log.Printf("IssueCommentEvent, action: %v  IN APPLY AFTER MERGE\n", *event.Action)
		if event.Sender.Type != nil && *event.Sender.Type == "Bot" {
			c.String(http.StatusOK, "OK")
			return
		}
		err := handleIssueCommentEvent(gh, event)
		if err != nil {
			log.Printf("handleIssueCommentEvent error: %v", err)
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
	case *github.PullRequestEvent:
		log.Printf("Got pull request event for %d  IN APPLY AFTER MERGE", *event.PullRequest.ID)
		err := handlePullRequestEvent(gh, event)
		if err != nil {
			log.Printf("handlePullRequestEvent error: %v", err)
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
	case *github.PushEvent:
		log.Printf("Got push event for %d", event.Repo.URL)
		err := handlePushEventApplyAfterMerge(gh, event)
		if err != nil {
			log.Printf("handlePushEvent error: %v", err)
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
	default:
		log.Printf("Unhandled event, event type %v", reflect.TypeOf(event))
	}

	c.JSON(200, "ok")
}

func handlePushEventApplyAfterMerge(gh utils.GithubClientProvider, payload *github.PushEvent) error {
	installationId := *payload.Installation.ID
	repoName := *payload.Repo.Name
	repoFullName := *payload.Repo.FullName
	repoOwner := *payload.Repo.Owner.Login
	cloneURL := *payload.Repo.CloneURL
	commitId := *payload.After
	requestedBy := *payload.Sender.Login
	ref := *payload.Ref
	defaultBranch := *payload.Repo.DefaultBranch

	if strings.HasSuffix(ref, defaultBranch) {
		link, err := models.DB.GetGithubAppInstallationLink(installationId)
		if err != nil {
			log.Printf("Error getting GetGithubAppInstallationLink: %v", err)
			return fmt.Errorf("error getting github app link")
		}

		orgId := link.OrganisationId
		diggerRepoName := strings.ReplaceAll(repoFullName, "/", "-")
		repo, err := models.DB.GetRepo(orgId, diggerRepoName)
		if err != nil {
			log.Printf("Error getting Repo: %v", err)
			return fmt.Errorf("error getting github app link")
		}
		if repo == nil {
			log.Printf("Repo not found: Org: %v | repo: %v", orgId, diggerRepoName)
			return fmt.Errorf("Repo not found: Org: %v | repo: %v", orgId, diggerRepoName)
		}

		service, token, err := utils.GetGithubService(gh, installationId, repoFullName, repoOwner, repoName)
		if err != nil {
			log.Printf("Error getting github service: %v", err)
			return fmt.Errorf("error getting github service")
		}
		utils.CloneGitRepoAndDoAction(cloneURL, defaultBranch, *token, func(dir string) error {
			dat, err := os.ReadFile(path.Join(dir, "digger.yml"))
			//TODO: fail here and return failure to main fn (need to refactor CloneGitRepoAndDoAction for that
			if err != nil {
				log.Printf("ERROR fetching digger.yml file: %v", err)
			}
			models.DB.UpdateRepoDiggerConfig(link.OrganisationId, string(dat), repo)
			return nil
		})

		// ==== starting apply after merge part  =======
		diggerYmlStr, ghService, config, projectsGraph, err := getDiggerConfigForBranch(gh, installationId, repoFullName, repoOwner, repoName, cloneURL, commitId)
		if err != nil {
			log.Printf("getDiggerConfigForPR error: %v", err)
			return fmt.Errorf("error getting digger config")
		}

		impactedProjects, requestedProject, _, err := dg_github.ProcessGitHubPushEvent(payload, config, projectsGraph, ghService)
		if err != nil {
			log.Printf("Error processing event: %v", err)
			return fmt.Errorf("error processing event")
		}
		log.Printf("GitHub IssueComment event processed successfully\n")

		// TODO: delete this line
		fmt.Sprintf(diggerYmlStr, impactedProjects, requestedProject, service)

		// create 2 jobspecs (digger plan, digger apply) using commitID
		planJobs, err := dg_github.CreateJobsForProjects(impactedProjects, "digger plan", "push", repoFullName, requestedBy, config.Workflows, nil, &commitId, defaultBranch, "")
		if err != nil {
			log.Printf("Error creating jobs: %v", err)
			return fmt.Errorf("error creating jobs")
		}

		applyJobs, err := dg_github.CreateJobsForProjects(impactedProjects, "digger apply", "push", repoFullName, requestedBy, config.Workflows, nil, &commitId, defaultBranch, "")
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

		impactedProjectsJobMap := make(map[string]orchestrator.Job)
		for _, j := range planJobs {
			impactedProjectsJobMap[j.ProjectName] = j
		}

		for i, _ := range planJobs {
			planJob := planJobs[i]
			applyJob := applyJobs[i]
			projectName := planJob.ProjectName
			planJobSpec, err := json.Marshal(orchestrator.JobToJson(planJob, impactedProjects[i]))
			if err != nil {
				log.Printf("Error creating jobspec: %v %v", projectName, err)
				return fmt.Errorf("error creating jobspec")

			}

			applyJobSpec, err := json.Marshal(orchestrator.JobToJson(applyJob, impactedProjects[i]))
			if err != nil {
				log.Printf("Error creating jobs: %v %v", projectName, err)
				return fmt.Errorf("error creating jobs")
			}
			// create batches
			planBatch, err := models.DB.CreateDiggerBatch(installationId, repoOwner, repoName, repoFullName, 0, diggerYmlStr, defaultBranch, scheduler.BatchTypePlan, nil)
			if err != nil {
				log.Printf("Error creating batch: %v", err)
				return fmt.Errorf("error creating batch")
			}

			applyBatch, err := models.DB.CreateDiggerBatch(installationId, repoOwner, repoName, repoFullName, 0, diggerYmlStr, defaultBranch, scheduler.BatchTypeApply, nil)
			if err != nil {
				log.Printf("Error creating batch: %v", err)
				return fmt.Errorf("error creating batch")
			}

			// create jobs
			_, err = models.DB.CreateDiggerJob(planBatch.ID, planJobSpec, impactedProjects[i].WorkflowFile)
			if err != nil {
				log.Printf("Error creating digger job: %v", err)
				return fmt.Errorf("error creating digger job")
			}

			_, err = models.DB.CreateDiggerJob(planBatch.ID, applyJobSpec, impactedProjects[i].WorkflowFile)
			if err != nil {
				log.Printf("Error creating digger job: %v", err)
				return fmt.Errorf("error creating digger job")
			}

			// creating run stages
			planStage, err := models.DB.CreateDiggerRunStage(planBatch.ID.String())
			if err != nil {
				log.Printf("Error creating digger run stage: %v", err)
				return fmt.Errorf("error creating digger run stage")
			}

			applyStage, err := models.DB.CreateDiggerRunStage(applyBatch.ID.String())
			if err != nil {
				log.Printf("Error creating digger run stage: %v", err)
				return fmt.Errorf("error creating digger run stage")
			}

			diggerRun, err := models.DB.CreateDiggerRun("push", 0, models.RunQueued, commitId, diggerYmlStr, installationId, repo.ID, projectName, models.PlanAndApply, &planStage.ID, &applyStage.ID)
			if err != nil {
				log.Printf("Error creating digger run: %v", err)
				return fmt.Errorf("error creating digger run")
			}

			models.DB.CreateDiggerRunQueueItem(diggerRun.ID)

		}

	}

	return nil
}
