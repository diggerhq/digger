package utils

import (
	"digger/pkg/aws"
	"digger/pkg/github"
	"fmt"
	"strconv"
)

type ProjectLockImpl struct {
	InternalLock aws.Lock
	PrManager    github.PullRequestManager
	ProjectName  string
	RepoName     string
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
		println("transactionId: " + strconv.Itoa(*transactionId))
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

	lockAcquired, err := projectLock.InternalLock.Lock(60*24, prNumber, lockId)
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
