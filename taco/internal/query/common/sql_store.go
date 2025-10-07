package common

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/query/types"
	"github.com/diggerhq/digger/opentaco/internal/rbac"
	"gorm.io/gorm"
)

// SQLStore provides a generic, GORM-based implementation of the Store interface.
// It can be used with any GORM-compatible database dialect (SQLite, Postgres, etc.).
type SQLStore struct {
	db *gorm.DB
}

// NewSQLStore is a constructor for our common store. It takes a pre-configured
// GORM DB object and handles the common setup tasks like migration and view creation.
func NewSQLStore(db *gorm.DB) (*SQLStore, error) {
	store := &SQLStore{db: db}

	if err := store.migrate(); err != nil {
		return nil, fmt.Errorf("failed to migrate common sql schema: %w", err)
	}
	if err := store.createViews(); err != nil {
		return nil, fmt.Errorf("failed to create common sql views: %w", err)
	}

	return store, nil
}

func (s *SQLStore) migrate() error {
	return s.db.AutoMigrate(types.DefaultModels...)
}

// createViews now introspects the database dialect to use the correct SQL syntax.
func (s *SQLStore) createViews() error {
	// Define the body of the view once.
	viewBody := `
	WITH user_permissions AS (
		SELECT DISTINCT u.subject as user_subject, r.id as rule_id, r.wildcard_resource, r.effect FROM users u
		JOIN user_roles ur ON u.id = ur.user_id JOIN role_permissions rp ON ur.role_id = rp.role_id
		JOIN rules r ON rp.permission_id = r.permission_id LEFT JOIN rule_actions ra ON r.id = ra.rule_id
		WHERE r.effect = 'allow' AND (r.wildcard_action = true OR ra.action = 'unit.read' OR ra.action IS NULL)
	),
	wildcard_access AS (
		SELECT DISTINCT up.user_subject, un.name as unit_name FROM user_permissions up CROSS JOIN units un
		WHERE up.wildcard_resource = true
	),
	specific_access AS (
		SELECT DISTINCT up.user_subject, un.name as unit_name FROM user_permissions up
		JOIN rule_units ru ON up.rule_id = ru.rule_id JOIN units un ON ru.unit_id = un.id
		WHERE up.wildcard_resource = false
	)
	SELECT user_subject, unit_name FROM wildcard_access
	UNION
	SELECT user_subject, unit_name FROM specific_access
	`

	var createViewSQL string
	dialect := s.db.Dialector.Name()

	// This switch statement is our "carve-out" for different SQL dialects.
	switch dialect {
	case "sqlserver":
		createViewSQL = fmt.Sprintf("CREATE OR ALTER VIEW user_unit_access AS %s", viewBody)
	case "sqlite", "postgres":
		fallthrough // Use the same syntax for both
	default:
		// Default to the most common syntax.
		createViewSQL = fmt.Sprintf("CREATE OR REPLACE VIEW user_unit_access AS %s", viewBody)
	}

	return s.db.Exec(createViewSQL).Error
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

func (s *SQLStore) SyncEnsureUnit(ctx context.Context, unitName string) error {
	unit := types.Unit{Name: unitName}
	return s.db.WithContext(ctx).FirstOrCreate(&unit, types.Unit{Name: unitName}).Error
}


func (s *SQLStore) SyncUnitMetadata(ctx context.Context, unitName string, size int64, updated time.Time) error {
	return s.db.WithContext(ctx).Model(&types.Unit{}).
		Where("name = ?", unitName).
		Updates(map[string]interface{}{
			"size":       size,
			"updated_at": updated,
		}).Error
}

func (s *SQLStore) SyncDeleteUnit(ctx context.Context, unitName string) error {
	return s.db.WithContext(ctx).Where("name = ?", unitName).Delete(&types.Unit{}).Error
}

func (s *SQLStore) SyncUnitLock(ctx context.Context, unitName string, lockID, lockWho string, lockCreated time.Time) error {
	return s.db.WithContext(ctx).Model(&types.Unit{}).
		Where("name = ?", unitName).
		Updates(map[string]interface{}{
			"locked":       true,
			"lock_id":      lockID,
			"lock_who":     lockWho,
			"lock_created": lockCreated,
		}).Error
}

func (s *SQLStore) SyncUnitUnlock(ctx context.Context, unitName string) error {
	return s.db.WithContext(ctx).Model(&types.Unit{}).
		Where("name = ?", unitName).
		Updates(map[string]interface{}{
			"locked":       false,
			"lock_id":      "",
			"lock_who":     "",
			"lock_created": time.Time{},
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
	querySQL := `
		SELECT MAX(CASE WHEN r.effect = 'allow' THEN 1 ELSE 0 END) FROM users u
		JOIN user_roles ur ON u.id = ur.user_id JOIN role_permissions rp ON ur.role_id = rp.role_id
		JOIN rules r ON rp.permission_id = r.permission_id
		WHERE u.subject = ? AND (r.wildcard_action = true OR EXISTS (SELECT 1 FROM rule_actions ra WHERE ra.rule_id = r.id AND ra.action = ?))
		AND (r.wildcard_resource = true OR EXISTS (SELECT 1 FROM rule_units ru JOIN units un ON ru.unit_id = un.id WHERE ru.rule_id = r.id AND un.name = ?))
	`
	err := s.db.WithContext(ctx).Raw(querySQL, userSubject, action, resourceID).Scan(&allowed).Error
	return allowed == 1, err
}

func (s *SQLStore) HasRBACRoles(ctx context.Context) (bool, error) {
	var count int64
	// We don't need to count them all, we just need to know if at least one exists.
	if err := s.db.WithContext(ctx).Model(&types.Role{}).Limit(1).Count(&count).Error; err != nil {
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
		p := types.Permission{
			PermissionId: perm.ID,
			Name:         perm.Name,
			Description:  perm.Description,
			CreatedBy:    perm.CreatedBy,
			CreatedAt:    perm.CreatedAt,
		}

		// Upsert using FirstOrCreate
		if err := tx.Where(types.Permission{PermissionId: perm.ID}).
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
			rule := types.Rule{
				PermissionID:     p.ID,
				Effect:           strings.ToLower(ruleData.Effect),
				WildcardAction:   hasStarAction(ruleData.Actions),
				WildcardResource: hasStarResource(ruleData.Resources),
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
					// Ensure unit exists
					var unit types.Unit
					if err := tx.Where("name = ?", resourceName).
						Attrs(types.Unit{Name: resourceName}).
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
		r := types.Role{
			RoleId:      role.ID,
			Name:        role.Name,
			Description: role.Description,
			CreatedBy:   role.CreatedBy,
			CreatedAt:   role.CreatedAt,
		}

		if err := tx.Where(types.Role{RoleId: role.ID}).
			Assign(r).
			FirstOrCreate(&r).Error; err != nil {
			return fmt.Errorf("upsert role %q: %w", role.ID, err)
		}

		// 2) Find all referenced permissions
		perms := make([]types.Permission, 0, len(role.Permissions))
		if len(role.Permissions) > 0 {
			var existing []types.Permission
			if err := tx.Where("permission_id IN ?", role.Permissions).Find(&existing).Error; err != nil {
				return fmt.Errorf("lookup permissions for role %q: %w", role.ID, err)
			}

			exists := make(map[string]types.Permission)
			for _, p := range existing {
				exists[p.PermissionId] = p
			}

			// Create missing permissions as placeholders
			for _, pid := range role.Permissions {
				if p, ok := exists[pid]; ok {
					perms = append(perms, p)
				} else {
					np := types.Permission{
						PermissionId: pid,
						Name:         pid,
						Description:  "",
						CreatedBy:    role.CreatedBy,
					}
					if err := tx.Where(types.Permission{PermissionId: pid}).
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
				byID[r.RoleId] = r
			}

			// Create missing roles as placeholders
			for _, rid := range user.Roles {
				if r, ok := byID[rid]; ok {
					roles = append(roles, r)
				} else {
					nr := types.Role{
						RoleId:      rid,
						Name:        rid,
						Description: "",
						CreatedBy:   user.Subject,
					}
					if err := tx.Where(types.Role{RoleId: rid}).
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
