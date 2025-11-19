package rbac

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/query/types"
	"gorm.io/gorm"
)

// queryRBACStore implements RBACStore using the existing normalized database models.
// This leverages the proper relational schema (User, Role, Permission tables).
type queryRBACStore struct {
	db *gorm.DB
}

// NewQueryRBACStore creates an RBAC store that uses the existing normalized models
func NewQueryRBACStore(db *gorm.DB) RBACStore {
	return &queryRBACStore{db: db}
}

// ============================================
// Permission Management
// ============================================

func (s *queryRBACStore) CreatePermission(ctx context.Context, perm *Permission) error {
	// Convert to GORM model
	// Map: rbac.Permission.ID → types.Permission.Name (identifier like "unit-read")
	//      rbac.Permission.Name → types.Permission.Description (friendly name like "Unit Read Permission")
	description := perm.Name // Use friendly name as description
	if perm.Description != "" {
		description = perm.Description // Prefer explicit description if provided
	}
	
	typePerm := types.Permission{
		OrgID:       perm.OrgID, // ✅ FIX: Set org_id for org-scoped RBAC
		Name:        perm.ID, // "unit-read" (identifier, NOT UUID)
		Description: description,
		CreatedBy:   perm.CreatedBy,
		CreatedAt:   perm.CreatedAt,
		Rules:       convertRbacRulesToTypes(perm.Rules),
	}

	return s.db.WithContext(ctx).Create(&typePerm).Error
}

func (s *queryRBACStore) GetPermission(ctx context.Context, orgID, id string) (*Permission, error) {
	var typePerm types.Permission
	err := s.db.WithContext(ctx).
		Where("org_id = ? AND name = ?", orgID, id).
		Preload("Rules").
		Preload("Rules.Actions").
		First(&typePerm).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return convertTypesPermissionToRbac(&typePerm), nil
}

func (s *queryRBACStore) ListPermissions(ctx context.Context, orgID string) ([]*Permission, error) {
	var typePerms []types.Permission
	err := s.db.WithContext(ctx).
		Where("org_id = ?", orgID).
		Preload("Rules").
		Preload("Rules.Actions").
		Find(&typePerms).Error

	if err != nil {
		return nil, err
	}

	perms := make([]*Permission, len(typePerms))
	for i, tp := range typePerms {
		perms[i] = convertTypesPermissionToRbac(&tp)
	}
	return perms, nil
}

func (s *queryRBACStore) DeletePermission(ctx context.Context, orgID, id string) error {
	return s.db.WithContext(ctx).
		Where("org_id = ? AND name = ?", orgID, id).
		Delete(&types.Permission{}).Error
}

// ============================================
// Role Management
// ============================================

func (s *queryRBACStore) CreateRole(ctx context.Context, role *Role) error {
	// Map: rbac.Role.ID → types.Role.Name (identifier like "admin")
	//      rbac.Role.Name → types.Role.Description (friendly name like "Administrator")
	description := role.Name // Use friendly name as description
	if role.Description != "" {
		description = role.Description // Prefer explicit description if provided
	}
	
	typeRole := types.Role{
		OrgID:       role.OrgID, // ✅ FIX: Set org_id for org-scoped RBAC
		Name:        role.ID, // "admin" (identifier, NOT UUID)
		Description: description,
		CreatedBy:   role.CreatedBy,
		CreatedAt:   role.CreatedAt,
	}

	// Create role
	if err := s.db.WithContext(ctx).Create(&typeRole).Error; err != nil {
		return err
	}

	// Link permissions if any
	if len(role.Permissions) > 0 {
		var typePerms []types.Permission
		if err := s.db.WithContext(ctx).
			Where("name IN ?", role.Permissions).
			Find(&typePerms).Error; err != nil {
			return err
		}

		if err := s.db.WithContext(ctx).Model(&typeRole).Association("Permissions").Append(typePerms); err != nil {
			return err
		}
	}

	return nil
}

func (s *queryRBACStore) GetRole(ctx context.Context, orgID, id string) (*Role, error) {
	var typeRole types.Role
	err := s.db.WithContext(ctx).
		Where("org_id = ? AND name = ?", orgID, id).
		Preload("Permissions").
		First(&typeRole).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return convertTypesRoleToRbac(&typeRole), nil
}

func (s *queryRBACStore) ListRoles(ctx context.Context, orgID string) ([]*Role, error) {
	var typeRoles []types.Role
	err := s.db.WithContext(ctx).
		Where("org_id = ?", orgID).
		Preload("Permissions").
		Find(&typeRoles).Error

	if err != nil {
		return nil, err
	}

	roles := make([]*Role, len(typeRoles))
	for i, tr := range typeRoles {
		roles[i] = convertTypesRoleToRbac(&tr)
	}
	return roles, nil
}

