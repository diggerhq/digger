package controllers

import (
	"encoding/json"
	"fmt"
	"github.com/diggerhq/digger/backend/ci_backends"
	"github.com/diggerhq/digger/backend/controllers"
	"github.com/diggerhq/digger/backend/locking"
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/segment"
	"github.com/diggerhq/digger/backend/utils"
	"github.com/diggerhq/digger/libs/ci/generic"
	dg_github "github.com/diggerhq/digger/libs/ci/github"
	comment_updater "github.com/diggerhq/digger/libs/comment_utils/reporting"
	dg_configuration "github.com/diggerhq/digger/libs/digger_config"
	dg_locking "github.com/diggerhq/digger/libs/locking"
	"github.com/diggerhq/digger/libs/orchestrator"
	"github.com/diggerhq/digger/libs/scheduler"
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
	GitlabProvider    utils.GitlabProvider
	CiBackendProvider ci_backends.CiBackendProvider
}

func (d DiggerEEController) GitlabWebHookHandler(c *gin.Context) {
	c.Header("Content-Type", "application/json")
	log.Printf("GitlabWebhook")

	//temp  to get orgID TODO: fetch from db
	organisation, err := models.DB.GetOrganisation(models.DEFAULT_ORG_NAME)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to get default organisation")
		return
	}
	organisationId := organisation.ID

	gitlabWebhookSecret := os.Getenv("DIGGER_GITLAB_WEBHOOK_SECRET")
	secret := c.GetHeader("X-Gitlab-Token")
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
		err := handleIssueCommentEvent(d.GitlabProvider, event, d.CiBackendProvider, organisationId)
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

