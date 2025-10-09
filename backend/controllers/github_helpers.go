package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/diggerhq/digger/backend/ci_backends"
	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/backend/services"
	"github.com/diggerhq/digger/backend/utils"
	"github.com/diggerhq/digger/libs/ci"
	github2 "github.com/diggerhq/digger/libs/ci/github"
	"github.com/diggerhq/digger/libs/digger_config"
	"github.com/diggerhq/digger/libs/digger_config/terragrunt/tac"
	"github.com/diggerhq/digger/libs/git_utils"
	"github.com/dominikbraun/graph"
	"github.com/google/go-github/v61/github"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// why this validation is needed: https://roadie.io/blog/avoid-leaking-github-org-data/
// validation based on https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-a-user-access-token-for-a-github-app , step 3
func validateGithubCallback(githubClientProvider utils.GithubClientProvider, clientId string, clientSecret string, code string, installationId int64) (bool, *github.Installation, error) {
	slog.Debug("Validating GitHub callback",
		"clientId", clientId,
		"installationId", installationId,
	)

	ctx := context.Background()
	type OAuthAccessResponse struct {
		AccessToken string `json:"access_token"`
	}
	httpClient := http.Client{}

	githubHostname := utils.GetGithubHostname()
	reqURL := fmt.Sprintf("https://%v/login/oauth/access_token?client_id=%s&client_secret=%s&code=%s", githubHostname, clientId, clientSecret, code)

	slog.Debug("Creating OAuth access token request", "hostname", githubHostname)

	req, err := http.NewRequest(http.MethodPost, reqURL, nil)
	if err != nil {
		slog.Error("Could not create HTTP request",
			"error", err,
		)
		return false, nil, fmt.Errorf("could not create HTTP request: %v", err)
	}
	req.Header.Set("accept", "application/json")

	slog.Debug("Sending OAuth token request")
	res, err := httpClient.Do(req)
	if err != nil {
		slog.Error("Request to OAuth access token endpoint failed",
			"error", err,
		)
		return false, nil, fmt.Errorf("request to login/oauth/access_token failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		slog.Error("OAuth token request returned non-200 status",
			"statusCode", res.StatusCode,
			"status", res.Status,
		)
		return false, nil, fmt.Errorf("OAuth token request failed with status: %s", res.Status)
	}

	var t OAuthAccessResponse
	if err := json.NewDecoder(res.Body).Decode(&t); err != nil {
		slog.Error("Could not parse JSON response from OAuth token request",
			"error", err,
		)
		return false, nil, fmt.Errorf("could not parse JSON response: %v", err)
	}

	if t.AccessToken == "" {
		slog.Error("OAuth response contained empty access token")
		return false, nil, fmt.Errorf("received empty access token in OAuth response")
	}

	slog.Debug("Successfully obtained OAuth access token")

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: t.AccessToken},
	)
	tc := oauth2.NewClient(ctx, ts)
	//tc := &http.Client{
	//	Transport: &oauth2.Transport{
	//		Base:   httpClient.Transport,
	//		Source: oauth2.ReuseTokenSource(nil, ts),
	//	},
	//}

	client, err := githubClientProvider.NewClient(tc)
	if err != nil {
		slog.Error("Could not create GitHub client",
			"error", err,
		)
		return false, nil, fmt.Errorf("could not create github client: %v", err)
	}

	slog.Debug("Listing user installations to validate installation ID",
		"installationIdToMatch", installationId,
	)

	installationIdMatch := false
	// list all installations for the user
	var matchedInstallation *github.Installation
	installations, _, err := client.Apps.ListUserInstallations(ctx, nil)
	if err != nil {
		slog.Error("Could not retrieve user installations",
			"error", err,
		)
		return false, nil, fmt.Errorf("could not retrieve installations: %v", err)
	}

	slog.Debug("Retrieved user installations",
		"count", len(installations),
	)

	for _, v := range installations {
		slog.Debug("Checking installation",
			"installationId", *v.ID,
			"targetId", installationId,
		)

		if *v.ID == installationId {
			matchedInstallation = v
			installationIdMatch = true

			slog.Info("Found matching installation",
				"installationId", *v.ID,
				"appId", *v.AppID,
				"account", *v.Account.Login,
			)
		}
	}

	if !installationIdMatch {
		slog.Warn("Installation ID does not match any installation for the authenticated user",
			"installationId", installationId,
			"availableInstallations", len(installations),
		)
		return false, nil, fmt.Errorf("installationId %v doesn't match any ID for specified user", installationId)
	}

	slog.Info("Successfully validated GitHub callback",
		"installationId", installationId,
		"appId", *matchedInstallation.AppID,
	)

	return true, matchedInstallation, nil
}

