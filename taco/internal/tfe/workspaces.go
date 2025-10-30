package tfe

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/diggerhq/digger/opentaco/internal/domain/tfe"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/rbac"
	"github.com/diggerhq/digger/opentaco/internal/storage"
	"github.com/google/jsonapi"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/mr-tron/base58"
)

// isAllDigits checks if a string contains only digits
func isAllDigits(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// parseStateVersionID safely parses a state version ID to extract the encoded state ID and timestamp
// Format: sv-{base58_encoded_state_id}-{unix_timestamp}
func parseStateVersionID(stateVersionID string) (stateID string, err error) {
	if !strings.HasPrefix(stateVersionID, "sv-") {
		return "", fmt.Errorf("invalid state version ID format: missing sv- prefix")
	}

	rest := stateVersionID[3:]

	// Find the timestamp part (should be all digits after last hyphen)
	// We need to be careful since base58 does not contain hyphens (unlike base64url)
	idx := -1
	for i := len(rest) - 1; i >= 0; i-- {
		if rest[i] == '-' {
			// Check if everything after this hyphen is digits (timestamp)
			timestampPart := rest[i+1:]
			if len(timestampPart) > 0 && isAllDigits(timestampPart) {
				idx = i
				break
			}
		}
	}

	if idx <= 0 || idx >= len(rest)-1 {
		return "", fmt.Errorf("invalid state version ID format: cannot find timestamp separator")
	}

	encodedStateID := rest[:idx]

	// Decode the base58-encoded state ID
	stateIDBytes, err := base58.Decode(encodedStateID)
	if err != nil {
		return "", fmt.Errorf("invalid state version ID encoding: %w", err)
	}

	stateID = string(stateIDBytes)
	if len(stateID) == 0 {
		return "", fmt.Errorf("decoded state ID is empty")
	}

	return stateID, nil
}

// generateStateVersionID creates a consistent state version ID using the provided timestamp
// Format: sv-{base58_encoded_state_id}-{unix_timestamp}
func generateStateVersionID(stateID string, timestamp int64) string {
	encodedStateID := base58.Encode([]byte(stateID))
	return fmt.Sprintf("sv-%s-%d", encodedStateID, timestamp)
}

// convertWorkspaceToStateIDWithOrg converts a workspace name to an org-scoped unit ID
// Workspace name is a human-readable unit name (e.g., "my-app-prod")
// Returns: "<org-uuid>/<unit-uuid>" for S3 storage (both UUIDs for immutable paths)
func (h *TfeHandler) convertWorkspaceToStateIDWithOrg(ctx context.Context, orgIdentifier, workspaceName string) (string, error) {
	// Validate input
	if strings.TrimSpace(workspaceName) == "" {
		return "", fmt.Errorf("workspace name cannot be empty")
	}

	// Strip "ws-" prefix if present (TFE compatibility for legacy workspace IDs)
	if strings.HasPrefix(workspaceName, "ws-") {
		workspaceName = strings.TrimPrefix(workspaceName, "ws-")
		if workspaceName == "" {
			return "", fmt.Errorf("invalid workspace name: empty after stripping ws- prefix")
		}
	}
	
	// If no org identifier provided or no resolver, return workspace name as-is (backwards compat)
	if orgIdentifier == "" || h.identifierResolver == nil {
		return workspaceName, nil
	}
	
	// Step 1: Resolve organization identifier (external_org_id or UUID) to UUID
	orgUUID, err := h.identifierResolver.ResolveOrganization(ctx, orgIdentifier)
	if err != nil {
		return "", fmt.Errorf("failed to resolve organization '%s': %w", orgIdentifier, err)
	}
	
	// Step 2: Resolve unit name to UUID within the organization
	unitUUID, err := h.identifierResolver.ResolveUnit(ctx, workspaceName, orgUUID)
	if err != nil {
		return "", fmt.Errorf("failed to resolve unit '%s' in org '%s': %w", workspaceName, orgIdentifier, err)
	}
	
	// Return org-scoped unit path for S3 storage
	// Format: <org-uuid>/<unit-uuid> (both UUIDs for immutable, rename-safe paths)
	// Example: "123e4567-e89b-12d3-a456-426614174000/987f6543-e21a-43d2-b789-123456789abc"
	return fmt.Sprintf("%s/%s", orgUUID, unitUUID), nil
}

// Legacy function for backwards compatibility - no org scoping
func convertWorkspaceToStateID(workspaceID string) string {
	if strings.TrimSpace(workspaceID) == "" {
		return ""
	}
	if strings.HasPrefix(workspaceID, "ws-") {
		result := strings.TrimPrefix(workspaceID, "ws-")
		if result == "" {
			return ""
		}
		return result
	}
	return workspaceID
}

// getOrgFromContext extracts organization identifier from the echo context
// The org is set by authentication middleware (JWT contains org claim)
// Returns error if no organization context is found
func getOrgFromContext(c echo.Context) (string, error) {
	// Try jwt_org first (set by RequireAuth middleware from JWT claims)
	if jwtOrg := c.Get("jwt_org"); jwtOrg != nil {
		if orgStr, ok := jwtOrg.(string); ok && orgStr != "" {
			return orgStr, nil
		}
	}
	
	// Try organization_id (set by WebhookAuth middleware)
	if orgID := c.Get("organization_id"); orgID != nil {
		if orgStr, ok := orgID.(string); ok && orgStr != "" {
			return orgStr, nil
		}
	}
	
	// No organization context found - this is an error condition
	return "", fmt.Errorf("no organization context found in request")
}

// parseOrgParam parses organization parameter in format "display:identifier" or just "identifier"
// Returns the identifier part that can be resolved (external_org_id, UUID, or name)
//
// Supported formats:
//   - "Personal:org_01K8..." -> returns "org_01K8..." (display name for convenience)
//   - "org_01K8..." -> returns "org_01K8..." (identifier only, works fine!)
//   - "Acme:123e4567-..." -> returns "123e4567-..." (UUID identifier)
//   - "123e4567-..." -> returns "123e4567-..." (UUID only)
//
// The display name (before colon) is purely cosmetic for user convenience.
// The identifier (after colon, or the whole string if no colon) is what gets resolved.
func parseOrgParam(orgParam string) (displayName, identifier string) {
	// Format: "DisplayName:identifier" or just "identifier"
	if strings.Contains(orgParam, ":") {
		parts := strings.SplitN(orgParam, ":", 2)
		if len(parts) == 2 {
			return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		}
	}
	// No colon - identifier only (display name omitted for convenience)
	identifier = strings.TrimSpace(orgParam)
	return "", identifier
}

// extractWorkspaceIDFromParam extracts workspace ID from URL parameter
func extractWorkspaceIDFromParam(c echo.Context) string {
	workspaceID := c.Param("workspace_id")
	if workspaceID == "" {
		// Fallback to workspace_name for routes that use that parameter
		workspaceName := c.Param("workspace_name")
		if workspaceName != "" {
			return tfe.NewTfeResourceIdentifier(tfe.WorkspaceType, workspaceName).String()
		}
	}
	return workspaceID
}

// checkWorkspacePermission handles the three RBAC scenarios correctly
func (h *TfeHandler) checkWorkspacePermission(c echo.Context, action string, workspaceID string) error {
	// Scenario 1: No RBAC manager (memory storage) → permissive mode
	if h.rbacManager == nil {
		return nil
	}

	// Scenario 2 & 3: Check if RBAC system has been initialized
	enabled, err := h.rbacManager.IsEnabled(c.Request().Context())
	if err != nil {
		// If we can't check RBAC status, log but don't block (fail open)
		fmt.Printf("Failed to check RBAC status: %v\n", err)
		return nil
	}

	// Scenario 2: RBAC manager exists but not initialized → permissive mode
	if !enabled {
		return nil
	}

	// Scenario 3: RBAC is initialized → enforce permissions
	stateID := convertWorkspaceToStateID(workspaceID)

	// Extract user subject from JWT token in Authorization header
	authHeader := c.Request().Header.Get("Authorization")
	if authHeader == "" {
		return fmt.Errorf("no authorization header")
	}

	if !strings.HasPrefix(authHeader, "Bearer ") {
		return fmt.Errorf("invalid authorization header format")
	}

	// Extract and verify JWT token to get user principal
	token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))

	// Get signer from auth handler to verify the token
	signer := h.authHandler.GetSigner()
	if signer == nil {
		// If no signer available, use a permissive approach for backwards compatibility
		principal := rbac.Principal{Subject: "unknown"}
		// Continue with permission check using unknown subject
		var rbacAction rbac.Action

		switch action {
		case "unit:read":
			rbacAction = rbac.ActionUnitRead
		case "unit:write":
			rbacAction = rbac.ActionUnitWrite
		case "unit:lock":
			rbacAction = rbac.ActionUnitLock
		default:
			return fmt.Errorf("unknown action: %s", action)
		}

		// Check permission using RBAC manager
		allowed, err := h.rbacManager.Can(c.Request().Context(), principal, rbacAction, stateID)
		if err != nil {
			return fmt.Errorf("failed to check permissions: %v", err)
		}

		if !allowed {
			return fmt.Errorf("insufficient permissions")
		}
		return nil
	}

	// TFE endpoints: verify opaque token only (for clear API boundaries)
	var principal rbac.Principal
	if h.apiTokens != nil {
		// Extract org from context or default to "default"
		orgID := getOrgFromContext(c)
		if tokenRecord, err := h.apiTokens.Verify(c.Request().Context(), orgID, token); err == nil {
			principal = rbac.Principal{
				Subject: tokenRecord.Subject,
				Email:   tokenRecord.Email,
				Roles:   []string{}, // Opaque tokens don't have roles directly
				Groups:  tokenRecord.Groups,
			}
		} else {
			return fmt.Errorf("invalid opaque token for TFE endpoint: %v", err)
		}
	} else {
		return fmt.Errorf("API token manager not available")
	}
	var rbacAction rbac.Action

	switch action {
	case "unit:read":
		rbacAction = rbac.ActionUnitRead
	case "unit:write":
		rbacAction = rbac.ActionUnitWrite
	case "unit:lock":
		rbacAction = rbac.ActionUnitLock
	default:
		return fmt.Errorf("unknown action: %s", action)
	}

	// Check permission using RBAC manager
	allowed, err := h.rbacManager.Can(c.Request().Context(), principal, rbacAction, stateID)
	if err != nil {
		return fmt.Errorf("failed to check permissions: %v", err)
	}

	if !allowed {
		return fmt.Errorf("insufficient permissions")
	}

	return nil
}

