package spec

import (
	"cloud.google.com/go/storage"
	"context"
	"errors"
	"fmt"
	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	backend2 "github.com/diggerhq/digger/libs/backendapi"
	"github.com/diggerhq/digger/libs/ci"
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
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type JobSpecProvider struct{}

func (j JobSpecProvider) GetJob(jobSpec scheduler.JobJson) (scheduler.Job, error) {
	return scheduler.JsonToJob(jobSpec), nil
}

type LockProvider struct{}

func (l LockProvider) GetLock(lockSpec LockSpec) (locking.Lock, error) {
	if lockSpec.LockType == "noop" {
		return locking.NoOpLock{}, nil
	}
	if lockSpec.LockType == "cloud" {
		switch lockSpec.LockProvider {
		case "aws":
			log.Println("Using AWS lock provider.")

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
				cfg, err = config.LoadDefaultConfig(context.Background(),
					config.WithSharedConfigProfile(awsProfile),
					config.WithRegion(awsRegion),
					config.WithCredentialsProvider(&envprovider.EnvProvider{}))
				if err != nil {
					return nil, err
				}
			} else {
				log.Printf("Using keyless aws digger_config\n")
				cfg, err = config.LoadDefaultConfig(context.Background(), config.WithRegion(awsRegion))
				if err != nil {
					return nil, err
				}
			}

			stsClient := sts.NewFromConfig(cfg)
			result, err := stsClient.GetCallerIdentity(context.Background(), &sts.GetCallerIdentityInput{})
			if err != nil {
				return nil, fmt.Errorf("failed to connect to AWS account. %v", err)
			}
			log.Printf("Successfully connected to AWS account %s, user Id: %s\n", *result.Account, *result.UserId)

			dynamoDb := dynamodb.NewFromConfig(cfg)
			dynamoDbLock := aws.DynamoDbLock{DynamoDb: dynamoDb}
			return &dynamoDbLock, nil
		case "gcp":
			log.Println("Using GCP lock provider.")
			ctx, client := gcp.GetGoogleStorageClient()
			defer func(client *storage.Client) {
				err := client.Close()
				if err != nil {
					log.Fatalf("Failed to close Google Storage client: %v", err)
				}
			}(client)

			bucketName := strings.ToLower(os.Getenv("GOOGLE_STORAGE_LOCK_BUCKET"))
			if bucketName == "" {
				return nil, errors.New("GOOGLE_STORAGE_LOCK_BUCKET is not set")
			}
			bucket := client.Bucket(bucketName)
			lock := gcp.GoogleStorageLock{Client: client, Bucket: bucket, Context: ctx}
			return &lock, nil
		case "azure":
			return azure.NewStorageAccountLock()

		}
	}
	return nil, fmt.Errorf("could not determine lock provider %v, %v", lockSpec.LockType, lockSpec.LockProvider)
}

type ReporterProvider struct{}

