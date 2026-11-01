package models

import (
	"context"
	"fmt"
	"hash/fnv"
	"log/slog"
	"sync"
	"time"
)

// LockPullRequestCommands takes a transaction-scoped Postgres advisory lock
// for this PR, retrying until ctx ends, so concurrent plan or apply requests
// check for duplicates one at a time. Call release to drop it; extra calls
// are no-ops. The lock drops itself after maxHold, in case its holder hangs.
// On other databases it does nothing.
func (db *Database) LockPullRequestCommands(ctx context.Context, repoFullName string, prNumber int, maxHold time.Duration) (release func(), err error) {
	if db.GormDB.Dialector.Name() != "postgres" {
		return func() {}, nil
	}
	sqlDB, err := db.GormDB.DB()
	if err != nil {
		return nil, err
	}

	h := fnv.New64a()
	fmt.Fprintf(h, "digger-pr-commands:%s#%d", repoFullName, prNumber)
	key := int64(h.Sum64())

	for wait := 100 * time.Millisecond; ; wait = min(2*wait, 2*time.Second) {
		// ctx bounds the wait for a pool connection, but not the transaction:
		// database/sql rolls a transaction back when its context ends.
		conn, err := sqlDB.Conn(ctx)
		if err != nil {
			return nil, err
		}
		tx, err := conn.BeginTx(context.Background(), nil)
		if err != nil {
			conn.Close()
			return nil, err
		}
		var locked bool
		err = tx.QueryRowContext(ctx, "SELECT pg_try_advisory_xact_lock($1)", key).Scan(&locked)
		if err == nil && locked {
			var once sync.Once
			unlock := func() {
				once.Do(func() {
					if err := tx.Rollback(); err != nil {
						slog.Warn("PR command lock ended early", "repoFullName", repoFullName, "prNumber", prNumber, "error", err)
					}
					conn.Close()
				})
			}
			timer := time.AfterFunc(maxHold, func() {
				slog.Warn("PR command lock held too long, releasing it", "repoFullName", repoFullName, "prNumber", prNumber, "maxHold", maxHold)
				unlock()
			})
			return func() {
				timer.Stop()
				unlock()
			}, nil
		}
		// Don't hold a connection while waiting.
		_ = tx.Rollback()
		conn.Close()
		if err != nil {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
	}
}