func handleIssueCommentEvent(gitlabProvider utils.GitlabProvider, payload *gitlab.MergeCommentEvent, ciBackendProvider ci_backends.CiBackendProvider, organisationId uint) error {
	projectId := payload.ProjectID
	repoFullName := payload.Project.PathWithNamespace
	repoOwner, repoName, _ := strings.Cut(repoFullName, "/")
	cloneURL := payload.Project.GitHTTPURL
	issueNumber := payload.MergeRequest.IID
	isDraft := payload.MergeRequest.WorkInProgress
	commentId := payload.ObjectAttributes.ID
	commentBody := payload.ObjectAttributes.Description
	branch := payload.MergeRequest.SourceBranch
	commitSha := payload.ObjectAttributes.CommitID
	defaultBranch := payload.Repository.DefaultBranch
	actor := payload.User.Username
	discussionId := payload.ObjectAttributes.DiscussionID

	if payload.ObjectAttributes.Action != gitlab.CommentEventActionCreate {
		log.Printf("comment is not of type 'created', ignoring")
		return nil
	}

	if !strings.HasPrefix(commentBody, "digger") {
		log.Printf("comment is not a Digger command, ignoring")
		return nil
	}

	glService, glerr := utils.GetGitlabService(gitlabProvider, projectId, repoName, repoFullName, issueNumber, discussionId)
	if glerr != nil {
		log.Printf("GetGithubService error: %v", glerr)
		return fmt.Errorf("error getting ghService to post error comment")
	}

	diggerYmlStr, config, projectsGraph, err := utils.GetDiggerConfigForBranch(gitlabProvider, projectId, repoFullName, repoOwner, repoName, cloneURL, branch, issueNumber, discussionId)
	if err != nil {
		utils.InitCommentReporter(glService, issueNumber, fmt.Sprintf(":x: Could not load digger config, error: %v", err))
		log.Printf("getDiggerConfigForPR error: %v", err)
		return fmt.Errorf("error getting digger config")
	}

	err = glService.CreateCommentReaction(strconv.Itoa(commentId), string(dg_github.GithubCommentEyesReaction))
	if err != nil {
		log.Printf("CreateCommentReaction error: %v", err)
	}

	if !config.AllowDraftPRs && isDraft {
		log.Printf("AllowDraftPRs is disabled, skipping PR: %v", issueNumber)
		return nil
	}

	commentReporter, err := utils.InitCommentReporter(glService, issueNumber, ":construction_worker: Digger starting....")
	if err != nil {
		log.Printf("Error initializing comment reporter: %v", err)
		return fmt.Errorf("error initializing comment reporter")
	}

	diggerCommand, err := scheduler.GetCommandFromComment(commentBody)
	if err != nil {
		log.Printf("unkown digger command in comment: %v", commentBody)
		utils.InitCommentReporter(glService, issueNumber, fmt.Sprintf(":x: Could not recognise comment, error: %v", err))
		return fmt.Errorf("unkown digger command in comment %v", err)
	}

	prBranchName, _, err := glService.GetBranchName(issueNumber)
	if err != nil {
		log.Printf("GetBranchName error: %v", err)
		utils.InitCommentReporter(glService, issueNumber, fmt.Sprintf(":x: GetBranchName error: %v", err))
		return fmt.Errorf("error while fetching branch name")
	}

	impactedProjects, impactedProjectsSourceMapping, requestedProject, _, err := generic.ProcessIssueCommentEvent(issueNumber, commentBody, config, projectsGraph, glService)
	if err != nil {
		log.Printf("Error processing event: %v", err)
		utils.InitCommentReporter(glService, issueNumber, fmt.Sprintf(":x: Error processing event: %v", err))
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
				CIService:        glService,
				Reporter:         comment_updater.NoopReporter{},
				ProjectName:      project.Name,
				ProjectNamespace: repoFullName,
				PrNumber:         issueNumber,
			}
			err = orchestrator.PerformLockingActionFromCommand(prLock, *diggerCommand)
			if err != nil {
				utils.InitCommentReporter(glService, issueNumber, fmt.Sprintf(":x: Failed perform lock action on project: %v %v", project.Name, err))
				return fmt.Errorf("failed perform lock action on project: %v %v", project.Name, err)
			}
		}
	}

	// if commands are locking or unlocking we don't need to trigger any jobs
	if *diggerCommand == scheduler.DiggerCommandUnlock ||
		*diggerCommand == scheduler.DiggerCommandLock {
		utils.InitCommentReporter(glService, issueNumber, fmt.Sprintf(":white_check_mark: Command %v completed successfully", *diggerCommand))
		return nil
	}

	jobs, _, err := generic.ConvertIssueCommentEventToJobs(repoFullName, actor, issueNumber, commentBody, impactedProjects, requestedProject, config.Workflows, prBranchName, defaultBranch)
	if err != nil {
		log.Printf("Error converting event to jobs: %v", err)
		utils.InitCommentReporter(glService, issueNumber, fmt.Sprintf(":x: Error converting event to jobs: %v", err))
		return fmt.Errorf("error converting event to jobs")
	}
	log.Printf("GitHub IssueComment event converted to Jobs successfully\n")

	err = utils.ReportInitialJobsStatus(commentReporter, jobs)
	if err != nil {
		log.Printf("Failed to comment initial status for jobs: %v", err)
		utils.InitCommentReporter(glService, issueNumber, fmt.Sprintf(":x: Failed to comment initial status for jobs: %v", err))
		return fmt.Errorf("failed to comment initial status for jobs")
	}

	if len(jobs) == 0 {
		log.Printf("no projects impacated, succeeding")
		// This one is for aggregate reporting
		err = utils.SetPRStatusForJobs(glService, issueNumber, jobs)
		return nil
	}

	err = utils.SetPRStatusForJobs(glService, issueNumber, jobs)
	if err != nil {
		log.Printf("error setting status for PR: %v", err)
		utils.InitCommentReporter(glService, issueNumber, fmt.Sprintf(":x: error setting status for PR: %v", err))
		fmt.Errorf("error setting status for PR: %v", err)
	}

	impactedProjectsMap := make(map[string]dg_configuration.Project)
	for _, p := range impactedProjects {
		impactedProjectsMap[p.Name] = p
	}

	impactedProjectsJobMap := make(map[string]scheduler.Job)
	for _, j := range jobs {
		impactedProjectsJobMap[j.ProjectName] = j
	}

	commentId64, err := strconv.ParseInt(commentReporter.CommentId, 10, 64)
	if err != nil {
		log.Printf("ParseInt err: %v", err)
		return fmt.Errorf("parseint error: %v", err)
	}
	batchId, _, err := utils.ConvertJobsToDiggerJobs(*diggerCommand, models.DiggerVCSGitlab, organisationId, impactedProjectsJobMap, impactedProjectsMap, projectsGraph, 0, branch, issueNumber, repoOwner, repoName, repoFullName, commitSha, commentId64, diggerYmlStr)
	if err != nil {
		log.Printf("ConvertJobsToDiggerJobs error: %v", err)
		utils.InitCommentReporter(glService, issueNumber, fmt.Sprintf(":x: ConvertJobsToDiggerJobs error: %v", err))
		return fmt.Errorf("error convertingjobs")
	}

	if config.CommentRenderMode == dg_configuration.CommentRenderModeGroupByModule &&
		(*diggerCommand == scheduler.DiggerCommandPlan || *diggerCommand == scheduler.DiggerCommandApply) {

		sourceDetails, err := comment_updater.PostInitialSourceComments(glService, issueNumber, impactedProjectsSourceMapping)
		if err != nil {
			log.Printf("PostInitialSourceComments error: %v", err)
			utils.InitCommentReporter(glService, issueNumber, fmt.Sprintf(":x: PostInitialSourceComments error: %v", err))
			return fmt.Errorf("error posting initial comments")
		}
		batch, err := models.DB.GetDiggerBatch(batchId)
		if err != nil {
			log.Printf("GetDiggerBatch error: %v", err)
			utils.InitCommentReporter(glService, issueNumber, fmt.Sprintf(":x: PostInitialSourceComments error: %v", err))
			return fmt.Errorf("error getting digger batch")
		}

		batch.SourceDetails, err = json.Marshal(sourceDetails)
		if err != nil {
			log.Printf("sourceDetails, json Marshal error: %v", err)
			utils.InitCommentReporter(glService, issueNumber, fmt.Sprintf(":x: json Marshal error: %v", err))
			return fmt.Errorf("error marshalling sourceDetails")
		}
		err = models.DB.UpdateDiggerBatch(batch)
		if err != nil {
			log.Printf("UpdateDiggerBatch error: %v", err)
			utils.InitCommentReporter(glService, issueNumber, fmt.Sprintf(":x: UpdateDiggerBatch error: %v", err))
			return fmt.Errorf("error updating digger batch")
		}
	}

	segment.Track(strconv.Itoa(int(organisationId)), "backend_trigger_job")

	ciBackend, err := ciBackendProvider.GetCiBackend(
		ci_backends.CiBackendOptions{
			RepoName:     repoName,
			RepoOwner:    repoOwner,
			RepoFullName: repoFullName,
		},
	)
	if err != nil {
		log.Printf("GetCiBackend error: %v", err)
		utils.InitCommentReporter(glService, issueNumber, fmt.Sprintf(":x: GetCiBackend error: %v", err))
		return fmt.Errorf("error fetching ci backed %v", err)
	}
	err = controllers.TriggerDiggerJobs(ciBackend, repoFullName, repoOwner, repoName, batchId, issueNumber, glService)
	if err != nil {
		log.Printf("TriggerDiggerJobs error: %v", err)
		utils.InitCommentReporter(glService, issueNumber, fmt.Sprintf(":x: TriggerDiggerJobs error: %v", err))
		return fmt.Errorf("error triggerring Digger Jobs")
	}
	return nil
}
