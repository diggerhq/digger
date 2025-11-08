package tfe

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/diggerhq/digger/opentaco/internal/auth"
	"github.com/diggerhq/digger/opentaco/internal/domain/tfe"

	"io"
	"net/http"
	"strings"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/logging"
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

// extractUnitUUID extracts the unit UUID from a state ID
// State ID can be either:
// - Just a unit UUID: "82ca6591-e01d-49ff-b0d5-4b6d73914260"
// - Org/unit path: "822d677a-aaa7-47cc-8b84-3c0df683c99e/82ca6591-e01d-49ff-b0d5-4b6d73914260"
// The repository layer expects just the unit UUID and constructs the blob path internally.
func extractUnitUUID(stateID string) string {
	if !strings.Contains(stateID, "/") {
		return stateID
	}
	parts := strings.Split(stateID, "/")
	if len(parts) == 2 {
		return parts[1] // Return unit UUID (second part)
	}
	return stateID
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
		case "unit.read":
			rbacAction = rbac.ActionUnitRead
		case "unit.write":
			rbacAction = rbac.ActionUnitWrite
		case "unit.lock":
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

	// Check if this is a webhook-authenticated request (internal endpoints)
	// Webhook auth uses internal token + X-User-ID/X-Email headers
	userIDHeader := c.Request().Header.Get("X-User-ID")
	userEmailHeader := c.Request().Header.Get("X-Email")
	
	
	var principal rbac.Principal
	if userIDHeader != "" && userEmailHeader != "" {
		// This is webhook auth from internal proxy (UI) - user already verified
		principal = rbac.Principal{
			Subject: userIDHeader,
			Email:   userEmailHeader,
			Roles:   []string{}, // Will be looked up from database by RBAC manager
			Groups:  []string{},
		}
	} else {
		// TFE endpoints: verify opaque token only (for clear API boundaries)
		if h.apiTokens != nil {
			// Extract org from context
			orgID, err := getOrgFromContext(c)
			if err != nil {
				return fmt.Errorf("failed to get organization context: %v", err)
			}
			
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
	}
	var rbacAction rbac.Action

	switch action {
	case "unit.read":
		rbacAction = rbac.ActionUnitRead
	case "unit.write":
		rbacAction = rbac.ActionUnitWrite
	case "unit.lock":
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
	logger := logging.FromContext(c)
	c.Response().Header().Set(echo.HeaderContentType, "application/vnd.api+json")
	c.Response().Header().Set("Tfp-Api-Version", "2.5")
	c.Response().Header().Set("X-Terraform-Enterprise-App", "Terraform Enterprise")

	orgParam := c.Param("org_name")
	workspaceName := c.Param("workspace_name")
	
	if workspaceName == "" {
		logger.Warn("Invalid workspace name",
			"operation", "tfe_get_workspace",
			"org_param", orgParam,
		)
		return c.JSON(400, map[string]string{"error": "workspace_name invalid"})
	}
	
	// Parse org param - supports both "Display:identifier" and just "identifier"
	displayName, orgIdentifier := parseOrgParam(orgParam)
	
	logger.Info("Getting TFE workspace",
		"operation", "tfe_get_workspace",
		"org_param", orgParam,
		"display_name", displayName,
		"org_identifier", orgIdentifier,
		"workspace_name", workspaceName,
	)
	
	// Convert workspace name to unit ID (org-scoped if org provided)
	// workspaceName is now the human-readable unit name, not a UUID
	stateID, err := h.convertWorkspaceToStateIDWithOrg(c.Request().Context(), orgIdentifier, workspaceName)
	if err != nil {
		logger.Error("Failed to resolve workspace",
			"operation", "tfe_get_workspace",
			"org_identifier", orgIdentifier,
			"workspace_name", workspaceName,
			"error", err,
		)
		return c.JSON(500, map[string]string{
			"error": "failed to resolve workspace",
			"detail": err.Error(),
		})
	}
	
	logger.Debug("Resolved workspace state ID",
		"operation", "tfe_get_workspace",
		"state_id", stateID,
	)
	
	// Extract unit UUID from state ID - repository expects just the UUID
	unitUUID := extractUnitUUID(stateID)
	fmt.Printf("GetWorkspace: Extracted unitUUID=%s from stateID=%s\n", unitUUID, stateID)
	
	// Check if unit exists (optional - may auto-create later)
	_, err = h.stateStore.Get(c.Request().Context(), unitUUID)
	locked := false
	var currentRun *tfe.TFERun
	if err == nil {
		// Check if locked and get lock details
		lockInfo, _ := h.stateStore.GetLock(c.Request().Context(), unitUUID)
		if lockInfo != nil {
			locked = true
			// Populate CurrentRun with lock details for Terraform force-unlock
			currentRun = &tfe.TFERun{
				ID: lockInfo.ID,
			}
		}
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
		CurrentRun:                 currentRun,  // Include lock details when workspace is locked
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
	logger := logging.FromContext(c)
	c.Response().Header().Set(echo.HeaderContentType, "application/json")
	c.Response().Header().Set("Tfp-Api-Version", "2.5")
	c.Response().Header().Set("X-Terraform-Enterprise-App", "Terraform Enterprise")

	// Extract workspace ID (format: ws-{workspace-name})
	workspaceID := extractWorkspaceIDFromParam(c)
	if workspaceID == "" {
		logger.Warn("Missing workspace ID",
			"operation", "tfe_lock_workspace",
		)
		return c.JSON(400, map[string]string{"error": "workspace_id required"})
	}

	// Strip ws- prefix to get workspace name
	workspaceName := convertWorkspaceToStateID(workspaceID)
	
	// Get org from authentication context (JWT claim or webhook header)
	orgIdentifier, err := getOrgFromContext(c)
	if err != nil {
		logger.Error("Failed to get org from context",
			"operation", "tfe_lock_workspace",
			"workspace_id", workspaceID,
			"error", err,
		)
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "Organization context required",
			"detail": err.Error(),
		})
	}
	
	logger.Info("Locking TFE workspace",
		"operation", "tfe_lock_workspace",
		"workspace_id", workspaceID,
		"workspace_name", workspaceName,
		"org_identifier", orgIdentifier,
	)

	// Resolve to UUID/UUID path
	stateID, err := h.convertWorkspaceToStateIDWithOrg(c.Request().Context(), orgIdentifier, workspaceName)
	if err != nil {
		logger.Error("Failed to resolve workspace",
			"operation", "tfe_lock_workspace",
			"org_identifier", orgIdentifier,
			"workspace_name", workspaceName,
			"error", err,
		)
		return c.JSON(500, map[string]string{
			"error": "failed to resolve workspace",
			"detail": err.Error(),
		})
	}

	// Check RBAC permission for locking workspace
	if err := h.checkWorkspacePermission(c, "unit.write", stateID); err != nil {
		logger.Warn("Insufficient permissions to lock workspace",
			"operation", "tfe_lock_workspace",
			"state_id", stateID,
			"error", err,
		)
		return c.JSON(http.StatusForbidden, map[string]string{
			"error": "insufficient permissions to lock workspace",
			"hint":  "contact your administrator to grant unit.write permission",
		})
	}

	if h.stateStore == nil {
		logger.Error("State store not initialized",
			"operation", "tfe_lock_workspace",
		)
		return c.JSON(500, map[string]string{"error": "State store not initialized"})
	}

	// Extract unit UUID from state ID - repository expects just the UUID
	unitUUID := extractUnitUUID(stateID)

	// Check if state exists, enot
	_, err = h.stateStore.Get(c.Request().Context(), unitUUID)
	if err == storage.ErrNotFound {
		logger.Warn("Unit not found for locking",
			"operation", "tfe_lock_workspace",
			"unit_uuid", unitUUID,
		)
		return c.JSON(404, map[string]string{
			"error": "Unit not found. Please create the unit first using 'taco unit create " + unitUUID + "' or the opentaco_unit Terraform resource.",
		})
	} else if err != nil {
		// Handle other errors from Get()
		logger.Error("Failed to check state existence",
			"operation", "tfe_lock_workspace",
			"unit_uuid", unitUUID,
			"error", err,
		)
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

	// Attempt to lock the state
	err = h.stateStore.Lock(c.Request().Context(), unitUUID, lockInfo)
	if err != nil {
		// Check for lock conflict using strings.Contains since error message may have additional text
		if strings.Contains(err.Error(), "lock conflict") {
			logger.Warn("Lock conflict detected",
				"operation", "tfe_lock_workspace",
				"unit_uuid", unitUUID,
			)
			// Get current lock for details
			currentLock, _ := h.stateStore.GetLock(c.Request().Context(), unitUUID)
			if currentLock != nil {
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
			return c.JSON(409, map[string]string{
				"error": "Workspace is already locked",
			})
		}
		logger.Error("Failed to lock workspace",
			"operation", "tfe_lock_workspace",
			"unit_uuid", unitUUID,
			"error", err,
		)
		return c.JSON(500, map[string]string{
			"error": "Failed to acquire workspace lock",
		})
	}

	// Return success with full workspace object (properly formatted JSON:API)
	fmt.Printf("LockWorkspace: Returning success\n")
	
	// Build a workspace response with lock info
	logger.Info("Workspace locked successfully",
		"operation", "tfe_lock_workspace",
		"unit_uuid", unitUUID,
		"lock_id", lockInfo.ID,
	)
	
	workspace := &tfe.TFEWorkspace{
		ID:     tfe.NewTfeResourceIdentifier(tfe.WorkspaceType, workspaceName).String(),
		Name:   workspaceName,
		Locked: true,
		CurrentRun: &tfe.TFERun{
			ID: lockInfo.ID,
		},
	}
	
	if err := jsonapi.MarshalPayload(c.Response().Writer, workspace); err != nil {
		logger.Error("Failed to marshal workspace payload",
			"operation", "tfe_lock_workspace",
			"error", err,
		)
		return err
	}
	return nil
}

func (h *TfeHandler) UnlockWorkspace(c echo.Context) error {
	logger := logging.FromContext(c)
	c.Response().Header().Set(echo.HeaderContentType, "application/json")
	c.Response().Header().Set("Tfp-Api-Version", "2.5")
	c.Response().Header().Set("X-Terraform-Enterprise-App", "Terraform Enterprise")

	// Extract workspace ID (format: ws-{workspace-name})
	workspaceID := extractWorkspaceIDFromParam(c)
	if workspaceID == "" {
		logger.Warn("Missing workspace ID",
			"operation", "tfe_unlock_workspace",
		)
		return c.JSON(400, map[string]string{"error": "workspace_id required"})
	}

	// Strip ws- prefix to get workspace name
	workspaceName := convertWorkspaceToStateID(workspaceID)
	
	// Get org from authentication context (JWT claim or webhook header)
	orgIdentifier, err := getOrgFromContext(c)
	if err != nil {
		logger.Error("Failed to get org from context",
			"operation", "tfe_unlock_workspace",
			"workspace_id", workspaceID,
			"error", err,
		)
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "Organization context required",
			"detail": err.Error(),
		})
	}
	
	logger.Info("Unlocking TFE workspace",
		"operation", "tfe_unlock_workspace",
		"workspace_id", workspaceID,
		"workspace_name", workspaceName,
		"org_identifier", orgIdentifier,
	)
	
	// Resolve to UUID/UUID path
	stateID, err := h.convertWorkspaceToStateIDWithOrg(c.Request().Context(), orgIdentifier, workspaceName)
	if err != nil {
		logger.Error("Failed to resolve workspace",
			"operation", "tfe_unlock_workspace",
			"org_identifier", orgIdentifier,
			"workspace_name", workspaceName,
			"error", err,
		)
		return c.JSON(500, map[string]string{
			"error": "failed to resolve workspace",
			"detail": err.Error(),
		})
	}

	// Extract unit UUID from state ID - repository expects just the UUID
	unitUUID := extractUnitUUID(stateID)

	// Get current lock to find lock ID
	currentLock, err := h.stateStore.GetLock(c.Request().Context(), unitUUID)
	if err != nil {
		if err == storage.ErrNotFound {
			logger.Warn("Workspace not found for unlock",
				"operation", "tfe_unlock_workspace",
				"unit_uuid", unitUUID,
			)
			return c.JSON(404, map[string]string{"error": "Workspace not found"})
		}
		logger.Error("Failed to get lock status",
			"operation", "tfe_unlock_workspace",
			"unit_uuid", unitUUID,
			"error", err,
		)
		return c.JSON(500, map[string]string{"error": "Failed to get lock status"})
	}

	if currentLock == nil {
		// Already unlocked, return success
		logger.Info("Workspace already unlocked",
			"operation", "tfe_unlock_workspace",
			"unit_uuid", unitUUID,
		)
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
	err = h.stateStore.Unlock(c.Request().Context(), unitUUID, currentLock.ID)
	if err != nil {
		if err == storage.ErrNotFound {
			logger.Warn("Workspace not found during unlock",
				"operation", "tfe_unlock_workspace",
				"unit_uuid", unitUUID,
			)
			return c.JSON(404, map[string]string{"error": "Workspace not found"})
		}
		if err == storage.ErrLockConflict {
			logger.Warn("Lock ID mismatch",
				"operation", "tfe_unlock_workspace",
				"unit_uuid", unitUUID,
				"lock_id", currentLock.ID,
			)
			return c.JSON(409, map[string]string{"error": "Lock ID mismatch"})
		}
		logger.Error("Failed to release lock",
			"operation", "tfe_unlock_workspace",
			"unit_uuid", unitUUID,
			"lock_id", currentLock.ID,
			"error", err,
		)
		return c.JSON(500, map[string]string{"error": "Failed to release lock"})
	}

	logger.Info("Workspace unlocked successfully",
		"operation", "tfe_unlock_workspace",
		"unit_uuid", unitUUID,
		"lock_id", currentLock.ID,
	)
	
	// Return success with full workspace object (properly formatted JSON:API)
	workspace := &tfe.TFEWorkspace{
		ID:         tfe.NewTfeResourceIdentifier(tfe.WorkspaceType, workspaceName).String(),
		Name:       workspaceName,
		Locked:     false,
		CurrentRun: nil,  // No lock, so no current run
	}
	
	if err := jsonapi.MarshalPayload(c.Response().Writer, workspace); err != nil {
		logger.Error("Failed to marshal workspace payload",
			"operation", "tfe_unlock_workspace",
			"error", err,
		)
		return err
	}
	return nil
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

	// Extract unit UUID from state ID - repository expects just the UUID
	unitUUID := extractUnitUUID(stateID)
	fmt.Printf("ForceUnlockWorkspace: Extracted unitUUID=%s from stateID=%s\n", unitUUID, stateID)

	// Try to get the lock ID from query parameter or request body
	requestedLockID := c.QueryParam("lock_id")
	if requestedLockID == "" {
		// Try to read from body
		var body map[string]interface{}
		if err := c.Bind(&body); err == nil {
			if id, ok := body["lock_id"].(string); ok {
				requestedLockID = id
			}
		}
	}
	fmt.Printf("ForceUnlockWorkspace: Requested lock ID: %s\n", requestedLockID)

	// Get current lock to find lock ID
	currentLock, err := h.stateStore.GetLock(c.Request().Context(), unitUUID)
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

	// Validate lock ID if provided
	if requestedLockID != "" && requestedLockID != currentLock.ID {
		fmt.Printf("ForceUnlockWorkspace: Lock ID mismatch - requested=%s, current=%s\n", requestedLockID, currentLock.ID)
		return c.JSON(409, map[string]interface{}{
			"error": "lock_id_mismatch",
			"message": fmt.Sprintf("Lock ID %q does not match existing lock ID %q", requestedLockID, currentLock.ID),
			"current_lock": map[string]interface{}{
				"id":      currentLock.ID,
				"who":     currentLock.Who,
				"version": currentLock.Version,
				"created": currentLock.Created,
			},
		})
	}

	fmt.Printf("ForceUnlockWorkspace: Force unlocking with lock ID: %s\n", currentLock.ID)

	// Force unlock the state using the current lock ID
	err = h.stateStore.Unlock(c.Request().Context(), unitUUID, currentLock.ID)
	if err != nil {
		fmt.Printf("ForceUnlockWorkspace: Failed to unlock: %v\n", err)
		return c.JSON(500, map[string]string{"error": "Failed to force unlock"})
	}

	fmt.Printf("ForceUnlockWorkspace: Successfully force unlocked\n")

	// Return success with full workspace object (properly formatted JSON:API)
	workspace := &tfe.TFEWorkspace{
		ID:         tfe.NewTfeResourceIdentifier(tfe.WorkspaceType, workspaceName).String(),
		Name:       workspaceName,
		Locked:     false,
		CurrentRun: nil,  // No lock, so no current run
	}
	
	if err := jsonapi.MarshalPayload(c.Response().Writer, workspace); err != nil {
		fmt.Printf("ForceUnlockWorkspace: error marshaling workspace payload: %v\n", err)
		return err
	}
	return nil
}

// GetCurrentStateVersion handles GET /tfe/api/v2/workspaces/:workspace_id/current-state-version
func (h *TfeHandler) GetCurrentStateVersion(c echo.Context) error {
	logger := logging.FromContext(c)
	c.Response().Header().Set(echo.HeaderContentType, "application/vnd.api+json")
	c.Response().Header().Set("Tfp-Api-Version", "2.5")
	c.Response().Header().Set("X-Terraform-Enterprise-App", "Terraform Enterprise")

	// Extract workspace ID (format: ws-{workspace-name})
	workspaceID := extractWorkspaceIDFromParam(c)
	if workspaceID == "" {
		logger.Warn("Missing workspace ID",
			"operation", "tfe_get_current_state",
		)
		return c.JSON(400, map[string]string{"error": "workspace_id required"})
	}

	// Strip ws- prefix to get workspace name
	workspaceName := convertWorkspaceToStateID(workspaceID)
	
	// Get org from authentication context (JWT claim or webhook header)
	orgIdentifier, err := getOrgFromContext(c)
	if err != nil {
		logger.Error("Failed to get org from context",
			"operation", "tfe_get_current_state",
			"workspace_id", workspaceID,
			"error", err,
		)
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "Organization context required",
			"detail": err.Error(),
		})
	}
	
	logger.Info("Getting current state version",
		"operation", "tfe_get_current_state",
		"workspace_id", workspaceID,
		"workspace_name", workspaceName,
		"org_identifier", orgIdentifier,
	)
	
	// Resolve to UUID/UUID path
	stateID, err := h.convertWorkspaceToStateIDWithOrg(c.Request().Context(), orgIdentifier, workspaceName)
	if err != nil {
		return c.JSON(500, map[string]string{
			"error": "failed to resolve workspace",
			"detail": err.Error(),
		})
	}

	// Check RBAC permission with correct three-scenario logic
	if err := h.checkWorkspacePermission(c, "unit.read", stateID); err != nil {
		return c.JSON(http.StatusForbidden, map[string]string{
			"error": "insufficient permissions to access workspace",
			"hint":  "contact your administrator to grant unit.read permission",
		})
	}

	// Check if state exists
	
	// Extract unit UUID from state ID - repository expects just the UUID
	unitUUID := extractUnitUUID(stateID)
	
	stateMeta, err := h.stateStore.Get(c.Request().Context(), unitUUID)
	
	if err != nil {
	}
	if stateMeta != nil {
	} else {
	}
	
	if err != nil {
		if err == storage.ErrNotFound {
			return c.JSON(404, map[string]string{
				"error": "Unit not found. Please create the unit first using 'taco unit create " + stateID + "' or the opentaco_unit Terraform resource.",
			})
		}
		return c.JSON(500, map[string]string{"error": "Failed to get workspace state"})
	}

	// Generate a state version ID based on state ID and timestamp
	stateVersionID := generateStateVersionID(stateID, stateMeta.Updated.Unix())

	baseURL := getBaseURL(c)
	// Sign the download URL for Terraform 1.5.x compatibility (doesn't send auth headers)
	downloadURL, err := auth.SignURL(baseURL, fmt.Sprintf("/tfe/api/v2/state-versions/%s/download", stateVersionID), time.Now().Add(10*time.Minute))
	if err != nil {
		return c.JSON(500, map[string]string{"error": "Failed to sign download URL"})
	}

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
	if err := h.checkWorkspacePermission(c, "unit.write", stateID); err != nil {
		return c.JSON(http.StatusForbidden, map[string]string{
			"error": "insufficient permissions to create state version",
			"hint":  "contact your administrator to grant unit.write permission",
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

	// Extract the actual state data from the request (if available)
	data, ok := request["data"].(map[string]interface{})
	if !ok {
		fmt.Printf("CreateStateVersion: ERROR - Invalid request format, missing data\n")
		return c.JSON(400, map[string]string{"error": "Invalid request format"})
	}
	attributes, _ := data["attributes"].(map[string]any)
	if !ok {
		fmt.Printf("CreateStateVersion: ERROR - Invalid request format, missing attributes\n")
		return c.JSON(400, map[string]string{"error": "Invalid request format"})
	}

	// INLINE STATE (Terraform <=1.5.x path) ------ upload directly in this case
	if enc, ok := attributes["state"].(string); ok && enc != "" {
		// 1) Decode inline JSON state
		stateBytes, decErr := base64.StdEncoding.DecodeString(enc)
		if decErr != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid base64 in json-state"})
		}
		fmt.Printf("CreateStateVersion: found state b64 bytes in JSON, treating as direct upload\n")
		// For direct upload without JSON wrapper, handle as raw state data
		return h.CreateStateVersionDirect(c, workspaceID, stateID, stateBytes)
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

	// Extract unit UUID from state ID - repository expects just the UUID
	unitUUID := extractUnitUUID(stateID)
	fmt.Printf("CreateStateVersion: Extracted unitUUID=%s from stateID=%s\n", unitUUID, stateID)

	// Check that state exists (no auto-creation)
	_, err = h.stateStore.Get(c.Request().Context(), unitUUID)
	if err == storage.ErrNotFound {
		fmt.Printf("CreateStateVersion: Unit not found\n")
		return c.JSON(404, map[string]string{
			"error": "Unit not found. Please create the unit first using 'taco unit create " + unitUUID + "' or the opentaco_unit Terraform resource.",
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

	signedUploadUrl, err := auth.SignURL(baseURL, fmt.Sprintf("/tfe/api/v2/state-versions/%s/upload", stateVersionID), time.Now().Add(2*time.Minute))
	if err != nil {
		fmt.Printf("CreateStateVersion: ERROR - Failed to sign URL: %v\n", err)
		return c.JSON(500, map[string]string{"error": "Failed to sign URL"})
	}

	signedJsonUploadUrl, err := auth.SignURL(baseURL, fmt.Sprintf("/tfe/api/v2/state-versions/%s/json-upload", stateVersionID), time.Now().Add(2*time.Minute))
	if err != nil {
		fmt.Printf("CreateStateVersion: ERROR - Failed to sign URL: %v\n", err)
		return c.JSON(500, map[string]string{"error": "Failed to sign URL"})
	}

	downloadURL := fmt.Sprintf("%s/tfe/api/v2/state-versions/%s/download", baseURL, stateVersionID)

	// Derive serial and lineage from existing state (if any)
	serial := 0
	lineage := ""
	if stateBytes, dErr := h.stateStore.Download(c.Request().Context(), unitUUID); dErr == nil {
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

	// Build the response
	response := map[string]interface{}{
		"data": map[string]interface{}{
			"id":   stateVersionID,
			"type": "state-versions",
			"attributes": map[string]interface{}{
				"created-at":                   time.Now().UTC().Format(time.RFC3339),
				"updated-at":                   time.Now().UTC().Format(time.RFC3339),
				"size":                         0,
				"upload-url":                   signedUploadUrl,
				"hosted-state-upload-url":      signedUploadUrl,
				"hosted-state-download-url":    downloadURL,
				"hosted-json-state-upload-url": signedJsonUploadUrl,
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

	// Extract unit UUID from state ID - repository expects just the UUID
	unitUUID := extractUnitUUID(stateID)
	fmt.Printf("CreateStateVersionDirect: Extracted unitUUID=%s from stateID=%s\n", unitUUID, stateID)

	// Check if state exists, create if not
	_, err := h.stateStore.Get(c.Request().Context(), unitUUID)
	if err == storage.ErrNotFound {
		fmt.Printf("CreateStateVersionDirect: Unit not found\n")
		return c.JSON(404, map[string]string{
			"error": "Unit not found. Please create the unit first using 'taco unit create " + unitUUID + "' or the opentaco_unit Terraform resource.",
		})
	}

	// Get the current lock to extract lock ID for state upload
	currentLock, lockErr := h.stateStore.GetLock(c.Request().Context(), unitUUID)
	if lockErr != nil && lockErr != storage.ErrNotFound {
		return c.JSON(500, map[string]string{"error": "Failed to get lock status"})
	}

	// Extract lock ID if state is locked
	lockID := ""
	if currentLock != nil {
		lockID = currentLock.ID
	}

	// Upload the state with proper lock ID
	err = h.stateStore.Upload(c.Request().Context(), unitUUID, body, lockID)
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
	stateMeta, err := h.stateStore.Get(c.Request().Context(), unitUUID)
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

	// Extract unit UUID from state ID - repository expects just the UUID
	unitUUID := extractUnitUUID(stateID)
	
	// Download the state data
	stateData, err := h.directStateStore.Download(c.Request().Context(), unitUUID)
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
	stateVersionID := c.Param("id")
	if stateVersionID == "" {
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
	if err := h.checkWorkspacePermission(c, "unit.write", workspaceID); err != nil {
		// Only enforce RBAC if we have a real auth error, not just missing headers
		if !strings.Contains(err.Error(), "no authorization header") {
			return c.JSON(http.StatusForbidden, map[string]string{
				"error": "insufficient permissions to upload state",
				"hint":  "contact your administrator to grant unit.write permission",
			})
		}
	}

	// Read the state data from request body
	stateData, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(400, map[string]string{"error": "Failed to read state data"})
	}

	// Extract unit UUID from state ID - repository expects just the UUID
	unitUUID := extractUnitUUID(stateID)

	// Use directStateStore for signed URL operations (pre-authorized, no RBAC checks)
	// Check if state exists (no auto-creation)
	_, err = h.directStateStore.Get(c.Request().Context(), unitUUID)
	if err == storage.ErrNotFound {
		return c.JSON(404, map[string]string{
			"error": "Unit not found. Please create the unit first using 'taco unit create " + unitUUID + "' or the opentaco_unit Terraform resource.",
		})
	} else if err != nil {
		return c.JSON(500, map[string]string{
			"error": "Failed to check state existence",
		})
	}

	// Get the current lock to extract lock ID for state upload
	currentLock, lockErr := h.directStateStore.GetLock(c.Request().Context(), unitUUID)
	if lockErr != nil && lockErr != storage.ErrNotFound {
		return c.JSON(500, map[string]string{"error": "Failed to get lock status"})
	}

	// Extract lock ID if state is locked
	lockID := ""
	if currentLock != nil {
		lockID = currentLock.ID
	}

	// Upload the state with proper lock ID
	err = h.directStateStore.Upload(c.Request().Context(), unitUUID, stateData, lockID)
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
	if err := h.checkWorkspacePermission(c, "unit.write", workspaceID); err != nil {
		// Only enforce RBAC if we have a real auth error, not just missing headers
		if !strings.Contains(err.Error(), "no authorization header") {
			fmt.Printf("UploadJSONStateOutputs: RBAC permission denied: %v\n", err)
			return c.JSON(http.StatusForbidden, map[string]string{
				"error": "insufficient permissions to upload state outputs",
				"hint":  "contact your administrator to grant unit.write permission",
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

	// Extract unit UUID from state ID - repository expects just the UUID
	unitUUID := extractUnitUUID(stateID)
	
	// Load metadata (and optionally content)
	meta, err := h.stateStore.Get(c.Request().Context(), unitUUID)
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
	if bytes, dErr := h.stateStore.Download(c.Request().Context(), unitUUID); dErr == nil && len(bytes) > 0 {
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
	// Sign the download URL for Terraform 1.5.x compatibility
	downloadURL, err := auth.SignURL(baseURL, fmt.Sprintf("/tfe/api/v2/state-versions/%s/download", id), time.Now().Add(10*time.Minute))
	if err != nil {
		return c.JSON(500, map[string]string{"error": "Failed to sign download URL"})
	}

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
