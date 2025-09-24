package resources

import (
    "context"
    "crypto/sha256"
    "strings"

    "github.com/diggerhq/digger/opentaco/internal/analytics"
    "github.com/hashicorp/terraform-plugin-framework/path"
    "github.com/hashicorp/terraform-plugin-framework/resource"
    "github.com/hashicorp/terraform-plugin-framework/resource/schema"
    "github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
    "github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
    "github.com/hashicorp/terraform-plugin-framework/types"
    "github.com/mr-tron/base58"
)

// Ensure the implementation satisfies the expected interfaces.
var (
    _ resource.Resource                = &dependencyResource{}
    _ resource.ResourceWithImportState = &dependencyResource{}
)

// NewDependencyResource returns a new instance
func NewDependencyResource() resource.Resource { return &dependencyResource{} }

type dependencyResource struct{}

type dependencyModel struct {
    ID           types.String `tfsdk:"id"`
    FromUnitID   types.String `tfsdk:"from_unit_id"`
    FromOutput   types.String `tfsdk:"from_output"`
    ToUnitID     types.String `tfsdk:"to_unit_id"`
    ToInput      types.String `tfsdk:"to_input"`

    InDigest     types.String `tfsdk:"in_digest"`
    OutDigest    types.String `tfsdk:"out_digest"`
    Status       types.String `tfsdk:"status"`
    LastInAt     types.String `tfsdk:"last_in_at"`
    LastOutAt    types.String `tfsdk:"last_out_at"`
}

func (r *dependencyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
    resp.TypeName = req.ProviderTypeName + "_dependency"
}

func (r *dependencyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
    resp.Schema = schema.Schema{
        Description: "Represents an output-level dependency between two units. Stores only digests and timestamps in tfstate.",
        Attributes: map[string]schema.Attribute{
            "id": schema.StringAttribute{
                Description: "Deterministic edge ID (computed).",
                Computed:    true,
                PlanModifiers: []planmodifier.String{ stringplanmodifier.UseStateForUnknown() },
            },
            "from_unit_id": schema.StringAttribute{
                Description: "Source unit ID (e.g., org/app/A).",
                Required:    true,
            },
            "from_output": schema.StringAttribute{
                Description: "Source output name.",
                Required:    true,
            },
            "to_unit_id": schema.StringAttribute{
                Description: "Target unit ID (e.g., org/app/B).",
                Required:    true,
            },
            "to_input": schema.StringAttribute{
                Description: "Target input name (for documentation/UX).",
                Optional:    true,
            },
            // Computed fields (populated by service on unit writes)
            "in_digest": schema.StringAttribute{ Computed: true },
            "out_digest": schema.StringAttribute{ Computed: true },
            "status": schema.StringAttribute{ Computed: true },
            "last_in_at": schema.StringAttribute{ Computed: true },
            "last_out_at": schema.StringAttribute{ Computed: true },
        },
    }
}

func (r *dependencyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
    analytics.SendEssential("terraform_apply_started")
    
    var plan dependencyModel
    diags := req.Plan.Get(ctx, &plan)
    resp.Diagnostics.Append(diags...)
    if resp.Diagnostics.HasError() { 
        analytics.SendEssential("terraform_apply_failed")
        return 
    }

    // Default to_input to from_output if not set
    toInput := plan.ToInput.ValueString()
    if toInput == "" {
        toInput = plan.FromOutput.ValueString()
    }

    // Compute deterministic ID
    id := computeEdgeID(plan.FromUnitID.ValueString(), plan.FromOutput.ValueString(), plan.ToUnitID.ValueString(), toInput)

    // Initial computed fields: leave digests empty; status unknown until source writes
    plan.ID = types.StringValue(id)
    plan.ToInput = types.StringValue(toInput)
    // Ensure no attribute remains Unknown after apply
    plan.InDigest = types.StringValue("")
    plan.OutDigest = types.StringValue("")
    plan.LastInAt = types.StringNull()
    plan.LastOutAt = types.StringNull()
    if plan.Status.IsNull() || plan.Status.IsUnknown() {
        plan.Status = types.StringValue("unknown")
    }

    diags = resp.State.Set(ctx, plan)
    resp.Diagnostics.Append(diags...)
    if !resp.Diagnostics.HasError() {
        analytics.SendEssential("terraform_apply_completed")
    }
}

func (r *dependencyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
    analytics.SendEssential("terraform_plan_started")
    
    // No remote calls; state is source of truth (service may have edited it via state surgery)
    var state dependencyModel
    diags := req.State.Get(ctx, &state)
    resp.Diagnostics.Append(diags...)
    if resp.Diagnostics.HasError() { 
        analytics.SendEssential("terraform_plan_failed")
        return 
    }
    diags = resp.State.Set(ctx, &state)
    resp.Diagnostics.Append(diags...)
    if !resp.Diagnostics.HasError() {
        analytics.SendEssential("terraform_plan_completed")
    }
}

func (r *dependencyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
    analytics.SendEssential("terraform_apply_started")
    
    // Carry over computed fields from current state to avoid Unknowns
    var state dependencyModel
    diags := req.State.Get(ctx, &state)
    resp.Diagnostics.Append(diags...)
    if resp.Diagnostics.HasError() { 
        analytics.SendEssential("terraform_apply_failed")
        return 
    }

    var plan dependencyModel
    diags = req.Plan.Get(ctx, &plan)
    resp.Diagnostics.Append(diags...)
    if resp.Diagnostics.HasError() { 
        analytics.SendEssential("terraform_apply_failed")
        return 
    }

    if plan.ToInput.ValueString() == "" {
        plan.ToInput = plan.FromOutput
    }
    // Recompute ID for safety
    plan.ID = types.StringValue(computeEdgeID(plan.FromUnitID.ValueString(), plan.FromOutput.ValueString(), plan.ToUnitID.ValueString(), plan.ToInput.ValueString()))
    // Preserve computed values from existing state unless explicitly set
    if plan.InDigest.IsUnknown() { plan.InDigest = state.InDigest }
    if plan.OutDigest.IsUnknown() { plan.OutDigest = state.OutDigest }
    if plan.LastInAt.IsUnknown() { plan.LastInAt = state.LastInAt }
    if plan.LastOutAt.IsUnknown() { plan.LastOutAt = state.LastOutAt }
    if plan.Status.IsUnknown() || plan.Status.IsNull() {
        if !state.Status.IsNull() && !state.Status.IsUnknown() { plan.Status = state.Status } else { plan.Status = types.StringValue("unknown") }
    }

    diags = resp.State.Set(ctx, plan)
    resp.Diagnostics.Append(diags...)
    if !resp.Diagnostics.HasError() {
        analytics.SendEssential("terraform_apply_completed")
    }
}

func (r *dependencyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
    analytics.SendEssential("terraform_apply_started")
    // Nothing to do; remove from state
    analytics.SendEssential("terraform_apply_completed")
}

func (r *dependencyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
    resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func computeEdgeID(fromUnit, fromOutput, toUnit, toInput string) string {
    // Normalize unit IDs by trimming suffix if present
    fs := normalizeUnitID(fromUnit)
    ts := normalizeUnitID(toUnit)
    material := strings.Join([]string{fs, fromOutput, ts, toInput}, "\n")
    sum := sha256.Sum256([]byte(material))
    return base58.Encode(sum[:])
}

func normalizeUnitID(id string) string {
    s := strings.Trim(id, "/")
    return strings.TrimSuffix(s, "/terraform.tfstate")
}