func (h *TfeHandler) GetWorkspace(c echo.Context) error {
	c.Response().Header().Set(echo.HeaderContentType, "application/vnd.api+json")
	c.Response().Header().Set("Tfp-Api-Version", "2.5")
	c.Response().Header().Set("X-Terraform-Enterprise-App", "Terraform Enterprise")

	orgParam := c.Param("org_name")
	workspaceName := c.Param("workspace_name")
	
	if workspaceName == "" {
		return c.JSON(400, map[string]string{"error": "workspace_name invalid"})
	}
	
	// Parse org param - supports both "Display:identifier" and just "identifier"
	displayName, orgIdentifier := parseOrgParam(orgParam)
	
	if displayName != "" {
		fmt.Printf("GetWorkspace: orgParam=%s (display=%s, identifier=%s), workspaceName=%s\n", 
			orgParam, displayName, orgIdentifier, workspaceName)
	} else {
		fmt.Printf("GetWorkspace: orgParam=%s (identifier only, no display name), workspaceName=%s\n", 
			orgIdentifier, workspaceName)
	}
	
	// Convert workspace name to unit ID (org-scoped if org provided)
	// workspaceName is now the human-readable unit name, not a UUID
	stateID, err := h.convertWorkspaceToStateIDWithOrg(c.Request().Context(), orgIdentifier, workspaceName)
	if err != nil {
		fmt.Printf("GetWorkspace: failed to resolve workspace: %v\n", err)
		return c.JSON(500, map[string]string{
			"error": "failed to resolve workspace",
			"detail": err.Error(),
		})
	}
	
	fmt.Printf("GetWorkspace: resolved stateID=%s\n", stateID)
	
	// Check if unit exists (optional - may auto-create later)
	_, err = h.stateStore.Get(c.Request().Context(), stateID)
	locked := false
	if err == nil {
		// Check if locked
		lockInfo, _ := h.stateStore.GetLock(c.Request().Context(), stateID)
		locked = (lockInfo != nil)
	}

	workspace := &tfe.TFEWorkspace{
		ID:                         tfe.NewTfeResourceIdentifier(tfe.WorkspaceType, workspaceName).String(),
		Actions:                    &tfe.TFEWorkspaceActions{IsDestroyable: true},
		AgentPoolID:                tfe.NewTfeResourceIdentifier(tfe.AgentPoolType, "HzEaJWMP5YTatZaS").String(),
		AllowDestroyPlan:           false,
		AutoApply:                  false,
		CanQueueDestroyPlan:        false,
		CreatedAt:                  time.Time{},
		UpdatedAt:                  time.Time{},
		Description:                workspaceName,
		Environment:                workspaceName,
		ExecutionMode:              "local",
		FileTriggersEnabled:        false,
		GlobalRemoteState:          false,
		Locked:                     locked,
		MigrationEnvironment:       "",
		Name:                       workspaceName,
		Operations:                 false,
		Permissions:                nil,
		QueueAllRuns:               false,
		SpeculativeEnabled:         false,
		SourceName:                 "",
		SourceURL:                  "",
		StructuredRunOutputEnabled: false,
		TerraformVersion:           nil,
		TriggerPrefixes:            nil,
		TriggerPatterns:            nil,
		VCSRepo:                    nil,
		WorkingDirectory:           "",
		ResourceCount:              0,
		ApplyDurationAverage:       0,
		PlanDurationAverage:        0,
		PolicyCheckFailures:        0,
		RunFailures:                0,
		RunsCount:                  0,
		TagNames:                   nil,
		CurrentRun:                 nil,
		Organization: &tfe.TFEOrganization{
			Name: orgParam,  // Return the full org param (includes display name if provided)
		},
		Outputs: nil,
	}

	if err := jsonapi.MarshalPayload(c.Response().Writer, workspace); err != nil {
		fmt.Printf("error marshaling workspace payload: %v", err)
		return err
	}
	return nil
}

