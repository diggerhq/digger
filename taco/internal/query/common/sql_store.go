package common

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/query/types"
	"github.com/diggerhq/digger/opentaco/internal/rbac"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	queryOrgAndName = "org_id = ? AND name = ?"
)

// SQLStore provides a generic, GORM-based implementation of the Store interface.
type SQLStore struct {
	db *gorm.DB
}

// NewSQLStore creates a new SQLStore.
// Note: Schema migrations are now handled by Atlas in queryfactory.NewQueryStore()
func NewSQLStore(db *gorm.DB) (*SQLStore, error) {
	store := &SQLStore{db: db}
	
	// All org-scoped unique indexes are now managed by Atlas migrations:
	//   - migrations/*/20251031000000_add_unique_unit_name_per_org.sql
	//   - migrations/*/20251031000001_add_unique_org_constraints.sql
	
	// Create database views (not handled by Atlas migrations)
	if err := store.createViews(); err != nil {
		return nil, fmt.Errorf("failed to create views: %w", err)
	}

	return store, nil
}

// GetDB returns the underlying GORM database connection
// This is used by components that need direct DB access (e.g., RBAC querystore)
func (s *SQLStore) GetDB() *gorm.DB {
	return s.db
}

// createViews now introspects the database dialect to use the correct SQL syntax.
func (s *SQLStore) createViews() error {
	dialect := s.db.Dialector.Name()
	
	// Define boolean literals based on dialect
	var trueVal, falseVal string
	if dialect == "sqlserver" {
		trueVal = "1"
		falseVal = "0"
	} else {
		trueVal = "true"
		falseVal = "false"
	}

	// Define the body of the view with dialect-specific boolean values
	// Support pattern matching: units with '*' in their name are treated as patterns
	viewBody := fmt.Sprintf(`
	WITH user_permissions AS (
		SELECT DISTINCT u.subject as user_subject, r.id as rule_id, r.wildcard_resource, r.effect FROM users u
		JOIN user_roles ur ON u.id = ur.user_id JOIN role_permissions rp ON ur.role_id = rp.role_id
		JOIN rules r ON rp.permission_id = r.permission_id LEFT JOIN rule_actions ra ON r.id = ra.rule_id
		WHERE r.effect = 'allow' AND (r.wildcard_action = %s OR ra.action = 'unit.read' OR ra.action IS NULL)
	),
	wildcard_access AS (
		SELECT DISTINCT up.user_subject, un.name as unit_name FROM user_permissions up CROSS JOIN units un
		WHERE up.wildcard_resource = %s
		AND un.name NOT LIKE '%%*%%'
	),
	specific_access AS (
		SELECT DISTINCT up.user_subject, target_units.name as unit_name 
		FROM user_permissions up
		JOIN rule_units ru ON up.rule_id = ru.rule_id 
		JOIN units pattern_units ON ru.unit_id = pattern_units.id
		CROSS JOIN units target_units
		WHERE up.wildcard_resource = %s
		AND target_units.name NOT LIKE '%%*%%'
		AND (
			pattern_units.name = target_units.name 
			OR (pattern_units.name LIKE '%%*%%' AND target_units.name LIKE REPLACE(pattern_units.name, '*', '%%'))
		)
	)
	SELECT user_subject, unit_name FROM wildcard_access
	UNION
	SELECT user_subject, unit_name FROM specific_access
	`, trueVal, trueVal, falseVal)

	// This switch statement is our "carve-out" for different SQL dialects.
	switch dialect {
	case "sqlserver":
		createViewSQL := fmt.Sprintf("CREATE OR ALTER VIEW user_unit_access AS %s", viewBody)
		return s.db.Exec(createViewSQL).Error
	case "sqlite":
		// SQLite doesn't support CREATE OR REPLACE VIEW, so we need to drop first
		s.db.Exec("DROP VIEW IF EXISTS user_unit_access")
		createViewSQL := fmt.Sprintf("CREATE VIEW user_unit_access AS %s", viewBody)
		return s.db.Exec(createViewSQL).Error
	case "postgres":
		createViewSQL := fmt.Sprintf("CREATE OR REPLACE VIEW user_unit_access AS %s", viewBody)
		return s.db.Exec(createViewSQL).Error
	default:
		// Default to the most common syntax for MySQL and others
		createViewSQL := fmt.Sprintf("CREATE OR REPLACE VIEW user_unit_access AS %s", viewBody)
		return s.db.Exec(createViewSQL).Error
	}
}

func (s *SQLStore) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func (s *SQLStore) IsEnabled() bool { return true }

