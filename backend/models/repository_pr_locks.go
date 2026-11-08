package models

import (
	"context"
	"fmt"
	"hash/fnv"
	"time"
)

// Advisory lock class for repository PR locks, so its keys can't collide
// with other advisory locks.
const repositoryPRLocksClass int32 = 0x44504c4b // "DPLK"

// WithRepositoryPRLocks runs fn in one transaction that also holds a Postgres
// advisory lock for the repository, so two PRs can't take project locks in
// it at the same time. fn's writes commit only if it succeeds, and roll back
// if it fails or panics, or if the lock is lost. A holder idle for longer
// than timeout is disconnected by Postgres. ctx bounds the whole call,
// including waiting for a pooled connection, and rolls back fn's writes if it
// ends first. On other databases fn still runs in a transaction, without the
// lock.
func (db *Database) WithRepositoryPRLocks(ctx context.Context, repoFullName string, timeout time.Duration, fn func(tx *Database) error) error {
	h := fnv.New32a()
	h.Write([]byte(repoFullName))
	key := int32(h.Sum32())

	for wait := 100 * time.Millisecond; ; wait = min(2*wait, 2*time.Second) {
		locked, err := db.withRepositoryPRLocksOnce(ctx, key, timeout, fn)
		if locked || err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
}

// withRepositoryPRLocksOnce runs fn if it gets the lock on the first try.
// It reports whether it did, and doesn't hold a connection otherwise.
func (db *Database) withRepositoryPRLocksOnce(ctx context.Context, key int32, timeout time.Duration, fn func(tx *Database) error) (locked bool, err error) {
	tx := db.GormDB.WithContext(ctx).Begin()
	if tx.Error != nil {
		return false, tx.Error
	}
	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
		}
	}()

	if db.GormDB.Dialector.Name() == "postgres" {
		if err := tx.Raw("SELECT pg_try_advisory_xact_lock(?, ?)", repositoryPRLocksClass, key).Scan(&locked).Error; err != nil || !locked {
			return false, err
		}
		if err := tx.Exec(fmt.Sprintf("SET LOCAL idle_in_transaction_session_timeout = %d", timeout.Milliseconds())).Error; err != nil {
			return true, err
		}
	}
	if err := fn(&Database{GormDB: tx}); err != nil {
		return true, err
	}
	committed = true
	return true, tx.Commit().Error
}