func (s *queryRBACStore) DeleteRole(ctx context.Context, orgID, id string) error {
	return s.db.WithContext(ctx).
		Where("org_id = ? AND name = ?", orgID, id).
		Delete(&types.Role{}).Error
}

// ============================================
// User Assignment Management
// ============================================

func (s *queryRBACStore) AssignRole(ctx context.Context, orgID, subject, email, roleID string) error {
	// Get or create user
	var user types.User
	err := s.db.WithContext(ctx).
		Where("subject = ?", subject).
		Preload("Roles").
		First(&user).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Create new user
			user = types.User{
				Subject:   subject,
				Email:     email,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
				Version:   1,
			}
			if err := s.db.WithContext(ctx).Create(&user).Error; err != nil {
				return err
			}
		} else {
			return err
		}
	}

	// Get the role to assign (org-scoped)
	var role types.Role
	if err := s.db.WithContext(ctx).Where("org_id = ? AND name = ?", orgID, roleID).First(&role).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		return err
	}

	// Check if already assigned (within this org)
	for _, r := range user.Roles {
		if r.Name == roleID && r.OrgID == orgID {
			return nil // Already assigned in this org
		}
	}

	// Assign role - explicitly insert into user_roles junction table with org_id
	// Note: Can't use Association API because it doesn't support custom junction table fields
	userRole := struct {
		UserID string `gorm:"column:user_id"`
		RoleID string `gorm:"column:role_id"`
		OrgID  string `gorm:"column:org_id"`
	}{
		UserID: user.ID,
		RoleID: role.ID,
		OrgID:  orgID,
	}
	
	if err := s.db.WithContext(ctx).Table("user_roles").Create(&userRole).Error; err != nil {
		return err
	}

	// Update version
	return s.db.WithContext(ctx).Model(&user).Update("version", user.Version+1).Error
}

func (s *queryRBACStore) AssignRoleByEmail(ctx context.Context, orgID, email, roleID string) error {
	var user types.User
	err := s.db.WithContext(ctx).Where("email = ?", email).First(&user).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Create user with email as subject (will be updated on first login)
			return s.AssignRole(ctx, orgID, email, email, roleID)
		}
		return err
	}

	return s.AssignRole(ctx, orgID, user.Subject, email, roleID)
}

func (s *queryRBACStore) RevokeRole(ctx context.Context, orgID, subject, roleID string) error {
	var user types.User
	err := s.db.WithContext(ctx).
		Where("subject = ?", subject).
		Preload("Roles").
		First(&user).Error

	if err != nil {
		return err
	}

	// Find the role to revoke (org-scoped)
	var role types.Role
	if err := s.db.WithContext(ctx).Where("org_id = ? AND name = ?", orgID, roleID).First(&role).Error; err != nil {
		return err
	}

	// Remove role association
	if err := s.db.WithContext(ctx).Model(&user).Association("Roles").Delete(&role); err != nil {
		return err
	}

	// Update version
	return s.db.WithContext(ctx).Model(&user).Update("version", user.Version+1).Error
}

func (s *queryRBACStore) RevokeRoleByEmail(ctx context.Context, orgID, email, roleID string) error {
	var user types.User
	err := s.db.WithContext(ctx).Where("email = ?", email).First(&user).Error

	if err != nil {
		return err
	}

	return s.RevokeRole(ctx, orgID, user.Subject, roleID)
}

func (s *queryRBACStore) GetUserAssignment(ctx context.Context, orgID, subject string) (*UserAssignment, error) {
	var user types.User
	err := s.db.WithContext(ctx).
		Where("subject = ?", subject).
		Preload("Roles", func(db *gorm.DB) *gorm.DB {
			// Join through user_roles to filter by org_id
			return db.Joins("JOIN user_roles ON user_roles.role_id = roles.id").
				Where("user_roles.org_id = ?", orgID)
		}).
		First(&user).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return convertTypesUserToAssignment(&user), nil
}

func (s *queryRBACStore) GetUserAssignmentByEmail(ctx context.Context, orgID, email string) (*UserAssignment, error) {
	var user types.User
	err := s.db.WithContext(ctx).
		Where("email = ?", email).
		Preload("Roles", "org_id = ?", orgID). // Filter roles by org
		First(&user).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return convertTypesUserToAssignment(&user), nil
}