func (h *TfeHandler) LockWorkspace(c echo.Context) error {
	c.Response().Header().Set(echo.HeaderContentType, "application/json")
	c.Response().Header().Set("Tfp-Api-Version", "2.5")
	c.Response().Header().Set("X-Terraform-Enterprise-App", "Terraform Enterprise")

	// Extract workspace ID (format: ws-{workspace-name})
	workspaceID := extractWorkspaceIDFromParam(c)
	if workspaceID == "" {
		return c.JSON(400, map[string]string{"error": "workspace_id required"})
	}

	// Strip ws- prefix to get workspace name
	workspaceName := convertWorkspaceToStateID(workspaceID)
	fmt.Printf("LockWorkspace: workspaceID=%s, workspaceName=%s\n", workspaceID, workspaceName)

	// Get org from authentication context (JWT claim or webhook header)
	orgIdentifier, err := getOrgFromContext(c)
	if err != nil {
		fmt.Printf("LockWorkspace: %v\n", err)
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "Organization context required",
			"detail": err.Error(),
		})
	}
	fmt.Printf("LockWorkspace: orgIdentifier=%s (from auth context)\n", orgIdentifier)

	// Resolve to UUID/UUID path
	stateID, err := h.convertWorkspaceToStateIDWithOrg(c.Request().Context(), orgIdentifier, workspaceName)
	if err != nil {
		fmt.Printf("LockWorkspace: failed to resolve workspace: %v\n", err)
		return c.JSON(500, map[string]string{
			"error": "failed to resolve workspace",
			"detail": err.Error(),
		})
	}
	fmt.Printf("LockWorkspace: resolved stateID=%s\n", stateID)

	// Check RBAC permission for locking workspace
	if err := h.checkWorkspacePermission(c, "unit:write", stateID); err != nil {
		return c.JSON(http.StatusForbidden, map[string]string{
			"error": "insufficient permissions to lock workspace",
			"hint":  "contact your administrator to grant unit:write permission",
		})
	}

	if h.stateStore == nil {
		fmt.Printf("LockWorkspace: stateStore is nil!\n")
		return c.JSON(500, map[string]string{"error": "State store not initialized"})
	}

	// Check if state exists, enot
	_, err := h.stateStore.Get(c.Request().Context(), stateID)
	fmt.Printf("LockWorkspace: Get result, err=%v\n", err)
	if err == storage.ErrNotFound {
		fmt.Printf("LockWorkspace: Unit not found - no auto-creation\n")
		return c.JSON(404, map[string]string{
			"error": "Unit not found. Please create the unit first using 'taco unit create " + stateID + "' or the opentaco_unit Terraform resource.",
		})
	} else if err != nil {
		// Handle other errors from Get()
		fmt.Printf("LockWorkspace: Get failed with: %v\n", err)
		return c.JSON(500, map[string]string{
			"error": "Failed to check state existence",
		})
	}

	// Create lock info
	lockInfo := &storage.LockInfo{
		ID:      uuid.New().String(),
		Who:     "terraform-cloud",
		Version: "1.0.0",
		Created: time.Now(),
	}
	fmt.Printf("LockWorkspace: Attempting to lock with info: %+v\n", lockInfo)

	// Attempt to lock the state
	err = h.stateStore.Lock(c.Request().Context(), stateID, lockInfo)
	fmt.Printf("LockWorkspace: Lock result, err=%v\n", err)
	if err != nil {
		// Check for lock conflict using strings.Contains since error message may have additional text
		if strings.Contains(err.Error(), "lock conflict") {
			fmt.Printf("LockWorkspace: Lock conflict detected\n")
			// Get current lock for details
			currentLock, _ := h.stateStore.GetLock(c.Request().Context(), stateID)
			if currentLock != nil {
				fmt.Printf("LockWorkspace: Returning 423 with lock details\n")
				return c.JSON(423, map[string]interface{}{
					"error": "workspace_locked",
					"lock": map[string]interface{}{
						"id":      currentLock.ID,
						"who":     currentLock.Who,
						"version": currentLock.Version,
						"created": currentLock.Created,
					},
				})
			}
			fmt.Printf("LockWorkspace: Returning 409 workspace locked\n")
			return c.JSON(409, map[string]string{
				"error": "Workspace is already locked",
			})
		}
		fmt.Printf("LockWorkspace: Returning 500 for non-lock error: %v\n", err)
		return c.JSON(500, map[string]string{
			"error": "Failed to acquire workspace lock",
		})
	}

	// Return success with lock info
	fmt.Printf("LockWorkspace: Returning success\n")
	return c.JSON(200, map[string]interface{}{
		"data": map[string]interface{}{
			"id":   workspaceID,
			"type": "workspaces",
			"attributes": map[string]interface{}{
				"locked": true,
			},
		},
	})
}

