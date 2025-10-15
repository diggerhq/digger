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
	typePerm := types.Permission{
		PermissionId: perm.ID,
		Name:         perm.Name,
		Description:  perm.Description,
		CreatedBy:    perm.CreatedBy,
		CreatedAt:    perm.CreatedAt,
		Rules:        convertRbacRulesToTypes(perm.Rules),
	}

	return s.db.WithContext(ctx).Create(&typePerm).Error
}

func (s *queryRBACStore) GetPermission(ctx context.Context, id string) (*Permission, error) {
	var typePerm types.Permission
	err := s.db.WithContext(ctx).
		Where("permission_id = ?", id).
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

func (s *queryRBACStore) ListPermissions(ctx context.Context) ([]*Permission, error) {
	var typePerms []types.Permission
	err := s.db.WithContext(ctx).
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

func (s *queryRBACStore) DeletePermission(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).
		Where("permission_id = ?", id).
		Delete(&types.Permission{}).Error
}

// ============================================
// Role Management
// ============================================

func (s *queryRBACStore) CreateRole(ctx context.Context, role *Role) error {
	typeRole := types.Role{
		RoleId:      role.ID,
		Name:        role.Name,
		Description: role.Description,
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
			Where("permission_id IN ?", role.Permissions).
			Find(&typePerms).Error; err != nil {
			return err
		}

		if err := s.db.WithContext(ctx).Model(&typeRole).Association("Permissions").Append(typePerms); err != nil {
			return err
		}
	}

	return nil
}

func (s *queryRBACStore) GetRole(ctx context.Context, id string) (*Role, error) {
	var typeRole types.Role
	err := s.db.WithContext(ctx).
		Where("role_id = ?", id).
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

func (s *queryRBACStore) ListRoles(ctx context.Context) ([]*Role, error) {
	var typeRoles []types.Role
	err := s.db.WithContext(ctx).
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

func (s *queryRBACStore) DeleteRole(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).
		Where("role_id = ?", id).
		Delete(&types.Role{}).Error
}

// ============================================
// User Assignment Management
// ============================================

func (s *queryRBACStore) AssignRole(ctx context.Context, subject, email, roleID string) error {
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

	// Get the role to assign
	var role types.Role
	if err := s.db.WithContext(ctx).Where("role_id = ?", roleID).First(&role).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		return err
	}

	// Check if already assigned
	for _, r := range user.Roles {
		if r.RoleId == roleID {
			return nil // Already assigned
		}
	}

	// Assign role
	if err := s.db.WithContext(ctx).Model(&user).Association("Roles").Append(&role); err != nil {
		return err
	}

	// Update version
	return s.db.WithContext(ctx).Model(&user).Update("version", user.Version+1).Error
}

func (s *queryRBACStore) AssignRoleByEmail(ctx context.Context, email, roleID string) error {
	var user types.User
	err := s.db.WithContext(ctx).Where("email = ?", email).First(&user).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Create user with email as subject (will be updated on first login)
			return s.AssignRole(ctx, email, email, roleID)
		}
		return err
	}

	return s.AssignRole(ctx, user.Subject, email, roleID)
}

func (s *queryRBACStore) RevokeRole(ctx context.Context, subject, roleID string) error {
	var user types.User
	err := s.db.WithContext(ctx).
		Where("subject = ?", subject).
		Preload("Roles").
		First(&user).Error

	if err != nil {
		return err
	}

	// Find the role to revoke
	var role types.Role
	if err := s.db.WithContext(ctx).Where("role_id = ?", roleID).First(&role).Error; err != nil {
		return err
	}

	// Remove role association
	if err := s.db.WithContext(ctx).Model(&user).Association("Roles").Delete(&role); err != nil {
		return err
	}

	// Update version
	return s.db.WithContext(ctx).Model(&user).Update("version", user.Version+1).Error
}

func (s *queryRBACStore) RevokeRoleByEmail(ctx context.Context, email, roleID string) error {
	var user types.User
	err := s.db.WithContext(ctx).Where("email = ?", email).First(&user).Error

	if err != nil {
		return err
	}

	return s.RevokeRole(ctx, user.Subject, roleID)
}

func (s *queryRBACStore) GetUserAssignment(ctx context.Context, subject string) (*UserAssignment, error) {
	var user types.User
	err := s.db.WithContext(ctx).
		Where("subject = ?", subject).
		Preload("Roles").
		First(&user).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return convertTypesUserToAssignment(&user), nil
}

func (s *queryRBACStore) GetUserAssignmentByEmail(ctx context.Context, email string) (*UserAssignment, error) {
	var user types.User
	err := s.db.WithContext(ctx).
		Where("email = ?", email).
		Preload("Roles").
		First(&user).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return convertTypesUserToAssignment(&user), nil
}

func (s *queryRBACStore) ListUserAssignments(ctx context.Context) ([]*UserAssignment, error) {
	var users []types.User
	err := s.db.WithContext(ctx).
		Preload("Roles").
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
		ID:          tp.PermissionId,
		Name:        tp.Name,
		Description: tp.Description,
		Rules:       rules,
		CreatedAt:   tp.CreatedAt,
		CreatedBy:   tp.CreatedBy,
	}
}

func convertTypesRoleToRbac(tr *types.Role) *Role {
	permIDs := make([]string, len(tr.Permissions))
	for i, p := range tr.Permissions {
		permIDs[i] = p.PermissionId
	}

	return &Role{
		ID:          tr.RoleId,
		Name:        tr.Name,
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
		roleIDs[i] = r.RoleId
	}

	return &UserAssignment{
		Subject:   tu.Subject,
		Email:     tu.Email,
		Roles:     roleIDs,
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