func (s *SQLStore) ListUnits(ctx context.Context, prefix string) ([]types.Unit, error) {
	var units []types.Unit
	q := s.db.WithContext(ctx).Preload("Tags")
	if prefix != "" {
		q = q.Where("name LIKE ?", prefix+"%")
	}
	return units, q.Find(&units).Error
}

func (s *SQLStore) GetUnit(ctx context.Context, id string) (*types.Unit, error) {
	var unit types.Unit
	err := s.db.WithContext(ctx).Preload("Tags").Where("name = ?", id).First(&unit).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, types.ErrNotFound
		}
		return nil, err
	}
	return &unit, nil
}

// parseBlobPath parses a UUID-based blob path into org UUID and unit UUID
// Expected format: "org-uuid/unit-uuid"
// This is the only format used - all blob paths are UUID-based for immutability
func (s *SQLStore) parseBlobPath(ctx context.Context, blobPath string) (orgUUID, unitUUID string, err error) {
	parts := strings.SplitN(strings.Trim(blobPath, "/"), "/", 2)
	
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid blob path format: expected 'org-uuid/unit-uuid', got '%s'", blobPath)
	}
	
	orgUUID = parts[0]
	unitUUID = parts[1]
	
	// Validate both are UUIDs
	if !isUUID(orgUUID) {
		return "", "", fmt.Errorf("invalid org UUID in blob path: %s", orgUUID)
	}
	if !isUUID(unitUUID) {
		return "", "", fmt.Errorf("invalid unit UUID in blob path: %s", unitUUID)
	}
	
	// Verify org exists
	var org types.Organization
	err = s.db.WithContext(ctx).Where("id = ?", orgUUID).First(&org).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", "", fmt.Errorf("organization not found: %s", orgUUID)
		}
		return "", "", fmt.Errorf("failed to lookup organization: %w", err)
	}
	
	// Verify unit exists
	var unit types.Unit
	err = s.db.WithContext(ctx).Where("id = ? AND org_id = ?", unitUUID, orgUUID).First(&unit).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", "", fmt.Errorf("unit not found: %s in org %s", unitUUID, orgUUID)
		}
		return "", "", fmt.Errorf("failed to lookup unit: %w", err)
	}
	
	return orgUUID, unitUUID, nil
}

// isUUID checks if a string is a valid UUID
// Uses proper UUID parsing to validate format and structure
// This is critical for distinguishing UUID-based paths from name-based paths:
//   - UUID: "123e4567-89ab-12d3-a456-426614174000" → lookup by ID
//   - Name: "my-app-prod" → lookup by name
func isUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}

// SyncEnsureUnit creates or updates a unit from blob storage
// Expects UUID-based blob path: "org-uuid/unit-uuid"
func (s *SQLStore) SyncEnsureUnit(ctx context.Context, blobPath string) error {
	orgUUID, unitUUID, err := s.parseBlobPath(ctx, blobPath)
	if err != nil {
		return err
	}
	
	// Check if unit exists
	var existing types.Unit
	err = s.db.WithContext(ctx).
		Where("id = ? AND org_id = ?", unitUUID, orgUUID).
		First(&existing).Error
	
	if err == nil {
		// Unit already exists, nothing to do
		return nil
	}
	
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	
	// Unit doesn't exist - this shouldn't happen with UUID-based paths
	// as units should be created via UnitRepository first
	return fmt.Errorf("unit %s not found in database (UUID-based paths require unit to exist)", unitUUID)
}

func (s *SQLStore) SyncUnitMetadata(ctx context.Context, blobPath string, size int64, updated time.Time) error {
	orgUUID, unitUUID, err := s.parseBlobPath(ctx, blobPath)
	if err != nil {
		return err
	}
	
	return s.db.WithContext(ctx).Model(&types.Unit{}).
		Where("id = ? AND org_id = ?", unitUUID, orgUUID).
		Updates(map[string]interface{}{
			"size":       size,
			"updated_at": updated,
		}).Error
}

func (s *SQLStore) SyncDeleteUnit(ctx context.Context, blobPath string) error {
	orgUUID, unitUUID, err := s.parseBlobPath(ctx, blobPath)
	if err != nil {
		return err
	}
	
	return s.db.WithContext(ctx).
		Where("id = ? AND org_id = ?", unitUUID, orgUUID).
		Delete(&types.Unit{}).Error
}

