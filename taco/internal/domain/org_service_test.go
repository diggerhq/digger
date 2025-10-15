package domain

import (
	"context"
	"testing"
)

func TestOrgService_GetOrgPrefix(t *testing.T) {
	service := NewOrgService()

	tests := []struct {
		name   string
		orgID  string
		want   string
	}{
		{
			name:  "simple org ID",
			orgID: "acme-corp",
			want:  "org-acme-corp/",
		},
		{
			name:  "numeric org ID",
			orgID: "123",
			want:  "org-123/",
		},
		{
			name:  "org ID with special chars",
			orgID: "startup_xyz",
			want:  "org-startup_xyz/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := service.GetOrgPrefix(tt.orgID)
			if got != tt.want {
				t.Errorf("GetOrgPrefix() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOrgService_ValidateUnitBelongsToOrg(t *testing.T) {
	service := NewOrgService()

	tests := []struct {
		name    string
		unitID  string
		orgID   string
		wantErr bool
	}{
		{
			name:    "unit belongs to org",
			unitID:  "org-acme-corp/my-state",
			orgID:   "acme-corp",
			wantErr: false,
		},
		{
			name:    "unit belongs to different org",
			unitID:  "org-other-corp/their-state",
			orgID:   "acme-corp",
			wantErr: true,
		},
		{
			name:    "unit not namespaced",
			unitID:  "my-state",
			orgID:   "acme-corp",
			wantErr: true,
		},
		{
			name:    "nested unit ID",
			unitID:  "org-acme-corp/production/terraform-state",
			orgID:   "acme-corp",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.ValidateUnitBelongsToOrg(tt.unitID, tt.orgID)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateUnitBelongsToOrg() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOrgService_GetOrgScopedPrefix(t *testing.T) {
	service := NewOrgService()

	tests := []struct {
		name       string
		orgID      string
		userPrefix string
		want       string
	}{
		{
			name:       "no user prefix",
			orgID:      "acme-corp",
			userPrefix: "",
			want:       "org-acme-corp/",
		},
		{
			name:       "user prefix without org",
			orgID:      "acme-corp",
			userPrefix: "production",
			want:       "org-acme-corp/production",
		},
		{
			name:       "user prefix with org already included",
			orgID:      "acme-corp",
			userPrefix: "org-acme-corp/production",
			want:       "org-acme-corp/production",
		},
		{
			name:       "user prefix with nested path",
			orgID:      "acme-corp",
			userPrefix: "production/us-east-1",
			want:       "org-acme-corp/production/us-east-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := service.GetOrgScopedPrefix(tt.orgID, tt.userPrefix)
			if got != tt.want {
				t.Errorf("GetOrgScopedPrefix() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOrgService_IsOrgNamespaced(t *testing.T) {
	service := NewOrgService()

	tests := []struct {
		name   string
		unitID string
		want   bool
	}{
		{
			name:   "properly namespaced",
			unitID: "org-acme-corp/my-state",
			want:   true,
		},
		{
			name:   "not namespaced",
			unitID: "my-state",
			want:   false,
		},
		{
			name:   "missing slash",
			unitID: "org-acme-corp",
			want:   false,
		},
		{
			name:   "wrong prefix",
			unitID: "organization-acme/state",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := service.IsOrgNamespaced(tt.unitID)
			if got != tt.want {
				t.Errorf("IsOrgNamespaced() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOrgService_ExtractOrgID(t *testing.T) {
	service := NewOrgService()

	tests := []struct {
		name   string
		unitID string
		want   string
	}{
		{
			name:   "extract org ID",
			unitID: "org-acme-corp/my-state",
			want:   "acme-corp",
		},
		{
			name:   "extract numeric org ID",
			unitID: "org-123/state",
			want:   "123",
		},
		{
			name:   "not namespaced",
			unitID: "my-state",
			want:   "",
		},
		{
			name:   "nested path",
			unitID: "org-acme-corp/production/us-east-1/state",
			want:   "acme-corp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := service.ExtractOrgID(tt.unitID)
			if got != tt.want {
				t.Errorf("ExtractOrgID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOrgContext(t *testing.T) {
	t.Run("store and retrieve org context", func(t *testing.T) {
		ctx := context.Background()
		orgID := "acme-corp"

		// Add org context
		ctx = ContextWithOrg(ctx, orgID)

		// Retrieve org context
		orgCtx, ok := OrgFromContext(ctx)
		if !ok {
			t.Fatal("expected org context to be present")
		}
		if orgCtx.OrgID != orgID {
			t.Errorf("OrgFromContext() = %v, want %v", orgCtx.OrgID, orgID)
		}
	})

	t.Run("missing org context", func(t *testing.T) {
		ctx := context.Background()

		// Try to retrieve without adding
		_, ok := OrgFromContext(ctx)
		if ok {
			t.Fatal("expected org context to be missing")
		}
	})
}

