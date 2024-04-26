package cli_e2e

import (
	"os"
	"testing"

	"github.com/diggerhq/digger/cli/pkg/locking"
)

func TestAWSDynamoDBLockE2E(t *testing.T) {
	// Requires AWS login
	os.Setenv("AWS_REGION", "us-east-1")
	os.Setenv("LOCK_PROVIDER", "aws")

	lock, err := locking.GetLock()
	if err != nil {
		t.Errorf("failed to get locking provider: %v\n", err)
	}
	if lock != nil {
		lock.Unlock("test")
	}

	lockID, err := lock.GetLock("test")
	if err != nil {
		t.Errorf("failed to get lock: %v\n", err)
	}
	t.Logf("lockID: %v\n", lockID)

	locked, err := lock.Lock(1, "test")
	if err != nil || locked != true {
		t.Errorf("failed to lock: %v, locked: %v\n", err, locked)
	}

	lockID2, err := lock.GetLock("test")
	if err != nil {
		t.Errorf("failed to get lock: %v\n", err)
	}
	if lockID2 == nil {
		t.Errorf("lock is nil while it should be set\n")
	}
	locked, err = lock.Lock(1, "test")
	if err != nil {
		t.Errorf("failed to lock a second time, but not due to condition: %v\n", err)
	}
	if locked != false {
		t.Errorf("locked: %v should have been locked\n", locked)
	}

	unlocked, err := lock.Unlock("test")
	if err != nil || unlocked != true {
		t.Errorf("failed to unlock: %v, unlocked: %v\n", err, unlocked)
	}
	if !unlocked {
		t.Logf("lock has not been unlocked\n")
	}
}
