package controllers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/diggerhq/digger/backend/ci_backends"
	"github.com/diggerhq/digger/backend/controllers"
	"github.com/diggerhq/digger/backend/locking"
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/utils"
	ci_backends2 "github.com/diggerhq/digger/ee/backend/ci_backends"
	"github.com/diggerhq/digger/libs/ci/generic"
	dg_github "github.com/diggerhq/digger/libs/ci/github"
	comment_updater "github.com/diggerhq/digger/libs/comment_utils/reporting"
	dg_configuration "github.com/diggerhq/digger/libs/digger_config"
	dg_locking "github.com/diggerhq/digger/libs/locking"
	"github.com/diggerhq/digger/libs/scheduler"
	"github.com/gin-gonic/gin"
)

type BBWebhookPayload map[string]interface{}

// TriggerPipeline triggers a CI pipeline with the specified type
func TriggerPipeline(pipelineType string) error {
	// Implement pipeline triggering logic
	return nil
}

// verifySignature verifies the X-Hub-Signature header
func verifySignature(c *gin.Context, body []byte, webhookSecret string) bool {
	// Get the signature from the header
	signature := c.GetHeader("X-Hub-Signature")
	if signature == "" {
		return false
	}

	// Remove the "sha256=" prefix if present
	if len(signature) > 7 && signature[0:7] == "sha256=" {
		signature = signature[7:]
	}

	// Create a new HMAC

	mac := hmac.New(sha256.New, []byte(webhookSecret))
	mac.Write(body)
	expectedMAC := mac.Sum(nil)
	expectedSignature := hex.EncodeToString(expectedMAC)

	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}

// WebhookHandler processes incoming Bitbucket webhook events
func (ee DiggerEEController) BitbucketWebhookHandler(c *gin.Context) {
	eventKey := c.GetHeader("X-Event-Key")
	if eventKey == "" {
		log.Printf("unknown event")
		return
	}

	connectionId := c.GetHeader("DIGGER_CONNECTION_ID")
	connectionEncrypted, err := models.DB.GetVCSConnectionById(connectionId)
	if err != nil {
		log.Printf("failed to fetch connection: %v", err)
		c.String(http.StatusInternalServerError, "error while processing connection")
		return
	}

	secret := os.Getenv("DIGGER_ENCRYPTION_SECRET")
	if secret == "" {
		log.Printf("ERROR: no encryption secret specified, please specify DIGGER_ENCRYPTION_SECRET as 32 bytes base64 string")
		c.String(http.StatusInternalServerError, "secret not specified")
		return
	}
	connectionDecrypted, err := utils.DecryptConnection(connectionEncrypted, []byte(secret))
	if err != nil {
		log.Printf("ERROR: could not perform decryption: %v", err)
		c.String(http.StatusInternalServerError, "unexpected error while fetching connection")
		return
	}
	bitbucketAccessToken := connectionDecrypted.BitbucketAccessToken

	orgId := connectionDecrypted.OrganisationID
	bitbucketWebhookSecret := connectionDecrypted.BitbucketWebhookSecret

	if bitbucketWebhookSecret == "" {
		log.Printf("ERROR: no encryption secret specified, please specify DIGGER_ENCRYPTION_SECRET as 32 bytes base64 string")
		c.String(http.StatusInternalServerError, "unexpected error while fetching connection")
		return
	}

	var pullRequestCommentCreated = BitbucketCommentCreatedEvent{}
	var repoPush = BitbucketPushEvent{}
	var pullRequestCreated = BitbucketPullRequestCreatedEvent{}

	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.String(http.StatusBadRequest, "Error reading request body", err)
		return
	}

	verifySignature(c, bodyBytes, bitbucketWebhookSecret)

	switch eventKey {
	case "pullrequest:comment_created":
		err := json.Unmarshal(bodyBytes, &pullRequestCommentCreated)
		if err != nil {
			log.Printf("error parsing pullrequest:comment_created event: %v", err)
			log.Printf("error parsing pullrequest:comment_created event: %v", err)
		}
		go handleIssueCommentEventBB(ee.BitbucketProvider, &pullRequestCommentCreated, ee.CiBackendProvider, orgId, &connectionEncrypted.ID, bitbucketAccessToken)
	case "pullrequest:created":
		err := json.Unmarshal(bodyBytes, &pullRequestCreated)
		if err != nil {
			log.Printf("error parsing pullrequest:created event: %v", err)
		}
		log.Printf("pullrequest:created")
	case "repo:push":
		err := json.Unmarshal(bodyBytes, &repoPush)
		if err != nil {
			log.Printf("error parsing repo:push event: %v", err)
		}
		log.Printf("repo:push")
	default:
		log.Printf("unknown event key: %s", eventKey)
		return
	}

	c.String(http.StatusAccepted, "ok")
}

