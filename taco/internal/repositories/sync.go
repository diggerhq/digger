package repositories

import (
	"context"
	"fmt"
	"log"
	"time"
)

// SyncOperation represents a database sync operation that can be retried
type SyncOperation func(ctx context.Context) error

// RetrySync executes a sync operation with exponential backoff retry logic.
// This ensures blob and query index stay in sync even during temporary failures.
// Uses 3 attempts with 100ms initial delay, up to 2s max delay.
func RetrySync(ctx context.Context, operation SyncOperation, operationName string) error {
	const (
		maxAttempts  = 3
		initialDelay = 100 * time.Millisecond
		maxDelay     = 2 * time.Second
		multiplier   = 2.0
	)

	var lastErr error
	delay := initialDelay

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := operation(ctx)
		if err == nil {
			if attempt > 1 {
				log.Printf("Sync succeeded for '%s' after %d attempts", operationName, attempt)
			}
			return nil
		}

		lastErr = err

		// If this was the last attempt, don't wait
		if attempt == maxAttempts {
			break
		}

		// Log retry attempt
		log.Printf("Sync failed for '%s' (attempt %d/%d): %v. Retrying in %v...", 
			operationName, attempt, maxAttempts, err, delay)

		// Wait with backoff
		select {
		case <-ctx.Done():
			return fmt.Errorf("sync cancelled for '%s': %w", operationName, ctx.Err())
		case <-time.After(delay):
			// Increase delay for next attempt (exponential backoff)
			delay = time.Duration(float64(delay) * multiplier)
			if delay > maxDelay {
				delay = maxDelay
			}
		}
	}

	return fmt.Errorf("sync failed for '%s' after %d attempts: %w", operationName, maxAttempts, lastErr)
}

// SyncWithFallback attempts a sync operation but doesn't fail if it errors.
// This is for non-critical sync operations where we prefer availability over consistency.
func SyncWithFallback(ctx context.Context, operation SyncOperation, operationName string) {
	if err := RetrySync(ctx, operation, operationName); err != nil {
		log.Printf("Warning: Sync operation '%s' failed after retries: %v", operationName, err)
		// Continue anyway - blob storage is source of truth
	}
}

