package datasources

import (
	"context"
	"fmt"

	"github.com/diggerhq/digger/opentaco/pkg/sdk"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource              = &stateDataSource{}
	_ datasource.DataSourceWithConfigure = &stateDataSource{}
)

// NewStateDataSource is a helper function to simplify the provider implementation.
func NewStateDataSource() datasource.DataSource {
	return &stateDataSource{}
}

// stateDataSource is the data source implementation.
type stateDataSource struct {
	client *sdk.Client
}

// stateDataSourceModel maps the data source schema data.
type stateDataSourceModel struct {
	ID      types.String `tfsdk:"id"`
	Size    types.Int64  `tfsdk:"size"`
	Updated types.String `tfsdk:"updated"`
	Locked  types.Bool   `tfsdk:"locked"`
	LockID  types.String `tfsdk:"lock_id"`
}

// Metadata returns the data source type name.
func (d *stateDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_state"
}

// Schema defines the schema for the data source.
func (d *stateDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches information about an OpenTaco state.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier for the state.",
				Required:    true,
			},
			"size": schema.Int64Attribute{
				Description: "The size of the state in bytes.",
				Computed:    true,
			},
			"updated": schema.StringAttribute{
				Description: "The last update timestamp.",
				Computed:    true,
			},
			"locked": schema.BoolAttribute{
				Description: "Whether the state is locked.",
				Computed:    true,
			},
			"lock_id": schema.StringAttribute{
				Description: "The lock ID if the state is locked.",
				Computed:    true,
			},
		},
	}
}

// Configure adds the provider configured client to the data source.
func (d *stateDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*sdk.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *sdk.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = client
}

// Read refreshes the Terraform state with the latest data.
func (d *stateDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config stateDataSourceModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get state from API
	result, err := d.client.GetState(ctx, config.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading state",
			"Could not read state ID "+config.ID.ValueString()+": "+err.Error(),
		)
		return
	}

	// Map response to model
	state := stateDataSourceModel{
		ID:      types.StringValue(result.ID),
		Size:    types.Int64Value(result.Size),
		Updated: types.StringValue(result.Updated.Format("2006-01-02T15:04:05Z")),
		Locked:  types.BoolValue(result.Locked),
	}

	if result.LockInfo != nil {
		state.LockID = types.StringValue(result.LockInfo.ID)
	} else {
		state.LockID = types.StringValue("")
	}

	// Set state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}