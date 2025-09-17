package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// TestVersioning_MaxVersionsEnvironmentVariable tests MAX_VERSIONS env var edge cases
func TestVersioning_MaxVersionsEnvironmentVariable(t *testing.T) {
	tests := []struct {
		name           string
		envValue       string
		expectedMax    int
		description    string
	}{
		{"unset", "", 10, "defaults to 10 when unset"},
		{"valid_positive", "5", 5, "uses valid positive number"},
		{"zero", "0", 10, "defaults to 10 for zero value"},
		{"negative", "-5", 10, "defaults to 10 for negative value"},
		{"negative_large", "-999", 10, "defaults to 10 for large negative"},
		{"invalid_string", "abc", 10, "defaults to 10 for non-numeric string"},
		{"invalid_float", "3.14", 10, "defaults to 10 for float"},
		{"injection_attempt", "5; rm -rf /", 10, "defaults to 10 for injection attempt"},
		{"script_injection", "$(rm -rf /)", 10, "defaults to 10 for command injection"},
		{"large_positive", "999999", 999999, "accepts large positive numbers"},
		{"leading_zeros", "005", 5, "handles leading zeros correctly"},
		{"whitespace", " 7 ", 10, "defaults to 10 for whitespace"},
		{"mixed_chars", "5abc", 10, "defaults to 10 for mixed alphanumeric"},
		{"special_chars", "5@#$", 10, "defaults to 10 for special characters"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test with memstore
			t.Run("memstore", func(t *testing.T) {
				// Set environment variable
				if tt.envValue == "" {
					os.Unsetenv("OPENTACO_MAX_VERSIONS")
				} else {
					os.Setenv("OPENTACO_MAX_VERSIONS", tt.envValue)
				}
				defer os.Unsetenv("OPENTACO_MAX_VERSIONS") // Clean up

				store := NewMemStore().(*memStore)
				actual := store.getMaxVersions()

				if actual != tt.expectedMax {
					t.Errorf("memstore: %s - expected %d, got %d", tt.description, tt.expectedMax, actual)
				}
			})

			// Test with s3store (mock the getMaxVersions method)
			t.Run("s3store", func(t *testing.T) {
				// Set environment variable
				if tt.envValue == "" {
					os.Unsetenv("OPENTACO_MAX_VERSIONS")
				} else {
					os.Setenv("OPENTACO_MAX_VERSIONS", tt.envValue)
				}
				defer os.Unsetenv("OPENTACO_MAX_VERSIONS") // Clean up

				// Create a mock s3Store to test getMaxVersions
				store := &s3Store{} 
				actual := store.getMaxVersions()

				if actual != tt.expectedMax {
					t.Errorf("s3store: %s - expected %d, got %d", tt.description, tt.expectedMax, actual)
				}
			})
		})
	}
}

