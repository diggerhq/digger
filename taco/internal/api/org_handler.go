package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/diggerhq/digger/opentaco/internal/domain"
	"github.com/diggerhq/digger/opentaco/internal/logging"
	"github.com/diggerhq/digger/opentaco/internal/rbac"
	"github.com/labstack/echo/v4"
)

// OrgHandler handles organization and user management endpoints
type OrgHandler struct {
	orgRepo     domain.OrganizationRepository 
	userRepo    domain.UserRepository
	rbacManager *rbac.RBACManager
}

// NewOrgHandler creates a new organization handler
func NewOrgHandler(orgRepo domain.OrganizationRepository, userRepo domain.UserRepository, rbacManager *rbac.RBACManager) *OrgHandler {
	return &OrgHandler{
		orgRepo:     orgRepo,
		userRepo:    userRepo,
		rbacManager: rbacManager,
	}
}

// CreateOrgRequest is the request body for creating an organization
type CreateOrgRequest struct {
	Name          string `json:"name" validate:"required"`         // Unique identifier (e.g., "acme")
	DisplayName   string `json:"display_name" validate:"required"` // Friendly name (e.g., "Acme Corp")
	ExternalOrgID string `json:"external_org_id"`                  // External org identifier (optional)
}

// CreateOrgResponse is the response for creating an organization
type CreateOrgResponse struct {
	ID            string `json:"id"`             // UUID
	Name          string `json:"name"`           // Unique identifier
	DisplayName   string `json:"display_name"`    // Friendly name
	ExternalOrgID string `json:"external_org_id"` // External org identifier
	CreatedBy     string `json:"created_by"`
	CreatedAt     string `json:"created_at"`
}

