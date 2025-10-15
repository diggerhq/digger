package tfe

import (
	"fmt"
	"github.com/diggerhq/digger/opentaco/internal/domain/tfe"
	"github.com/google/jsonapi"
	"github.com/labstack/echo/v4"
)

func (h *TfeHandler) GetOrganizationEntitlements(c echo.Context) error {
	tfidStr := tfe.NewTfeResourceIdentifier(tfe.OrganizationType, "RoiPNhWzpjaKhjcV")
	payload := tfe.DefaultFeatureEntitlements(tfidStr.String())

	c.Response().Header().Set(echo.HeaderContentType, "application/vnd.api+json")
	c.Response().Header().Set("Tfp-Api-Version", "2.5")
	c.Response().Header().Set("X-Terraform-Enterprise-App", "OpenTaco")

	if err := jsonapi.MarshalPayload(c.Response().Writer, payload); err != nil {
		fmt.Printf("an error occured in marshal payload %v", err)
		return err
	}
	return nil
}
