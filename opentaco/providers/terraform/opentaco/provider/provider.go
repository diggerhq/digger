package provider

import (
	"context"
	"os"

	"github.com/diggerhq/digger/opentaco/pkg/sdk"
	"github.com/diggerhq/digger/opentaco/providers/terraform/opentaco/datasources"
	"github.com/diggerhq/digger/opentaco/providers/terraform/opentaco/resources"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ provider.Provider = &opentacoProvider{}
)

// New is a helper function to simplify provider server and testing implementation.
func New() provider.Provider {
	return &opentacoProvider{}
}

// opentacoProvider is the provider implementation.
type opentacoProvider struct{}

// opentacoProviderModel maps provider schema data to a Go type.
type opentacoProviderModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
}

// Metadata returns the provider type name.
func (p *opentacoProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "opentaco"
}

// Schema defines the provider-level schema for configuration data.
func (p *opentacoProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "The OpenTaco provider allows you to manage Terraform states through the OpenTaco service.",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				Description: "The endpoint URL for the OpenTaco service. Can also be set via OPENTACO_ENDPOINT environment variable.",
				Optional:    true,
			},
		},
	}
}

// Configure prepares an API client for data sources and resources.
func (p *opentacoProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	// Retrieve provider data from configuration
	var config opentacoProviderModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Default endpoint
	endpoint := "http://localhost:8080"

	// Check environment variable first
	if envEndpoint := os.Getenv("OPENTACO_ENDPOINT"); envEndpoint != "" {
		endpoint = envEndpoint
	}

	// Override with configuration value if set
	if !config.Endpoint.IsNull() {
		endpoint = config.Endpoint.ValueString()
	}

	// Create SDK client
	client := sdk.NewClient(endpoint)

	// Make the client available during DataSource and Resource type Configure methods.
	resp.DataSourceData = client
	resp.ResourceData = client
}

// DataSources defines the data sources implemented in the provider.
func (p *opentacoProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		datasources.NewStateDataSource,
	}
}

// Resources defines the resources implemented in the provider.
func (p *opentacoProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		resources.NewStateResource,
	}
}