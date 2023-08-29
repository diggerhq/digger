package locking

import (
	"digger/pkg/aws"
	"digger/pkg/aws/envprovider"
	"digger/pkg/azure"
	"digger/pkg/core/locking"
	"digger/pkg/core/reporting"
	"digger/pkg/core/utils"
	"digger/pkg/gcp"
	"errors"
	"fmt"
	orchestrator "github.com/diggerhq/lib-orchestrator"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/service/sts"

	"cloud.google.com/go/storage"
	awssdk "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
)

type PullRequestLock struct {
	InternalLock     locking.Lock
	CIService        orchestrator.PullRequestService
	Reporter         reporting.Reporter
	ProjectName      string
	ProjectNamespace string
	PrNumber         int
}

type NoOpLock struct {
}

func (noOpLock *NoOpLock) Lock(transactionId int, resource string) (bool, error) {
	return true, nil
}

func (noOpLock *NoOpLock) Unlock(resource string) (bool, error) {
	return true, nil
}

func (noOpLock *NoOpLock) GetLock(resource string) (*int, error) {
	return nil, nil
}

func (projectLock *PullRequestLock) Lock() (bool, error) {
	lockId := projectLock.LockId()
	fmt.Printf("Lock %s\n", lockId)

	noHangingLocks, err := projectLock.verifyNoHangingLocks()

	if err != nil {
		return false, err
	}

	if !noHangingLocks {
		return false, nil
	}

	existingLockTransactionId, err := projectLock.InternalLock.GetLock(lockId)
	if err != nil {
		fmt.Printf("failed to get lock: %v\n", err)
		return false, err
	}
	if existingLockTransactionId != nil {
		if *existingLockTransactionId == projectLock.PrNumber {
			return true, nil
		} else {
			transactionIdStr := strconv.Itoa(*existingLockTransactionId)
			comment := "Project " + projectLock.projectId() + " locked by another PR #" + transactionIdStr + " (failed to acquire lock " + projectLock.ProjectNamespace + "). The locking plan must be applied or discarded before future plans can execute"

			err = projectLock.Reporter.Report(comment, utils.AsCollapsibleComment("Locking failed"))
			return false, nil
		}
	}
	lockAcquired, err := projectLock.InternalLock.Lock(projectLock.PrNumber, lockId)
	if err != nil {
		return false, err
	}

	_, isNoOpLock := projectLock.InternalLock.(*NoOpLock)

	if lockAcquired && !isNoOpLock {
		comment := "Project " + projectLock.projectId() + " has been locked by PR #" + strconv.Itoa(projectLock.PrNumber)
		err = projectLock.Reporter.Report(comment, utils.AsCollapsibleComment("Locking successful"))
		if err != nil {
			println("failed to publish comment: " + err.Error())
		}
		println("project " + projectLock.projectId() + " locked successfully. PR # " + strconv.Itoa(projectLock.PrNumber))

	}
	return lockAcquired, nil
}

func (projectLock *PullRequestLock) verifyNoHangingLocks() (bool, error) {
	lockId := projectLock.LockId()
	transactionId, err := projectLock.InternalLock.GetLock(lockId)

	if err != nil {
		return false, err
	}

	if transactionId != nil {
		if *transactionId != projectLock.PrNumber {
			isPrClosed, err := projectLock.CIService.IsClosed(*transactionId)
			if err != nil {
				return false, fmt.Errorf("failed to check if PR holding a lock is closed: %w", err)
			}
			if isPrClosed {
				_, err := projectLock.InternalLock.Unlock(lockId)
				if err != nil {
					return false, fmt.Errorf("failed to unlock a lock acquired by closed PR %v: %w", transactionId, err)
				}
				return true, nil
			}
			transactionIdStr := strconv.Itoa(*transactionId)
			comment := "Project " + projectLock.projectId() + " locked by another PR #" + transactionIdStr + "(failed to acquire lock " + projectLock.ProjectName + "). The locking plan must be applied or discarded before future plans can execute"
			err = projectLock.Reporter.Report(comment, utils.AsCollapsibleComment("Locking failed"))
			return false, nil
		}
		return true, nil
	}
	return true, nil
}