func TriggerDiggerJobs(ciBackend ci_backends.CiBackend, repoFullName string, repoOwner string, repoName string, batchId *uuid.UUID, prNumber int, prService ci.PullRequestService, gh utils.GithubClientProvider) error {
	slog.Info("Triggering Digger jobs for batch",
		"batchId", batchId,
		slog.Group("repository",
			slog.String("fullName", repoFullName),
			slog.String("owner", repoOwner),
			slog.String("name", repoName),
		),
		"prNumber", prNumber,
	)

	batch, err := models.DB.GetDiggerBatch(batchId)
	if err != nil {
		slog.Error("Failed to get Digger batch",
			"batchId", batchId,
			"error", err,
		)
		return fmt.Errorf("failed to get digger batch, %v", err)
	}

	diggerJobs, err := models.DB.GetPendingParentDiggerJobs(batchId)
	if err != nil {
		slog.Error("Failed to get pending Digger jobs",
			"batchId", batchId,
			"error", err,
		)
		return fmt.Errorf("failed to get pending digger jobs, %v", err)
	}

	slog.Info("Retrieved pending jobs for batch",
		"batchId", batchId,
		"jobCount", len(diggerJobs),
		"batchType", batch.BatchType,
	)

	for i, job := range diggerJobs {
		slog.Debug("Processing job",
			"jobIndex", i+1,
			"jobCount", len(diggerJobs),
			"jobId", job.DiggerJobID,
			"batchId", batchId,
		)

		if job.SerializedJobSpec == nil {
			slog.Error("GitHub job specification is nil",
				"jobId", job.DiggerJobID,
				"batchId", batchId,
			)
			return fmt.Errorf("GitHub job can't be nil")
		}

		slog.Debug("Scheduling job",
			"jobId", job.DiggerJobID,
			"specLength", len(job.SerializedJobSpec),
			"batchId", batchId,
		)

		// TODO: make workflow file name configurable
		err = services.ScheduleJob(ciBackend, repoFullName, repoOwner, repoName, batchId, &job, gh)
		if err != nil {
			slog.Error("Failed to trigger CI workflow",
				"jobId", job.DiggerJobID,
				"batchId", batchId,
				"error", err,
			)
			return fmt.Errorf("failed to trigger CI workflow, %v", err)
		}

		slog.Info("Successfully scheduled job",
			"jobId", job.DiggerJobID,
			"batchId", batchId,
			"jobNumber", i+1,
			"totalJobs", len(diggerJobs),
		)
	}

	slog.Info("Successfully triggered all Digger jobs",
		"batchId", batchId,
		"jobCount", len(diggerJobs),
	)

	return nil
}

