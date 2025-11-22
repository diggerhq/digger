package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/diggerhq/digger/opentaco/pkg/sdk"
	"github.com/spf13/cobra"
)

// Local types for RBAC CLI (avoid importing internal packages)
type UserAssignment struct {
	Subject   string   `json:"subject"`
	Email     string   `json:"email"`
	Roles     []string `json:"roles"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
}

type Role struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
	CreatedAt   string   `json:"created_at"`
	CreatedBy   string   `json:"created_by"`
}

type Permission struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Rules       []PermissionRule `json:"rules"`
	CreatedAt   string           `json:"created_at"`
	CreatedBy   string           `json:"created_by"`
}

type PermissionRule struct {
	Actions   []string `json:"actions"`
	Resources []string `json:"resources"`
	Effect    string   `json:"effect"`
}

type paginatedRolesResponse struct {
	Roles    []Role `json:"roles"`
	Count    int    `json:"count"`
	Total    int64  `json:"total"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

type paginatedPermissionsResponse struct {
	Permissions []Permission `json:"permissions"`
	Count       int          `json:"count"`
	Total       int64        `json:"total"`
	Page        int          `json:"page"`
	PageSize    int          `json:"page_size"`
}

var (
	rbacRoleListPage       = 1
	rbacRoleListPageSize   = 50
	rbacPermissionListPage = 1
	rbacPermissionListSize = 50
)

// rbacCmd represents the rbac command
var rbacCmd = &cobra.Command{
	Use:   "rbac",
	Short: "Manage RBAC (Role-Based Access Control)",
	Long:  `Manage RBAC including initialization, role management, and user assignments.`,
}

func init() {
	rootCmd.AddCommand(rbacCmd)

	// Add subcommands
	rbacCmd.AddCommand(rbacInitCmd)
	rbacCmd.AddCommand(rbacMeCmd)
	rbacCmd.AddCommand(rbacUserCmd)
	rbacCmd.AddCommand(rbacRoleCmd)
	rbacCmd.AddCommand(rbacPermissionCmd)
	rbacCmd.AddCommand(rbacTestCmd)
}

// rbac init command
var rbacInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize RBAC system",
	Long:  `Initialize RBAC system for the current user. Creates default policies and roles, assigns admin role to current user.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newAuthedClient()

		// Get current user info
		userInfo, err := getCurrentUserInfo()
		if err != nil {
			return fmt.Errorf("failed to get current user info: %w", err)
		}

		printVerbose("Initializing RBAC for user: %s (%s)", userInfo.Subject, userInfo.Email)

		// Call RBAC init endpoint
		req := map[string]string{
			"subject": userInfo.Subject,
			"email":   userInfo.Email,
		}

		resp, err := client.PostJSON(context.Background(), "/v1/rbac/init", req)
		if err != nil {
			return fmt.Errorf("failed to initialize RBAC: %w", err)
		}

		if resp.StatusCode != 200 {
			// Try to parse error response
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("RBAC initialization failed with status %d", resp.StatusCode)
			}

			var errorResp struct {
				Error   string `json:"error"`
				Message string `json:"message"`
			}

			if err := json.Unmarshal(body, &errorResp); err == nil {
				if errorResp.Message != "" {
					return fmt.Errorf("%s: %s", errorResp.Error, errorResp.Message)
				}
				return fmt.Errorf("%s", errorResp.Error)
			}

			return fmt.Errorf("RBAC initialization failed with status %d", resp.StatusCode)
		}

		fmt.Println("RBAC system initialized successfully!")
		fmt.Printf("Admin role assigned to: %s (%s)\n", userInfo.Email, userInfo.Subject)
		fmt.Println("Default permissions and roles created.")

		return nil
	},
}

// rbac me command
var rbacMeCmd = &cobra.Command{
	Use:   "me",
	Short: "Show current user's RBAC information",
	Long:  `Show current user's roles, permissions, and RBAC status.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newAuthedClient()

		printVerbose("Getting RBAC information for current user")

		resp, err := client.Get(context.Background(), "/v1/rbac/me")
		if err != nil {
			return fmt.Errorf("failed to get RBAC info: %w", err)
		}

		if resp.StatusCode != 200 {
			return fmt.Errorf("failed to get RBAC info with status %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}

		var data map[string]interface{}
		if err := json.Unmarshal(body, &data); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		// Pretty print the response
		jsonData, _ := json.MarshalIndent(data, "", "  ")
		fmt.Println(string(jsonData))

		return nil
	},
}

// rbac user command
var rbacUserCmd = &cobra.Command{
	Use:   "user",
	Short: "Manage user role assignments",
	Long:  `Manage user role assignments including assign and revoke operations.`,
}

func init() {
	rbacUserCmd.AddCommand(rbacUserAssignCmd)
	rbacUserCmd.AddCommand(rbacUserRevokeCmd)
	rbacUserCmd.AddCommand(rbacUserListCmd)
}

// rbac user assign command
var rbacUserAssignCmd = &cobra.Command{
	Use:   "assign <email> <role-name>",
	Short: "Assign a role to a user",
	Long:  `Assign a role to a user by email address. The user must have logged in at least once to be found in the system.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newAuthedClient()

		email := args[0]
		roleID := mustResolveRoleID(context.Background(), client, args[1])

		printVerbose("Assigning role %s to user %s", roleID, email)

		req := map[string]string{
			"email":   email,
			"role_id": roleID,
		}

		resp, err := client.PostJSON(context.Background(), "/v1/rbac/users/assign", req)
		if err != nil {
			return fmt.Errorf("failed to assign role: %w", err)
		}

		if resp.StatusCode != 200 {
			return fmt.Errorf("role assignment failed with status %d", resp.StatusCode)
		}

		fmt.Printf("Role %s assigned to %s\n", roleID, email)
		return nil
	},
}

// rbac user revoke command
var rbacUserRevokeCmd = &cobra.Command{
	Use:   "revoke <email> <role-name>",
	Short: "Revoke a role from a user",
	Long:  `Revoke a role from a user by email address.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newAuthedClient()

		email := args[0]
		roleID := mustResolveRoleID(context.Background(), client, args[1])

		printVerbose("Revoking role %s from user %s", roleID, email)

		req := map[string]string{
			"email":   email,
			"role_id": roleID,
		}

		resp, err := client.PostJSON(context.Background(), "/v1/rbac/users/revoke", req)
		if err != nil {
			return fmt.Errorf("failed to revoke role: %w", err)
		}

		if resp.StatusCode != 200 {
			return fmt.Errorf("role revocation failed with status %d", resp.StatusCode)
		}

		fmt.Printf("Role %s revoked from %s\n", roleID, email)
		return nil
	},
}

