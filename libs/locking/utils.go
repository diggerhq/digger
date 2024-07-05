package locking

import (
	"fmt"
	"github.com/diggerhq/digger/libs/scheduler"
)

func PerformLockingActionFromCommand(prLock PullRequestLock, command scheduler.DiggerCommand) error {
	var err error
	switch command {
	case scheduler.DiggerCommandUnlock:
		_, err = prLock.Unlock()
		if err != nil {
			err = fmt.Errorf("failed to unlock project: %v", err)
		}
	case scheduler.DiggerCommandPlan:
		_, err = prLock.Lock()
		if err != nil {
			err = fmt.Errorf("failed to lock project: %v", err)
		}
	case scheduler.DiggerCommandApply:
		_, err = prLock.Lock()
		if err != nil {
			err = fmt.Errorf("failed to lock project: %v", err)
		}
	case scheduler.DiggerCommandLock:
		_, err = prLock.Lock()
		if err != nil {
			err = fmt.Errorf("failed to lock project: %v", err)
		}
	}
	return err
}
