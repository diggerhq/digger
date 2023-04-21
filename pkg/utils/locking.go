package utils

import (
	"digger/pkg/aws"
	"digger/pkg/aws/envprovider"
	"digger/pkg/azure"
	"digger/pkg/gcp"
	"digger/pkg/github"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"cloud.google.com/go/storage"
	awssdk "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
)

type ProjectLockImpl struct {
	InternalLock Lock
	PrManager    github.PullRequestManager
	ProjectName  string
	RepoName     string
	RepoOwner    string
}

func (p ProjectLockImpl) LockId() string {
	return p.RepoOwner + "/" + p.RepoName + "#" + p.ProjectName
}

type Lock interface {
	Lock(transactionId int, resource string) (bool, error)
	Unlock(resource string) (bool, error)
	GetLock(resource string) (*int, error)
}

type ProjectLock interface {
	Lock(prNumber int) (bool, error)
	Unlock(prNumber int) (bool, error)
	ForceUnlock(prNumber int) error
	LockId() string
}

func (projectLock *ProjectLockImpl) Lock(prNumber int) (bool, error) {
	lockId := projectLock.LockId()
	fmt.Printf("Lock %s\n", lockId)
	transactionId, err := projectLock.InternalLock.GetLock(lockId)
	var transactionIdStr string

	if err != nil {
		return false, err
	}

	if transactionId != nil {
		transactionIdStr := strconv.Itoa(*transactionId)
		if *transactionId != prNumber {
			comment := "Project " + projectLock.projectId() + " locked by another PR #" + transactionIdStr + "(failed to acquire lock " + projectLock.ProjectName + "). The locking plan must be applied or discarded before future plans can execute"
			projectLock.PrManager.PublishComment(prNumber, comment)
			return false, nil
		}
		return true, nil
	}

	lockAcquired, err := projectLock.InternalLock.Lock(prNumber, lockId)
	if err != nil {
		return false, err
	}

	if lockAcquired {
		comment := "Project " + projectLock.projectId() + " has been locked by PR #" + strconv.Itoa(prNumber)
		projectLock.PrManager.PublishComment(prNumber, comment)
		println("project " + projectLock.projectId() + " locked successfully. PR # " + strconv.Itoa(prNumber))
		return true, nil
	}

	transactionId, _ = projectLock.InternalLock.GetLock(lockId)
	transactionIdStr = strconv.Itoa(*transactionId)

	comment := "Project " + projectLock.projectId() + " locked by another PR #" + transactionIdStr + " (failed to acquire lock " + projectLock.RepoName + "). The locking plan must be applied or discarded before future plans can execute"
	projectLock.PrManager.PublishComment(prNumber, comment)
	println(comment)
	return false, nil
}

func (projectLock *ProjectLockImpl) Unlock(prNumber int) (bool, error) {
	lockId := projectLock.LockId()
	fmt.Printf("Unlock %s\n", lockId)
	lock, err := projectLock.InternalLock.GetLock(lockId)
	if err != nil {
		return false, err
	}

	if lock != nil {
		transactionId := *lock
		if prNumber == transactionId {
			lockReleased, err := projectLock.InternalLock.Unlock(lockId)
			if err != nil {
				return false, err
			}
			if lockReleased {
				comment := "Project unlocked (" + projectLock.projectId() + ")."
				projectLock.PrManager.PublishComment(prNumber, comment)
				println("Project unlocked")
				return true, nil
			}
		}
	}
	return false, nil
}

func (projectLock *ProjectLockImpl) ForceUnlock(prNumber int) error {
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
			projectLock.PrManager.PublishComment(prNumber, comment)
			println("Project unlocked")
		}
		return nil
	}
	return nil
}

func (projectLock *ProjectLockImpl) projectId() string {
	return projectLock.RepoOwner + "/" + projectLock.RepoName + "#" + projectLock.ProjectName
}

func GetLock() (Lock, error) {
	awsRegion := strings.ToLower(os.Getenv("AWS_REGION"))
	awsProfile := strings.ToLower(os.Getenv("AWS_PROFILE"))
	lockProvider := strings.ToLower(os.Getenv("LOCK_PROVIDER"))
	if lockProvider == "" || lockProvider == "aws" {
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
		dynamoDb := dynamodb.New(sess)
		dynamoDbLock := aws.DynamoDbLock{DynamoDb: dynamoDb}
		return &dynamoDbLock, nil
	} else if lockProvider == "gcp" {
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
