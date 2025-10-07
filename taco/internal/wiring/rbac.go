package wiring

import (
	"context"
	"log"

	"github.com/diggerhq/digger/opentaco/internal/query"
	"github.com/diggerhq/digger/opentaco/internal/rbac"
	"github.com/diggerhq/digger/opentaco/internal/storage"
)

// SyncRBACFromStorage syncs RBAC data from storage to the query database.
// This is called at startup to populate the database with roles, permissions, and users.
func SyncRBACFromStorage(ctx context.Context, store storage.UnitStore, queryStore query.Store) error {
	// Check if it's S3 storage
	s3Store, ok := store.(storage.S3Store)
	if !ok {
		log.Println("RBAC sync skipped: storage backend does not support RBAC")
		return nil
	}

	// Create the S3 RBAC store
	rbacStore := rbac.NewS3RBACStore(
		s3Store.GetS3Client(),
		s3Store.GetS3Bucket(),
		s3Store.GetS3Prefix(),
	)

	log.Println("Starting RBAC data sync from S3 to database...")

	// Sync permissions
	permissions, err := rbacStore.ListPermissions(ctx)
	if err != nil {
		return err
	}
	for _, perm := range permissions {
		if err := queryStore.SyncPermission(ctx, perm); err != nil {
			log.Printf("Warning: Failed to sync permission %s: %v", perm.ID, err)
		}
	}
	log.Printf("Synced %d permissions", len(permissions))

	// Sync roles
	roles, err := rbacStore.ListRoles(ctx)
	if err != nil {
		return err
	}
	for _, role := range roles {
		if err := queryStore.SyncRole(ctx, role); err != nil {
			log.Printf("Warning: Failed to sync role %s: %v", role.ID, err)
		}
	}
	log.Printf("Synced %d roles", len(roles))

	// Sync users
	users, err := rbacStore.ListUserAssignments(ctx)
	if err != nil {
		return err
	}
	for _, user := range users {
		if err := queryStore.SyncUser(ctx, user); err != nil {
			log.Printf("Warning: Failed to sync user %s: %v", user.Subject, err)
		}
	}
	log.Printf("Synced %d user assignments", len(users))

	log.Println("RBAC data sync completed successfully")
	return nil
}
