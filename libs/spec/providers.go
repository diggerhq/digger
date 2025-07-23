package spec

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	backend2 "github.com/diggerhq/digger/libs/backendapi"
	"github.com/diggerhq/digger/libs/ci"
	"github.com/diggerhq/digger/libs/ci/bitbucket"
	"github.com/diggerhq/digger/libs/ci/github"
	"github.com/diggerhq/digger/libs/ci/gitlab"
	"github.com/diggerhq/digger/libs/comment_utils/reporting"
	"github.com/diggerhq/digger/libs/locking"
	"github.com/diggerhq/digger/libs/locking/aws"
	"github.com/diggerhq/digger/libs/locking/aws/envprovider"
	"github.com/diggerhq/digger/libs/locking/azure"
	"github.com/diggerhq/digger/libs/locking/gcp"
	policy2 "github.com/diggerhq/digger/libs/policy"
	"github.com/diggerhq/digger/libs/scheduler"
	storage2 "github.com/diggerhq/digger/libs/storage"
	"github.com/samber/lo"
)

type JobSpecProvider struct{}

func (j JobSpecProvider) GetJob(jobSpec scheduler.JobJson) (scheduler.Job, error) {
	return scheduler.JsonToJob(jobSpec), nil
}

type LockProvider struct{}

func (l LockProvider) GetLock(lockSpec LockSpec) (locking.Lock, error) {
	if lockSpec.LockType == "noop" {
		slog.Debug("Using NoOp lock provider")
		return locking.NoOpLock{}, nil
	}
	if lockSpec.LockType == "cloud" {
		switch lockSpec.LockProvider {
		case "aws":
			slog.Info("Using AWS lock provider")

			// https://aws.github.io/aws-sdk-go-v2/docs/configuring-sdk/
			// https://aws.github.io/aws-sdk-go-v2/docs/migrating/
			envNamesToCheck := []string{"DIGGER_AWS_ACCESS_KEY_ID", "AWS_ACCESS_KEY_ID", "AWS_ACCESS_KEY"}
			awsCredsProvided := lo.Reduce(envNamesToCheck, func(agg bool, envName string, index int) bool {
				_, exists := os.LookupEnv(envName)
				return agg || exists
			}, false)

			awsRegion := strings.ToLower(os.Getenv("AWS_REGION"))
			awsProfile := strings.ToLower(os.Getenv("AWS_PROFILE"))

			var cfg awssdk.Config
			var err error
			if awsCredsProvided {
				slog.Debug("Using AWS credentials from environment variables")
				cfg, err = config.LoadDefaultConfig(context.Background(),
					config.WithSharedConfigProfile(awsProfile),
					config.WithRegion(awsRegion),
					config.WithCredentialsProvider(&envprovider.EnvProvider{}))
				if err != nil {
					slog.Error("Failed to load AWS config with credentials provider", "error", err)
					return nil, err
				}
			} else {
				slog.Debug("Using keyless AWS configuration")
				cfg, err = config.LoadDefaultConfig(context.Background(), config.WithRegion(awsRegion))
				if err != nil {
					slog.Error("Failed to load AWS config", "error", err)
					return nil, err
				}
			}

			stsClient := sts.NewFromConfig(cfg)
			result, err := stsClient.GetCallerIdentity(context.Background(), &sts.GetCallerIdentityInput{})
			if err != nil {
				slog.Error("Failed to connect to AWS account", "error", err)
				return nil, fmt.Errorf("failed to connect to AWS account. %v", err)
			}
			slog.Info("Successfully connected to AWS account",
				"accountId", *result.Account,
				"userId", *result.UserId)

			dynamoDb := dynamodb.NewFromConfig(cfg)
			dynamoDbLock := aws.DynamoDbLock{DynamoDb: dynamoDb}
			return &dynamoDbLock, nil

		case "gcp":
			slog.Info("Using GCP lock provider")
			ctx, client := gcp.GetGoogleStorageClient()
			defer func(client *storage.Client) {
				err := client.Close()
				if err != nil {
					slog.Error("Failed to close Google Storage client", "error", err)
				}
			}(client)

			bucketName := strings.ToLower(os.Getenv("GOOGLE_STORAGE_LOCK_BUCKET"))
			if bucketName == "" {
				slog.Error("GOOGLE_STORAGE_LOCK_BUCKET environment variable not set")
				return nil, errors.New("GOOGLE_STORAGE_LOCK_BUCKET is not set")
			}
			bucket := client.Bucket(bucketName)
			lock := gcp.GoogleStorageLock{Client: client, Bucket: bucket, Context: ctx}
			return &lock, nil

		case "azure":
			slog.Info("Using Azure lock provider")
			return azure.NewStorageAccountLock()

		}
	}
	slog.Error("Could not determine lock provider",
		"lockType", lockSpec.LockType,
		"lockProvider", lockSpec.LockProvider)
	return nil, fmt.Errorf("could not determine lock provider %v, %v", lockSpec.LockType, lockSpec.LockProvider)
}

