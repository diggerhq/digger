package db

import (
	"context"
	"fmt"
	"strings"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"

	rbac "github.com/diggerhq/digger/opentaco/internal/rbac"
)


type S3RoleDoc struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Permissions []string  `json:"permissions"`
	CreatedAt   time.Time `json:"created_at"`
	CreatedBy   string    `json:"created_by"`
	Version     int64     `json:"version"`
}


type S3UserDoc struct {
	Subject   string    `json:"subject"`
	Email     string    `json:"email"`
	Roles     []string  `json:"roles"`       // e.g., ["admin","brian1-developer"]
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Version   int64     `json:"version"`
}



func hasStarResource(list []string) bool {
	for _, s := range list { if s == "*" { return true } }
	return false
}

func hasStarAction(list []rbac.Action) bool {
	for _, s := range list { if string(s) == "*" { return true } }
	return false
}

func SeedPermission(ctx context.Context, db *gorm.DB, s3Perm *rbac.Permission) error {

	
	p := Permission{
		PermissionId: s3Perm.ID,
		Name:         s3Perm.Name,
		Description:  s3Perm.Description,
		CreatedBy:    s3Perm.CreatedBy,
		CreatedAt:    s3Perm.CreatedAt,
	}
	if err := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "permission_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "description", "created_by"}),
	}).Create(&p).Error; err != nil {
		return fmt.Errorf("permission upsert %s: %w", s3Perm.ID, err)
	}

	// 2) Replace rules (simple + idempotent for seeds)
	if err := db.WithContext(ctx).
		Where("permission_id = ?", p.ID).
		Delete(&Rule{}).Error; err != nil {
		return fmt.Errorf("clear rules %s: %w", s3Perm.ID, err)
	}

	for _, rr := range s3Perm.Rules {
		rule := Rule{
			PermissionID:     p.ID,
			Effect:           strings.ToLower(rr.Effect),
			WildcardAction:   hasStarAction(rr.Actions),
			WildcardResource: hasStarResource(rr.Resources),
		}
		if err := db.WithContext(ctx).Create(&rule).Error; err != nil {
			return fmt.Errorf("create rule: %w", err)
		}

		// Only create children if not wildcard
		if !rule.WildcardAction {
			rows := make([]RuleAction, 0, len(rr.Actions))
			for _, a := range rr.Actions {
				rows = append(rows, RuleAction{RuleID: rule.ID, Action: string(a)})
			}
			if len(rows) > 0 {
				if err := db.WithContext(ctx).Create(&rows).Error; err != nil {
					return fmt.Errorf("actions: %w", err)
				}
			}
		}
		if !rule.WildcardResource {
			// Resolve unit names -> Unit IDs, creating Units if missing
			us := make([]RuleUnit, 0, len(rr.Resources))
			for _, name := range rr.Resources {
				var u Unit
				if err := db.WithContext(ctx).
					Where(&Unit{Name: name}).
					FirstOrCreate(&u).Error; err != nil {
					return fmt.Errorf("ensure unit %q: %w", name, err)
				}
				us = append(us, RuleUnit{RuleID: rule.ID, UnitID: u.ID})
			}
			if len(us) > 0 {
				if err := db.WithContext(ctx).Create(&us).Error; err != nil {
					return fmt.Errorf("units: %w", err)
				}
			}
		}
	}
	return nil
}






