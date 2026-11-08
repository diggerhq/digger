package models

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestWithRepositoryPRLocksPostgres(t *testing.T) {
	url := os.Getenv("DIGGER_TEST_POSTGRES_URL")
	if url == "" {
		t.Skip("DIGGER_TEST_POSTGRES_URL not set")
	}
	gdb, err := gorm.Open(postgres.Open(url), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, gdb.AutoMigrate(&Organisation{}, &DiggerLock{}))
	db := &Database{GormDB: gdb}
	org, err := db.CreateOrganisation("repo-lock", "test", "repo-lock-"+time.Now().Format(time.RFC3339Nano), nil)
	require.NoError(t, err)

	t.Run("a failure rolls back its locks", func(t *testing.T) {
		err := db.WithRepositoryPRLocks(context.Background(), "o/r", time.Minute, func(tx *Database) error {
			_, err := tx.CreateDiggerLock("o/r#rolled-back", 1, org.ID)
			require.NoError(t, err)
			return errors.New("conflict")
		})
		require.EqualError(t, err, "conflict")
		_, err = db.GetDiggerLock("o/r#rolled-back")
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})

	t.Run("a holder that stalls loses its locks", func(t *testing.T) {
		err := db.WithRepositoryPRLocks(context.Background(), "o/r", 200*time.Millisecond, func(tx *Database) error {
			_, err := tx.CreateDiggerLock("o/r#stalled", 1, org.ID)
			require.NoError(t, err)
			time.Sleep(time.Second)
			return nil
		})
		require.Error(t, err)
		_, err = db.GetDiggerLock("o/r#stalled")
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})

	t.Run("a panic rolls back and frees the lock", func(t *testing.T) {
		assert.Panics(t, func() {
			_ = db.WithRepositoryPRLocks(context.Background(), "o/r", time.Minute, func(tx *Database) error {
				_, err := tx.CreateDiggerLock("o/r#panicked", 1, org.ID)
				require.NoError(t, err)
				panic("boom")
			})
		})
		_, err := db.GetDiggerLock("o/r#panicked")
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		assert.NoError(t, db.WithRepositoryPRLocks(ctx, "o/r", time.Minute, func(*Database) error { return nil }))
	})

	t.Run("the timeout covers waiting for a connection", func(t *testing.T) {
		busy, err := gorm.Open(postgres.Open(url), &gorm.Config{})
		require.NoError(t, err)
		sqlDB, err := busy.DB()
		require.NoError(t, err)
		sqlDB.SetMaxOpenConns(1)
		hold := busy.Begin()
		require.NoError(t, hold.Error)
		defer hold.Rollback()

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		err = (&Database{GormDB: busy}).WithRepositoryPRLocks(ctx, "o/r", time.Minute, func(*Database) error { return nil })
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	})
}