func handleIssueCommentEventBB(bitbucketProvider utils.BitbucketProvider, payload *BitbucketCommentCreatedEvent, ciBackendProvider ci_backends.CiBackendProvider, organisationId uint, vcsConnectionId *uint, bbAccessToken string) error {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic in handleIssueCommentEventBB handler: %v", r)
			log.Printf("\n=== PANIC RECOVERED ===\n")
			log.Printf("Error: %v\n", r)
			log.Printf("Stack Trace:\n%s", string(debug.Stack()))
			log.Printf("=== END PANIC ===\n")
		}
	}()

	repoFullName := payload.Repository.FullName
	repoOwner := payload.Repository.Owner.Username
	repoName := payload.Repository.Name
	cloneURL := payload.Repository.Links.HTML.Href
	issueNumber := payload.PullRequest.ID
	// TODO: fetch right draft status
	isDraft := false
	commentId := payload.Comment.ID
	commentBody := payload.Comment.Content.Raw
	branch := payload.PullRequest.Source.Branch.Name
	// TODO: figure why git fetch fails in bb pipeline
	commitSha := "" //payload.PullRequest.Source.Commit.Hash
	defaultBranch := payload.PullRequest.Source.Branch.Name
	actor := payload.Actor.Nickname
	//discussionId := payload.Comment.ID

	if !strings.HasPrefix(commentBody, "digger") {
		log.Printf("comment is not a Digger command, ignoring")
		return nil
	}

	bbService, bberr := utils.GetBitbucketService(bitbucketProvider, bbAccessToken, repoOwner, repoName, issueNumber)
	if bberr != nil {
		log.Printf("GetGithubService error: %v", bberr)
		return fmt.Errorf("error getting ghService to post error comment")
	}

	diggerYmlStr, config, projectsGraph, err := utils.GetDiggerConfigForBitbucketBranch(bitbucketProvider, bbAccessToken, repoFullName, repoOwner, repoName, cloneURL, branch, issueNumber)
	if err != nil {
		log.Printf("getDiggerConfigForPR error: %v", err)
		utils.InitCommentReporter(bbService, issueNumber, fmt.Sprintf(":x: Could not load digger config, error: %v", err))
		return fmt.Errorf("error getting digger config")
	}

	err = bbService.CreateCommentReaction(strconv.Itoa(commentId), string(dg_github.GithubCommentEyesReaction))
	if err != nil {
		log.Printf("CreateCommentReaction error: %v", err)
	}

	if !config.AllowDraftPRs && isDraft {
		log.Printf("AllowDraftPRs is disabled, skipping PR: %v", issueNumber)
		return nil
	}

	commentReporter, err := utils.InitCommentReporter(bbService, issueNumber, ":construction_worker: Digger starting....")
	if err != nil {
		log.Printf("Error initializing comment reporter: %v", err)
		return fmt.Errorf("error initializing comment reporter")
	}

	diggerCommand, err := scheduler.GetCommandFromComment(commentBody)
	if err != nil {
		log.Printf("unknown digger command in comment: %v", commentBody)
		utils.InitCommentReporter(bbService, issueNumber, fmt.Sprintf(":x: Could not recognise comment, error: %v", err))
		return fmt.Errorf("unknown digger command in comment %v", err)
	}

	prBranchName, _, _, _, err := bbService.GetBranchName(issueNumber)
	if err != nil {
		log.Printf("GetBranchName error: %v", err)
		utils.InitCommentReporter(bbService, issueNumber, fmt.Sprintf(":x: GetBranchName error: %v", err))
		return fmt.Errorf("error while fetching branch name")
	}

	processIssueCommentResult, err := generic.ProcessIssueCommentEvent(issueNumber, config, projectsGraph, bbService)
	if err != nil {
		log.Printf("Error processing event: %v", err)
		utils.InitCommentReporter(bbService, issueNumber, fmt.Sprintf(":x: Error processing event: %v", err))
		return fmt.Errorf("error processing event")
	}
	log.Printf("Bitbucket IssueComment event processed successfully\n")

	impactedProjectsSourceMapping := processIssueCommentResult.ImpactedProjectsSourceMapping
	allImpactedProjects := processIssueCommentResult.AllImpactedProjects

	impactedProjectsForComment, err := generic.FilterOutProjectsFromComment(allImpactedProjects, commentBody)
	if err != nil {
		log.Printf("error filtering out projects from comment issueNumber: %v, error: %v", issueNumber, err)
		utils.InitCommentReporter(bbService, issueNumber, fmt.Sprintf(":x: Error filtering out projects from comment: %v", err))
		return fmt.Errorf("error filtering out projects from comment")
	}

	// perform unlocking in backend
	if config.PrLocks {
		for _, project := range impactedProjectsForComment {
			prLock := dg_locking.PullRequestLock{
				InternalLock: locking.BackendDBLock{
					OrgId: organisationId,
				},
				CIService:        bbService,
				Reporter:         comment_updater.NoopReporter{},
				ProjectName:      project.Name,
				ProjectNamespace: repoFullName,
				PrNumber:         issueNumber,
			}
			err = dg_locking.PerformLockingActionFromCommand(prLock, *diggerCommand)
			if err != nil {
				utils.InitCommentReporter(bbService, issueNumber, fmt.Sprintf(":x: Failed perform lock action on project: %v %v", project.Name, err))
				return fmt.Errorf("failed perform lock action on project: %v %v", project.Name, err)
			}
		}
	}

	// if commands are locking or unlocking we don't need to trigger any jobs
	if *diggerCommand == scheduler.DiggerCommandUnlock ||
		*diggerCommand == scheduler.DiggerCommandLock {
		utils.InitCommentReporter(bbService, issueNumber, fmt.Sprintf(":white_check_mark: Command %v completed successfully", *diggerCommand))
		return nil
	}

	jobs, _, err := generic.ConvertIssueCommentEventToJobs(repoFullName, actor, issueNumber, commentBody, impactedProjectsForComment, allImpactedProjects, config.Workflows, prBranchName, defaultBranch, false)
	if err != nil {
		log.Printf("Error converting event to jobs: %v", err)
		utils.InitCommentReporter(bbService, issueNumber, fmt.Sprintf(":x: Error converting event to jobs: %v", err))
		return fmt.Errorf("error converting event to jobs")
	}
	log.Printf("GitHub IssueComment event converted to Jobs successfully\n")

	err = utils.ReportInitialJobsStatus(commentReporter, jobs)
	if err != nil {
		log.Printf("Failed to comment initial status for jobs: %v", err)
		utils.InitCommentReporter(bbService, issueNumber, fmt.Sprintf(":x: Failed to comment initial status for jobs: %v", err))
		return fmt.Errorf("failed to comment initial status for jobs")
	}

	if len(jobs) == 0 {
		log.Printf("no projects impacated, succeeding")
		// This one is for aggregate reporting
		err = utils.SetPRStatusForJobs(bbService, issueNumber, jobs)
		return nil
	}

	err = utils.SetPRStatusForJobs(bbService, issueNumber, jobs)
	if err != nil {
		log.Printf("error setting status for PR: %v", err)
		utils.InitCommentReporter(bbService, issueNumber, fmt.Sprintf(":x: error setting status for PR: %v", err))
		fmt.Errorf("error setting status for PR: %v", err)
	}

	impactedProjectsMap := make(map[string]dg_configuration.Project)
	for _, p := range impactedProjectsForComment {
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

	batchId, _, err := utils.ConvertJobsToDiggerJobs(*diggerCommand, models.DiggerVCSBitbucket, organisationId, impactedProjectsJobMap, impactedProjectsMap, projectsGraph, 0, branch, issueNumber, repoOwner, repoName, repoFullName, commitSha, commentId64, diggerYmlStr, 0, "", false, true, vcsConnectionId)
	if err != nil {
		log.Printf("ConvertJobsToDiggerJobs error: %v", err)
		utils.InitCommentReporter(bbService, issueNumber, fmt.Sprintf(":x: ConvertJobsToDiggerJobs error: %v", err))
		return fmt.Errorf("error convertingjobs")
	}

	if config.CommentRenderMode == dg_configuration.CommentRenderModeGroupByModule &&
		(*diggerCommand == scheduler.DiggerCommandPlan || *diggerCommand == scheduler.DiggerCommandApply) {

		sourceDetails, err := comment_updater.PostInitialSourceComments(bbService, issueNumber, impactedProjectsSourceMapping)
		if err != nil {
			log.Printf("PostInitialSourceComments error: %v", err)
			utils.InitCommentReporter(bbService, issueNumber, fmt.Sprintf(":x: PostInitialSourceComments error: %v", err))
			return fmt.Errorf("error posting initial comments")
		}
		batch, err := models.DB.GetDiggerBatch(batchId)
		if err != nil {
			log.Printf("GetDiggerBatch error: %v", err)
			utils.InitCommentReporter(bbService, issueNumber, fmt.Sprintf(":x: PostInitialSourceComments error: %v", err))
			return fmt.Errorf("error getting digger batch")
		}

		batch.SourceDetails, err = json.Marshal(sourceDetails)
		if err != nil {
			log.Printf("sourceDetails, json Marshal error: %v", err)
			utils.InitCommentReporter(bbService, issueNumber, fmt.Sprintf(":x: json Marshal error: %v", err))
			return fmt.Errorf("error marshalling sourceDetails")
		}
		err = models.DB.UpdateDiggerBatch(batch)
		if err != nil {
			log.Printf("UpdateDiggerBatch error: %v", err)
			utils.InitCommentReporter(bbService, issueNumber, fmt.Sprintf(":x: UpdateDiggerBatch error: %v", err))
			return fmt.Errorf("error updating digger batch")
		}
	}

	// hardcoded bitbucket ci backend for this controller
	// TODO: making this configurable based on env variable and connection
	ciBackend := ci_backends2.BitbucketPipelineCI{
		RepoName:  repoName,
		RepoOwner: repoOwner,
		Branch:    branch,
		Client:    bbService,
	}
	err = controllers.TriggerDiggerJobs(ciBackend, repoFullName, repoOwner, repoName, batchId, issueNumber, bbService, nil)
	if err != nil {
		log.Printf("TriggerDiggerJobs error: %v", err)
		utils.InitCommentReporter(bbService, issueNumber, fmt.Sprintf(":x: TriggerDiggerJobs error: %v", err))
		return fmt.Errorf("error triggering Digger Jobs")
	}
	return nil
}