// TestVersioning_LockInteraction tests versioning behavior with locks
func TestVersioning_LockInteraction(t *testing.T) {
	store := NewMemStore()
	ctx := context.Background()

	// Create a state
	_, err := store.Create(ctx, "test/versioning-locks")
	if err != nil {
		t.Fatalf("failed to create state: %v", err)
	}

	// Upload initial data
	data1 := []byte(`{"version": 4, "resources": []}`)
	err = store.Upload(ctx, "test/versioning-locks", data1, "")
	if err != nil {
		t.Fatalf("failed to upload initial data: %v", err)
	}

	// Lock the state
	lockInfo := &LockInfo{
		ID:      "lock-versioning-123",
		Who:     "test-user",
		Version: "1.0.0",
		Created: time.Now(),
	}
	err = store.Lock(ctx, "test/versioning-locks", lockInfo)
	if err != nil {
		t.Fatalf("failed to lock state: %v", err)
	}

	t.Run("upload_without_lock_id_when_locked", func(t *testing.T) {
		data2 := []byte(`{"version": 4, "resources": [{"name": "test"}]}`)
		err := store.Upload(ctx, "test/versioning-locks", data2, "")
		if !errors.Is(err, ErrLockConflict) {
			t.Errorf("expected ErrLockConflict when uploading to locked state without lockID, got %v", err)
		}

		// Verify no versions were created due to failed upload
		versions, err := store.ListVersions(ctx, "test/versioning-locks")
		if err != nil {
			t.Fatalf("failed to list versions: %v", err)
		}
		if len(versions) != 0 {
			t.Errorf("expected no versions after failed locked upload, got %d", len(versions))
		}
	})

	t.Run("upload_with_wrong_lock_id", func(t *testing.T) {
		data2 := []byte(`{"version": 4, "resources": [{"name": "wrong-lock"}]}`)
		err := store.Upload(ctx, "test/versioning-locks", data2, "wrong-lock-id")
		if !errors.Is(err, ErrLockConflict) {
			t.Errorf("expected ErrLockConflict when uploading with wrong lockID, got %v", err)
		}
	})

	t.Run("upload_with_correct_lock_id", func(t *testing.T) {
		data2 := []byte(`{"version": 4, "resources": [{"name": "locked-upload"}]}`)
		err := store.Upload(ctx, "test/versioning-locks", data2, "lock-versioning-123")
		if err != nil {
			t.Errorf("expected no error when uploading with correct lockID, got %v", err)
		}

		// Verify version was created for previous content
		versions, err := store.ListVersions(ctx, "test/versioning-locks")
		if err != nil {
			t.Fatalf("failed to list versions: %v", err)
		}
		if len(versions) != 1 {
			t.Errorf("expected 1 version after successful locked upload, got %d", len(versions))
		}
	})

	t.Run("restore_version_without_lock_id_when_locked", func(t *testing.T) {
		versions, err := store.ListVersions(ctx, "test/versioning-locks")
		if err != nil {
			t.Fatalf("failed to list versions: %v", err)
		}
		if len(versions) == 0 {
			t.Skip("no versions to restore")
		}

		err = store.RestoreVersion(ctx, "test/versioning-locks", versions[0].Timestamp, "")
		if !errors.Is(err, ErrLockConflict) {
			t.Errorf("expected ErrLockConflict when restoring version without lockID on locked state, got %v", err)
		}
	})

	t.Run("restore_version_with_correct_lock_id", func(t *testing.T) {
		versions, err := store.ListVersions(ctx, "test/versioning-locks")
		if err != nil {
			t.Fatalf("failed to list versions: %v", err)
		}
		if len(versions) == 0 {
			t.Skip("no versions to restore")
		}

		err = store.RestoreVersion(ctx, "test/versioning-locks", versions[0].Timestamp, "lock-versioning-123")
		if err != nil {
			t.Errorf("expected no error when restoring version with correct lockID, got %v", err)
		}
	})

	// Cleanup: unlock the state
	err = store.Unlock(ctx, "test/versioning-locks", "lock-versioning-123")
	if err != nil {
		t.Fatalf("failed to unlock state: %v", err)
	}
}

// TestVersioning_Cleanup tests version cleanup functionality
func TestVersioning_Cleanup(t *testing.T) {
	// Test cleanup with different MAX_VERSIONS values
	testCases := []struct {
		name         string
		maxVersions  int
		uploads      int
		expectedKept int
	}{
		{"cleanup_with_max_3", 3, 5, 3},     // 5 uploads = 4 versions, cleanup to 3
		{"cleanup_with_max_1", 1, 4, 1},     // 4 uploads = 3 versions, cleanup to 1
		{"no_cleanup_needed", 10, 5, 4},     // 5 uploads = 4 versions, no cleanup needed
		{"cleanup_exact_limit", 2, 3, 2},    // 3 uploads = 2 versions, keep both
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set MAX_VERSIONS for this test
			os.Setenv("OPENTACO_MAX_VERSIONS", fmt.Sprintf("%d", tc.maxVersions))
			defer os.Unsetenv("OPENTACO_MAX_VERSIONS")

			store := NewMemStore()
			ctx := context.Background()

			// Create state
			_, err := store.Create(ctx, "test/cleanup")
			if err != nil {
				t.Fatalf("failed to create state: %v", err)
			}

			// Upload initial data
			data := []byte(`{"version": 4, "serial": 0}`)
			err = store.Upload(ctx, "test/cleanup", data, "")
			if err != nil {
				t.Fatalf("failed to upload initial data: %v", err)
			}

			// Upload multiple versions to trigger cleanup
			for i := 1; i < tc.uploads; i++ {
				// Sleep to ensure different timestamps
				time.Sleep(10 * time.Millisecond)
				data := []byte(fmt.Sprintf(`{"version": 4, "serial": %d}`, i))
				err = store.Upload(ctx, "test/cleanup", data, "")
				if err != nil {
					t.Fatalf("failed to upload data %d: %v", i, err)
				}
			}

			// Check number of versions kept
			versions, err := store.ListVersions(ctx, "test/cleanup")
			if err != nil {
				t.Fatalf("failed to list versions: %v", err)
			}

			if len(versions) != tc.expectedKept {
				t.Errorf("expected %d versions kept, got %d", tc.expectedKept, len(versions))
			}

			// Verify versions are sorted by timestamp (newest first)
			for i := 1; i < len(versions); i++ {
				if versions[i-1].Timestamp.Before(versions[i].Timestamp) {
					t.Errorf("versions not sorted correctly: version %d timestamp %v is before version %d timestamp %v",
						i-1, versions[i-1].Timestamp, i, versions[i].Timestamp)
				}
			}
		})
	}
}