func GenerateTerraformFromCode(payload *github.IssueCommentEvent, commentReporterManager utils.CommentReporterManager, config *digger_config.DiggerConfig, defaultBranch string, ghService *github2.GithubService, repoOwner string, repoName string, commitSha *string, issueNumber int, branch *string) error {
	if !strings.HasPrefix(*payload.Comment.Body, "digger generate") {
		return nil
	}

	slog.Info("Processing Terraform generation request",
		"issueNumber", issueNumber,
		slog.Group("repository",
			slog.String("owner", repoOwner),
			slog.String("name", repoName),
		),
		"branch", *branch,
		"defaultBranch", defaultBranch,
		"comment", *payload.Comment.Body,
	)

	projectName := ci.ParseProjectName(*payload.Comment.Body)
	if projectName == "" {
		slog.Error("Missing project name in generate command",
			"issueNumber", issueNumber,
			"comment", *payload.Comment.Body,
		)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: generate requires argument -p <project_name>"))
		return fmt.Errorf("generate requires argument -p <project_name>")
	}

	slog.Debug("Looking for project in config",
		"projectName", projectName,
		"issueNumber", issueNumber,
	)

	project := config.GetProject(projectName)
	if project == nil {
		slog.Error("Project not found in digger.yml",
			"projectName", projectName,
			"issueNumber", issueNumber,
		)
		commentReporterManager.UpdateComment(fmt.Sprintf("could not find project %v in digger.yml", projectName))
		return fmt.Errorf("could not find project %v in digger.yml", projectName)
	}

	slog.Info("Found project in configuration",
		"projectName", projectName,
		"projectDir", project.Dir,
		"issueNumber", issueNumber,
	)

	commentReporterManager.UpdateComment(fmt.Sprintf(":white_check_mark: Successfully loaded project"))

	generationEndpoint := os.Getenv("DIGGER_GENERATION_ENDPOINT")
	if generationEndpoint == "" {
		slog.Error("Generation endpoint not configured",
			"issueNumber", issueNumber,
			"projectName", projectName,
		)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: server does not have generation endpoint configured, please verify"))
		return fmt.Errorf("server does not have generation endpoint configured, please verify")
	}
	apiToken := os.Getenv("DIGGER_GENERATION_API_TOKEN")

	// Get all code content from the repository at a specific commit
	getCodeFromCommit := func(ghService *github2.GithubService, repoOwner, repoName string, commitSha *string, projectDir string) (string, error) {
		const MaxPatchSize = 1024 * 1024 // 1MB limit

		slog.Debug("Getting code from commit",
			"commitSha", *commitSha,
			"repoOwner", repoOwner,
			"repoName", repoName,
			"projectDir", projectDir,
		)

		// Get the commit's changes compared to default branch
		comparison, _, err := ghService.Client.Repositories.CompareCommits(
			context.Background(),
			repoOwner,
			repoName,
			defaultBranch,
			*commitSha,
			nil,
		)
		if err != nil {
			slog.Error("Error comparing commits",
				"commitSha", *commitSha,
				"defaultBranch", defaultBranch,
				"error", err,
			)
			return "", fmt.Errorf("error comparing commits: %v", err)
		}

		slog.Debug("Retrieved commit comparison",
			"filesChanged", len(comparison.Files),
			"commitSha", *commitSha,
		)

		var appCode strings.Builder
		for _, file := range comparison.Files {
			if file.Patch == nil {
				continue // Skip files without patches
			}

			slog.Debug("Processing file patch",
				"filename", *file.Filename,
				"additions", *file.Additions,
				"patchSize", len(*file.Patch),
			)

			if *file.Additions > 0 {
				lines := strings.Split(*file.Patch, "\n")
				for _, line := range lines {
					if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
						appCode.WriteString(strings.TrimPrefix(line, "+"))
						appCode.WriteString("\n")
					}
				}
			}
			appCode.WriteString("\n")
		}

		if appCode.Len() == 0 {
			slog.Error("No code changes found in commit",
				"commitSha", *commitSha,
				"filesChecked", len(comparison.Files),
			)
			return "", fmt.Errorf("no code changes found in commit %s. Please ensure the PR contains added or modified code", *commitSha)
		}

		slog.Info("Extracted code changes from commit",
			"commitSha", *commitSha,
			"codeLength", appCode.Len(),
		)

		return appCode.String(), nil
	}

	appCode, err := getCodeFromCommit(ghService, repoOwner, repoName, commitSha, project.Dir)
	if err != nil {
		slog.Error("Failed to get code content",
			"projectName", projectName,
			"issueNumber", issueNumber,
			"error", err,
		)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: Failed to get code content: %v", err))
		return fmt.Errorf("error getting code content: %v", err)
	}

	slog.Debug("Successfully loaded code from commit",
		"issueNumber", issueNumber,
		"codeLength", len(appCode),
	)

	commentReporterManager.UpdateComment(fmt.Sprintf(":white_check_mark: Successfully loaded code from commit"))

	slog.Info("Generating Terraform code",
		"projectName", projectName,
		"issueNumber", issueNumber,
		"endpoint", generationEndpoint,
		"codeLength", len(appCode),
	)

	commentReporterManager.UpdateComment(fmt.Sprintf("Generating terraform..."))
	terraformCode, err := utils.GenerateTerraformCode(appCode, generationEndpoint, apiToken)
	if err != nil {
		slog.Error("Failed to generate Terraform code",
			"projectName", projectName,
			"issueNumber", issueNumber,
			"error", err,
		)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: could not generate terraform code: %v", err))
		return fmt.Errorf("could not generate terraform code: %v", err)
	}

	slog.Info("Successfully generated Terraform code",
		"projectName", projectName,
		"issueNumber", issueNumber,
		"codeLength", len(terraformCode),
	)

	commentReporterManager.UpdateComment(fmt.Sprintf(":white_check_mark: Generated terraform"))

	// Committing the generated Terraform code to the repository
	slog.Info("Preparing to commit generated Terraform code",
		"projectName", projectName,
		"issueNumber", issueNumber,
		"branch", *branch,
	)

	baseTree, _, err := ghService.Client.Git.GetTree(context.Background(), repoOwner, repoName, *commitSha, false)
	if err != nil {
		slog.Error("Failed to get base tree",
			"commitSha", *commitSha,
			"error", err,
		)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: Failed to get base tree: %v", err))
		return fmt.Errorf("error getting base tree: %v", err)
	}

	tfFilePath := filepath.Join(project.Dir, fmt.Sprintf("generated_%v.tf", issueNumber))

	// Create a new tree with the new file
	treeEntries := []*github.TreeEntry{
		{
			Path:    github.String(tfFilePath),
			Mode:    github.String("100644"),
			Type:    github.String("blob"),
			Content: github.String(terraformCode),
		},
	}

	slog.Debug("Creating new Git tree",
		"baseSHA", *baseTree.SHA,
		"tfFilePath", tfFilePath,
	)

	newTree, _, err := ghService.Client.Git.CreateTree(context.Background(), repoOwner, repoName, *baseTree.SHA, treeEntries)
	if err != nil {
		slog.Error("Failed to create new tree",
			"baseSHA", *baseTree.SHA,
			"error", err,
		)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: Failed to create new tree: %v", err))
		return fmt.Errorf("error creating new tree: %v", err)
	}

	// Create the commit
	commitMsg := fmt.Sprintf("Add generated Terraform code for %v", projectName)
	commit := &github.Commit{
		Message: &commitMsg,
		Tree:    newTree,
		Parents: []*github.Commit{{SHA: commitSha}},
	}

	slog.Debug("Creating new commit",
		"message", commitMsg,
		"treeSHA", *newTree.SHA,
		"parentSHA", *commitSha,
	)

	newCommit, _, err := ghService.Client.Git.CreateCommit(context.Background(), repoOwner, repoName, commit, nil)
	if err != nil {
		slog.Error("Failed to create commit",
			"message", commitMsg,
			"error", err,
		)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: Failed to commit Terraform file: %v", err))
		return fmt.Errorf("error committing Terraform file: %v", err)
	}

	// Update the reference to point to the new commit
	refName := fmt.Sprintf("refs/heads/%s", *branch)
	ref := &github.Reference{
		Ref: github.String(refName),
		Object: &github.GitObject{
			SHA: newCommit.SHA,
		},
	}

	slog.Debug("Updating branch reference",
		"ref", refName,
		"newCommitSHA", *newCommit.SHA,
	)

	_, _, err = ghService.Client.Git.UpdateRef(context.Background(), repoOwner, repoName, ref, false)
	if err != nil {
		slog.Error("Failed to update branch reference",
			"ref", refName,
			"newCommitSHA", *newCommit.SHA,
			"error", err,
		)
		commentReporterManager.UpdateComment(fmt.Sprintf(":x: Failed to update branch reference: %v", err))
		return fmt.Errorf("error updating branch reference: %v", err)
	}

	slog.Info("Successfully committed generated Terraform code",
		"projectName", projectName,
		"issueNumber", issueNumber,
		"branch", *branch,
		"commitSHA", *newCommit.SHA,
		"filePath", tfFilePath,
	)

	commentReporterManager.UpdateComment(":white_check_mark: Successfully generated and committed Terraform code")
	return nil
}

