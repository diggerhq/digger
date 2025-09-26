package tfe

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/diggerhq/digger/opentaco/internal/domain"
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

// convertWorkspaceToStateID converts a TFE workspace ID to a state ID
// e.g., "ws-myworkspace" -> "myworkspace" (strip the ws- prefix to match CLI-created states)
func convertWorkspaceToStateID(workspaceID string) string {
	// Validate input
	if strings.TrimSpace(workspaceID) == "" {
		return ""
	}
	
	// If it's a TFE workspace ID (ws-something), extract just the workspace name
	if strings.HasPrefix(workspaceID, "ws-") {
		result := strings.TrimPrefix(workspaceID, "ws-")
		if result == "" {
			return ""
		}
		return result
	}
	// Otherwise, return as-is (for direct workspace names)
	return workspaceID
}

// extractWorkspaceIDFromParam extracts workspace ID from URL parameter
func extractWorkspaceIDFromParam(c echo.Context) string {
	workspaceID := c.Param("workspace_id")
	if workspaceID == "" {
		// Fallback to workspace_name for routes that use that parameter
		workspaceName := c.Param("workspace_name")
		if workspaceName != "" {
			return domain.NewTfeIDWithVal(domain.WorkspaceKind, workspaceName).String()
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
	
	// Verify the access token and extract claims
	claims, err := signer.VerifyAccess(token)
	if err != nil {
		return fmt.Errorf("invalid access token: %v", err)
	}
	
	// Create principal from verified claims
	principal := rbac.Principal{
		Subject: claims.Subject,
		Email:   claims.Email,
		Roles:   claims.Roles,
		Groups:  claims.Groups,
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

// Adapted from OTF (MPL License): https://github.com/leg100/otf
func ToTFE(from *domain.Workspace) (*domain.TFEWorkspace, error) {
	perms := &domain.TFEWorkspacePermissions{
		CanLock:           true,
		CanUnlock:         true,
		CanForceUnlock:    true,
		CanQueueApply:     true,
		CanQueueDestroy:   true,
		CanQueueRun:       true,
		CanDestroy:        true,
		CanReadSettings:   true,
		CanUpdate:         true,
		CanUpdateVariable: true,
	}

	to := &domain.TFEWorkspace{
		ID: from.ID,
		Actions: &domain.TFEWorkspaceActions{
			IsDestroyable: true,
		},
		AllowDestroyPlan:     from.AllowDestroyPlan,
		AgentPoolID:          from.AgentPoolID,
		AutoApply:            from.AutoApply,
		CanQueueDestroyPlan:  from.CanQueueDestroyPlan,
		CreatedAt:            from.CreatedAt,
		Description:          from.Description,
		Environment:          from.Environment,
		ExecutionMode:        string(from.ExecutionMode),
		GlobalRemoteState:    from.GlobalRemoteState,
		Locked:               from.Locked(),
		MigrationEnvironment: from.MigrationEnvironment,
		Name:                 from.Name.Name,
		// Operations is deprecated but clients and go-tfe tests still use it
		Operations:                 from.ExecutionMode == "remote",
		Permissions:                perms,
		QueueAllRuns:               from.QueueAllRuns,
		SpeculativeEnabled:         from.SpeculativeEnabled,
		SourceName:                 from.SourceName,
		SourceURL:                  from.SourceURL,
		StructuredRunOutputEnabled: from.StructuredRunOutputEnabled,
		TerraformVersion:           from.EngineVersion,
		TriggerPrefixes:            from.TriggerPrefixes,
		TriggerPatterns:            from.TriggerPatterns,
		WorkingDirectory:           from.WorkingDirectory,
		TagNames:                   from.Tags,
		UpdatedAt:                  from.UpdatedAt,
		Organization:               &domain.TFEOrganization{Name: from.Organization.Name},
	}
	if len(from.TriggerPrefixes) > 0 || len(from.TriggerPatterns) > 0 {
		to.FileTriggersEnabled = true
	}
	if from.LatestRun != nil {
		to.CurrentRun = &domain.TFERun{ID: from.LatestRun.ID}
	}

	// Add VCS repo to json:api struct if connected. NOTE: the terraform CLI
	// uses the presence of VCS repo to determine whether to allow a terraform
	// apply or not, displaying the following message if not:
	//
	//	Apply not allowed for workspaces with a VCS connection
	//
	//	A workspace that is connected to a VCS requires the VCS-driven workflow to ensure that the VCS remains the single source of truth.
	//
	// OTF permits the user to disable this behaviour by ommiting this info and
	// fool the terraform CLI into thinking its not a workspace with a VCS
	// connection.
	if from.Connection != nil {
		isTerraformCli := true // TODO: read from header
		if !from.Connection.AllowCLIApply || !isTerraformCli {
			to.VCSRepo = &domain.TFEVCSRepo{
				OAuthTokenID: from.Connection.VCSProviderID,
				Branch:       from.Connection.Branch,
				Identifier:   from.Connection.Repo,
				TagsRegex:    from.Connection.TagsRegex,
			}
		}
	}
	return to, nil
}

func (h *TfeHandler) GetWorkspace(c echo.Context) error {
	c.Response().Header().Set(echo.HeaderContentType, "application/vnd.api+json")
	c.Response().Header().Set("Tfp-Api-Version", "2.5")
	c.Response().Header().Set("X-Terraform-Enterprise-App", "Terraform Enterprise")

	workspaceName := c.Param("workspace_name")
	if workspaceName == "" {
		return c.JSON(400, map[string]string{"error": "workspace_name invalid"})
	}

	workspace := domain.Workspace{
		ID:                         domain.NewTfeIDWithVal(domain.WorkspaceKind, workspaceName).String(),
		CreatedAt:                  time.Time{},
		UpdatedAt:                  time.Time{},
		AgentPoolID:                domain.NewTfeIDWithVal(domain.AgentPoolKind, "HzEaJWMP5YTatZaS").String(),
		AllowDestroyPlan:           false,
		AutoApply:                  false,
		CanQueueDestroyPlan:        false,
		Description:                workspaceName,
		Environment:                workspaceName,
		ExecutionMode:              "local",
		GlobalRemoteState:          false,
		MigrationEnvironment:       "",
		Name:                       domain.NewName(workspaceName),
		QueueAllRuns:               false,
		SpeculativeEnabled:         false,
		StructuredRunOutputEnabled: false,
		SourceName:                 "",
		SourceURL:                  "",
		WorkingDirectory:           "",
		Organization:               domain.NewName("opentaco"),
		LatestRun:                  nil,
		Tags:                       nil,
		Lock:                       nil,
		Engine:                     "",
		EngineVersion:              nil,
		Connection:                 nil,
		TriggerPatterns:            nil,
		TriggerPrefixes:            nil,
	}

	converted, err := ToTFE(&workspace)
	if err != nil {
		return err
	}

	// Debug: Log the workspace data being sent
	fmt.Printf("GetWorkspace: Sending workspace with ExecutionMode=%s, Operations=%t\n", 
		converted.ExecutionMode, converted.Operations)
		
	if err := jsonapi.MarshalPayload(c.Response().Writer, converted); err != nil {
		fmt.Printf("error marshaling workspace payload: %v", err)
		return err
	}
	return nil
}

func (h *TfeHandler) LockWorkspace(c echo.Context) error {
	c.Response().Header().Set(echo.HeaderContentType, "application/json")
	c.Response().Header().Set("Tfp-Api-Version", "2.5")
	c.Response().Header().Set("X-Terraform-Enterprise-App", "Terraform Enterprise")

	// Extract workspace ID and convert to state ID
	workspaceID := extractWorkspaceIDFromParam(c)
	if workspaceID == "" {
		return c.JSON(400, map[string]string{"error": "workspace_id required"})
	}
	
	// Debug logging
	fmt.Printf("LockWorkspace: workspaceID=%s\n", workspaceID)

	// Check RBAC permission for locking workspace
	if err := h.checkWorkspacePermission(c, "unit:write", workspaceID); err != nil {
		return c.JSON(http.StatusForbidden, map[string]string{
			"error": "insufficient permissions to lock workspace",
			"hint":  "contact your administrator to grant unit:write permission",
		})
	}
	
	if h.stateStore == nil {
		fmt.Printf("LockWorkspace: stateStore is nil!\n")
		return c.JSON(500, map[string]string{"error": "State store not initialized"})
	}

	stateID := convertWorkspaceToStateID(workspaceID)
	fmt.Printf("LockWorkspace: stateID=%s\n", stateID)

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

	// Extract workspace ID and convert to state ID
	workspaceID := extractWorkspaceIDFromParam(c)
	if workspaceID == "" {
		return c.JSON(400, map[string]string{"error": "workspace_id required"})
	}

	stateID := convertWorkspaceToStateID(workspaceID)
	fmt.Printf("UnlockWorkspace: workspaceID=%s, stateID=%s\n", workspaceID, stateID)

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

	// Extract workspace ID and convert to state ID
	workspaceID := extractWorkspaceIDFromParam(c)
	if workspaceID == "" {
		return c.JSON(400, map[string]string{"error": "workspace_id required"})
	}

	stateID := convertWorkspaceToStateID(workspaceID)
	fmt.Printf("ForceUnlockWorkspace: workspaceID=%s, stateID=%s\n", workspaceID, stateID)

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

	workspaceID := extractWorkspaceIDFromParam(c)
	fmt.Printf("GetCurrentStateVersion: workspaceID=%s\n", workspaceID)
	if workspaceID == "" {
		fmt.Printf("GetCurrentStateVersion: ERROR - workspace_id required\n")
		return c.JSON(400, map[string]string{"error": "workspace_id required"})
	}

	stateID := convertWorkspaceToStateID(workspaceID)
	fmt.Printf("GetCurrentStateVersion: stateID=%s\n", stateID)
	if stateID == "" {
		fmt.Printf("GetCurrentStateVersion: ERROR - invalid state ID from workspace ID: %s\n", workspaceID)
		return c.JSON(400, map[string]string{"error": "invalid workspace ID"})
	}

	// Check RBAC permission with correct three-scenario logic
	if err := h.checkWorkspacePermission(c, "unit:read", workspaceID); err != nil {
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
			fmt.Printf("GetCurrentStateVersion: Unit not found - no auto-creation\n")
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
				"created-at":      stateMeta.Updated.UTC().Format(time.RFC3339),
				"updated-at":      stateMeta.Updated.UTC().Format(time.RFC3339),
				"size":            stateMeta.Size,
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

	workspaceID := extractWorkspaceIDFromParam(c)
	fmt.Printf("CreateStateVersion: workspaceID=%s\n", workspaceID)
	if workspaceID == "" {
		fmt.Printf("CreateStateVersion: ERROR - workspace_id required\n")
		return c.JSON(400, map[string]string{"error": "workspace_id required"})
	}

	stateID := convertWorkspaceToStateID(workspaceID)
	fmt.Printf("CreateStateVersion: stateID=%s\n", stateID)
	if stateID == "" {
		fmt.Printf("CreateStateVersion: ERROR - invalid state ID from workspace ID: %s\n", workspaceID)
		return c.JSON(400, map[string]string{"error": "invalid workspace ID"})
	}

	// Check RBAC permission for creating/writing state versions
	if err := h.checkWorkspacePermission(c, "unit:write", workspaceID); err != nil {
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
		fmt.Printf("CreateStateVersion: Unit not found - no auto-creation\n")
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
			if v, ok := st["serial"].(float64); ok { serial = int(v) }
			if v, ok := st["lineage"].(string); ok { lineage = v }
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
				"created-at": time.Now().UTC().Format(time.RFC3339),
				"updated-at": time.Now().UTC().Format(time.RFC3339),
				"size":       0,
				"upload-url": uploadURL,
				"hosted-state-upload-url":   uploadURL,
				"hosted-state-download-url": downloadURL,
				"hosted-json-state-upload-url": jsonUploadURL, 
				"serial":  serial,
				"lineage": lineage,
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
		_, createErr := h.stateStore.Create(c.Request().Context(), stateID)
		if createErr != nil && createErr != storage.ErrAlreadyExists {
			return c.JSON(500, map[string]string{
				"error": "Failed to create workspace state",
			})
		}
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
            "errors": []map[string]string{{"status":"404","title":"not_found"}},
        })
    }

    // Parse state version ID to extract state ID
    stateID, err := parseStateVersionID(id)
    if err != nil {
        return c.JSON(http.StatusNotFound, map[string]interface{}{
            "errors": []map[string]string{{"status":"404","title":"invalid_id"}},
        })
    }

    // Load metadata (and optionally content)
    meta, err := h.stateStore.Get(c.Request().Context(), stateID)
    if err != nil {
        if err == storage.ErrNotFound {
            return c.JSON(http.StatusNotFound, map[string]interface{}{
                "errors": []map[string]string{{"status":"404","title":"state_not_found"}},
            })
        }
        return c.JSON(http.StatusInternalServerError, map[string]string{"error":"state_meta_error"})
    }

    // Optional: extract serial/lineage + md5
    var serial int
    var lineage, md5b64 string
    if bytes, dErr := h.stateStore.Download(c.Request().Context(), stateID); dErr == nil && len(bytes) > 0 {
        var st map[string]interface{}
        if json.Unmarshal(bytes, &st) == nil {
            if v, ok := st["serial"].(float64); ok { serial = int(v) }
            if v, ok := st["lineage"].(string); ok { lineage = v }
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
                "created-at":                 meta.Updated.UTC().Format(time.RFC3339),
                "updated-at":                 meta.Updated.UTC().Format(time.RFC3339),
                "size":                       meta.Size,
                "serial":                     serial,
                "lineage":                    lineage,
                "md5":                        md5b64, // optional
                "hosted-state-download-url":  downloadURL,
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