func (h *TfeHandler) UnlockWorkspace(c echo.Context) error {
	c.Response().Header().Set(echo.HeaderContentType, "application/json")
	c.Response().Header().Set("Tfp-Api-Version", "2.5")
	c.Response().Header().Set("X-Terraform-Enterprise-App", "Terraform Enterprise")

	// Extract workspace ID (format: ws-{workspace-name})
	workspaceID := extractWorkspaceIDFromParam(c)
	if workspaceID == "" {
		return c.JSON(400, map[string]string{"error": "workspace_id required"})
	}

	// Strip ws- prefix to get workspace name
	workspaceName := convertWorkspaceToStateID(workspaceID)
	
	// Get org from authentication context (JWT claim or webhook header)
	orgIdentifier, err := getOrgFromContext(c)
	if err != nil {
		fmt.Printf("UnlockWorkspace: %v\n", err)
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "Organization context required",
			"detail": err.Error(),
		})
	}
	
	// Resolve to UUID/UUID path
	stateID, err := h.convertWorkspaceToStateIDWithOrg(c.Request().Context(), orgIdentifier, workspaceName)
	if err != nil {
		fmt.Printf("UnlockWorkspace: failed to resolve workspace: %v\n", err)
		return c.JSON(500, map[string]string{
			"error": "failed to resolve workspace",
			"detail": err.Error(),
		})
	}
	fmt.Printf("UnlockWorkspace: workspaceID=%s, resolved stateID=%s\n", workspaceID, stateID)

	// Get current lock to find lock ID
	currentLock, err := h.stateStore.GetLock(c.Request().Context(), stateID)
	fmt.Printf("UnlockWorkspace: GetLock result, err=%v, currentLock=%v\n", err, currentLock)
	if err != nil {
		if err == storage.ErrNotFound {
			fmt.Printf("UnlockWorkspace: State not found\n")
			return c.JSON(404, map[string]string{"error": "Workspace not found"})
		}
		fmt.Printf("UnlockWorkspace: Failed to get lock status: %v\n", err)
		return c.JSON(500, map[string]string{"error": "Failed to get lock status"})
	}

	if currentLock == nil {
		// Already unlocked, return success
		fmt.Printf("UnlockWorkspace: No lock found, already unlocked\n")
		return c.JSON(200, map[string]interface{}{
			"data": map[string]interface{}{
				"id":   workspaceID,
				"type": "workspaces",
				"attributes": map[string]interface{}{
					"locked": false,
				},
			},
		})
	}

	fmt.Printf("UnlockWorkspace: Unlocking with lock ID: %s\n", currentLock.ID)

	// Unlock the state using the current lock ID
	err = h.stateStore.Unlock(c.Request().Context(), stateID, currentLock.ID)
	fmt.Printf("UnlockWorkspace: Unlock result, err=%v\n", err)
	if err != nil {
		if err == storage.ErrNotFound {
			return c.JSON(404, map[string]string{"error": "Workspace not found"})
		}
		if err == storage.ErrLockConflict {
			return c.JSON(409, map[string]string{"error": "Lock ID mismatch"})
		}
		return c.JSON(500, map[string]string{"error": "Failed to release lock"})
	}

	// Return success
	return c.JSON(200, map[string]interface{}{
		"data": map[string]interface{}{
			"id":   workspaceID,
			"type": "workspaces",
			"attributes": map[string]interface{}{
				"locked": false,
			},
		},
	})
}

// ForceUnlockWorkspace handles POST /tfe/api/v2/workspaces/:workspace_id/actions/force-unlock
func (h *TfeHandler) ForceUnlockWorkspace(c echo.Context) error {
	c.Response().Header().Set(echo.HeaderContentType, "application/json")
	c.Response().Header().Set("Tfp-Api-Version", "2.5")
	c.Response().Header().Set("X-Terraform-Enterprise-App", "Terraform Enterprise")

	// Extract workspace ID (format: ws-{workspace-name})
	workspaceID := extractWorkspaceIDFromParam(c)
	if workspaceID == "" {
		return c.JSON(400, map[string]string{"error": "workspace_id required"})
	}

	// Strip ws- prefix to get workspace name
	workspaceName := convertWorkspaceToStateID(workspaceID)
	
	// Get org from authentication context (JWT claim or webhook header)
	orgIdentifier, err := getOrgFromContext(c)
	if err != nil {
		fmt.Printf("ForceUnlockWorkspace: %v\n", err)
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "Organization context required",
			"detail": err.Error(),
		})
	}
	
	// Resolve to UUID/UUID path
	stateID, err := h.convertWorkspaceToStateIDWithOrg(c.Request().Context(), orgIdentifier, workspaceName)
	if err != nil {
		fmt.Printf("ForceUnlockWorkspace: failed to resolve workspace: %v\n", err)
		return c.JSON(500, map[string]string{
			"error": "failed to resolve workspace",
			"detail": err.Error(),
		})
	}
	fmt.Printf("ForceUnlockWorkspace: workspaceID=%s, resolved stateID=%s\n", workspaceID, stateID)

	// Get current lock to find lock ID
	currentLock, err := h.stateStore.GetLock(c.Request().Context(), stateID)
	if err != nil {
		if err == storage.ErrNotFound {
			return c.JSON(404, map[string]string{"error": "Workspace not found"})
		}
		return c.JSON(500, map[string]string{"error": "Failed to get lock status"})
	}

	if currentLock == nil {
		// Already unlocked, return success
		fmt.Printf("ForceUnlockWorkspace: No lock found, already unlocked\n")
		return c.JSON(200, map[string]interface{}{
			"data": map[string]interface{}{
				"id":   workspaceID,
				"type": "workspaces",
				"attributes": map[string]interface{}{
					"locked": false,
				},
			},
		})
	}

	fmt.Printf("ForceUnlockWorkspace: Force unlocking with lock ID: %s\n", currentLock.ID)

	// Force unlock the state using the current lock ID
	err = h.stateStore.Unlock(c.Request().Context(), stateID, currentLock.ID)
	if err != nil {
		fmt.Printf("ForceUnlockWorkspace: Failed to unlock: %v\n", err)
		return c.JSON(500, map[string]string{"error": "Failed to force unlock"})
	}

	fmt.Printf("ForceUnlockWorkspace: Successfully force unlocked\n")

	// Return success
	return c.JSON(200, map[string]interface{}{
		"data": map[string]interface{}{
			"id":   workspaceID,
			"type": "workspaces",
			"attributes": map[string]interface{}{
				"locked": false,
			},
		},
	})
}