func retrieveConfigFromCache(orgId uint, repoFullName string) (string, *digger_config.DiggerConfig, *graph.Graph[string, digger_config.Project], *tac.AtlantisConfig, error) {
	slog.Debug("Retrieving config from cache",
		"orgId", orgId,
		"repoFullName", repoFullName,
	)

	repoCache, err := models.DB.GetRepoCache(orgId, repoFullName)
	if err != nil {
		slog.Info("Failed to load repo cache",
			"orgId", orgId,
			"repoFullName", repoFullName,
			"error", err,
		)
		return "", nil, nil, nil, fmt.Errorf("failed to load repo cache: %v", err)
	}

	var config digger_config.DiggerConfig
	err = json.Unmarshal(repoCache.DiggerConfig, &config)
	if err != nil {
		slog.Error("Failed to unmarshal config from cache",
			"orgId", orgId,
			"repoFullName", repoFullName,
			"error", err,
		)
		return "", nil, nil, nil, fmt.Errorf("failed to unmarshal config from cache: %v", err)
	}

	var taConfig tac.AtlantisConfig
	err = json.Unmarshal(repoCache.TerragruntAtlantisConfig, &taConfig)
	if err != nil {
		slog.Error("Failed to unmarshal config from cache",
			"orgId", orgId,
			"repoFullName", repoFullName,
			"error", err,
		)
		return "", nil, nil, nil, fmt.Errorf("failed to unmarshal config from cache: %v", err)
	}

	slog.Debug("Creating project dependency graph from cached config",
		"orgId", orgId,
		"repoFullName", repoFullName,
		"projectCount", len(config.Projects),
	)

	projectsGraph, err := digger_config.CreateProjectDependencyGraph(config.Projects)
	if err != nil {
		slog.Error("Error creating dependency graph from cached config",
			"orgId", orgId,
			"repoFullName", repoFullName,
			"error", err,
		)
		return "", nil, nil, nil, fmt.Errorf("error creating dependency graph from cached config: %v", err)
	}

	slog.Info("Successfully retrieved config from cache",
		"orgId", orgId,
		"repoFullName", repoFullName,
		"projectCount", len(config.Projects),
	)

	return repoCache.DiggerYmlStr, &config, &projectsGraph, &taConfig, nil
}



