package controllers

import (
	"fmt"
	"github.com/diggerhq/digger/backend/ci_backends"
	"github.com/diggerhq/digger/backend/locking"
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/segment"
	"github.com/diggerhq/digger/backend/utils"
	utils2 "github.com/diggerhq/digger/ee/backend/utils"
	comment_updater "github.com/diggerhq/digger/libs/comment_utils/reporting"
	dg_configuration "github.com/diggerhq/digger/libs/digger_config"
	dg_locking "github.com/diggerhq/digger/libs/locking"
	"github.com/diggerhq/digger/libs/orchestrator"
	dg_github "github.com/diggerhq/digger/libs/orchestrator/github"
	"github.com/gin-gonic/gin"
	"github.com/xanzy/go-gitlab"
	"io"
	"log"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"
)

type DiggerEEController struct {
	GitlabProvider    utils2.GitlabProvider
	CiBackendProvider ci_backends.CiBackendProvider
}

func (d DiggerEEController) GitlabWebHookHandler(c *gin.Context) {
	c.Header("Content-Type", "application/json")
	log.Printf("GitlabWebhook")

	gitlabWebhookSecret := os.Getenv("DIGGER_GITLAB_WEBHOOK_SECRET")
	secret := c.GetHeader("X-Gitlab-Token")
	log.Printf("%v, %v", secret, gitlabWebhookSecret)
	if gitlabWebhookSecret != secret {
		log.Printf("Error validating gitlab webhook payload: invalid signature")
		c.String(http.StatusBadRequest, "Error validating gitlab webhook payload: invalid signature")
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.String(http.StatusBadRequest, "Error reading request body", err)
		return
	}
	webhookType := gitlab.WebhookEventType(c.Request)
	event, err := gitlab.ParseHook(webhookType, body)
	if err != nil {
		log.Printf("Failed to parse gitlab Event. :%v\n", err)
		c.String(http.StatusInternalServerError, "Failed to parse gitlab Event")
		return
	}

	log.Printf("gitlab event type: %v\n", reflect.TypeOf(event))

	switch event := event.(type) {
	case *gitlab.MergeCommentEvent:
		log.Printf("IssueCommentEvent, action: %v \n", event.ObjectAttributes.Description)
		c.String(http.StatusOK, "OK2")
		err := handleIssueCommentEvent(d.GitlabProvider, event, d.CiBackendProvider, 000)
		if err != nil {
			log.Printf("handleIssueCommentEvent error: %v", err)
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
	case *gitlab.MergeEvent:
		log.Printf("Got pull request event for %d", event.Project.ID)
		//err := handlePullRequestEvent(gh, event, d.CiBackendProvider)
		//if err != nil {
		//	log.Printf("handlePullRequestEvent error: %v", err)
		//	c.String(http.StatusInternalServerError, err.Error())
		//	return
		//}
	case *gitlab.PushEvent:
		log.Printf("Got push event for %v %v", event.Project.URL, event.Ref)
		//err := handlePushEvent(gh, event)
		//if err != nil {
		//	log.Printf("handlePushEvent error: %v", err)
		//	c.String(http.StatusInternalServerError, err.Error())
		//	return
		//}
	default:
		log.Printf("Unhandled event, event type %v", reflect.TypeOf(event))
	}

	c.JSON(200, "ok")
}

func handleIssueCommentEvent(gitlabProvider utils2.GitlabProvider, payload *gitlab.MergeCommentEvent, ciBackendProvider ci_backends.CiBackendProvider, organisationId uint) error {
	repoName := payload.Project.Name
	repoOwner := payload.Repository.Name
	repoFullName := payload.Repository.PathWithNamespace
	cloneURL := payload.Repository.HTTPURL
	issueNumber := payload.MergeRequest.ID
	isDraft := payload.MergeRequest.WorkInProgress
	commentId := payload.ObjectAttributes.ID
	commentBody := payload.ObjectAttributes.Description

	if payload.ObjectAttributes.Action != gitlab.CommentEventActionCreate {
		log.Printf("comment is not of type 'created', ignoring")
		return nil
	}

	if !strings.HasPrefix(commentBody, "digger") {
		log.Printf("comment is not a Digger command, ignoring")
		return nil
	}

	diggerYmlStr, ghService, config, projectsGraph, branch, commitSha, err := getDiggerConfigForPR(gh, installationId, repoFullName, repoOwner, repoName, cloneURL, issueNumber)
	if err != nil {
		ghService, _, err := utils.GetGithubService(gh, installationId, repoFullName, repoOwner, repoName)
		if err != nil {
			log.Printf("GetGithubService error: %v", err)
			return fmt.Errorf("error getting ghService to post error comment")
		}
		utils.InitCommentReporter(ghService, issueNumber, fmt.Sprintf(":x: Could not load digger config, error: %v", err))
		log.Printf("getDiggerConfigForPR error: %v", err)
		return fmt.Errorf("error getting digger config")
	}

	err = ghService.CreateCommentReaction(commentId, string(dg_github.GithubCommentEyesReaction))
	if err != nil {
		log.Printf("CreateCommentReaction error: %v", err)
	}

	if !config.AllowDraftPRs && isDraft {
		log.Printf("AllowDraftPRs is disabled, skipping PR: %v", issueNumber)
		return nil
	}

	commentReporter, err := utils.InitCommentReporter(ghService, issueNumber, ":construction_worker: Digger starting....")
	if err != nil {
		log.Printf("Error initializing comment reporter: %v", err)
		return fmt.Errorf("error initializing comment reporter")
	}

	diggerCommand, err := orchestrator.GetCommandFromComment(*payload.Comment.Body)
	if err != nil {
		log.Printf("unkown digger command in comment: %v", *payload.Comment.Body)
		utils.InitCommentReporter(ghService, issueNumber, fmt.Sprintf(":x: Could not recognise comment, error: %v", err))
		return fmt.Errorf("unkown digger command in comment %v", err)
	}

	prBranchName, _, err := ghService.GetBranchName(issueNumber)
	if err != nil {
		log.Printf("GetBranchName error: %v", err)
		utils.InitCommentReporter(ghService, issueNumber, fmt.Sprintf(":x: GetBranchName error: %v", err))
		return fmt.Errorf("error while fetching branch name")
	}

	impactedProjects, impactedProjectsSourceMapping, requestedProject, _, err := dg_github.ProcessGitHubIssueCommentEvent(payload, config, projectsGraph, ghService)
	if err != nil {
		log.Printf("Error processing event: %v", err)
		utils.InitCommentReporter(ghService, issueNumber, fmt.Sprintf(":x: Error processing event: %v", err))
		return fmt.Errorf("error processing event")
	}
	log.Printf("GitHub IssueComment event processed successfully\n")

	// perform unlocking in backend
	if config.PrLocks {
		for _, project := range impactedProjects {
			prLock := dg_locking.PullRequestLock{
				InternalLock: locking.BackendDBLock{
					OrgId: organisationId,
				},
				CIService:        ghService,
				Reporter:         comment_updater.NoopReporter{},
				ProjectName:      project.Name,
				ProjectNamespace: repoFullName,
				PrNumber:         issueNumber,
			}
			err = PerformLockingActionFromCommand(prLock, *diggerCommand)
			if err != nil {
				utils.InitCommentReporter(ghService, issueNumber, fmt.Sprintf(":x: Failed perform lock action on project: %v %v", project.Name, err))
				return fmt.Errorf("failed perform lock action on project: %v %v", project.Name, err)
			}
		}
	}

	// if commands are locking or unlocking we don't need to trigger any jobs
	if *diggerCommand == orchestrator.DiggerCommandUnlock ||
		*diggerCommand == orchestrator.DiggerCommandLock {
		utils.InitCommentReporter(ghService, issueNumber, fmt.Sprintf(":white_check_mark: Command %v completed successfully", *diggerCommand))
		return nil
	}

	jobs, _, err := dg_github.ConvertGithubIssueCommentEventToJobs(payload, impactedProjects, requestedProject, config.Workflows, prBranchName)
	if err != nil {
		log.Printf("Error converting event to jobs: %v", err)
		utils.InitCommentReporter(ghService, issueNumber, fmt.Sprintf(":x: Error converting event to jobs: %v", err))
		return fmt.Errorf("error converting event to jobs")
	}
	log.Printf("GitHub IssueComment event converted to Jobs successfully\n")

	err = utils.ReportInitialJobsStatus(commentReporter, jobs)
	if err != nil {
		log.Printf("Failed to comment initial status for jobs: %v", err)
		utils.InitCommentReporter(ghService, issueNumber, fmt.Sprintf(":x: Failed to comment initial status for jobs: %v", err))
		return fmt.Errorf("failed to comment initial status for jobs")
	}

	if len(jobs) == 0 {
		log.Printf("no projects impacated, succeeding")
		// This one is for aggregate reporting
		err = utils.SetPRStatusForJobs(ghService, issueNumber, jobs)
		return nil
	}

	err = utils.SetPRStatusForJobs(ghService, issueNumber, jobs)
	if err != nil {
		log.Printf("error setting status for PR: %v", err)
		utils.InitCommentReporter(ghService, issueNumber, fmt.Sprintf(":x: error setting status for PR: %v", err))
		fmt.Errorf("error setting status for PR: %v", err)
	}

	impactedProjectsMap := make(map[string]dg_configuration.Project)
	for _, p := range impactedProjects {
		impactedProjectsMap[p.Name] = p
	}

	impactedProjectsJobMap := make(map[string]orchestrator.Job)
	for _, j := range jobs {
		impactedProjectsJobMap[j.ProjectName] = j
	}

	batchId, _, err := utils.ConvertJobsToDiggerJobs(*diggerCommand, orgId, impactedProjectsJobMap, impactedProjectsMap, projectsGraph, installationId, *branch, issueNumber, repoOwner, repoName, repoFullName, *commitSha, commentReporter.CommentId, diggerYmlStr)
	if err != nil {
		log.Printf("ConvertJobsToDiggerJobs error: %v", err)
		utils.InitCommentReporter(ghService, issueNumber, fmt.Sprintf(":x: ConvertJobsToDiggerJobs error: %v", err))
		return fmt.Errorf("error convertingjobs")
	}

	if config.CommentRenderMode == dg_configuration.CommentRenderModeGroupByModule &&
		(*diggerCommand == orchestrator.DiggerCommandPlan || *diggerCommand == orchestrator.DiggerCommandApply) {

		sourceDetails, err := comment_updater.PostInitialSourceComments(ghService, issueNumber, impactedProjectsSourceMapping)
		if err != nil {
			log.Printf("PostInitialSourceComments error: %v", err)
			utils.InitCommentReporter(ghService, issueNumber, fmt.Sprintf(":x: PostInitialSourceComments error: %v", err))
			return fmt.Errorf("error posting initial comments")
		}
		batch, err := models.DB.GetDiggerBatch(batchId)
		if err != nil {
			log.Printf("GetDiggerBatch error: %v", err)
			utils.InitCommentReporter(ghService, issueNumber, fmt.Sprintf(":x: PostInitialSourceComments error: %v", err))
			return fmt.Errorf("error getting digger batch")
		}

		batch.SourceDetails, err = json.Marshal(sourceDetails)
		if err != nil {
			log.Printf("sourceDetails, json Marshal error: %v", err)
			utils.InitCommentReporter(ghService, issueNumber, fmt.Sprintf(":x: json Marshal error: %v", err))
			return fmt.Errorf("error marshalling sourceDetails")
		}
		err = models.DB.UpdateDiggerBatch(batch)
		if err != nil {
			log.Printf("UpdateDiggerBatch error: %v", err)
			utils.InitCommentReporter(ghService, issueNumber, fmt.Sprintf(":x: UpdateDiggerBatch error: %v", err))
			return fmt.Errorf("error updating digger batch")
		}
	}

	segment.Track(strconv.Itoa(int(orgId)), "backend_trigger_job")

	ciBackend, err := ciBackendProvider.GetCiBackend(
		ci_backends.CiBackendOptions{
			GithubClientProvider: gh,
			GithubInstallationId: installationId,
			RepoName:             repoName,
			RepoOwner:            repoOwner,
			RepoFullName:         repoFullName,
		},
	)
	if err != nil {
		log.Printf("GetCiBackend error: %v", err)
		utils.InitCommentReporter(ghService, issueNumber, fmt.Sprintf(":x: GetCiBackend error: %v", err))
		return fmt.Errorf("error fetching ci backed %v", err)
	}
	err = TriggerDiggerJobs(ciBackend, repoOwner, repoName, batchId, issueNumber, ghService)
	if err != nil {
		log.Printf("TriggerDiggerJobs error: %v", err)
		utils.InitCommentReporter(ghService, issueNumber, fmt.Sprintf(":x: TriggerDiggerJobs error: %v", err))
		return fmt.Errorf("error triggerring Digger Jobs")
	}
	return nil
}