// GetCurrentStateVersion handles GET /tfe/api/v2/workspaces/:workspace_id/current-state-version
func (h *TfeHandler) GetCurrentStateVersion(c echo.Context) error {
	c.Response().Header().Set(echo.HeaderContentType, "application/vnd.api+json")
	c.Response().Header().Set("Tfp-Api-Version", "2.5")
	c.Response().Header().Set("X-Terraform-Enterprise-App", "Terraform Enterprise")

	// Extract workspace ID (format: ws-{workspace-name})
	workspaceID := extractWorkspaceIDFromParam(c)
	fmt.Printf("GetCurrentStateVersion: workspaceID=%s\n", workspaceID)
	if workspaceID == "" {
		fmt.Printf("GetCurrentStateVersion: ERROR - workspace_id required\n")
		return c.JSON(400, map[string]string{"error": "workspace_id required"})
	}

	// Strip ws- prefix to get workspace name
	workspaceName := convertWorkspaceToStateID(workspaceID)
	
	// Get org from authentication context (JWT claim or webhook header)
	orgIdentifier, err := getOrgFromContext(c)
	if err != nil {
		fmt.Printf("GetCurrentStateVersion: %v\n", err)
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "Organization context required",
			"detail": err.Error(),
		})
	}
	
	// Resolve to UUID/UUID path
	stateID, err := h.convertWorkspaceToStateIDWithOrg(c.Request().Context(), orgIdentifier, workspaceName)
	if err != nil {
		fmt.Printf("GetCurrentStateVersion: failed to resolve workspace: %v\n", err)
		return c.JSON(500, map[string]string{
			"error": "failed to resolve workspace",
			"detail": err.Error(),
		})
	}
	fmt.Printf("GetCurrentStateVersion: resolved stateID=%s\n", stateID)

	// Check RBAC permission with correct three-scenario logic
	if err := h.checkWorkspacePermission(c, "unit:read", stateID); err != nil {
		return c.JSON(http.StatusForbidden, map[string]string{
			"error": "insufficient permissions to access workspace",
			"hint":  "contact your administrator to grant unit:read permission",
		})
	}

	// Check if state exists
	stateMeta, err := h.stateStore.Get(c.Request().Context(), stateID)
	fmt.Printf("GetCurrentStateVersion: Get result, err=%v\n", err)
	if err != nil {
		if err == storage.ErrNotFound {
			fmt.Printf("GetCurrentStateVersion: Unit not found\n")
			return c.JSON(404, map[string]string{
				"error": "Unit not found. Please create the unit first using 'taco unit create " + stateID + "' or the opentaco_unit Terraform resource.",
			})
		}
		fmt.Printf("GetCurrentStateVersion: ERROR - Failed to get workspace state: %v\n", err)
		return c.JSON(500, map[string]string{"error": "Failed to get workspace state"})
	}

	// Generate a state version ID based on state ID and timestamp
	stateVersionID := generateStateVersionID(stateID, stateMeta.Updated.Unix())
	fmt.Printf("GetCurrentStateVersion: Returning stateVersionID=%s, size=%d\n", stateVersionID, stateMeta.Size)

	baseURL := getBaseURL(c)
	downloadURL := fmt.Sprintf("%s/tfe/api/v2/state-versions/%s/download", baseURL, stateVersionID)

	// Return current state version info
	return c.JSON(200, map[string]interface{}{
		"data": map[string]interface{}{
			"id":   stateVersionID,
			"type": "state-versions",
			"attributes": map[string]interface{}{
				"created-at":                stateMeta.Updated.UTC().Format(time.RFC3339),
				"updated-at":                stateMeta.Updated.UTC().Format(time.RFC3339),
				"size":                      stateMeta.Size,
				"hosted-state-download-url": downloadURL,
			},
			"relationships": map[string]interface{}{
				"workspace": map[string]interface{}{
					"data": map[string]interface{}{
						"id":   workspaceID,
						"type": "workspaces",
					},
				},
			},
		},
	})
}