// UpdateUnitTFESettings updates TFE-specific settings for a unit
func (s *SQLStore) UpdateUnitTFESettings(ctx context.Context, unitID string, autoApply *bool, executionMode *string, terraformVersion *string, engine *string, workingDirectory *string) error {
	updates := make(map[string]interface{})
	
	if autoApply != nil {
		updates["tfe_auto_apply"] = *autoApply
	}
	if executionMode != nil {
		updates["tfe_execution_mode"] = *executionMode
	}
	if terraformVersion != nil {
		updates["tfe_terraform_version"] = *terraformVersion
	}
	if engine != nil {
		updates["tfe_engine"] = *engine
	}
	if workingDirectory != nil {
		updates["tfe_working_directory"] = *workingDirectory
	}
	
	if len(updates) == 0 {
		return nil // Nothing to update
	}
	
	return s.db.WithContext(ctx).Model(&types.Unit{}).
		Where("id = ?", unitID).
		Updates(updates).Error
}

func (s *SQLStore) SyncUnitLock(ctx context.Context, blobPath string, lockID, lockWho string, lockCreated time.Time) error {
	orgUUID, unitUUID, err := s.parseBlobPath(ctx, blobPath)
	if err != nil {
		return err
	}
	
	return s.db.WithContext(ctx).Model(&types.Unit{}).
		Where("id = ? AND org_id = ?", unitUUID, orgUUID).
		Updates(map[string]interface{}{
			"locked":       true,
			"lock_id":      lockID,
			"lock_who":     lockWho,
			"lock_created": lockCreated,
		}).Error
}

func (s *SQLStore) SyncUnitUnlock(ctx context.Context, blobPath string) error {
	orgUUID, unitUUID, err := s.parseBlobPath(ctx, blobPath)
	if err != nil {
		return err
	}
	
	return s.db.WithContext(ctx).Model(&types.Unit{}).
		Where("id = ? AND org_id = ?", unitUUID, orgUUID).
		Updates(map[string]interface{}{
			"locked":       false,
			"lock_id":      "",
			"lock_who":     "",
			"lock_created": nil,
		}).Error
}

func (s *SQLStore) ListUnitsForUser(ctx context.Context, userSubject string, prefix string) ([]types.Unit, error) {
	var units []types.Unit
	q := s.db.WithContext(ctx).Table("units").Select("units.*").
		Joins("JOIN user_unit_access ON units.name = user_unit_access.unit_name").
		Where("user_unit_access.user_subject = ?", userSubject).
		Preload("Tags")

	if prefix != "" {
		q = q.Where("units.name LIKE ?", prefix+"%")
	}
	
	return units, q.Find(&units).Error
}

func (s *SQLStore) FilterUnitIDsByUser(ctx context.Context, userSubject string, unitIDs []string) ([]string, error) {
	if len(unitIDs) == 0 {
		return []string{}, nil
	}
	var allowedUnitIDs []string
	return allowedUnitIDs, s.db.WithContext(ctx).Table("user_unit_access").
		Select("unit_name").
		Where("user_subject = ?", userSubject).
		Where("unit_name IN ?", unitIDs).
		Pluck("unit_name", &allowedUnitIDs).Error
}

func (s *SQLStore) CanPerformAction(ctx context.Context, userSubject string, action string, resourceID string) (bool, error) {
	var allowed int
	// GORM's Raw SQL uses '?' and the dialect converts it to '$1', etc. for Postgres automatically.
	// Use COALESCE to handle NULL when no rows match
	// Support pattern matching: if unit name contains '*', treat it as a wildcard pattern
	querySQL := `
		SELECT COALESCE(MAX(CASE WHEN r.effect = 'allow' THEN 1 ELSE 0 END), 0) FROM users u
		JOIN user_roles ur ON u.id = ur.user_id JOIN role_permissions rp ON ur.role_id = rp.role_id
		JOIN rules r ON rp.permission_id = r.permission_id
		WHERE u.subject = ? AND (r.wildcard_action = true OR EXISTS (SELECT 1 FROM rule_actions ra WHERE ra.rule_id = r.id AND ra.action = ?))
		AND (r.wildcard_resource = true OR EXISTS (
			SELECT 1 FROM rule_units ru 
			JOIN units un ON ru.unit_id = un.id 
			WHERE ru.rule_id = r.id 
			AND (
				un.name = ? 
				OR (un.name LIKE '%*%' AND ? LIKE REPLACE(un.name, '*', '%'))
			)
		))
	`
	err := s.db.WithContext(ctx).Raw(querySQL, userSubject, action, resourceID, resourceID).Scan(&allowed).Error
	return allowed == 1, err
}

