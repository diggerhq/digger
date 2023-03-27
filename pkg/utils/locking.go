package utils

import (
	"cloud.google.com/go/storage"
	"digger/pkg/aws"
	"digger/pkg/gcp"
	"digger/pkg/github"
	"errors"
	"fmt"
	awssdk "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"log"
	"os"
	"strconv"
	"strings"
)

type ProjectLockImpl struct {
	InternalLock Lock
	PrManager    github.PullRequestManager
	ProjectName  string
	RepoName     string
}

type Lock interface {
	Lock(transactionId int, resource string) (bool, error)
	Unlock(resource string) (bool, error)
	GetLock(resource string) (*int, error)
}

type ProjectLock interface {
	Lock(lockId string, prNumber int) (bool, error)
	Unlock(lockId string, prNumber int) (bool, error)
	ForceUnlock(lockId string, prNumber int)
}

func (projectLock *ProjectLockImpl) Lock(lockId string, prNumber int) (bool, error) {
	fmt.Printf("Lock %s\n", lockId)
	transactionId, err := projectLock.InternalLock.GetLock(lockId)
	var transactionIdStr string

	if err != nil {
		return false, err
	}

	if transactionId != nil {
		transactionIdStr := strconv.Itoa(*transactionId)
		if *transactionId != prNumber {
			comment := "Project " + projectLock.ProjectName + " locked by another PR #" + transactionIdStr + "(failed to acquire lock " + projectLock.ProjectName + "). The locking plan must be applied or discarded before future plans can execute"
			projectLock.PrManager.PublishComment(prNumber, comment)
			return false, nil
		}
		comment := "Project " + projectLock.ProjectName + " locked by this PR #" + transactionIdStr + " already."
		projectLock.PrManager.PublishComment(prNumber, comment)
		return true, nil
	}

	lockAcquired, err := projectLock.InternalLock.Lock(prNumber, lockId)
	if err != nil {
		return false, err
	}

	if lockAcquired {
		comment := "Project " + projectLock.ProjectName + " has been locked by PR #" + strconv.Itoa(prNumber)
		projectLock.PrManager.PublishComment(prNumber, comment)
		println("project " + projectLock.ProjectName + " locked successfully. PR # " + strconv.Itoa(prNumber))
		return true, nil
	}

	transactionId, _ = projectLock.InternalLock.GetLock(lockId)
	transactionIdStr = strconv.Itoa(*transactionId)

	comment := "Project " + projectLock.ProjectName + " locked by another PR #" + transactionIdStr + " (failed to acquire lock " + projectLock.RepoName + "). The locking plan must be applied or discarded before future plans can execute"
	projectLock.PrManager.PublishComment(prNumber, comment)
	println(comment)
	return false, nil
}

func (projectLock *ProjectLockImpl) Unlock(lockId string, prNumber int) (bool, error) {
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
				comment := "Project unlocked (" + projectLock.ProjectName + ")."
				projectLock.PrManager.PublishComment(prNumber, comment)
				println("Project unlocked")
				return true, nil
			}
		}
	}
	return false, nil
}

func (projectLock *ProjectLockImpl) ForceUnlock(lockId string, prNumber int) {
	fmt.Printf("ForceUnlock %s\n", lockId)
	lock, _ := projectLock.InternalLock.GetLock(lockId)
	if lock != nil {
		lockReleased, _ := projectLock.InternalLock.Unlock(lockId)

		if lockReleased {
			comment := "Project unlocked (" + projectLock.ProjectName + ")."
			projectLock.PrManager.PublishComment(prNumber, comment)
			println("Project unlocked")
		}
	}
}

func GetLock() (Lock, error) {
	awsRegion := strings.ToLower(os.Getenv("AWS_REGION"))
	awsProfile := strings.ToLower(os.Getenv("AWS_PROFILE"))
	lockProvider := strings.ToLower(os.Getenv("LOCK_PROVIDER"))
	if lockProvider == "" || lockProvider == "aws" {
		sess, err := session.NewSessionWithOptions(session.Options{
			Profile: awsProfile,
			Config: awssdk.Config{
				Region: awssdk.String(awsRegion),
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
		bucket := client.Bucket(bucketName)
		lock := gcp.GoogleStorageLock{client, bucket, ctx}

		return &lock, nil

	}
	return nil, errors.New("failed to find lock provider")
}
