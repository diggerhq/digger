package locking

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/diggerhq/digger/backend/models"
	"github.com/diggerhq/digger/libs/ci"
	"github.com/diggerhq/digger/libs/digger_config"
	"github.com/diggerhq/digger/libs/locking"
	"github.com/diggerhq/digger/libs/scheduler"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Needs a scratch Postgres: DIGGER_TEST_POSTGRES_URL=postgres://...
func TestPerformPRLockingActionPostgresRace(t *testing.T) {
	url := os.Getenv("DIGGER_TEST_POSTGRES_URL")
	if url == "" {
		t.Skip("DIGGER_TEST_POSTGRES_URL not set")
	}
	gdb, err := gorm.Open(postgres.Open(url), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, gdb.AutoMigrate(&models.Organisation{}, &models.DiggerLock{}))
	prev := models.DB
	models.DB = &models.Database{GormDB: gdb}
	t.Cleanup(func() { models.DB = prev })
	org, err := models.DB.CreateOrganisation("lock-race", "test", uuid.NewString(), nil)
	require.NoError(t, err)

	config := &digger_config.DiggerConfig{PrLocks: true}
	a, b := digger_config.Project{Name: "a"}, digger_config.Project{Name: "b"}
	orders := [][]digger_config.Project{{a, b}, {b, a}}
	for i := 0; i < 20; i++ {
		repo := fmt.Sprintf("o/race-%s", uuid.NewString())
		var wg sync.WaitGroup
		var errs [2]error
		for pr := 1; pr <= 2; pr++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				errs[pr-1] = PerformPRLockingAction(config, org.ID, ci.MockPullRequestManager{}, repo, pr, orders[pr-1], scheduler.DiggerCommandPlan)
			}()
		}
		wg.Wait()

		winner := 1
		if errs[0] != nil {
			winner = 2
		}
		require.True(t, (errs[0] == nil) != (errs[1] == nil), "exactly one PR should get its locks: %v", errs)
		for _, p := range []string{"a", "b"} {
			holder, err := BackendDBLock{OrgId: org.ID, DB: models.DB}.GetLock(repo + "#" + p)
			require.NoError(t, err)
			require.NotNil(t, holder, "project %s", p)
			assert.Equal(t, winner, *holder, "project %s", p)
		}
	}
}

func TestLockFailureComment(t *testing.T) {
	held := fmt.Errorf("project b: failed to lock project: Project o/r#b %w #2(failed to acquire lock b)", locking.ErrLockedByAnotherPR)
	assert.Equal(t, ":x: Failed perform lock action on project b: failed to lock project: Project o/r#b locked by another PR #2(failed to acquire lock b)", LockFailureComment(held))
	assert.Contains(t, LockFailureComment(context.DeadlineExceeded), "still taking its project locks")
	assert.Equal(t, ":x: Couldn't check project locks, so this request was skipped. Try again shortly.", LockFailureComment(errors.New("FATAL: terminating connection")))
}
