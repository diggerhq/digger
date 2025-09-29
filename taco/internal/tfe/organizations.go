package tfe

import (
	"github.com/google/jsonapi"
)

import (
	"fmt"
	"github.com/diggerhq/digger/opentaco/internal/domain"
	"github.com/labstack/echo/v4"
)

// Adapted from OTF (MPL License): https://github.com/leg100/otf
func (h *TfeHandler) GetOrganizationEntitlements(c echo.Context) error {
	tfidStr := domain.NewTfeResourceIDWithVal(domain.OrganizationType, "RoiPNhWzpjaKhjcV")
	ents := domain.DefaultEntitlements(tfidStr)

	// map to the JSON:API DTO
	payload := &domain.TFEEntitlements{
		ID:                    ents.ID.String(), // same concrete type domain.TFEResourceID
		Agents:                ents.Agents,
		AuditLogging:          ents.AuditLogging,
		CostEstimation:        ents.CostEstimation,
		Operations:            ents.Operations,
		PrivateModuleRegistry: ents.PrivateModuleRegistry,
		SSO:                   ents.SSO,
		Sentinel:              ents.Sentinel,
		StateStorage:          ents.StateStorage,
		Teams:                 ents.Teams,
		VCSIntegrations:       ents.VCSIntegrations,
	}

	c.Response().Header().Set(echo.HeaderContentType, "application/vnd.api+json")
	c.Response().Header().Set("Tfp-Api-Version", "2.5")
	c.Response().Header().Set("X-Terraform-Enterprise-App", "OpenTaco")

	if err := jsonapi.MarshalPayload(c.Response().Writer, payload); err != nil {
		fmt.Printf("an error occured in marshal payload %v", err)
		return err
	}
	return nil
}