// CreateStateVersion handles POST /tfe/api/v2/workspaces/:workspace_id/state-versions
func (h *TfeHandler) CreateStateVersion(c echo.Context) error {
	fmt.Printf("CreateStateVersion: START - Method=%s, URI=%s\n", c.Request().Method, c.Request().RequestURI)
	c.Response().Header().Set(echo.HeaderContentType, "application/vnd.api+json")
	c.Response().Header().Set("Tfp-Api-Version", "2.5")
	c.Response().Header().Set("X-Terraform-Enterprise-App", "Terraform Enterprise")

	// Extract workspace ID (format: ws-{workspace-name})
	workspaceID := extractWorkspaceIDFromParam(c)
	fmt.Printf("CreateStateVersion: workspaceID=%s\n", workspaceID)
	if workspaceID == "" {
		fmt.Printf("CreateStateVersion: ERROR - workspace_id required\n")
		return c.JSON(400, map[string]string{"error": "workspace_id required"})
	}

	// Strip ws- prefix to get workspace name
	workspaceName := convertWorkspaceToStateID(workspaceID)
	
	// Get org from authentication context (JWT claim or webhook header)
	orgIdentifier, err := getOrgFromContext(c)
	if err != nil {
		fmt.Printf("CreateStateVersion: %v\n", err)
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "Organization context required",
			"detail": err.Error(),
		})
	}
	
	// Resolve to UUID/UUID path
	stateID, err := h.convertWorkspaceToStateIDWithOrg(c.Request().Context(), orgIdentifier, workspaceName)
	if err != nil {
		fmt.Printf("CreateStateVersion: failed to resolve workspace: %v\n", err)
		return c.JSON(500, map[string]string{
			"error": "failed to resolve workspace",
			"detail": err.Error(),
		})
	}
	fmt.Printf("CreateStateVersion: resolved stateID=%s\n", stateID)

	// Check RBAC permission for creating/writing state versions
	if err := h.checkWorkspacePermission(c, "unit:write", stateID); err != nil {
		return c.JSON(http.StatusForbidden, map[string]string{
			"error": "insufficient permissions to create state version",
			"hint":  "contact your administrator to grant unit:write permission",
		})
	}

	// Read request body length first
	bodyBytes, err := io.ReadAll(c.Request().Body)
	if err != nil {
		fmt.Printf("CreateStateVersion: ERROR - Failed to read body: %v\n", err)
		return c.JSON(400, map[string]string{"error": "Failed to read request body"})
	}
	fmt.Printf("CreateStateVersion: Body length=%d bytes\n", len(bodyBytes))
	fmt.Printf("CreateStateVersion: Body preview: %s\n", string(bodyBytes[:min(200, len(bodyBytes))]))

	// Parse the JSON request body for metadata (not state content)
	var request map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &request); err != nil {
		fmt.Printf("CreateStateVersion: Body is not JSON, treating as direct state upload\n")
		// For direct upload without JSON wrapper, handle as raw state data
		return h.CreateStateVersionDirect(c, workspaceID, stateID, bodyBytes)
	}
	fmt.Printf("CreateStateVersion: Parsed JSON request: %+v\n", request)

	// Extract the actual state data from the request
	data, ok := request["data"].(map[string]interface{})
	if !ok {
		fmt.Printf("CreateStateVersion: ERROR - Invalid request format, missing data\n")
		return c.JSON(400, map[string]string{"error": "Invalid request format"})
	}

	attributes, ok := data["attributes"].(map[string]interface{})
	if !ok {
		fmt.Printf("CreateStateVersion: ERROR - Invalid request format, missing attributes\n")
		return c.JSON(400, map[string]string{"error": "Invalid request format"})
	}

	// Look for the actual state content - it might be base64 encoded or in a specific field
	if jsonStateOutputs, exists := attributes["json-state-outputs"]; exists {
		fmt.Printf("CreateStateVersion: Found json-state-outputs field\n")
		// This might be base64 encoded JSON
		if encoded, ok := jsonStateOutputs.(string); ok {
			_, err := base64.StdEncoding.DecodeString(encoded)
			if err == nil {
				fmt.Printf("CreateStateVersion: Successfully decoded base64 state outputs\n")
				// This contains outputs metadata only; Terraform will upload state to the hosted URL
			}
		}
	}

	// Check that state exists (no auto-creation)
	_, err = h.stateStore.Get(c.Request().Context(), stateID)
	if err == storage.ErrNotFound {
		fmt.Printf("CreateStateVersion: Unit not found\n")
		return c.JSON(404, map[string]string{
			"error": "Unit not found. Please create the unit first using 'taco unit create " + stateID + "' or the opentaco_unit Terraform resource.",
		})
	} else if err != nil {
		fmt.Printf("CreateStateVersion: ERROR - Failed to check state existence: %v\n", err)
		return c.JSON(500, map[string]string{
			"error": "Failed to check state existence",
		})
	}

	// Generate a state version ID (before upload) based on state ID and current time
	stateVersionID := generateStateVersionID(stateID, time.Now().Unix())
	fmt.Printf("CreateStateVersion: Returning pending stateVersionID=%s (awaiting upload)\n", stateVersionID)

	// Build URLs
	baseURL := getBaseURL(c)
	uploadURL := fmt.Sprintf("%s/tfe/api/v2/state-versions/%s/upload", baseURL, stateVersionID)
	downloadURL := fmt.Sprintf("%s/tfe/api/v2/state-versions/%s/download", baseURL, stateVersionID)
	jsonUploadURL := fmt.Sprintf("%s/tfe/api/v2/state-versions/%s/json-upload", baseURL, stateVersionID)

	// Derive serial and lineage from existing state (if any)
	serial := 0
	lineage := ""
	if stateBytes, dErr := h.stateStore.Download(c.Request().Context(), stateID); dErr == nil {
		var st map[string]interface{}
		if uErr := json.Unmarshal(stateBytes, &st); uErr == nil {
			if v, ok := st["serial"].(float64); ok {
				serial = int(v)
			}
			if v, ok := st["lineage"].(string); ok {
				lineage = v
			}
		}
	}

	fmt.Printf("CreateStateVersion: baseURL='%s'\n", baseURL)
	fmt.Printf("CreateStateVersion: uploadURL='%s'\n", uploadURL)

	// Build the response
	response := map[string]interface{}{
		"data": map[string]interface{}{
			"id":   stateVersionID,
			"type": "state-versions",
			"attributes": map[string]interface{}{
				"created-at":                   time.Now().UTC().Format(time.RFC3339),
				"updated-at":                   time.Now().UTC().Format(time.RFC3339),
				"size":                         0,
				"upload-url":                   uploadURL,
				"hosted-state-upload-url":      uploadURL,
				"hosted-state-download-url":    downloadURL,
				"hosted-json-state-upload-url": jsonUploadURL,
				"serial":                       serial,
				"lineage":                      lineage,
			},
			"relationships": map[string]interface{}{
				"workspace": map[string]interface{}{
					"data": map[string]interface{}{
						"id":   workspaceID,
						"type": "workspaces",
					},
				},
			},
		},
	}

	// Convert to actual JSON to see what gets sent to Terraform
	jsonBytes, err := json.Marshal(response)
	if err != nil {
		fmt.Printf("CreateStateVersion: ERROR - Failed to marshal JSON: %v\n", err)
		return c.JSON(500, map[string]string{"error": "Failed to create response"})
	}
	fmt.Printf("CreateStateVersion: Actual JSON being sent: %s\n", string(jsonBytes))

	return c.JSON(201, response)
}