func SeedRole(ctx context.Context, db *gorm.DB, rbacRole *rbac.Role) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1) Upsert role by RoleId
		var role Role
		if err := tx.
			Where(&Role{RoleId: rbacRole.ID}).
			Attrs(Role{
				Name:        rbacRole.Name,
				Description: rbacRole.Description,
				CreatedBy:   rbacRole.CreatedBy,
				CreatedAt:   rbacRole.CreatedAt, // keep if you want to trust S3 timestamp
			}).
			FirstOrCreate(&role).Error; err != nil {
			return fmt.Errorf("upsert role %q: %w", rbacRole.ID, err)
		}

		// 2) Ensure all permissions exist (by PermissionId)
		perms := make([]Permission, 0, len(rbacRole.Permissions))
		if len(rbacRole.Permissions) > 0 {
			// fetch existing
			var existing []Permission
			if err := tx.
				Where("permission_id IN ?", rbacRole.Permissions).
				Find(&existing).Error; err != nil {
				return fmt.Errorf("lookup permissions for role %q: %w", rbacRole.ID, err)
			}

			exists := map[string]Permission{}
			for _, p := range existing {
				exists[p.PermissionId] = p
			}

			// create any missing (minimal rows; names can be filled by permission seeder later)
			for _, pid := range rbacRole.Permissions {
				if p, ok := exists[pid]; ok {
					perms = append(perms, p)
					continue
				}
				np := Permission{
					PermissionId: pid,
					Name:         pid, // placeholder; your permission seeder will update
					Description:  "",
					CreatedBy:    rbacRole.CreatedBy,
				}
				if err := tx.
					Where(&Permission{PermissionId: pid}).
					Attrs(np).
					FirstOrCreate(&np).Error; err != nil {
					return fmt.Errorf("create missing permission %q: %w", pid, err)
				}
				perms = append(perms, np)
			}
		}

		// 3) Replace role -> permissions to match S3 exactly
		//    (idempotent; deletes any stale links, inserts new ones)
		if err := tx.Model(&role).Association("Permissions").Replace(perms); err != nil {
			return fmt.Errorf("set role permissions for %q: %w", rbacRole.ID, err)
		}

		return nil
	})
}


func SeedUser(ctx context.Context, db *gorm.DB, rbacUser *rbac.UserAssignment) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1) Upsert user by unique Subject
		u := User{
			Subject:   rbacUser.Subject,
			Email:     rbacUser.Email,
			CreatedAt: rbacUser.CreatedAt, // optional: trust S3 timestamps
			UpdatedAt: rbacUser.UpdatedAt,
			Version:   rbacUser.Version,
		}

		// If row exists (subject unique), update mutable fields
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "subject"}},
			DoUpdates: clause.AssignmentColumns([]string{"email", "updated_at", "version"}),
		}).Create(&u).Error; err != nil {
			return fmt.Errorf("upsert user %q: %w", rbacUser.Subject, err)
		}

		// Ensure we have the actual row (ID may be needed for associations)
		if err := tx.Where(&User{Subject: rbacUser.Subject}).First(&u).Error; err != nil {
			return fmt.Errorf("load user %q: %w", rbacUser.Subject, err)
		}

		// 2) Ensure all roles exist (by RoleId); create placeholders if missing
		roles := make([]Role, 0, len(rbacUser.Roles))
		if len(rbacUser.Roles) > 0 {
			var existing []Role
			if err := tx.Where("role_id IN ?", rbacUser.Roles).Find(&existing).Error; err != nil {
				return fmt.Errorf("lookup roles: %w", err)
			}
			byID := make(map[string]Role, len(existing))
			for _, r := range existing {
				byID[r.RoleId] = r
			}
			for _, rid := range rbacUser.Roles {
				if r, ok := byID[rid]; ok {
					roles = append(roles, r)
					continue
				}
				nr := Role{
					RoleId:      rid,
					Name:        rid,      // placeholder; your role seeder can update later
					Description: "",
					CreatedBy:   rbacUser.Subject,
				}
				if err := tx.Where(&Role{RoleId: rid}).Attrs(nr).FirstOrCreate(&nr).Error; err != nil {
					return fmt.Errorf("create missing role %q: %w", rid, err)
				}
				roles = append(roles, nr)
			}
		}

		// 3) Set user->roles to exactly match the S3 doc
		if err := tx.Model(&u).Association("Roles").Replace(roles); err != nil {
			return fmt.Errorf("set user roles for %q: %w", rbacUser.Subject, err)
		}

		return nil
	})
}