func (projectLock *PullRequestLock) Unlock() (bool, error) {
	lockId := projectLock.LockId()
	fmt.Printf("Unlock %s\n", lockId)
	lock, err := projectLock.InternalLock.GetLock(lockId)
	if err != nil {
		return false, err
	}

	if lock != nil {
		transactionId := *lock
		if projectLock.PrNumber == transactionId {
			lockReleased, err := projectLock.InternalLock.Unlock(lockId)
			if err != nil {
				return false, err
			}
			if lockReleased {
				comment := "Project unlocked (" + projectLock.projectId() + ")."
				projectLock.Reporter.Report(comment, utils.AsCollapsibleComment("Unlocking successful"))

				println("Project unlocked")
				return true, nil
			}
		}
	}
	return false, nil
}

func (projectLock *PullRequestLock) ForceUnlock() error {
	lockId := projectLock.LockId()
	fmt.Printf("ForceUnlock %s\n", lockId)
	lock, err := projectLock.InternalLock.GetLock(lockId)
	if err != nil {
		return err
	}
	if lock != nil {
		lockReleased, err := projectLock.InternalLock.Unlock(lockId)
		if err != nil {
			return err
		}

		if lockReleased {
			comment := "Project unlocked (" + projectLock.projectId() + ")."
			projectLock.Reporter.Report(comment, utils.AsCollapsibleComment("Unlocking successful"))
			println("Project unlocked")
		}
		return nil
	}
	return nil
}

func (projectLock *PullRequestLock) projectId() string {
	return projectLock.ProjectNamespace + "#" + projectLock.ProjectName
}

func (projectLock *PullRequestLock) LockId() string {
	return projectLock.ProjectNamespace + "#" + projectLock.ProjectName
}

func GetLock() (locking.Lock, error) {
	awsRegion := strings.ToLower(os.Getenv("AWS_REGION"))
	awsProfile := strings.ToLower(os.Getenv("AWS_PROFILE"))
	lockProvider := strings.ToLower(os.Getenv("LOCK_PROVIDER"))
	disableLocking := strings.ToLower(os.Getenv("DISABLE_LOCKING")) == "true"
	if disableLocking {
		log.Println("Using NoOp lock provider.")
		return &NoOpLock{}, nil
	}
	if lockProvider == "" || lockProvider == "aws" {
		log.Println("Using AWS lock provider.")
		sess, err := session.NewSessionWithOptions(session.Options{
			Profile: awsProfile,
			Config: awssdk.Config{
				Region:      awssdk.String(awsRegion),
				Credentials: credentials.NewCredentials(&envprovider.EnvProvider{}),
			},
		})
		if err != nil {
			return nil, err
		}

		svc := sts.New(sess)
		input := &sts.GetCallerIdentityInput{}
		result, err := svc.GetCallerIdentity(input)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to AWS account. %v\n", err)
		}
		log.Printf("Successfully connected to AWS account %s, user Id: %s\n", *result.Account, *result.UserId)

		dynamoDb := dynamodb.New(sess)
		dynamoDbLock := aws.DynamoDbLock{DynamoDb: dynamoDb}
		return &dynamoDbLock, nil
	} else if lockProvider == "gcp" {
		log.Println("Using GCP lock provider.")
		ctx, client := gcp.GetGoogleStorageClient()
		defer func(client *storage.Client) {
			err := client.Close()
			if err != nil {
				log.Fatalf("Failed to close Google Storage client: %v", err)
			}
		}(client)

		bucketName := strings.ToLower(os.Getenv("GOOGLE_STORAGE_BUCKET"))
		if bucketName == "" {
			return nil, errors.New("GOOGLE_STORAGE_BUCKET is not set")
		}
		bucket := client.Bucket(bucketName)
		lock := gcp.GoogleStorageLock{Client: client, Bucket: bucket, Context: ctx}
		return &lock, nil
	} else if lockProvider == "azure" {
		return azure.NewStorageAccountLock()
	}

	return nil, errors.New("failed to find lock provider")
}