func (s *SQLStore) HasRBACRoles(ctx context.Context) (bool, error) {
	var count int64
	if err := s.db.WithContext(ctx).Model(&types.Role{}).Order("").Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// SyncPermission syncs a permission from storage to the database
func (s *SQLStore) SyncPermission(ctx context.Context, permissionData interface{}) error {
	// Import at the top: "github.com/diggerhq/digger/opentaco/internal/rbac"
	perm, ok := permissionData.(*rbac.Permission)
	if !ok {
		return fmt.Errorf("invalid permission data type")
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1) Upsert the permission
		// Map: rbac.Permission.ID → types.Permission.Name (identifier like "unit-read")
		//      rbac.Permission.Name → types.Permission.Description (friendly name like "Unit Read Permission")
		description := perm.Name // Use friendly name as description
		if perm.Description != "" {
			description = perm.Description // Prefer explicit description if provided
		}
		
		p := types.Permission{
			OrgID:       perm.OrgID, // ✅ FIX: Set org_id for org-scoped RBAC
			Name:        perm.ID, // "unit-read" (identifier, NOT UUID)
			Description: description,
			CreatedBy:   perm.CreatedBy,
			CreatedAt:   perm.CreatedAt,
		}

		// Upsert using FirstOrCreate
		if err := tx.Where(types.Permission{OrgID: perm.OrgID, Name: perm.ID}).
			Assign(p).
			FirstOrCreate(&p).Error; err != nil {
			return fmt.Errorf("upsert permission %s: %w", perm.ID, err)
		}

		// 2) Clear old rules for idempotency
		if err := tx.Where("permission_id = ?", p.ID).Delete(&types.Rule{}).Error; err != nil {
			return fmt.Errorf("clear rules for %s: %w", perm.ID, err)
		}

		// 3) Insert new rules
		for _, ruleData := range perm.Rules {
			// Marshal resource patterns to JSON for storage
			resourcePatternsJSON, _ := json.Marshal(ruleData.Resources)
			
			rule := types.Rule{
				PermissionID:     p.ID,
				Effect:           strings.ToLower(ruleData.Effect),
				WildcardAction:   hasStarAction(ruleData.Actions),
				WildcardResource: hasStarResource(ruleData.Resources),
				ResourcePatterns: string(resourcePatternsJSON),
			}

			if err := tx.Create(&rule).Error; err != nil {
				return fmt.Errorf("create rule: %w", err)
			}

			// Create rule actions if not wildcard
			if !rule.WildcardAction {
				for _, action := range ruleData.Actions {
					ra := types.RuleAction{
						RuleID: rule.ID,
						Action: string(action),
					}
					if err := tx.Create(&ra).Error; err != nil {
						return fmt.Errorf("create rule action: %w", err)
					}
				}
			}

			// Create rule units if not wildcard
			if !rule.WildcardResource {
				for _, resourceName := range ruleData.Resources {
					// Ensure unit exists (org-scoped)
					var unit types.Unit
					if err := tx.Where("org_id = ? AND name = ?", perm.OrgID, resourceName).
						Attrs(types.Unit{OrgID: perm.OrgID, Name: resourceName}).
						FirstOrCreate(&unit).Error; err != nil {
						return fmt.Errorf("ensure unit %q: %w", resourceName, err)
					}

					ru := types.RuleUnit{
						RuleID: rule.ID,
						UnitID: unit.ID,
					}
					if err := tx.Create(&ru).Error; err != nil {
						return fmt.Errorf("create rule unit: %w", err)
					}
				}
			}
		}

		return nil
	})
}