// CreateOrganization handles POST /internal/orgs
// Creates a new organization and assigns the creating user as admin
func (h *OrgHandler) CreateOrganization(c echo.Context) error {
	ctx := c.Request().Context()

	// Get user context from webhook middleware
	logger := logging.FromContext(c)
	userID := c.Get("user_id")
	email := c.Get("email")

	if userID == nil || email == nil {
		logger.Error("Missing user context in create org request")
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "user context required",
		})
	}

	userIDStr, ok := userID.(string)
	if !ok || userIDStr == "" {
		logger.Error("Invalid user_id type in context")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "invalid user context - webhook middleware misconfigured",
		})
	}

	emailStr, ok := email.(string)
	if !ok || emailStr == "" {
		logger.Error("Invalid email type in context")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "invalid email context - webhook middleware misconfigured",
		})
	}

	// Parse request
	var req CreateOrgRequest
	if err := c.Bind(&req); err != nil {
		logger.Error("Failed to bind create org request", "error", err)
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
		})
	}

	if req.Name == "" || req.DisplayName == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "name and display_name are required",
		})
	}

	logger.Info("Creating organization",
		"name", req.Name,
		"displayName", req.DisplayName,
		"externalOrgID", req.ExternalOrgID,
		"createdBy", userIDStr,
	)

	// ========================================
	//  Create org first, then init RBAC (SQLite-friendly)
	// ========================================
	var org *domain.Organization

	// Create organization in transaction
	err := h.orgRepo.WithTransaction(ctx, func(ctx context.Context, txRepo domain.OrganizationRepository) error {
		createdOrg, err := txRepo.Create(ctx, req.Name, req.Name, req.DisplayName, req.ExternalOrgID, userIDStr)
		if err != nil {
			return err
		}
		org = createdOrg
		return nil
	})

	// Handle transaction errors
	if err != nil {
		if errors.Is(err, domain.ErrOrgExists) {
			return c.JSON(http.StatusConflict, map[string]string{
				"error": "organization already exists",
			})
		}
		if errors.Is(err, domain.ErrInvalidOrgID) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": err.Error(),
			})
		}
		
		// Check for external org ID conflict
		if strings.Contains(err.Error(), "external org ID already exists") {
			return c.JSON(http.StatusConflict, map[string]string{
				"error": err.Error(),
			})
		}

		logger := logging.FromContext(c)
		logger.Error("Failed to create organization",
			"name", req.Name,
			"externalOrgID", req.ExternalOrgID,
			"error", err,
		)
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to create organization",
			"detail": err.Error(),
		})
	}

	// Initialize RBAC after org creation (outside transaction for SQLite compatibility)
	if h.rbacManager != nil {
		logger.Info("Initializing RBAC for new organization",
			"orgName", req.Name,
			"orgID", org.ID,
			"adminUser", userIDStr,
		)

		if err := h.rbacManager.InitializeRBAC(ctx, org.ID, userIDStr, emailStr); err != nil {
			// Org was created but RBAC failed - log warning but don't fail the request
			// User can retry RBAC initialization or assign roles manually
			logger.Warn("Organization created but RBAC initialization failed",
				"orgName", req.Name,
				"orgID", org.ID,
				"error", err,
				"recommendation", "RBAC can be initialized later via /rbac/init endpoint",
			)
			// Continue with success response - org was created
		} else {
			logger.Info("RBAC initialized successfully",
				"orgName", req.Name,
				"orgID", org.ID,
				"adminUser", userIDStr,
			)
		}
	}

	// Success - org created (and RBAC initialized if available)
	return c.JSON(http.StatusCreated, CreateOrgResponse{
		ID:            org.ID,
		Name:          org.Name,
		DisplayName:   org.DisplayName,
		ExternalOrgID: org.ExternalOrgID,
		CreatedBy:     org.CreatedBy,
		CreatedAt:     org.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// SyncExternalOrgRequest is the request body for syncing an external organization
type SyncExternalOrgRequest struct {
	Name          string `json:"name" validate:"required"`           // Internal name (e.g., "acme")
	DisplayName   string `json:"display_name" validate:"required"` // Friendly name (e.g., "Acme Corp")
	ExternalOrgID string `json:"external_org_id" validate:"required"` // External org identifier
}

// SyncExternalOrgResponse is the response for syncing an external organization
type SyncExternalOrgResponse struct {
	Status       string `json:"status"`        // "created" or "existing"
	Organization *domain.Organization `json:"organization"`
}

// SyncExternalOrg handles POST /internal/orgs/sync
// Creates a new organization with external mapping or returns existing one
func (h *OrgHandler) SyncExternalOrg(c echo.Context) error {
	ctx := c.Request().Context()
	logger := logging.FromContext(c)

	// Get user context from webhook middleware
	userID := c.Get("user_id")
	email := c.Get("email")

	if userID == nil || email == nil {
		slog.Error("Missing user context in sync org request")
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "user context required",
		})
	}

	userIDStr, ok := userID.(string)
	if !ok || userIDStr == "" {
		slog.Error("Invalid user_id type in context")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "invalid user context - webhook middleware misconfigured",
		})
	}

	// Parse request
	var req SyncExternalOrgRequest
	if err := c.Bind(&req); err != nil {
		slog.Error("Failed to bind sync org request", "error", err)
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
		})
	}

	if req.Name == "" || req.DisplayName == "" || req.ExternalOrgID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "name, display_name, and external_org_id are required",
		})
	}

	slog.Info("Syncing external organization",
		"name", req.Name,
		"displayName", req.DisplayName,
		"externalOrgID", req.ExternalOrgID,
		"createdBy", userIDStr,
	)

	// Check if external org ID already exists
	existingOrg, err := h.orgRepo.GetByExternalID(ctx, req.ExternalOrgID)
	if err == nil {
		// External org ID exists, return existing org
		logger.Info("External organization already exists",
			"externalOrgID", req.ExternalOrgID,
			"orgID", existingOrg.ID,
		)
		return c.JSON(http.StatusOK, SyncExternalOrgResponse{
			Status:       "existing",
			Organization: existingOrg,
		})
	}

	if err != domain.ErrOrgNotFound {
		logger.Error("Failed to check existing external org ID",
			"externalOrgID", req.ExternalOrgID,
			"error", err,
		)
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to check existing external organization",
		})
	}

	// Create new organization with external mapping
	var org *domain.Organization
	err = h.orgRepo.WithTransaction(ctx, func(ctx context.Context, txRepo domain.OrganizationRepository) error {
		createdOrg, err := txRepo.Create(ctx, req.Name, req.Name, req.DisplayName, req.ExternalOrgID, userIDStr)
		if err != nil {
			return err
		}
		org = createdOrg
		return nil
	})

	if err != nil {
		if errors.Is(err, domain.ErrOrgExists) {
			return c.JSON(http.StatusConflict, map[string]string{
				"error": "organization name already exists",
			})
		}
		if errors.Is(err, domain.ErrInvalidOrgID) {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": err.Error(),
			})
		}
		
		// Check for external org ID conflict
		if strings.Contains(err.Error(), "external org ID already exists") {
			return c.JSON(http.StatusConflict, map[string]string{
				"error": err.Error(),
			})
		}

		logger.Error("Failed to create organization during sync",
			"name", req.Name,
			"externalOrgID", req.ExternalOrgID,
			"error", err,
		)
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to create organization",
			"detail": err.Error(),
		})
	}

	logger.Info("External organization synced successfully",
		"name", req.Name,
		"externalOrgID", req.ExternalOrgID,
		"orgID", org.ID,
	)

	return c.JSON(http.StatusCreated, SyncExternalOrgResponse{
		Status:       "created",
		Organization: org,
	})
}

// GetOrganization handles GET /internal/orgs/:orgId
func (h *OrgHandler) GetOrganization(c echo.Context) error {
	ctx := c.Request().Context()
	orgID := c.Param("orgId")

	if orgID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "org_id required",
		})
	}

	org, err := h.orgRepo.Get(ctx, orgID)
	if err != nil {
		if errors.Is(err, domain.ErrOrgNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{
				"error": "organization not found",
			})
		}
		logger := logging.FromContext(c)
		logger.Error("Failed to get organization",
			"orgID", orgID,
			"error", err,
		)
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to get organization",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"id":           org.ID,
		"name":         org.Name,
		"display_name": org.DisplayName,
		"created_by":   org.CreatedBy,
		"created_at":   org.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		"updated_at":   org.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// ListOrganizations handles GET /internal/orgs
