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
	gitlab2 "github.com/diggerhq/digger/libs/ci/gitlab"
	comment_updater "github.com/diggerhq/digger/libs/comment_utils/reporting"
	dg_configuration "github.com/diggerhq/digger/libs/digger_config"
	dg_locking "github.com/diggerhq/digger/libs/locking"
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
	GithubClientProvider utils.GithubClientProvider
	GitlabProvider       utils.GitlabProvider
	CiBackendProvider    ci_backends.CiBackendProvider
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

	repoUrl := GetGitlabRepoUrl(event)
	if !utils.IsInRepoAllowList(repoUrl) {
		log.Printf("repo: '%v' is not in allow list, ignoring ...", repoUrl)
		return
	}

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
		err := handlePullRequestEvent(d.GitlabProvider, event, d.CiBackendProvider, organisationId)
		if err != nil {
			log.Printf("handlePullRequestEvent error: %v", err)
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
	case *gitlab.PushEvent:
		log.Printf("Got push event for %v %v", event.Project.URL, event.Ref)
		err := handlePushEvent(d.GitlabProvider, event, organisationId)
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

func handlePushEvent(gh utils.GitlabProvider, payload *gitlab.PushEvent, organisationId uint) error {
	//projectId := payload.Project.ID
	repoFullName := payload.Project.PathWithNamespace
	repoOwner, repoName, _ := strings.Cut(repoFullName, "/")
	cloneURL := payload.Project.GitHTTPURL
	webURL := payload.Project.WebURL
	ref := payload.Ref
	defaultBranch := payload.Project.DefaultBranch

	pushBranch := ""
	if strings.HasPrefix(ref, "refs/heads/") {
		pushBranch = strings.TrimPrefix(ref, "refs/heads/")
	} else {
		log.Printf("push was not to a branch, ignoring %v", ref)
		return nil
	}

	diggerRepoName := strings.ReplaceAll(repoFullName, "/", "-")
	//repo, err := models.DB.GetRepo(organisationId, diggerRepoName)
	//if err != nil {
	//	log.Printf("Error getting Repo: %v", err)
	//	return fmt.Errorf("error getting github app link")
	//}
	// create repo if not exists
	org, err := models.DB.GetOrganisationById(organisationId)
	if err != nil {
		log.Printf("Error: could not get organisation: %v", err)
		return nil
	}

	repo, err := models.DB.CreateRepo(diggerRepoName, repoFullName, repoOwner, repoName, webURL, org, "")
	if err != nil {
		log.Printf("Error: could not create repo: %v", err)
		return nil
	}

	token := os.Getenv("DIGGER_GITLAB_ACCESS_TOKEN")
	if token == "" {
		log.Printf("could not find gitlab token: %v", err)
		return fmt.Errorf("could not find gitlab token")
	}

	var isMainBranch bool
	if strings.HasSuffix(ref, defaultBranch) {
		isMainBranch = true
	} else {
		isMainBranch = false
	}

	err = utils.CloneGitRepoAndDoAction(cloneURL, pushBranch, "", token, func(dir string) error {
		config, err := dg_configuration.LoadDiggerConfigYaml(dir, true, nil)
		if err != nil {
			log.Printf("ERROR load digger.yml: %v", err)
			return fmt.Errorf("error loading digger.yml %v", err)
		}
		models.DB.UpdateRepoDiggerConfig(organisationId, *config, repo, isMainBranch)
		return nil
	})
	if err != nil {
		return fmt.Errorf("error while cloning repo: %v", err)
	}

	return nil
}

func GetGitlabRepoUrl(event interface{}) string {
	var repoUrl = ""
	switch event := event.(type) {
	case *gitlab.MergeCommentEvent:
		repoUrl = event.Project.GitHTTPURL
	case *gitlab.MergeEvent:
		repoUrl = event.Project.GitHTTPURL
	case *gitlab.PushEvent:
		repoUrl = event.Project.GitHTTPURL
	default:
		log.Printf("Unhandled event, event type %v", reflect.TypeOf(event))
	}
	return repoUrl
}

func handlePullRequestEvent(gitlabProvider utils.GitlabProvider, payload *gitlab.MergeEvent, ciBackendProvider ci_backends.CiBackendProvider, organisationId uint) error {
	projectId := payload.Project.ID
	repoFullName := payload.Project.PathWithNamespace
	repoOwner, repoName, _ := strings.Cut(repoFullName, "/")
	cloneURL := payload.Project.GitHTTPURL
	prNumber := payload.ObjectAttributes.IID
	isDraft := payload.ObjectAttributes.WorkInProgress
	branch := payload.ObjectAttributes.SourceBranch
	commitSha := payload.ObjectAttributes.LastCommit.ID
	//defaultBranch := payload.Repository.DefaultBranch
	//actor := payload.User.Username
	//discussionId := ""
	//action := payload.ObjectAttributes.Action

	// temp hack: we initialize glService to publish an initial comment and then use that as a discussionId onwards (
	glService, glerr := utils.GetGitlabService(gitlabProvider, projectId, repoName, repoFullName, prNumber, "")
	if glerr != nil {
		log.Printf("GetGithubService error: %v", glerr)
		return fmt.Errorf("error getting ghService to post error comment")
	}
	comment, err := glService.PublishComment(prNumber, fmt.Sprintf("Report for pull request (%v)", commitSha))
	discussionId := comment.DiscussionId

	log.Printf("got first discussion id: %v", discussionId)
	// re-initialize with the right discussion ID
	glService, glerr = utils.GetGitlabService(gitlabProvider, projectId, repoName, repoFullName, prNumber, discussionId)
	if glerr != nil {
		log.Printf("GetGithubService error: %v", glerr)
		return fmt.Errorf("error getting ghService to post error comment")
	}

	diggeryamlStr, config, projectsGraph, err := utils.GetDiggerConfigForBranch(gitlabProvider, projectId, repoFullName, repoOwner, repoName, cloneURL, branch, prNumber, discussionId)
	if err != nil {
		log.Printf("getDiggerConfigForPR error: %v", err)
		utils.InitCommentReporter(glService, prNumber, fmt.Sprintf(":x: Could not load digger config, error: %v", err))
		return fmt.Errorf("error getting digger config")
	}

	if !config.AllowDraftPRs && isDraft {
		log.Printf("AllowDraftPRs is disabled, skipping PR: %v", prNumber)
		return nil
	}

	impactedProjects, _, _, err := gitlab2.ProcessGitlabPullRequestEvent(payload, config, projectsGraph, glService)
	if err != nil {
		log.Printf("Error processing event: %v", err)
		utils.InitCommentReporter(glService, prNumber, fmt.Sprintf(":x: Error processing event: %v", err))
		return fmt.Errorf("error processing event")
	}

	jobsForImpactedProjects, _, err := gitlab2.ConvertGithubPullRequestEventToJobs(payload, impactedProjects, nil, *config)
	if err != nil {
		log.Printf("Error converting event to jobsForImpactedProjects: %v", err)
		utils.InitCommentReporter(glService, prNumber, fmt.Sprintf(":x: Error converting event to jobsForImpactedProjects: %v", err))
		return fmt.Errorf("error converting event to jobsForImpactedProjects")
	}

	if len(jobsForImpactedProjects) == 0 {
		// do not report if no projects are impacted to minimise noise in the PR thread
		// TODO use status checks instead: https://github.com/diggerhq/digger/issues/1135
		log.Printf("No projects impacted; not starting any jobs")
		// This one is for aggregate reporting
		err = utils.SetPRStatusForJobs(glService, prNumber, jobsForImpactedProjects)
		return nil
	}

	diggerCommand, err := scheduler.GetCommandFromJob(jobsForImpactedProjects[0])
	if err != nil {
		log.Printf("could not determine digger command from job: %v", jobsForImpactedProjects[0].Commands)
		utils.InitCommentReporter(glService, prNumber, fmt.Sprintf(":x: could not determine digger command from job: %v", err))
		return fmt.Errorf("unknown digger command in comment %v", err)
	}

	if *diggerCommand == scheduler.DiggerCommandNoop {
		log.Printf("job is of type noop, no actions top perform")
		return nil
	}

	// perform locking/unlocking in backend
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
				PrNumber:         prNumber,
			}
			err = dg_locking.PerformLockingActionFromCommand(prLock, *diggerCommand)
			if err != nil {
				utils.InitCommentReporter(glService, prNumber, fmt.Sprintf(":x: Failed perform lock action on project: %v %v", project.Name, err))
				return fmt.Errorf("failed to perform lock action on project: %v, %v", project.Name, err)
			}
		}
	}

	// if commands are locking or unlocking we don't need to trigger any jobs
	if *diggerCommand == scheduler.DiggerCommandUnlock ||
		*diggerCommand == scheduler.DiggerCommandLock {
		utils.InitCommentReporter(glService, prNumber, fmt.Sprintf(":white_check_mark: Command %v completed successfully", *diggerCommand))
		return nil
	}

	if !config.AllowDraftPRs && isDraft {
		log.Printf("Draft PRs are disabled, skipping PR: %v", prNumber)
		return nil
	}

	commentReporter, err := utils.InitCommentReporter(glService, prNumber, ":construction_worker: Digger starting...")
	if err != nil {
		log.Printf("Error initializing comment reporter: %v", err)
		return fmt.Errorf("error initializing comment reporter")
	}

	err = utils.ReportInitialJobsStatus(commentReporter, jobsForImpactedProjects)
	if err != nil {
		log.Printf("Failed to comment initial status for jobs: %v", err)
		utils.InitCommentReporter(glService, prNumber, fmt.Sprintf(":x: Failed to comment initial status for jobs: %v", err))
		return fmt.Errorf("failed to comment initial status for jobs")
	}

	err = utils.SetPRStatusForJobs(glService, prNumber, jobsForImpactedProjects)
	if err != nil {
		log.Printf("error setting status for PR: %v", err)
		utils.InitCommentReporter(glService, prNumber, fmt.Sprintf(":x: error setting status for PR: %v", err))
		fmt.Errorf("error setting status for PR: %v", err)
	}

	impactedProjectsMap := make(map[string]dg_configuration.Project)
	for _, p := range impactedProjects {
		impactedProjectsMap[p.Name] = p
	}

	impactedJobsMap := make(map[string]scheduler.Job)
	for _, j := range jobsForImpactedProjects {
		impactedJobsMap[j.ProjectName] = j
	}

	commentId, err := strconv.ParseInt(commentReporter.CommentId, 10, 64)
	if err != nil {
		log.Printf("strconv.ParseInt error: %v", err)
		utils.InitCommentReporter(glService, prNumber, fmt.Sprintf(":x: could not handle commentId: %v", err))
	}
	batchId, _, err := utils.ConvertJobsToDiggerJobs(*diggerCommand, models.DiggerVCSGitlab, organisationId, impactedJobsMap, impactedProjectsMap, projectsGraph, 0, branch, prNumber, repoOwner, repoName, repoFullName, commitSha, commentId, diggeryamlStr, projectId)
	if err != nil {
		log.Printf("ConvertJobsToDiggerJobs error: %v", err)
		utils.InitCommentReporter(glService, prNumber, fmt.Sprintf(":x: ConvertJobsToDiggerJobs error: %v", err))
		return fmt.Errorf("error converting jobs")
	}

	segment.Track(strconv.Itoa(int(organisationId)), "backend_trigger_job")

	ciBackend, err := ciBackendProvider.GetCiBackend(
		ci_backends.CiBackendOptions{
			RepoName:                 repoName,
			RepoOwner:                repoOwner,
			RepoFullName:             repoFullName,
			GitlabProjectId:          projectId,
			GitlabCIMergeRequestID:   payload.ObjectAttributes.ID,
			GitlabCIMergeRequestIID:  payload.ObjectAttributes.IID,
			GitlabciprojectId:        payload.Project.ID,
			GitlabciprojectNamespace: payload.Project.Namespace,
			//GitlabciprojectNamespaceId: 0,
			GitlabmergeRequestEventName: payload.EventType,
			//GitlabCIPipelineID: ,
			//GitlabCIPipelineIID: "",
			GitlabCIProjectName: payload.Project.Name,
			GitlabDiscussionId:  discussionId,
		},
	)
	if err != nil {
		log.Printf("GetCiBackend error: %v", err)
		utils.InitCommentReporter(glService, prNumber, fmt.Sprintf(":x: GetCiBackend error: %v", err))
		return fmt.Errorf("error fetching ci backed %v", err)
	}

	err = controllers.TriggerDiggerJobs(ciBackend, repoFullName, repoOwner, repoName, batchId, prNumber, glService, nil)
	if err != nil {
		log.Printf("TriggerDiggerJobs error: %v", err)
		utils.InitCommentReporter(glService, prNumber, fmt.Sprintf(":x: TriggerDiggerJobs error: %v", err))
		return fmt.Errorf("error triggering Digger Jobs")
	}

	return nil
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
		log.Printf("getDiggerConfigForPR error: %v", err)
		utils.InitCommentReporter(glService, issueNumber, fmt.Sprintf(":x: Could not load digger config, error: %v", err))
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
		log.Printf("unknown digger command in comment: %v", commentBody)
		utils.InitCommentReporter(glService, issueNumber, fmt.Sprintf(":x: Could not recognise comment, error: %v", err))
		return fmt.Errorf("unknown digger command in comment %v", err)
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
			err = dg_locking.PerformLockingActionFromCommand(prLock, *diggerCommand)
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
	batchId, _, err := utils.ConvertJobsToDiggerJobs(*diggerCommand, models.DiggerVCSGitlab, organisationId, impactedProjectsJobMap, impactedProjectsMap, projectsGraph, 0, branch, issueNumber, repoOwner, repoName, repoFullName, commitSha, commentId64, diggerYmlStr, projectId)
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
			RepoName:                 repoName,
			RepoOwner:                repoOwner,
			RepoFullName:             repoFullName,
			GitlabProjectId:          projectId,
			GitlabCIMergeRequestID:   payload.MergeRequest.ID,
			GitlabCIMergeRequestIID:  payload.MergeRequest.IID,
			GitlabciprojectId:        payload.ProjectID,
			GitlabciprojectNamespace: payload.Project.Namespace,
			//GitlabciprojectNamespaceId:  payload.Project.Namespace,
			GitlabmergeRequestEventName: payload.EventType,
			//GitlabCIPipelineID: ,
			//GitlabCIPipelineIID: "",
			GitlabCIProjectName: payload.Project.Name,
			GitlabDiscussionId:  discussionId,
		},
	)
	if err != nil {
		log.Printf("GetCiBackend error: %v", err)
		utils.InitCommentReporter(glService, issueNumber, fmt.Sprintf(":x: GetCiBackend error: %v", err))
		return fmt.Errorf("error fetching ci backed %v", err)
	}
	err = controllers.TriggerDiggerJobs(ciBackend, repoFullName, repoOwner, repoName, batchId, issueNumber, glService, nil)
	if err != nil {
		log.Printf("TriggerDiggerJobs error: %v", err)
		utils.InitCommentReporter(glService, issueNumber, fmt.Sprintf(":x: TriggerDiggerJobs error: %v", err))
		return fmt.Errorf("error triggering Digger Jobs")
	}
	return nil
}