// handles post-merge branch not found 
func GetDiggerConfigForBranchWithFallback(gh utils.GithubClientProvider, installationId int64, repoFullName string, repoOwner string, repoName string, cloneUrl string, branch string, isMerged bool, changedFiles []string, taConfig *tac.AtlantisConfig) (string, *github2.GithubService, *digger_config.DiggerConfig, graph.Graph[string, digger_config.Project], error) {
	slog.Info("Attempting to get Digger config for branch...",
		"repoFullName", repoFullName,
		"primaryBranch", branch,
	)

	ghService, _, err := utils.GetGithubService(gh, installationId, repoFullName, repoOwner, repoName)
	if err != nil {
		return "", nil, nil, nil, fmt.Errorf("error getting github service")
	}

	diggerYmlStr, ghService, config, dependencyGraph, err := GetDiggerConfigForBranch(
		gh, installationId, repoFullName, repoOwner, repoName, cloneUrl, branch, changedFiles, taConfig,
	)

	if err != nil {
		// Check if it's a "branch not found" error
		errMsg := err.Error()
		isBranchNotFound := strings.Contains(errMsg, "Remote branch") && strings.Contains(errMsg, "not found") ||
			strings.Contains(errMsg, "couldn't find remote ref") ||
			strings.Contains(errMsg, "exit status 128")

		if isBranchNotFound && isMerged {
			// branch doesn't exist 
			// log the error but don't return it if we've merged already
			slog.Warn("Branch not found, PR is merged...",
				"missingBranch", branch,
				"repoFullName", repoFullName,
				"originalError", err,
			)

			return "", ghService, nil, nil, nil
		} else {
			// some other case 
			slog.Error("There was a problem loading the config for the branch.",
					"missingBranch", branch,
					"repoFullName", repoFullName,
			)
		
			return "", nil, nil, nil, err
		}
	}

	slog.Info("Config loaded successfully",
		"repoFullName", repoFullName,
		"branch", branch,
	)

	return diggerYmlStr, ghService, config, dependencyGraph, nil
}