type ReporterProvider struct{}

func (r ReporterProvider) GetReporter(title string, reporterSpec ReporterSpec, ciService ci.PullRequestService, prNumber int, vcs string) (reporting.Reporter, error) {
	slog.Debug("Getting reporter",
		"title", title,
		"reporterType", reporterSpec.ReporterType,
		"strategy", reporterSpec.ReportingStrategy,
		"prNumber", prNumber,
		"vcs", vcs)

	getStrategy := func(strategy string) reporting.ReportStrategy {
		switch reporterSpec.ReportingStrategy {
		case "comments_per_run":
			return reporting.CommentPerRunStrategy{
				Title:     title,
				TimeOfRun: time.Now(),
			}
		case "latest_run_comment":
			return reporting.LatestRunCommentStrategy{
				TimeOfRun: time.Now(),
			}
		default:
			return reporting.MultipleCommentsStrategy{}
		}
	}

	isSupportMarkdown := !(vcs == "bitbucket")

	switch reporterSpec.ReporterType {
	case "noop":
		slog.Debug("Using NoOp reporter")
		return reporting.NoopReporter{}, nil
	case "basic":
		strategy := getStrategy(reporterSpec.ReportingStrategy)
		slog.Debug("Using basic CI reporter")
		return reporting.CiReporter{
			CiService:         ciService,
			PrNumber:          prNumber,
			IsSupportMarkdown: isSupportMarkdown,
			ReportStrategy:    strategy,
		}, nil
	case "lazy":
		strategy := getStrategy(reporterSpec.ReportingStrategy)
		ciReporter := reporting.CiReporter{
			CiService:         ciService,
			PrNumber:          prNumber,
			IsSupportMarkdown: isSupportMarkdown,
			ReportStrategy:    strategy,
		}
		slog.Debug("Using lazy CI reporter")
		return reporting.NewCiReporterLazy(ciReporter), nil
	default:
		slog.Warn("Unknown reporter type, falling back to NoOp reporter",
			"reporterType", reporterSpec.ReporterType)
		return reporting.NoopReporter{}, nil
	}
}

type BackendApiProvider struct{}

func (b BackendApiProvider) GetBackendApi(backendSpec BackendSpec) (backend2.Api, error) {
	slog.Debug("Getting backend API", "backendType", backendSpec.BackendType)

	switch backendSpec.BackendType {
	case "noop":
		slog.Debug("Using NoOp backend API")
		return backend2.NoopApi{}, nil
	case "backend":
		slog.Debug("Using Digger backend API", "hostname", backendSpec.BackendHostname)
		return backend2.NewBackendApi(backendSpec.BackendHostname, backendSpec.BackendJobToken), nil
	default:
		slog.Warn("Unknown backend type, falling back to NoOp backend API",
			"backendType", backendSpec.BackendType)
		return backend2.NoopApi{}, nil
	}
}

type VCSProvider interface {
	GetPrService(vcsSpec VcsSpec) (ci.PullRequestService, error)
	GetOrgService(vcsSpec VcsSpec) (ci.OrgService, error)
}

type VCSProviderBasic struct{}

func (v VCSProviderBasic) GetPrService(vcsSpec VcsSpec) (ci.PullRequestService, error) {
	slog.Debug("Getting PR service",
		"vcsType", vcsSpec.VcsType,
		"repoOwner", vcsSpec.RepoOwner,
		"repoName", vcsSpec.RepoName)

	switch vcsSpec.VcsType {
	case "noop":
		slog.Debug("Using mock PR service")
		return ci.MockPullRequestManager{}, nil
	case "github":
		token := os.Getenv("GITHUB_TOKEN")
		if token == "" {
			slog.Error("GITHUB_TOKEN environment variable not set")
			return nil, fmt.Errorf("failed to get github service: GITHUB_TOKEN not specified")
		}
		slog.Debug("Using GitHub PR service")
		return github.GithubServiceProviderBasic{}.NewService(token, vcsSpec.RepoName, vcsSpec.RepoOwner)
	case "bitbucket":
		token := os.Getenv("DIGGER_BITBUCKET_ACCESS_TOKEN")
		if token == "" {
			slog.Error("DIGGER_BITBUCKET_ACCESS_TOKEN environment variable not set")
			return nil, fmt.Errorf("failed to get bitbucket service: DIGGER_BITBUCKET_ACCESS_TOKEN not specified")
		}
		slog.Debug("Using Bitbucket PR service")
		return bitbucket.BitbucketAPI{
			AuthToken:     token,
			HttpClient:    http.Client{},
			RepoWorkspace: vcsSpec.RepoOwner,
			RepoName:      vcsSpec.RepoName,
		}, nil
	case "gitlab":
		token := os.Getenv("GITLAB_TOKEN")
		if token == "" {
			slog.Error("GITLAB_TOKEN environment variable not set")
			return nil, fmt.Errorf("failed to get gitlab service: GITLAB_TOKEN not specified")
		}
		context, err := gitlab.ParseGitLabContext()
		if err != nil {
			slog.Error("Failed to parse GitLab context", "error", err)
			return nil, fmt.Errorf("failed to get gitlab service, could not parse context: %v", err)
		}
		slog.Debug("Using GitLab PR service")
		return gitlab.NewGitLabService(token, context, "")

	default:
		slog.Error("Unknown VCS type", "vcsType", vcsSpec.VcsType)
		return nil, fmt.Errorf("could not get PRService, unknown type %v", vcsSpec.VcsType)
	}
}

