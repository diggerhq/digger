package models

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Needs a scratch Postgres: DIGGER_TEST_POSTGRES_URL=postgres://...
func TestLockPullRequestCommandsPostgres(t *testing.T) {
	url := os.Getenv("DIGGER_TEST_POSTGRES_URL")
	if url == "" {
		t.Skip("DIGGER_TEST_POSTGRES_URL not set")
	}
	gdb, err := gorm.Open(postgres.Open(url), &gorm.Config{})
	require.NoError(t, err)
	db := &Database{GormDB: gdb}

	release, err := db.LockPullRequestCommands(context.Background(), "o/r", 1, time.Minute)
	require.NoError(t, err)
	t.Cleanup(release)

	// Another PR isn't blocked.
	other, err := db.LockPullRequestCommands(context.Background(), "o/r", 2, time.Minute)
	require.NoError(t, err)
	other()

	// The same PR waits, and gives up when its context ends.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, err = db.LockPullRequestCommands(ctx, "o/r", 1, time.Minute)
	assert.Error(t, err)

	type result struct {
		release func()
		err     error
	}
	acquired := make(chan result)
	go func() {
		r, err := db.LockPullRequestCommands(context.Background(), "o/r", 1, time.Minute)
		acquired <- result{r, err}
	}()
	select {
	case <-acquired:
		t.Fatal("acquired the lock while it was held")
	case <-time.After(300 * time.Millisecond):
	}
	release()
	release()
	select {
	case r := <-acquired:
		require.NoError(t, r.err)
		r.release()
	case <-time.After(5 * time.Second):
		t.Fatal("lock wasn't handed over after release")
	}
}