func (s *queryRBACStore) ListUserAssignments(ctx context.Context, orgID string) ([]*UserAssignment, error) {
	var users []types.User
	err := s.db.WithContext(ctx).
		Preload("Roles", "org_id = ?", orgID). // Filter roles by org
		Find(&users).Error

	if err != nil {
		return nil, err
	}

	assignments := make([]*UserAssignment, len(users))
	for i, u := range users {
		assignments[i] = convertTypesUserToAssignment(&u)
	}
	return assignments, nil
}

// ============================================
// Conversion Helpers
// ============================================

func convertTypesPermissionToRbac(tp *types.Permission) *Permission {
	rules := make([]PermissionRule, len(tp.Rules))
	for i, r := range tp.Rules {
		resources := extractResourcesFromRule(&r)
		
		// Convert actions - check WildcardAction flag
		var actions []Action
		if r.WildcardAction {
			// If wildcard action flag is set, use "*"
			actions = []Action{"*"}
		} else {
			// Otherwise, convert from RuleAction records
			actions = convertRuleActionsToActions(r.Actions)
		}
		
		rules[i] = PermissionRule{
			Actions:   actions,
			Resources: resources,
			Effect:    r.Effect,
		}
	}

	return &Permission{
		ID:          tp.ID,   // UUID
		OrgID:       tp.OrgID, // ✅ FIX: Copy org_id from database model
		Name:        tp.Name, // Identifier like "unit-read"
		Description: tp.Description,
		Rules:       rules,
		CreatedAt:   tp.CreatedAt,
		CreatedBy:   tp.CreatedBy,
	}
}

func convertTypesRoleToRbac(tr *types.Role) *Role {
	permIDs := make([]string, len(tr.Permissions))
	for i, p := range tr.Permissions {
		permIDs[i] = p.Name // Permission identifiers like "unit-read"
	}

	return &Role{
		ID:          tr.ID,   // UUID
		OrgID:       tr.OrgID, // ✅ FIX: Copy org_id from database model
		Name:        tr.Name, // Identifier like "admin"
		Description: tr.Description,
		Permissions: permIDs,
		CreatedAt:   tr.CreatedAt,
		CreatedBy:   tr.CreatedBy,
		Version:     1, // types.Role doesn't track version
	}
}

func convertTypesUserToAssignment(tu *types.User) *UserAssignment {
	roleIDs := make([]string, len(tu.Roles))
	for i, r := range tu.Roles {
		roleIDs[i] = r.Name // Role identifiers like "admin"
	}

	return &UserAssignment{
		Subject:   tu.Subject,
		Email:     tu.Email,
		Roles:     roleIDs, // Role identifiers (not UUIDs)
		CreatedAt: tu.CreatedAt,
		UpdatedAt: tu.UpdatedAt,
		Version:   tu.Version,
	}
}

func convertRbacRulesToTypes(rules []PermissionRule) []types.Rule {
	typeRules := make([]types.Rule, len(rules))
	for i, r := range rules {
		// Marshal resource patterns to JSON
		resourcePatternsJSON, _ := json.Marshal(r.Resources)
		
		typeRules[i] = types.Rule{
			Effect:           r.Effect,
			WildcardAction:   containsWildcard(actionsToStrings(r.Actions)),
			WildcardResource: containsWildcard(r.Resources),
			ResourcePatterns: string(resourcePatternsJSON),
			Actions:          convertActionsToRuleActions(r.Actions),
		}
	}
	return typeRules
}

func convertRuleActionsToActions(ras []types.RuleAction) []Action {
	actions := make([]Action, len(ras))
	for i, ra := range ras {
		actions[i] = Action(ra.Action)
	}
	return actions
}

func convertActionsToRuleActions(actions []Action) []types.RuleAction {
	ras := make([]types.RuleAction, len(actions))
	for i, a := range actions {
		ras[i] = types.RuleAction{
			Action: string(a),
		}
	}
	return ras
}

func extractResourcesFromRule(r *types.Rule) []string {
	// Try to unmarshal resource patterns from JSON
	if r.ResourcePatterns != "" {
		var patterns []string
		if err := json.Unmarshal([]byte(r.ResourcePatterns), &patterns); err == nil && len(patterns) > 0 {
			return patterns
		}
	}
	
	// Fallback: check wildcard flag
	if r.WildcardResource {
		return []string{"*"}
	}
	
	// Default: no resources (deny by default)
	return []string{}
}

func actionsToStrings(actions []Action) []string {
	strs := make([]string, len(actions))
	for i, a := range actions {
		strs[i] = string(a)
	}
	return strs
}

func containsWildcard(strs []string) bool {
	for _, s := range strs {
		if s == "*" {
			return true
		}
	}
	return false
}

