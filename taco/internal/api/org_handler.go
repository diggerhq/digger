package api

import (
	"errors"
	"log/slog"
	"net/http"
	"context"
	"fmt"

	"github.com/diggerhq/digger/opentaco/internal/domain"
	"github.com/diggerhq/digger/opentaco/internal/rbac"
	"github.com/labstack/echo/v4"
)

// OrgHandler handles organization and user management endpoints
type OrgHandler struct {
	orgRepo     domain.OrganizationRepository // ✅ Interface, not concrete implementation
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
	OrgID string `json:"org_id" validate:"required"`
	Name  string `json:"name" validate:"required"`
}

// CreateOrgResponse is the response for creating an organization
type CreateOrgResponse struct {
	OrgID     string `json:"org_id"`
	Name      string `json:"name"`
	CreatedBy string `json:"created_by"`
	CreatedAt string `json:"created_at"`
}

// CreateOrganization handles POST /internal/orgs
// Creates a new organization and assigns the creating user as admin
func (h *OrgHandler) CreateOrganization(c echo.Context) error {
	ctx := c.Request().Context()

	// Get user context from webhook middleware
	userID := c.Get("user_id")
	email := c.Get("email")

	if userID == nil || email == nil {
		slog.Error("Missing user context in create org request")
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

	emailStr, ok := email.(string)
	if !ok || emailStr == "" {
		slog.Error("Invalid email type in context")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "invalid email context - webhook middleware misconfigured",
		})
	}

	// Parse request
	var req CreateOrgRequest
	if err := c.Bind(&req); err != nil {
		slog.Error("Failed to bind create org request", "error", err)
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
		})
	}

	if req.OrgID == "" || req.Name == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "org_id and name are required",
		})
	}

	slog.Info("Creating organization",
		"orgID", req.OrgID,
		"name", req.Name,
		"createdBy", userIDStr,
	)

	// ========================================
	//  Use transaction to create org + init RBAC atomically
	// ========================================
	var org *domain.Organization

	err := h.orgRepo.WithTransaction(ctx, func(ctx context.Context, txRepo domain.OrganizationRepository) error {
		// Create organization within transaction
		createdOrg, err := txRepo.Create(ctx, req.OrgID, req.Name, userIDStr)
		if err != nil {
			return err
		}
		org = createdOrg

		// Initialize RBAC within the same transaction
		if h.rbacManager != nil {
			slog.Info("Initializing RBAC for new organization",
				"orgID", req.OrgID,
				"adminUser", userIDStr,
			)

			if err := h.rbacManager.InitializeRBAC(ctx, userIDStr, emailStr); err != nil {
				// IMPORTANT: Returning error here will rollback the entire transaction
				slog.Error("Failed to initialize RBAC, rolling back org creation",
					"orgID", req.OrgID,
					"error", err,
				)
				return fmt.Errorf("failed to initialize RBAC: %w", err)
			}

			slog.Info("RBAC initialized successfully",
				"orgID", req.OrgID,
				"adminUser", userIDStr,
			)
		}

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

		// Any other error (including RBAC init failure) returns 500
		slog.Error("Failed to create organization with RBAC",
			"orgID", req.OrgID,
			"error", err,
		)
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to create organization",
			"detail": err.Error(),
		})
	}

	// Success - both org and RBAC were created
	return c.JSON(http.StatusCreated, CreateOrgResponse{
		OrgID:     org.OrgID,
		Name:      org.Name,
		CreatedBy: org.CreatedBy,
		CreatedAt: org.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
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
		slog.Error("Failed to get organization",
			"orgID", orgID,
			"error", err,
		)
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to get organization",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"org_id":     org.OrgID,
		"name":       org.Name,
		"created_by": org.CreatedBy,
		"created_at": org.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		"updated_at": org.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// ListOrganizations handles GET /internal/orgs
func (h *OrgHandler) ListOrganizations(c echo.Context) error {
	ctx := c.Request().Context()

	orgs, err := h.orgRepo.List(ctx)
	if err != nil {
		slog.Error("Failed to list organizations", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to list organizations",
		})
	}

	response := make([]map[string]interface{}, len(orgs))
	for i, org := range orgs {
		response[i] = map[string]interface{}{
			"org_id":     org.OrgID,
			"name":       org.Name,
			"created_by": org.CreatedBy,
			"created_at": org.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			"updated_at": org.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
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
		slog.Error("Failed to ensure user",
			"subject", req.Subject,
			"error", err,
		)
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to create user",
		})
	}

	// Optionally assign RBAC role
	if req.RoleID != "" && h.rbacManager != nil {
		slog.Info("Assigning role to user",
			"subject", req.Subject,
			"email", req.Email,
			"roleID", req.RoleID,
		)

		if err := h.rbacManager.AssignRole(ctx, req.Subject, req.Email, req.RoleID); err != nil {
			slog.Error("Failed to assign role to user",
				"subject", req.Subject,
				"roleID", req.RoleID,
				"error", err,
			)
			// Don't fail the request, just log the error
			// User was created successfully, role assignment can be retried
		} else {
			slog.Info("Role assigned successfully",
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
		slog.Error("Failed to get user",
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

	users, err := h.userRepo.List(ctx)
	if err != nil {
		slog.Error("Failed to list users", "error", err)
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