// TODO: Refactor this func to receive ghService as input
func getDiggerConfigForPR(gh utils.GithubClientProvider, orgId uint, prLabels []string, installationId int64, repoFullName string, repoOwner string, repoName string, cloneUrl string, prNumber int) (string, *github2.GithubService, *digger_config.DiggerConfig, graph.Graph[string, digger_config.Project], *string, *string, []string, error) {
	slog.Info("Getting Digger config for PR",
		slog.Group("repository",
			slog.String("fullName", repoFullName),
			slog.String("owner", repoOwner),
			slog.String("name", repoName),
		),
		"orgId", orgId,
		"prNumber", prNumber,
		"installationId", installationId,
		"labels", prLabels,
	)

	ghService, _, err := utils.GetGithubService(gh, installationId, repoFullName, repoOwner, repoName)
	if err != nil {
		slog.Error("Error getting GitHub service",
			"installationId", installationId,
			"repoFullName", repoFullName,
			"error", err,
		)
		return "", nil, nil, nil, nil, nil, nil, fmt.Errorf("error getting github service")
	}

	var prBranch string
	prBranch, prCommitSha, _, _, err := ghService.GetBranchName(prNumber)
	if err != nil {
		slog.Error("Error getting branch name for PR",
			"prNumber", prNumber,
			"repoFullName", repoFullName,
			"error", err,
		)
		return "", nil, nil, nil, nil, nil, nil, fmt.Errorf("error getting branch name")
	}


	slog.Debug("Retrieved PR details",
		"prNumber", prNumber,
		"branch", prBranch,
		"commitSha", prCommitSha,
	)

	changedFiles, err := ghService.GetChangedFiles(prNumber)
	if err != nil {
		slog.Error("Error getting changed files for PR",
			"prNumber", prNumber,
			"repoFullName", repoFullName,
			"error", err,
		)
		return "", nil, nil, nil, nil, nil, nil, fmt.Errorf("error getting changed files")
	}

	slog.Debug("Retrieved changed files for PR",
		"prNumber", prNumber,
		"fileCount", len(changedFiles),
	)

	// check if items should be loaded from cache
	useCache := false
	var taConfig *tac.AtlantisConfig = nil
	if val, _ := os.LookupEnv("DIGGER_CONFIG_REPO_CACHE_ENABLED"); val == "1" && !slices.Contains(prLabels, "digger:no-cache") {
		useCache = true
		slog.Info("Attempting to load config from cache",
			"orgId", orgId,
			"repoFullName", repoFullName,
			"prNumber", prNumber,
		)

		_, _, _, taConfigTemp, err := retrieveConfigFromCache(orgId, repoFullName)
		if err != nil {
			slog.Info("Could not load from cache, falling back to live loading",
				"orgId", orgId,
				"repoFullName", repoFullName,
				"error", err,
			)
		} else {
			slog.Info("Successfully loaded config from cache",
				"orgId", orgId,
				"repoFullName", repoFullName,
			)
			taConfig = taConfigTemp
		}
	}

	if !useCache {
		slog.Debug("Cache disabled or skipped due to labels",
			"cacheEnabled", os.Getenv("DIGGER_CONFIG_REPO_CACHE_ENABLED") == "1",
			"hasNoCacheLabel", slices.Contains(prLabels, "digger:no-cache"),
		)
	}

	slog.Info("Loading config from repository",
		"repoFullName", repoFullName,
		"branch", prBranch,
		"prNumber", prNumber,
	)

	// Check if PR is merged
	isMerged := false
	isMerged, err = ghService.IsMerged(prNumber)
	if err != nil {
		slog.Warn("Could not check PR merge status",
			"prNumber", prNumber,
			"repoFullName", repoFullName,
			"error", err,
		)
	} else {
		slog.Debug("PR merge status checked",
			"prNumber", prNumber,
			"isMerged", isMerged,
		)
	}

	// get config 
	diggerYmlStr, ghService, config, dependencyGraph, err := GetDiggerConfigForBranchWithFallback(
		gh, installationId, repoFullName, repoOwner, repoName, cloneUrl, 
		prBranch, isMerged, changedFiles, taConfig,
	)
	if err != nil {
		slog.Error("Error loading Digger config from repository",
			"prNumber", prNumber,
			"repoFullName", repoFullName,
			"branch", prBranch,
			"error", err,
		)
		return "", nil, nil, nil, nil, nil, nil, fmt.Errorf("error loading digger.yml: %v", err)
	}

	return diggerYmlStr, ghService, config, dependencyGraph, &prBranch, &prCommitSha, changedFiles, nil
}