// SyncRole syncs a role from storage to the database
func (s *SQLStore) SyncRole(ctx context.Context, roleData interface{}) error {
	role, ok := roleData.(*rbac.Role)
	if !ok {
		return fmt.Errorf("invalid role data type")
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1) Upsert role
		// Map: rbac.Role.ID → types.Role.Name (identifier like "admin")
		//      rbac.Role.Name → types.Role.Description (friendly name like "Administrator")
		description := role.Name // Use friendly name as description
		if role.Description != "" {
			description = role.Description // Prefer explicit description if provided
		}
		
		r := types.Role{
			OrgID:       role.OrgID, // ✅ FIX: Set org_id for org-scoped RBAC
			Name:        role.ID, // "admin" (identifier, NOT UUID)
			Description: description,
			CreatedBy:   role.CreatedBy,
			CreatedAt:   role.CreatedAt,
		}

		if err := tx.Where(types.Role{OrgID: role.OrgID, Name: role.ID}).
			Assign(r).
			FirstOrCreate(&r).Error; err != nil {
			return fmt.Errorf("upsert role %q: %w", role.ID, err)
		}

		// 2) Find all referenced permissions
		perms := make([]types.Permission, 0, len(role.Permissions))
		if len(role.Permissions) > 0 {
			var existing []types.Permission
			if err := tx.Where("org_id = ? AND name IN ?", role.OrgID, role.Permissions).Find(&existing).Error; err != nil {
				return fmt.Errorf("lookup permissions for role %q: %w", role.ID, err)
			}

			exists := make(map[string]types.Permission)
			for _, p := range existing {
				exists[p.Name] = p
			}

			// Create missing permissions as placeholders (org-scoped)
			for _, pid := range role.Permissions {
				if p, ok := exists[pid]; ok {
					perms = append(perms, p)
				} else {
					np := types.Permission{
						OrgID:        role.OrgID, // ✅ FIX: Set org_id for org-scoped RBAC
						Name:         pid,
						Description:  "",
						CreatedBy:    role.CreatedBy,
					}
					if err := tx.Where(types.Permission{OrgID: role.OrgID, Name: pid}).
						Attrs(np).
						FirstOrCreate(&np).Error; err != nil {
						return fmt.Errorf("create missing permission %q: %w", pid, err)
					}
					perms = append(perms, np)
				}
			}
		}

		// 3) Replace role->permission associations
		if err := tx.Model(&r).Association("Permissions").Replace(perms); err != nil {
			return fmt.Errorf("set role permissions for %q: %w", role.ID, err)
		}

		return nil
	})
}

// SyncUser syncs a user assignment from storage to the database
func (s *SQLStore) SyncUser(ctx context.Context, userData interface{}) error {
	user, ok := userData.(*rbac.UserAssignment)
	if !ok {
		return fmt.Errorf("invalid user data type")
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1) Upsert user
		u := types.User{
			Subject:   user.Subject,
			Email:     user.Email,
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
			Version:   user.Version,
		}

		if err := tx.Where(types.User{Subject: user.Subject}).
			Assign(u).
			FirstOrCreate(&u).Error; err != nil {
			return fmt.Errorf("upsert user %q: %w", user.Subject, err)
		}

		// 2) Find all referenced roles
		roles := make([]types.Role, 0, len(user.Roles))
		if len(user.Roles) > 0 {
			var existing []types.Role
			if err := tx.Where("role_id IN ?", user.Roles).Find(&existing).Error; err != nil {
				return fmt.Errorf("lookup roles: %w", err)
			}

			byID := make(map[string]types.Role)
			for _, r := range existing {
				byID[r.Name] = r
			}

			// Create missing roles as placeholders
			for _, rid := range user.Roles {
				if r, ok := byID[rid]; ok {
					roles = append(roles, r)
				} else {
					nr := types.Role{
						Name:      rid,
						Description: "",
						CreatedBy:   user.Subject,
					}
					if err := tx.Where(types.Role{Name: rid}).
						Attrs(nr).
						FirstOrCreate(&nr).Error; err != nil {
						return fmt.Errorf("create missing role %q: %w", rid, err)
					}
					roles = append(roles, nr)
				}
			}
		}

		// 3) Replace user->role associations
		if err := tx.Model(&u).Association("Roles").Replace(roles); err != nil {
			return fmt.Errorf("set user roles for %q: %w", user.Subject, err)
		}

		return nil
	})
}

func (s *SQLStore) SyncDeletePermission(ctx context.Context, permissionID string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var perm types.Permission
		if err := tx.Where("permission_id = ?", permissionID).First(&perm).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		
		if err := tx.Where("permission_id = ?", perm.ID).Delete(&types.Rule{}).Error; err != nil {
			return err
		}
		
		return tx.Delete(&perm).Error
	})
}

func (s *SQLStore) SyncDeleteRole(ctx context.Context, roleID string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var role types.Role
		if err := tx.Where("role_id = ?", roleID).First(&role).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		
		if err := tx.Model(&role).Association("Permissions").Clear(); err != nil {
			return err
		}
		
		return tx.Delete(&role).Error
	})
}

func (s *SQLStore) SyncDeleteUser(ctx context.Context, subject string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user types.User
		if err := tx.Where("subject = ?", subject).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		
		if err := tx.Model(&user).Association("Roles").Clear(); err != nil {
			return err
		}
		
		return tx.Delete(&user).Error
	})
}

// Helper functions for checking wildcards
func hasStarAction(actions []rbac.Action) bool {
	for _, a := range actions {
		if string(a) == "*" {
			return true
		}
	}
	return false
}

func hasStarResource(resources []string) bool {
	for _, r := range resources {
		if r == "*" {
			return true
		}
	}
	return false
}