// CreateStateVersionDirect handles direct state upload without JSON wrapper (fallback)
func (h *TfeHandler) CreateStateVersionDirect(c echo.Context, workspaceID, stateID string, body []byte) error {
	fmt.Printf("CreateStateVersionDirect: START - workspaceID=%s, stateID=%s, bodyLen=%d\n", workspaceID, stateID, len(body))

	// Check if state exists, create if not
	_, err := h.stateStore.Get(c.Request().Context(), stateID)
	if err == storage.ErrNotFound {
		fmt.Printf("CreateStateVersionDirect: Unit not found\n")
		return c.JSON(404, map[string]string{
			"error": "Unit not found. Please create the unit first using 'taco unit create " + stateID + "' or the opentaco_unit Terraform resource.",
		})
	}

	// Get the current lock to extract lock ID for state upload
	currentLock, lockErr := h.stateStore.GetLock(c.Request().Context(), stateID)
	if lockErr != nil && lockErr != storage.ErrNotFound {
		return c.JSON(500, map[string]string{"error": "Failed to get lock status"})
	}

	// Extract lock ID if state is locked
	lockID := ""
	if currentLock != nil {
		lockID = currentLock.ID
	}

	// Upload the state with proper lock ID
	err = h.stateStore.Upload(c.Request().Context(), stateID, body, lockID)
	if err != nil {
		if err == storage.ErrLockConflict {
			return c.JSON(423, map[string]string{
				"error": "Workspace is locked",
			})
		}
		return c.JSON(500, map[string]string{
			"error": "Failed to upload state",
		})
	}

	// Get updated metadata
	stateMeta, err := h.stateStore.Get(c.Request().Context(), stateID)
	if err != nil {
		return c.JSON(500, map[string]string{"error": "Failed to get updated state metadata"})
	}
	stateVersionID := generateStateVersionID(stateID, stateMeta.Updated.Unix())

	// Return the new state version
	return c.JSON(201, map[string]interface{}{
		"data": map[string]interface{}{
			"id":   stateVersionID,
			"type": "state-versions",
			"attributes": map[string]interface{}{
				"created-at": stateMeta.Updated.UTC().Format(time.RFC3339),
				"updated-at": stateMeta.Updated.UTC().Format(time.RFC3339),
				"size":       stateMeta.Size,
			},
			"relationships": map[string]interface{}{
				"workspace": map[string]interface{}{
					"data": map[string]interface{}{
						"id":   workspaceID,
						"type": "workspaces",
					},
				},
			},
		},
	})
}

// DownloadStateVersion handles GET /tfe/api/v2/state-versions/:id/download
func (h *TfeHandler) DownloadStateVersion(c echo.Context) error {
	c.Response().Header().Set(echo.HeaderContentType, "application/vnd.api+json")
	stateVersionID := c.Param("id")
	if stateVersionID == "" {
		return c.JSON(400, map[string]string{"error": "state_version_id required"})
	}

	// Parse state version ID to extract workspace/state ID
	stateID, err := parseStateVersionID(stateVersionID)
	if err != nil {
		return c.JSON(400, map[string]string{"error": "Invalid state version ID format"})
	}

	// Download the state data
	stateData, err := h.stateStore.Download(c.Request().Context(), stateID)
	if err != nil {
		if err == storage.ErrNotFound {
			return c.JSON(404, map[string]string{"error": "State version not found"})
		}
		return c.JSON(500, map[string]string{"error": "Failed to download state"})
	}

	// Return the raw state data
	c.Response().Header().Set("Content-Type", "application/json")
	return c.Blob(http.StatusOK, "application/json", stateData)
}

// UploadStateVersion handles PUT /tfe/api/v2/state-versions/:id/upload
func (h *TfeHandler) UploadStateVersion(c echo.Context) error {
	fmt.Printf("UploadStateVersion: START - Method=%s, URI=%s\n", c.Request().Method, c.Request().RequestURI)

	// Debug: Check if Authorization header is present
	authHeader := c.Request().Header.Get("Authorization")
	fmt.Printf("UploadStateVersion: Authorization header present: %t\n", authHeader != "")
	if authHeader != "" {
		// Don't log the full token for security, just whether it looks like a Bearer token
		fmt.Printf("UploadStateVersion: Authorization header format: %s\n",
			strings.SplitN(authHeader, " ", 2)[0])
	}

	stateVersionID := c.Param("id")
	fmt.Printf("UploadStateVersion: stateVersionID=%s\n", stateVersionID)
	if stateVersionID == "" {
		fmt.Printf("UploadStateVersion: ERROR - state_version_id required\n")
		return c.JSON(400, map[string]string{"error": "state_version_id required"})
	}

	// Parse state version ID to extract workspace/state ID
	stateID, err := parseStateVersionID(stateVersionID)
	if err != nil {
		return c.JSON(400, map[string]string{"error": "Invalid state version ID format"})
	}
	workspaceID := stateID // For TFE, workspace ID matches state ID

	// Check RBAC permission for uploading state (if auth available)
	// Note: Upload endpoints are exempt from auth middleware since Terraform doesn't send headers
	// Security relies on: valid upload URL + lock ownership + this RBAC check when possible
	if err := h.checkWorkspacePermission(c, "unit:write", workspaceID); err != nil {
		// Only enforce RBAC if we have a real auth error, not just missing headers
		if !strings.Contains(err.Error(), "no authorization header") {
			fmt.Printf("UploadStateVersion: RBAC permission denied: %v\n", err)
			return c.JSON(http.StatusForbidden, map[string]string{
				"error": "insufficient permissions to upload state",
				"hint":  "contact your administrator to grant unit:write permission",
			})
		}
		// If no auth header, allow but log for security monitoring
		fmt.Printf("UploadStateVersion: No auth header - allowing upload based on lock validation\n")
	}

	// Read the state data from request body
	stateData, err := io.ReadAll(c.Request().Body)
	fmt.Printf("UploadStateVersion: Read %d bytes from body, err=%v\n", len(stateData), err)
	if err != nil {
		fmt.Printf("UploadStateVersion: ERROR - Failed to read state data: %v\n", err)
		return c.JSON(400, map[string]string{"error": "Failed to read state data"})
	}
	if len(stateData) > 0 {
		fmt.Printf("UploadStateVersion: Body preview: %s\n", string(stateData))
	}

	// Check if state exists (no auto-creation)
	_, err = h.stateStore.Get(c.Request().Context(), stateID)
	if err == storage.ErrNotFound {
		fmt.Printf("UploadStateVersion: Unit not found - no auto-creation\n")
		return c.JSON(404, map[string]string{
			"error": "Unit not found. Please create the unit first using 'taco unit create " + stateID + "' or the opentaco_unit Terraform resource.",
		})
	} else if err != nil {
		fmt.Printf("UploadStateVersion: ERROR - Failed to check state existence: %v\n", err)
		return c.JSON(500, map[string]string{
			"error": "Failed to check state existence",
		})
	}

	// Get the current lock to extract lock ID for state upload
	currentLock, lockErr := h.stateStore.GetLock(c.Request().Context(), stateID)
	if lockErr != nil && lockErr != storage.ErrNotFound {
		return c.JSON(500, map[string]string{"error": "Failed to get lock status"})
	}

	// Extract lock ID if state is locked
	lockID := ""
	if currentLock != nil {
		lockID = currentLock.ID
	}

	// Upload the state with proper lock ID
	fmt.Printf("UploadStateVersion: Uploading to storage with lockID=%s\n", lockID)
	err = h.stateStore.Upload(c.Request().Context(), stateID, stateData, lockID)
	fmt.Printf("UploadStateVersion: Upload result, err=%v\n", err)
	if err != nil {
		if err == storage.ErrLockConflict {
			fmt.Printf("UploadStateVersion: ERROR - Workspace is locked\n")
			return c.JSON(423, map[string]string{
				"error": "Workspace is locked",
			})
		}
		fmt.Printf("UploadStateVersion: ERROR - Failed to upload state: %v\n", err)
		return c.JSON(500, map[string]string{
			"error": "Failed to upload state",
		})
	}

	fmt.Printf("UploadStateVersion: SUCCESS - State uploaded successfully\n")
	// Return 204 No Content as expected by Terraform
	return c.NoContent(204)
}