func (r ReporterProvider) GetReporter(reporterSpec ReporterSpec, ciService ci.PullRequestService, prNumber int) (reporting.Reporter, error) {
	getStrategy := func(strategy string) reporting.ReportStrategy {
		switch reporterSpec.ReportingStrategy {
		case "comments_per_run":
			return reporting.CommentPerRunStrategy{
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

	switch reporterSpec.ReporterType {
	case "noop":
		return reporting.NoopReporter{}, nil
	case "basic":
		strategy := getStrategy(reporterSpec.ReportingStrategy)
		return reporting.CiReporter{
			CiService:         ciService,
			PrNumber:          prNumber,
			IsSupportMarkdown: true,
			ReportStrategy:    strategy,
		}, nil
	case "lazy":
		strategy := getStrategy(reporterSpec.ReportingStrategy)
		ciReporter := reporting.CiReporter{
			CiService:         ciService,
			PrNumber:          prNumber,
			IsSupportMarkdown: true,
			ReportStrategy:    strategy,
		}
		return reporting.NewCiReporterLazy(ciReporter), nil
	default:
		return reporting.NoopReporter{}, nil
	}
}

type BackendApiProvider struct{}

func (b BackendApiProvider) GetBackendApi(backendSpec BackendSpec) (backend2.Api, error) {
	switch backendSpec.BackendType {
	case "noop":
		return backend2.NoopApi{}, nil
	case "backend":
		return backend2.NewBackendApi(backendSpec.BackendHostname, backendSpec.BackendJobToken), nil
	default:
		return backend2.NoopApi{}, nil
	}
}

type VCSProvider struct{}

func (v VCSProvider) GetPrService(vcsSpec VcsSpec) (ci.PullRequestService, error) {
	switch vcsSpec.VcsType {
	case "github":
		token := os.Getenv("GITHUB_TOKEN")
		if token == "" {
			return nil, fmt.Errorf("failed to get github service: GITHUB_TOKEN not specified")
		}
		return github.GithubServiceProviderBasic{}.NewService(token, vcsSpec.RepoName, vcsSpec.RepoOwner)
	case "gitlab":
		token := os.Getenv("GITLAB_TOKEN")
		if token == "" {
			return nil, fmt.Errorf("failed to get gitlab service: GITLAB_TOKEN not specified")
		}
		context, err := gitlab.ParseGitLabContext()
		if err != nil {
			return nil, fmt.Errorf("failed to get gitlab service, could not parse context: %v", err)
		}
		return gitlab.NewGitLabService(token, context)
	default:
		return nil, fmt.Errorf("could not get PRService, unknown type %v", vcsSpec.VcsType)
	}
}

func (v VCSProvider) GetOrgService(vcsSpec VcsSpec) (ci.OrgService, error) {
	switch vcsSpec.VcsType {
	case "github":
		token := os.Getenv("GITHUB_TOKEN")
		if token == "" {
			return nil, fmt.Errorf("failed to get github service: GITHUB_TOKEN not specified")
		}
		return github.GithubServiceProviderBasic{}.NewService(token, vcsSpec.RepoName, vcsSpec.RepoOwner)
	case "gitlab":
		token := os.Getenv("GITLAB_TOKEN")
		if token == "" {
			return nil, fmt.Errorf("failed to get gitlab service: GITLAB_TOKEN not specified")
		}
		context, err := gitlab.ParseGitLabContext()
		if err != nil {
			return nil, fmt.Errorf("failed to get gitlab service, could not parse context: %v", err)
		}
		return gitlab.NewGitLabService(token, context)
	default:
		return nil, fmt.Errorf("could not get PRService, unknown type %v", vcsSpec.VcsType)
	}

}

type SpecPolicyProvider interface {
	GetPolicyProvider(policySpec PolicySpec, diggerHost string, diggerOrg string, token string) (policy2.Checker, error)
}

type BasicPolicyProvider struct{}

func (p BasicPolicyProvider) GetPolicyProvider(policySpec PolicySpec, diggerHost string, diggerOrg string, token string) (policy2.Checker, error) {
	switch policySpec.PolicyType {
	case "http":
		return policy2.DiggerPolicyChecker{
			PolicyProvider: policy2.DiggerHttpPolicyProvider{
				DiggerHost:         diggerHost,
				DiggerOrganisation: diggerOrg,
				AuthToken:          token,
				HttpClient:         http.DefaultClient,
			},
		}, nil
	default:
		return nil, fmt.Errorf("could not find ")
	}
}

type PlanStorageProvider struct{}

func (p PlanStorageProvider) GetPlanStorage(repoOwner string, repositoryName string, prNumber int) (storage2.PlanStorage, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("failed to get github service: GITHUB_TOKEN not specified")
	}
	storage, err := storage2.NewPlanStorage(token, repoOwner, repositoryName, &prNumber)
	return storage, err
}
