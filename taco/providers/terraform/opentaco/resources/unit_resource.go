package resources

import (
    "context"
    "fmt"

    "github.com/diggerhq/digger/opentaco/pkg/sdk"
    "github.com/hashicorp/terraform-plugin-framework/path"
    "github.com/hashicorp/terraform-plugin-framework/resource"
    "github.com/hashicorp/terraform-plugin-framework/resource/schema"
    "github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
    "github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
    "github.com/hashicorp/terraform-plugin-framework/types"
)

var (
    _ resource.Resource                = &unitResource{}
    _ resource.ResourceWithConfigure   = &unitResource{}
    _ resource.ResourceWithImportState = &unitResource{}
)

func NewUnitResource() resource.Resource { return &unitResource{} }

type unitResource struct { client *sdk.Client }

type unitResourceModel struct {
    ID     types.String            `tfsdk:"id"`
    Labels map[string]types.String `tfsdk:"labels"`
}

func (r *unitResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
    resp.TypeName = req.ProviderTypeName + "_unit"
}

func (r *unitResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
    resp.Schema = schema.Schema{
        Description: "Manages an OpenTaco unit registration.",
        Attributes: map[string]schema.Attribute{
            "id": schema.StringAttribute{
                Description: "The unique identifier for the unit.",
                Required:    true,
                PlanModifiers: []planmodifier.String{ stringplanmodifier.RequiresReplace() },
            },
            "labels": schema.MapAttribute{
                Description: "Labels to associate with the unit.",
                Optional:    true,
                ElementType: types.StringType,
            },
        },
    }
}

func (r *unitResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
    if req.ProviderData == nil { return }
    client, ok := req.ProviderData.(*sdk.Client)
    if !ok {
        resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected *sdk.Client, got: %T.", req.ProviderData))
        return
    }
    r.client = client
}

func (r *unitResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
    var plan unitResourceModel
    diags := req.Plan.Get(ctx, &plan)
    resp.Diagnostics.Append(diags...)
    if resp.Diagnostics.HasError() { return }

    result, err := r.client.CreateUnit(ctx, plan.ID.ValueString())
    if err != nil {
        resp.Diagnostics.AddError("Error creating unit", "Could not create unit, unexpected error: "+err.Error())
        return
    }
    plan.ID = types.StringValue(result.ID)
    diags = resp.State.Set(ctx, plan)
    resp.Diagnostics.Append(diags...)
}

func (r *unitResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
    var state unitResourceModel
    diags := req.State.Get(ctx, &state)
    resp.Diagnostics.Append(diags...)
    if resp.Diagnostics.HasError() { return }

    _, err := r.client.GetUnit(ctx, state.ID.ValueString())
    if err != nil {
        resp.Diagnostics.AddError("Error reading unit", "Could not read unit ID "+state.ID.ValueString()+": "+err.Error())
        return
    }
    diags = resp.State.Set(ctx, &state)
    resp.Diagnostics.Append(diags...)
}

func (r *unitResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
    var plan unitResourceModel
    diags := req.Plan.Get(ctx, &plan)
    resp.Diagnostics.Append(diags...)
    if resp.Diagnostics.HasError() { return }
    diags = resp.State.Set(ctx, plan)
    resp.Diagnostics.Append(diags...)
}

func (r *unitResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
    var state unitResourceModel
    diags := req.State.Get(ctx, &state)
    resp.Diagnostics.Append(diags...)
    if resp.Diagnostics.HasError() { return }
    if err := r.client.DeleteUnit(ctx, state.ID.ValueString()); err != nil {
        resp.Diagnostics.AddError("Error deleting unit", "Could not delete unit, unexpected error: "+err.Error())
        return
    }
}

func (r *unitResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
    resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

