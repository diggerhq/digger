package datasources

import (
    "context"
    "fmt"

    "github.com/diggerhq/digger/opentaco/pkg/sdk"
    "github.com/hashicorp/terraform-plugin-framework/datasource"
    "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
    "github.com/hashicorp/terraform-plugin-framework/types"
)

var (
    _ datasource.DataSource              = &unitDataSource{}
    _ datasource.DataSourceWithConfigure = &unitDataSource{}
)

func NewUnitDataSource() datasource.DataSource { return &unitDataSource{} }

type unitDataSource struct { client *sdk.Client }

type unitDataSourceModel struct {
    ID      types.String `tfsdk:"id"`
    Size    types.Int64  `tfsdk:"size"`
    Updated types.String `tfsdk:"updated"`
    Locked  types.Bool   `tfsdk:"locked"`
    LockID  types.String `tfsdk:"lock_id"`
}

func (d *unitDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
    resp.TypeName = req.ProviderTypeName + "_unit"
}

func (d *unitDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
    resp.Schema = schema.Schema{
        Description: "Fetches information about an OpenTaco unit.",
        Attributes: map[string]schema.Attribute{
            "id": schema.StringAttribute{Description: "The unique identifier for the unit.", Required: true},
            "size": schema.Int64Attribute{Description: "The size of the unit's tfstate in bytes.", Computed: true},
            "updated": schema.StringAttribute{Description: "The last update timestamp.", Computed: true},
            "locked": schema.BoolAttribute{Description: "Whether the unit is locked.", Computed: true},
            "lock_id": schema.StringAttribute{Description: "The lock ID if the unit is locked.", Computed: true},
        },
    }
}

func (d *unitDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
    if req.ProviderData == nil { return }
    client, ok := req.ProviderData.(*sdk.Client)
    if !ok {
        resp.Diagnostics.AddError("Unexpected Data Source Configure Type", fmt.Sprintf("Expected *sdk.Client, got: %T.", req.ProviderData))
        return
    }
    d.client = client
}

func (d *unitDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
    var config unitDataSourceModel
    diags := req.Config.Get(ctx, &config)
    resp.Diagnostics.Append(diags...)
    if resp.Diagnostics.HasError() { return }

    result, err := d.client.GetUnit(ctx, config.ID.ValueString())
    if err != nil {
        resp.Diagnostics.AddError("Error reading unit", "Could not read unit ID "+config.ID.ValueString()+": "+err.Error())
        return
    }

    state := unitDataSourceModel{
        ID:      types.StringValue(result.ID),
        Size:    types.Int64Value(result.Size),
        Updated: types.StringValue(result.Updated.Format("2006-01-02T15:04:05Z")),
        Locked:  types.BoolValue(result.Locked),
    }
    if result.LockInfo != nil { state.LockID = types.StringValue(result.LockInfo.ID) } else { state.LockID = types.StringValue("") }
    diags = resp.State.Set(ctx, &state)
    resp.Diagnostics.Append(diags...)
}