func GetDiggerConfigForBranch(gh utils.GithubClientProvider, installationId int64, repoFullName string, repoOwner string, repoName string, cloneUrl string, branch string, changedFiles []string, taConfig *tac.AtlantisConfig) (string, *github2.GithubService, *digger_config.DiggerConfig, graph.Graph[string, digger_config.Project], error) {
	slog.Info("Getting Digger config for branch",
		slog.Group("repository",
			slog.String("fullName", repoFullName),
			slog.String("owner", repoOwner),
			slog.String("name", repoName),
		),
		"installationId", installationId,
		"branch", branch,
		"changedFileCount", len(changedFiles),
	)

	ghService, token, err := utils.GetGithubService(gh, installationId, repoFullName, repoOwner, repoName)
	if err != nil {
		slog.Error("Error getting GitHub service",
			"installationId", installationId,
			"repoFullName", repoFullName,
			"error", err,
		)
		return "", nil, nil, nil, fmt.Errorf("error getting github service")
	}

	var config *digger_config.DiggerConfig
	var diggerYmlStr string
	var dependencyGraph graph.Graph[string, digger_config.Project]

	err = git_utils.CloneGitRepoAndDoAction(cloneUrl, branch, "", *token, "", func(dir string) error {
		slog.Debug("Reading Digger config from cloned repository", "directory", dir)

		diggerYmlStr, err = digger_config.ReadDiggerYmlFileContents(dir)
		if err != nil {
			slog.Error("Could not load Digger config file",
				"directory", dir,
				"error", err,
			)
			return err
		}

		slog.Debug("Successfully read digger.yml file", "configLength", len(diggerYmlStr))

		config, _, dependencyGraph, _, err = digger_config.LoadDiggerConfig(dir, true, changedFiles, taConfig)
		if err != nil {
			slog.Error("Error loading and parsing Digger config",
				"directory", dir,
				"error", err,
			)
			return err
		}
		return nil
	})

	if err != nil {
		slog.Error("Error cloning and loading config",
			"repoFullName", repoFullName,
			"branch", branch,
			"error", err,
		)
		return "", nil, nil, nil, fmt.Errorf("error cloning and loading config %v", err)
	}

	projectCount := 0
	if config != nil {
		projectCount = len(config.Projects)
	}

	slog.Info("Digger config loaded successfully",
		"repoFullName", repoFullName,
		"branch", branch,
		"projectCount", projectCount,
	)

	return diggerYmlStr, ghService, config, dependencyGraph, nil
}