// TestVersioning_FileNameCreation tests version file name generation and parsing
func TestVersioning_FileNameCreation(t *testing.T) {
	// Test data with known hash
	testData := []byte(`{"version": 4, "test": "data"}`)
	hash := sha256.Sum256(testData)
	expectedHash := hex.EncodeToString(hash[:4])

	testTimestamp := time.Date(2024, 1, 15, 14, 30, 22, 123456000, time.UTC)

	t.Run("memstore_version_format", func(t *testing.T) {
		store := NewMemStore()
		ctx := context.Background()

		// Create and populate state to generate a version
		_, err := store.Create(ctx, "test/filename")
		if err != nil {
			t.Fatalf("failed to create state: %v", err)
		}

		// Upload initial data
		err = store.Upload(ctx, "test/filename", testData, "")
		if err != nil {
			t.Fatalf("failed to upload initial data: %v", err)
		}

		// Upload different data to create a version
		newData := []byte(`{"version": 4, "test": "updated"}`)
		err = store.Upload(ctx, "test/filename", newData, "")
		if err != nil {
			t.Fatalf("failed to upload new data: %v", err)
		}

		// List versions and check format
		versions, err := store.ListVersions(ctx, "test/filename")
		if err != nil {
			t.Fatalf("failed to list versions: %v", err)
		}

		if len(versions) != 1 {
			t.Fatalf("expected 1 version, got %d", len(versions))
		}

		version := versions[0]
		
		// Check hash matches expected
		if version.Hash != expectedHash {
			t.Errorf("expected hash %s, got %s", expectedHash, version.Hash)
		}

		// Verify hash is 8 characters (4 bytes hex encoded)
		if len(version.Hash) != 8 {
			t.Errorf("expected hash length 8, got %d", len(version.Hash))
		}

		// Verify hash is valid hex
		if _, err := hex.DecodeString(version.Hash); err != nil {
			t.Errorf("hash is not valid hex: %v", err)
		}
	})

	t.Run("s3store_version_key_format", func(t *testing.T) {
		// Test the versionKeyWithHash method directly
		store := &s3Store{
			bucket: "test-bucket",
			prefix: "test-prefix",
		}

		versionKey := store.versionKeyWithHash("myapp/prod", testTimestamp, testData)
		
		// Expected format: test-prefix/myapp/prod/versions/v-20240115T143022.123456Z-{hash}.tfstate
		expectedPrefix := "test-prefix/myapp/prod/versions/v-20240115T143022.123456Z-"
		expectedSuffix := ".tfstate"
		
		if !strings.HasPrefix(versionKey, expectedPrefix) {
			t.Errorf("version key should start with %s, got %s", expectedPrefix, versionKey)
		}
		
		if !strings.HasSuffix(versionKey, expectedSuffix) {
			t.Errorf("version key should end with %s, got %s", expectedSuffix, versionKey)
		}
		
		// Extract and verify hash
		withoutPrefix := strings.TrimPrefix(versionKey, expectedPrefix)
		hashPart := strings.TrimSuffix(withoutPrefix, expectedSuffix)
		
		if hashPart != expectedHash {
			t.Errorf("expected hash %s in key, got %s", expectedHash, hashPart)
		}
	})

	t.Run("timestamp_format_consistency", func(t *testing.T) {
		testCases := []time.Time{
			time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			time.Date(2024, 12, 31, 23, 59, 59, 999999000, time.UTC),
			time.Date(2000, 1, 1, 12, 30, 45, 123456000, time.UTC),
		}

		store := &s3Store{}
		for i, ts := range testCases {
			t.Run(fmt.Sprintf("timestamp_%d", i), func(t *testing.T) {
				key := store.versionKeyWithHash("test", ts, testData)
				
				// Verify timestamp is in correct format
				expectedTimestamp := ts.UTC().Format("20060102T150405.000000Z")
				if !strings.Contains(key, expectedTimestamp) {
					t.Errorf("key should contain timestamp %s, got %s", expectedTimestamp, key)
				}
			})
		}
	})
}