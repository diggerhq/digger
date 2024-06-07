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
	backend2 "github.com/diggerhq/digger/cli/pkg/backend"
	"github.com/diggerhq/digger/cli/pkg/core/backend"
	"github.com/diggerhq/digger/libs/comment_utils/reporting"
	"github.com/diggerhq/digger/libs/locking"
	"github.com/diggerhq/digger/libs/locking/aws"
	"github.com/diggerhq/digger/libs/locking/aws/envprovider"
	"github.com/diggerhq/digger/libs/locking/azure"
	"github.com/diggerhq/digger/libs/locking/gcp"
	"github.com/diggerhq/digger/libs/orchestrator"
	"github.com/diggerhq/digger/libs/orchestrator/github"
	"github.com/samber/lo"
	"log"
	"os"
	"strings"
	"time"
)

type JobSpecProvider struct{}

func (j JobSpecProvider) GetJob(jobSpec orchestrator.JobJson) (orchestrator.Job, error) {
	return orchestrator.JsonToJob(jobSpec), nil
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

func (r ReporterProvider) GetReporter(reporterSpec ReporterSpec, ciService orchestrator.PullRequestService, prNumber int) (reporting.Reporter, error) {
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
	default:
		return reporting.NoopReporter{}, nil
	}
}

type BackendApiProvider struct{}

func (b BackendApiProvider) GetBackendApi(backendSpec BackendSpec) (backend.Api, error) {
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

func (v VCSProvider) GetPrService(vcsSpec VcsSpec) (orchestrator.PullRequestService, error) {
	switch vcsSpec.VcsType {
	case "github":
		token := os.Getenv("GITHUB_TOKEN")
		if token == "" {
			return nil, fmt.Errorf("failed to get githbu service: GITHUB_TOKEN not specified")
		}
		return github.NewGitHubService(token, vcsSpec.RepoName, vcsSpec.RepoOwner), nil
	default:
		return nil, fmt.Errorf("could not get PRService, unknown type %v", vcsSpec.VcsType)
	}
}