// rbac user list command
var rbacUserListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all user role assignments",
	Long:  `List all user role assignments in the system.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newAuthedClient()

		printVerbose("Listing all user role assignments")

		resp, err := client.Get(context.Background(), "/v1/rbac/users")
		if err != nil {
			return fmt.Errorf("failed to list user assignments: %w", err)
		}

		if resp.StatusCode != 200 {
			return fmt.Errorf("failed to list user assignments with status %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}

		var assignments []UserAssignment
		if err := json.Unmarshal(body, &assignments); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		if len(assignments) == 0 {
			fmt.Println("No user assignments found")
			return nil
		}

		// Create tabwriter
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "SUBJECT\tEMAIL\tROLES\tUPDATED")

		for _, assignment := range assignments {
			roles := strings.Join(assignment.Roles, ", ")
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				assignment.Subject,
				assignment.Email,
				roles,
				assignment.UpdatedAt,
			)
		}

		w.Flush()
		fmt.Printf("\nTotal: %d user assignments\n", len(assignments))

		return nil
	},
}

// rbac role command
var rbacRoleCmd = &cobra.Command{
	Use:   "role",
	Short: "Manage roles",
	Long:  `Manage roles including create, list, and delete operations.`,
}

func init() {
	rbacRoleCmd.AddCommand(rbacRoleCreateCmd)
	rbacRoleCmd.AddCommand(rbacRoleListCmd)
	rbacRoleCmd.AddCommand(rbacRoleDeleteCmd)
	rbacRoleCmd.AddCommand(rbacRoleAssignPolicyCmd)
	rbacRoleCmd.AddCommand(rbacRoleRevokePermissionCmd)
	rbacRoleListCmd.Flags().IntVar(&rbacRoleListPage, "page", 1, "Page number for roles")
	rbacRoleListCmd.Flags().IntVar(&rbacRoleListPageSize, "page-size", 50, "Roles per page")
}

// rbac role create command
var rbacRoleCreateCmd = &cobra.Command{
	Use:   "create <role-id> <name> <description>",
	Short: "Create a new role",
	Long:  `Create a new role with the specified ID, name, and description.`,
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newAuthedClient()

		roleID := args[0]
		name := args[1]
		description := args[2]

		printVerbose("Creating role %s: %s", roleID, name)

		req := map[string]interface{}{
			"id":          roleID,
			"name":        name,
			"description": description,
			"permissions": []string{}, // Empty permissions for now
		}

		resp, err := client.PostJSON(context.Background(), "/v1/rbac/roles", req)
		if err != nil {
			return fmt.Errorf("failed to create role: %w", err)
		}

		if resp.StatusCode != 200 {
			return fmt.Errorf("role creation failed with status %d", resp.StatusCode)
		}

		fmt.Printf("Role %s created successfully: %s\n", roleID, name)
		return nil
	},
}

// rbac role list command
var rbacRoleListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all roles",
	Long:  `List all roles in the system.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newAuthedClient()
		page := rbacRoleListPage
		if page < 1 {
			page = 1
		}
		pageSize := rbacRoleListPageSize
		if pageSize < 1 {
			pageSize = 50
		}

		query := url.Values{}
		query.Set("page", strconv.Itoa(page))
		query.Set("page_size", strconv.Itoa(pageSize))

		path := "/v1/rbac/roles"
		if encoded := query.Encode(); encoded != "" {
			path += "?" + encoded
		}
		printVerbose("Listing roles (page %d, size %d)", page, pageSize)

		resp, err := client.Get(context.Background(), path)
		if err != nil {
			return fmt.Errorf("failed to list roles: %w", err)
		}

		if resp.StatusCode != 200 {
			return fmt.Errorf("failed to list roles with status %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}

		var rolesPage paginatedRolesResponse
		if err := json.Unmarshal(body, &rolesPage); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		if len(rolesPage.Roles) == 0 {
			fmt.Println("No roles found")
			return nil
		}

		// Create tabwriter
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tDESCRIPTION\tPERMISSIONS\tCREATED")

		for _, role := range rolesPage.Roles {
			permissions := strings.Join(role.Permissions, ", ")
			name := role.Name
			if name == "" {
				name = role.ID
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				name,
				role.Description,
				permissions,
				role.CreatedAt,
			)
		}

		w.Flush()
		fmt.Printf("\nPage %d (size %d) — showing %d of %d roles\n", rolesPage.Page, rolesPage.PageSize, len(rolesPage.Roles), rolesPage.Total)

		return nil
	},
}