func (h *OrgHandler) ListOrganizations(c echo.Context) error {
	ctx := c.Request().Context()

	logger := logging.FromContext(c)
	orgs, err := h.orgRepo.List(ctx)
	if err != nil {
		logger.Error("Failed to list organizations", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to list organizations",
		})
	}

	response := make([]map[string]interface{}, len(orgs))
	for i, org := range orgs {
		response[i] = map[string]interface{}{
			"id":           org.ID,
			"name":         org.Name,
			"display_name": org.DisplayName,
			"created_by":   org.CreatedBy,
			"created_at":   org.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			"updated_at":   org.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"organizations": response,
	})
}

// ============================================
// User Management Endpoints
// ============================================

// CreateUserRequest is the request body for creating a user
type CreateUserRequest struct {
	Subject string `json:"subject" validate:"required"` // Unique user ID (email, auth0 ID, etc.)
	Email   string `json:"email" validate:"required"`
	RoleID  string `json:"role_id,omitempty"` // Optional: assign RBAC role
}

// CreateUserResponse is the response for creating a user
type CreateUserResponse struct {
	Subject   string `json:"subject"`
	Email     string `json:"email"`
	CreatedAt string `json:"created_at"`
	RoleID    string `json:"role_id,omitempty"`
}

// CreateUser handles POST /internal/users
// Creates or ensures a user exists (idempotent)
// Optionally assigns an RBAC role
func (h *OrgHandler) CreateUser(c echo.Context) error {
	ctx := c.Request().Context()
	logger := logging.FromContext(c)

	// Parse request
	var req CreateUserRequest
	if err := c.Bind(&req); err != nil {
		slog.Error("Failed to bind create user request", "error", err)
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
		})
	}

	if req.Subject == "" || req.Email == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "subject and email are required",
		})
	}

	slog.Info("Creating/ensuring user",
		"subject", req.Subject,
		"email", req.Email,
		"roleID", req.RoleID,
	)

	// Create or get user (idempotent)
	user, err := h.userRepo.EnsureUser(ctx, req.Subject, req.Email)
	if err != nil {
		logger.Error("Failed to ensure user",
			"subject", req.Subject,
			"error", err,
		)
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to create user",
		})
	}

	// Optionally assign RBAC role
	if req.RoleID != "" && h.rbacManager != nil {
		logger.Info("Assigning role to user",
			"subject", req.Subject,
			"email", req.Email,
			"roleID", req.RoleID,
		)

		if err := h.rbacManager.AssignRole(ctx, req.Subject, req.Email, req.RoleID); err != nil {
			logger.Error("Failed to assign role to user",
				"subject", req.Subject,
				"roleID", req.RoleID,
				"error", err,
			)
			// Don't fail the request, just log the error
			// User was created successfully, role assignment can be retried
		} else {
			logger.Info("Role assigned successfully",
				"subject", req.Subject,
				"roleID", req.RoleID,
			)
		}
	}

	response := CreateUserResponse{
		Subject:   user.Subject,
		Email:     user.Email,
		CreatedAt: user.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	if req.RoleID != "" {
		response.RoleID = req.RoleID
	}

	return c.JSON(http.StatusCreated, response)
}

// GetUser handles GET /internal/users/:subject
func (h *OrgHandler) GetUser(c echo.Context) error {
	ctx := c.Request().Context()
	subject := c.Param("subject")

	if subject == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "subject required",
		})
	}

	user, err := h.userRepo.Get(ctx, subject)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{
				"error": "user not found",
			})
		}
		logger := logging.FromContext(c)
		logger.Error("Failed to get user",
			"subject", subject,
			"error", err,
		)
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to get user",
		})
	}

	// Get RBAC roles if available
	var roles []string
	if h.rbacManager != nil {
		if assignment, err := h.rbacManager.GetUserInfo(ctx, subject); err == nil && assignment != nil {
			roles = assignment.Roles
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"subject":    user.Subject,
		"email":      user.Email,
		"created_at": user.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		"updated_at": user.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		"roles":      roles,
	})
}

// ListUsers handles GET /internal/users
func (h *OrgHandler) ListUsers(c echo.Context) error {
	ctx := c.Request().Context()

	logger := logging.FromContext(c)
	users, err := h.userRepo.List(ctx)
	if err != nil {
		logger.Error("Failed to list users", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to list users",
		})
	}

	response := make([]map[string]interface{}, len(users))
	for i, user := range users {
		userData := map[string]interface{}{
			"subject":    user.Subject,
			"email":      user.Email,
			"created_at": user.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			"updated_at": user.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}

		// Get RBAC roles if available
		if h.rbacManager != nil {
			if assignment, err := h.rbacManager.GetUserInfo(ctx, user.Subject); err == nil && assignment != nil {
				userData["roles"] = assignment.Roles
			}
		}

		response[i] = userData
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"users": response,
	})
}