func (h *TfeHandler) UploadJSONStateOutputs(c echo.Context) error {
	id := c.Param("id")
	fmt.Printf("UploadJSONStateOutputs: stateVersionID=%s\n", id)

	// Debug: Check if Authorization header is present
	authHeader := c.Request().Header.Get("Authorization")
	fmt.Printf("UploadJSONStateOutputs: Authorization header present: %t\n", authHeader != "")
	if authHeader != "" {
		fmt.Printf("UploadJSONStateOutputs: Authorization header format: %s\n",
			strings.SplitN(authHeader, " ", 2)[0])
	}

	// Parse state version ID to get workspace ID for RBAC check
	stateID, err := parseStateVersionID(id)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid state version ID format"})
	}
	workspaceID := stateID // For TFE, workspace ID matches state ID

	// Check RBAC permission for uploading state outputs (if auth available)
	// Note: Upload endpoints are exempt from auth middleware since Terraform doesn't send headers
	// Security relies on: valid upload URL + lock ownership + this RBAC check when possible
	if err := h.checkWorkspacePermission(c, "unit:write", workspaceID); err != nil {
		// Only enforce RBAC if we have a real auth error, not just missing headers
		if !strings.Contains(err.Error(), "no authorization header") {
			fmt.Printf("UploadJSONStateOutputs: RBAC permission denied: %v\n", err)
			return c.JSON(http.StatusForbidden, map[string]string{
				"error": "insufficient permissions to upload state outputs",
				"hint":  "contact your administrator to grant state:write permission",
			})
		}
		// If no auth header, allow but log for security monitoring
		fmt.Printf("UploadJSONStateOutputs: No auth header - allowing upload based on lock validation\n")
	}

	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Failed to read outputs"})
	}
	if len(body) > 0 {
		fmt.Printf("UploadJSONStateOutputs: %d bytes (preview: %s)\n", len(body), string(body[:min(200, len(body))]))
	}
	return c.NoContent(http.StatusNoContent) //
}

func (h *TfeHandler) ShowStateVersion(c echo.Context) error {
	c.Response().Header().Set(echo.HeaderContentType, "application/vnd.api+json")
	id := c.Param("id")
	if id == "" || !strings.HasPrefix(id, "sv-") {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"errors": []map[string]string{{"status": "404", "title": "not_found"}},
		})
	}

	// Parse state version ID to extract state ID
	stateID, err := parseStateVersionID(id)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"errors": []map[string]string{{"status": "404", "title": "invalid_id"}},
		})
	}

	// Load metadata (and optionally content)
	meta, err := h.stateStore.Get(c.Request().Context(), stateID)
	if err != nil {
		if err == storage.ErrNotFound {
			return c.JSON(http.StatusNotFound, map[string]interface{}{
				"errors": []map[string]string{{"status": "404", "title": "state_not_found"}},
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "state_meta_error"})
	}

	// Optional: extract serial/lineage + md5
	var serial int
	var lineage, md5b64 string
	if bytes, dErr := h.stateStore.Download(c.Request().Context(), stateID); dErr == nil && len(bytes) > 0 {
		var st map[string]interface{}
		if json.Unmarshal(bytes, &st) == nil {
			if v, ok := st["serial"].(float64); ok {
				serial = int(v)
			}
			if v, ok := st["lineage"].(string); ok {
				lineage = v
			}
		}
		sum := md5.Sum(bytes)
		md5b64 = base64.StdEncoding.EncodeToString(sum[:])
	}

	baseURL := getBaseURL(c)
	downloadURL := fmt.Sprintf("%s/tfe/api/v2/state-versions/%s/download", baseURL, id)

	resp := map[string]interface{}{
		"data": map[string]interface{}{
			"id":   id,
			"type": "state-versions",
			"attributes": map[string]interface{}{
				"created-at":                meta.Updated.UTC().Format(time.RFC3339),
				"updated-at":                meta.Updated.UTC().Format(time.RFC3339),
				"size":                      meta.Size,
				"serial":                    serial,
				"lineage":                   lineage,
				"md5":                       md5b64, // optional
				"hosted-state-download-url": downloadURL,
			},
			"relationships": map[string]interface{}{
				"workspace": map[string]interface{}{
					"data": map[string]interface{}{"id": stateID, "type": "workspaces"},
				},
			},
		},
	}
	return c.JSON(http.StatusOK, resp)
}