// rbac role delete command
var rbacRoleDeleteCmd = &cobra.Command{
	Use:   "delete <role-name>",
	Short: "Delete a role",
	Long:  `Delete a role by name.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newAuthedClient()

		roleID := mustResolveRoleID(context.Background(), client, args[0])

		printVerbose("Deleting role %s", roleID)

		resp, err := client.Delete(context.Background(), "/v1/rbac/roles/"+roleID)
		if err != nil {
			return fmt.Errorf("failed to delete role: %w", err)
		}

		if resp.StatusCode != 200 {
			return fmt.Errorf("role deletion failed with status %d", resp.StatusCode)
		}

		fmt.Printf("Role %s deleted successfully\n", roleID)
		return nil
	},
}

// Helper function to get current user info
func getCurrentUserInfo() (*UserInfo, error) {
	base := normalizedBase(serverURL)
	cf, err := loadCreds()
	if err != nil {
		return nil, err
	}

	tok, ok := cf.Profiles[base]
	if !ok || tok.AccessToken == "" {
		// Fallback: if only one profile exists, use it
		if len(cf.Profiles) == 1 {
			for _, t := range cf.Profiles {
				tok = t
				ok = true
				break
			}
		}
		if !ok || tok.AccessToken == "" {
			return nil, fmt.Errorf("not logged in; run 'taco login' first")
		}
	}

	// Get user info from /v1/auth/me
	req, _ := http.NewRequest("GET", base+"/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get user info: HTTP %d", resp.StatusCode)
	}

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	subject, _ := data["subject"].(string)
	email := ""

	// Try to extract email from various possible fields
	if emailVal, ok := data["email"].(string); ok {
		email = emailVal
	} else if emailVal, ok := data["preferred_username"].(string); ok {
		email = emailVal
	} else if emailVal, ok := data["sub"].(string); ok && strings.Contains(emailVal, "@") {
		email = emailVal
	}

	return &UserInfo{
		Subject: subject,
		Email:   email,
	}, nil
}

// UserInfo represents basic user information
type UserInfo struct {
	Subject string
	Email   string
}

// rbac permission command
var rbacPermissionCmd = &cobra.Command{
	Use:   "permission",
	Short: "Manage RBAC permissions",
	Long:  `Manage RBAC permissions that define access rights for roles.`,
}

func init() {
	rbacPermissionCmd.AddCommand(rbacPermissionCreateCmd)
	rbacPermissionCmd.AddCommand(rbacPermissionListCmd)
	rbacPermissionCmd.AddCommand(rbacPermissionDeleteCmd)
	rbacPermissionListCmd.Flags().IntVar(&rbacPermissionListPage, "page", 1, "Page number for permissions")
	rbacPermissionListCmd.Flags().IntVar(&rbacPermissionListSize, "page-size", 50, "Permissions per page")
}

// rbac permission create command
var rbacPermissionCreateCmd = &cobra.Command{
	Use:   "create <id> <name> <description>",
	Short: "Create a new permission",
	Long:  `Create a new permission with specified rules. Use --rule flag to add rules.`,
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		name := args[1]
		description := args[2]

		// Get rules from flags
		rules, _ := cmd.Flags().GetStringArray("rule")

		client := newAuthedClient()

		// Parse rules
		var permissionRules []PermissionRule
		for _, ruleStr := range rules {
			// Format: "effect:actions:resources"
			// Example: "allow:unit.read,unit.write:dev/*"
			parts := strings.Split(ruleStr, ":")
			if len(parts) != 3 {
				return fmt.Errorf("invalid rule format: %s. Expected: effect:actions:resources", ruleStr)
			}

			effect := parts[0]
			actions := strings.Split(parts[1], ",")
			resources := strings.Split(parts[2], ",")

			permissionRules = append(permissionRules, PermissionRule{
				Actions:   actions,
				Resources: resources,
				Effect:    effect,
			})
		}

		req := map[string]interface{}{
			"id":          id,
			"name":        name,
			"description": description,
			"rules":       permissionRules,
		}

		resp, err := client.PostJSON(context.Background(), "/v1/rbac/permissions", req)
		if err != nil {
			return fmt.Errorf("failed to create permission: %w", err)
		}

		if resp.StatusCode != 201 {
			return fmt.Errorf("failed to create permission with status %d", resp.StatusCode)
		}

		fmt.Printf("Permission '%s' created successfully\n", id)
		return nil
	},
}

// rbac permission list command
var rbacPermissionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all permissions",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newAuthedClient()
		page := rbacPermissionListPage
		if page < 1 {
			page = 1
		}
		pageSize := rbacPermissionListSize
		if pageSize < 1 {
			pageSize = 50
		}

		query := url.Values{}
		query.Set("page", strconv.Itoa(page))
		query.Set("page_size", strconv.Itoa(pageSize))

		path := "/v1/rbac/permissions"
		if encoded := query.Encode(); encoded != "" {
			path += "?" + encoded
		}

		resp, err := client.Get(context.Background(), path)
		if err != nil {
			return fmt.Errorf("failed to list permissions: %w", err)
		}

		if resp.StatusCode != 200 {
			return fmt.Errorf("failed to list permissions with status %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}

		var permissionsPage paginatedPermissionsResponse
		if err := json.Unmarshal(body, &permissionsPage); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		if len(permissionsPage.Permissions) == 0 {
			fmt.Println("No permissions found")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tDESCRIPTION\tRULES\tCREATED")

		for _, permission := range permissionsPage.Permissions {
			rules := ""
			for i, rule := range permission.Rules {
				if i > 0 {
					rules += "; "
				}
				rules += fmt.Sprintf("%s:%s:%s", rule.Effect, strings.Join(rule.Actions, ","), strings.Join(rule.Resources, ","))
			}

			name := permission.Name
			if name == "" {
				name = permission.ID
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				name,
				permission.Description,
				rules,
				permission.CreatedAt,
			)
		}

		w.Flush()
		fmt.Printf("\nPage %d (size %d) — showing %d of %d permissions\n", permissionsPage.Page, permissionsPage.PageSize, len(permissionsPage.Permissions), permissionsPage.Total)
		return nil
	},
}

// rbac permission delete command
var rbacPermissionDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a permission",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newAuthedClient()
		id := mustResolvePermissionID(context.Background(), client, args[0])

		resp, err := client.Delete(context.Background(), "/v1/rbac/permissions/"+id)
		if err != nil {
			return fmt.Errorf("failed to delete permission: %w", err)
		}

		if resp.StatusCode != 204 {
			return fmt.Errorf("failed to delete permission with status %d", resp.StatusCode)
		}

		fmt.Printf("Permission '%s' deleted successfully\n", id)
		return nil
	},
}

func init() {
	rbacPermissionCreateCmd.Flags().StringArray("rule", []string{}, "Permission rule in format: effect:actions:resources (e.g., allow:state.read,state.write:dev/*)")
}

// rbac test command
var rbacTestCmd = &cobra.Command{
	Use:   "test <email> <operation> [args...]",
	Short: "Test RBAC permissions for a user without executing operations",
	Long:  `Test what operations a user would be able to perform based on their RBAC roles and permissions.`,
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		email := args[0]
		operation := args[1]
		operationArgs := args[2:]

		client := newAuthedClient()

		// Test the operation
		result, err := testUserOperation(client, email, operation, operationArgs)
		if err != nil {
			return fmt.Errorf("failed to test operation: %w", err)
		}

		// Display results
		fmt.Printf("Testing operation for user: %s\n", email)
		fmt.Printf("Operation: %s %s\n", operation, strings.Join(operationArgs, " "))
		fmt.Printf("Result: %s\n", result.Status)
		if result.Reason != "" {
			fmt.Printf("Reason: %s\n", result.Reason)
		}
		if len(result.UserRoles) > 0 {
			fmt.Printf("User roles: %s\n", strings.Join(result.UserRoles, ", "))
		}
		if len(result.ApplicablePermissions) > 0 {
			fmt.Printf("Applicable permissions: %s\n", strings.Join(result.ApplicablePermissions, ", "))
		}

		return nil
	},
}

// TestResult represents the result of a permission test
type TestResult struct {
	Status                string   `json:"status"`                 // "allowed", "denied", "error"
	Reason                string   `json:"reason"`                 // Explanation of the result
	UserRoles             []string `json:"user_roles"`             // Roles assigned to the user
	ApplicablePermissions []string `json:"applicable_permissions"` // Permissions that apply to this operation
}

// testUserOperation tests what a user can do for a given operation
func testUserOperation(client *sdk.Client, email, operation string, args []string) (*TestResult, error) {
	// Map operations to actions and resources
	var action, resource string

	switch operation {
	case "lock":
		if len(args) < 1 {
			return &TestResult{Status: "error", Reason: "lock operation requires unit ID"}, nil
		}
		action = "unit.lock"
		resource = args[0]
	case "unlock":
		if len(args) < 1 {
			return &TestResult{Status: "error", Reason: "unlock operation requires unit ID"}, nil
		}
		action = "unit.lock" // Same permission as lock
		resource = args[0]
	case "assign":
		if len(args) < 2 {
			return &TestResult{Status: "error", Reason: "assign operation requires role ID"}, nil
		}
		action = "rbac.manage"
		resource = "rbac.users"
	case "revoke":
		if len(args) < 2 {
			return &TestResult{Status: "error", Reason: "revoke operation requires role ID"}, nil
		}
		action = "rbac.manage"
		resource = "rbac.users"
	case "unit", "push":
		if len(args) < 1 {
			return &TestResult{Status: "error", Reason: "push operation requires unit ID"}, nil
		}
		action = "unit.write"
		resource = args[0]
	case "pull":
		if len(args) < 1 {
			return &TestResult{Status: "error", Reason: "pull operation requires unit ID"}, nil
		}
		action = "unit.read"
		resource = args[0]
	case "create":
		if len(args) < 1 {
			return &TestResult{Status: "error", Reason: "create operation requires unit ID"}, nil
		}
		action = "unit.write"
		resource = args[0]
	case "delete", "rm":
		if len(args) < 1 {
			return &TestResult{Status: "error", Reason: "delete operation requires unit ID"}, nil
		}
		action = "unit.delete"
		resource = args[0]
	case "ls", "list":
		action = "unit.read"
		resource = "*" // List operation checks read access to all units
	case "ls-output":
		// Special case: show actual filtered list output
		return testUserListOutput(client, email, args)
	default:
		return &TestResult{Status: "error", Reason: fmt.Sprintf("unknown operation: %s", operation)}, nil
	}

	// Call the test endpoint
	req := map[string]interface{}{
		"email":    email,
		"action":   action,
		"resource": resource,
	}

	resp, err := client.PostJSON(context.Background(), "/v1/rbac/test", req)
	if err != nil {
		return nil, fmt.Errorf("failed to test permissions: %w", err)
	}

	if resp.StatusCode != 200 {
		return &TestResult{Status: "error", Reason: fmt.Sprintf("test failed with status %d", resp.StatusCode)}, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result TestResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

// testUserListOutput shows the actual filtered unit list for a user
func testUserListOutput(client *sdk.Client, email string, args []string) (*TestResult, error) {
	// Get all units from server (skip the problematic * permission check)
	prefix := ""
	if len(args) > 0 {
		prefix = args[0]
	}

	unitsResp, err := client.Get(context.Background(), "/v1/units?prefix="+url.QueryEscape(prefix))
	if err != nil {
		return &TestResult{Status: "error", Reason: fmt.Sprintf("failed to fetch units: %v", err)}, nil
	}
	defer unitsResp.Body.Close()

	if unitsResp.StatusCode != 200 {
		return &TestResult{Status: "error", Reason: fmt.Sprintf("failed to fetch units with status %d", unitsResp.StatusCode)}, nil
	}

	unitsBody, err := io.ReadAll(unitsResp.Body)
	if err != nil {
		return &TestResult{Status: "error", Reason: fmt.Sprintf("failed to read units response: %v", err)}, nil
	}

	var unitsData struct {
		Units []struct {
			ID      string `json:"id"`
			Size    int    `json:"size"`
			Updated string `json:"updated"`
			Locked  bool   `json:"locked"`
		} `json:"units"`
	}

	if err := json.Unmarshal(unitsBody, &unitsData); err != nil {
		return &TestResult{Status: "error", Reason: fmt.Sprintf("failed to parse units response: %v", err)}, nil
	}

	// Filter units based on user's read permissions
	var accessibleUnits []struct {
		ID      string `json:"id"`
		Size    int    `json:"size"`
		Updated string `json:"updated"`
		Locked  bool   `json:"locked"`
	}

	for _, unit := range unitsData.Units {
		// Test read permission for this specific unit
		unitTestReq := map[string]interface{}{
			"email":    email,
			"action":   "unit.read",
			"resource": unit.ID,
		}

		unitResp, err := client.PostJSON(context.Background(), "/v1/rbac/test", unitTestReq)
		if err != nil {
			continue // Skip on error
		}

		if unitResp.StatusCode != 200 {
			unitResp.Body.Close()
			continue
		}

		unitBody, err := io.ReadAll(unitResp.Body)
		unitResp.Body.Close()
		if err != nil {
			continue
		}

		var unitResult struct {
			Status string `json:"status"`
		}
		if err := json.Unmarshal(unitBody, &unitResult); err != nil {
			continue
		}

		if unitResult.Status == "allowed" {
			accessibleUnits = append(accessibleUnits, unit)
		}
	}

	// Format the output similar to unit ls command
	result := fmt.Sprintf("Units accessible to %s:\n\n", email)
	if len(accessibleUnits) == 0 {
		result += "No units accessible to this user.\n"
	} else {
		result += "ID\tSIZE\tUPDATED\tLOCKED\n"
		for _, unit := range accessibleUnits {
			locked := ""
			if unit.Locked {
				locked = "yes"
			} else {
				locked = "no"
			}
			result += fmt.Sprintf("%s\t%d\t%s\t%s\n", unit.ID, unit.Size, unit.Updated, locked)
		}
		result += fmt.Sprintf("\nTotal: %d units", len(accessibleUnits))
	}

	return &TestResult{
		Status: "allowed",
		Reason: result,
	}, nil
}

// rbac role assign-policy command
var rbacRoleAssignPolicyCmd = &cobra.Command{
	Use:   "assign-policy <role-name> <permission-name>",
	Short: "Assign a policy to a role",
	Long:  `Assign a policy to a role, giving the role the permissions defined in the policy.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newAuthedClient()
		roleID := mustResolveRoleID(context.Background(), client, args[0])
		permissionID := mustResolvePermissionID(context.Background(), client, args[1])

		req := map[string]string{
			"role_id":       roleID,
			"permission_id": permissionID,
		}

		resp, err := client.PostJSON(context.Background(), "/v1/rbac/roles/"+roleID+"/permissions", req)
		if err != nil {
			return fmt.Errorf("failed to assign permission to role: %w", err)
		}

		if resp.StatusCode != 200 {
			return fmt.Errorf("failed to assign permission to role with status %d", resp.StatusCode)
		}

		fmt.Printf("Permission '%s' assigned to role '%s' successfully\n", permissionID, roleID)
		return nil
	},
}