func (v VCSProviderBasic) GetOrgService(vcsSpec VcsSpec) (ci.OrgService, error) {
	slog.Debug("Getting organization service",
		"vcsType", vcsSpec.VcsType,
		"repoOwner", vcsSpec.RepoOwner,
		"repoName", vcsSpec.RepoName)

	switch vcsSpec.VcsType {
	case "noop":
		slog.Debug("Using mock organization service")
		return ci.MockPullRequestManager{}, nil
	case "github":
		token := os.Getenv("GITHUB_TOKEN")
		if token == "" {
			slog.Error("GITHUB_TOKEN environment variable not set")
			return nil, fmt.Errorf("failed to get github service: GITHUB_TOKEN not specified")
		}
		slog.Debug("Using GitHub organization service")
		return github.GithubServiceProviderBasic{}.NewService(token, vcsSpec.RepoName, vcsSpec.RepoOwner)
	case "bitbucket":
		token := os.Getenv("DIGGER_BITBUCKET_ACCESS_TOKEN")
		if token == "" {
			slog.Error("DIGGER_BITBUCKET_ACCESS_TOKEN environment variable not set")
			return nil, fmt.Errorf("failed to get bitbucket service: GITLAB_TOKEN not specified")
		}
		slog.Debug("Using Bitbucket organization service")
		return bitbucket.BitbucketAPI{
			AuthToken:     token,
			HttpClient:    http.Client{},
			RepoWorkspace: vcsSpec.RepoOwner,
			RepoName:      vcsSpec.RepoName,
		}, nil
	case "gitlab":
		token := os.Getenv("GITLAB_TOKEN")
		if token == "" {
			slog.Error("GITLAB_TOKEN environment variable not set")
			return nil, fmt.Errorf("failed to get gitlab service: GITLAB_TOKEN not specified")
		}
		context, err := gitlab.ParseGitLabContext()
		if err != nil {
			slog.Error("Failed to parse GitLab context", "error", err)
			return nil, fmt.Errorf("failed to get gitlab service, could not parse context: %v", err)
		}
		slog.Debug("Using GitLab organization service")
		return gitlab.NewGitLabService(token, context, "")
	default:
		slog.Error("Unknown VCS type", "vcsType", vcsSpec.VcsType)
		return nil, fmt.Errorf("could not get PRService, unknown type %v", vcsSpec.VcsType)
	}
}

type SpecPolicyProvider interface {
	GetPolicyProvider(policySpec PolicySpec, diggerHost, diggerOrg, token, vcsType string) (policy2.Checker, error)
}

type BasicPolicyProvider struct{}

func (p BasicPolicyProvider) GetPolicyProvider(policySpec PolicySpec, diggerHost, diggerOrg, token, vcsType string) (policy2.Checker, error) {
	slog.Debug("Getting policy provider",
		"policyType", policySpec.PolicyType,
		"diggerHost", diggerHost,
		"diggerOrg", diggerOrg)

	switch policySpec.PolicyType {
	case "http":
		slog.Debug("Using HTTP policy provider")
		return policy2.DiggerPolicyChecker{
			PolicyProvider: policy2.DiggerHttpPolicyProvider{
				DiggerHost:         diggerHost,
				DiggerOrganisation: diggerOrg,
				AuthToken:          token,
				HttpClient:         http.DefaultClient,
			},
		}, nil
	default:
		slog.Error("Unknown policy type", "policyType", policySpec.PolicyType)
		return nil, fmt.Errorf("unknown policy type: %s", policySpec.PolicyType)
	}
}

type PlanStorageProvider struct{}

func (p PlanStorageProvider) GetPlanStorage(repoOwner, repositoryName string, prNumber int) (storage2.PlanStorage, error) {
	slog.Debug("Getting plan storage",
		"repoOwner", repoOwner,
		"repositoryName", repositoryName,
		"prNumber", prNumber)

	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		slog.Warn("GITHUB_TOKEN not set for plan storage")
	}

	storage, err := storage2.NewPlanStorage(token, repoOwner, repositoryName, &prNumber)
	if err != nil {
		slog.Error("Failed to create plan storage", "error", err)
	} else {
		slog.Debug("Successfully created plan storage")
	}
	return storage, err
}