func createOrGetDiggerRepoForGithubRepo(ghRepoFullName string, ghRepoOrganisation string, ghRepoName string, ghRepoUrl string, installationId int64, appId int64, defaultBranch string, cloneUrl string) (*models.Repo, *models.Organisation, error) {
	slog.Info("Creating or getting Digger repo for GitHub repo",
		slog.Group("githubRepo",
			slog.String("fullName", ghRepoFullName),
			slog.String("organization", ghRepoOrganisation),
			slog.String("name", ghRepoName),
			slog.String("url", ghRepoUrl),
		),
		"installationId", installationId,
	)

	link, err := models.DB.GetGithubInstallationLinkForInstallationId(installationId)
	if err != nil {
		slog.Error("Error fetching installation link", "installationId", installationId, "error", err)
		return nil, nil, err
	}

	orgId := link.OrganisationId
	org, err := models.DB.GetOrganisationById(orgId)
	if err != nil {
		slog.Error("Error fetching organisation", "orgId", orgId, "error", err)
		return nil, nil, err
	}

	diggerRepoName := strings.ReplaceAll(ghRepoFullName, "/", "-")

	// using Unscoped because we also need to include deleted repos (and undelete them if they exist)
	var existingRepo models.Repo
	r := models.DB.GormDB.Unscoped().Where("organisation_id=? AND repos.name=?", orgId, diggerRepoName).Find(&existingRepo)

	if r.Error != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			slog.Info("Repo not found, will create a new one", "diggerRepoName", diggerRepoName)
		} else {
			slog.Error("Error fetching repo", "diggerRepoName", diggerRepoName, "error", err)
			return nil, nil, err
		}
	}

	if r.RowsAffected > 0 {
		slog.Info("Digger repo already exists, restoring if deleted", "diggerRepoName", diggerRepoName, "repoId", existingRepo.ID)
		existingRepo.DeletedAt = gorm.DeletedAt{}
		existingRepo.GithubAppId = appId
		existingRepo.GithubAppInstallationId = installationId
		existingRepo.CloneUrl = cloneUrl
		existingRepo.DefaultBranch = defaultBranch
		models.DB.GormDB.Save(&existingRepo)
		return &existingRepo, org, nil
	}

	slog.Info("Creating new Digger repo", "diggerRepoName", diggerRepoName, "orgId", orgId)
	repo, err := models.DB.CreateRepo(diggerRepoName, ghRepoFullName, ghRepoOrganisation, ghRepoName, ghRepoUrl, org, `
generate_projects:
 include: "."
`, installationId, appId, defaultBranch, cloneUrl)
	if err != nil {
		slog.Error("Error creating Digger repo", "diggerRepoName", diggerRepoName, "error", err)
		return nil, nil, err
	}

	slog.Info("Created Digger repo", "repoId", repo.ID, "diggerRepoName", diggerRepoName)
	return repo, org, nil
}