// rbac role revoke-permission command
var rbacRoleRevokePermissionCmd = &cobra.Command{
	Use:   "revoke-permission <role-name> <permission-name>",
	Short: "Revoke a permission from a role",
	Long:  `Revoke a permission from a role, removing the access rights defined in the permission.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := newAuthedClient()
		roleID := mustResolveRoleID(context.Background(), client, args[0])
		permissionID := mustResolvePermissionID(context.Background(), client, args[1])

		resp, err := client.Delete(context.Background(), "/v1/rbac/roles/"+roleID+"/permissions/"+permissionID)
		if err != nil {
			return fmt.Errorf("failed to revoke permission from role: %w", err)
		}

		if resp.StatusCode != 204 {
			return fmt.Errorf("failed to revoke permission from role with status %d", resp.StatusCode)
		}

		fmt.Printf("Permission '%s' revoked from role '%s' successfully\n", permissionID, roleID)
		return nil
	},
}

// mustResolveRoleID resolves a role name to its ID
// If the argument is already a valid identifier, it's returned as-is
func mustResolveRoleID(ctx context.Context, client *sdk.Client, arg string) string {
	resp, err := client.Get(ctx, "/v1/rbac/roles")
	if err != nil || resp.StatusCode != 200 {
		return arg // fallback
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return arg
	}

	var roles []Role
	if err := json.Unmarshal(body, &roles); err != nil {
		return arg
	}

	for _, r := range roles {
		if r.Name == arg || r.ID == arg {
			if r.ID != "" {
				return r.ID
			}
			return arg
		}
	}
	return arg
}

// mustResolvePermissionID resolves a permission name to its ID
// If the argument is already a valid identifier, it's returned as-is
func mustResolvePermissionID(ctx context.Context, client *sdk.Client, arg string) string {
	resp, err := client.Get(ctx, "/v1/rbac/permissions")
	if err != nil || resp.StatusCode != 200 {
		return arg // fallback
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return arg
	}

	var permissions []Permission
	if err := json.Unmarshal(body, &permissions); err != nil {
		return arg
	}

	for _, p := range permissions {
		if p.Name == arg || p.ID == arg {
			if p.ID != "" {
				return p.ID
			}
			return arg
		}
	}
	return arg
}
