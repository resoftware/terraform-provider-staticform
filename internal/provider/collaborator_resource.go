package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/resoftware/terraform-provider-staticform/internal/client"
)

var (
	_ resource.Resource              = (*collaboratorResource)(nil)
	_ resource.ResourceWithConfigure = (*collaboratorResource)(nil)
)

// NewCollaboratorResource constructs the staticform_collaborator resource. It
// manages a form collaboration by email: create sends an invitation, and the
// resource tracks it through acceptance. The `status` attribute reports whether
// the invitation is still `Pending` or the user has `Accepted`.
func NewCollaboratorResource() resource.Resource { return &collaboratorResource{} }

type collaboratorResource struct {
	client *client.Client
}

type collaboratorResourceModel struct {
	ID                     types.String `tfsdk:"id"`
	FormID                 types.String `tfsdk:"form_id"`
	Email                  types.String `tfsdk:"email"`
	Status                 types.String `tfsdk:"status"`
	UserID                 types.String `tfsdk:"user_id"`
	CanViewForm            types.Bool   `tfsdk:"can_view_form"`
	CanEditForm            types.Bool   `tfsdk:"can_edit_form"`
	CanViewSubmissions     types.Bool   `tfsdk:"can_view_submissions"`
	CanEditSubmissions     types.Bool   `tfsdk:"can_edit_submissions"`
	CanManageCollaborators types.Bool   `tfsdk:"can_manage_collaborators"`
}

func (r *collaboratorResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_collaborator"
}

func (r *collaboratorResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("expected *client.Client, got %T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *collaboratorResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	rr := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		MarkdownDescription: "A collaborator on a form, managed by email. Creating the resource sends an invitation; the invitee must accept it out of band. Permission changes apply once the invitation is accepted.",
		Attributes: map[string]schema.Attribute{
			"id":                       schema.StringAttribute{Computed: true, MarkdownDescription: "Stable identifier (`form_id:email`).", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"form_id":                  schema.StringAttribute{Required: true, PlanModifiers: rr, MarkdownDescription: "Form to add the collaborator to."},
			"email":                    schema.StringAttribute{Required: true, PlanModifiers: rr, MarkdownDescription: "Collaborator's email address."},
			"status":                   schema.StringAttribute{Computed: true, MarkdownDescription: "`Pending` (invitation sent) or `Accepted`."},
			"user_id":                  schema.StringAttribute{Computed: true, MarkdownDescription: "User ID, once the invitation is accepted."},
			"can_view_form":            schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
			"can_edit_form":            schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false)},
			"can_view_submissions":     schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
			"can_edit_submissions":     schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false)},
			"can_manage_collaborators": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false)},
		},
	}
}

func (m *collaboratorResourceModel) perms() client.CollaboratorPermissions {
	return client.CollaboratorPermissions{
		CanViewForm:            valueBoolOr(m.CanViewForm, true),
		CanEditForm:            valueBoolOr(m.CanEditForm, false),
		CanViewSubmissions:     valueBoolOr(m.CanViewSubmissions, true),
		CanEditSubmissions:     valueBoolOr(m.CanEditSubmissions, false),
		CanManageCollaborators: valueBoolOr(m.CanManageCollaborators, false),
	}
}

func applyPerms(m *collaboratorResourceModel, p client.CollaboratorPermissions) {
	m.CanViewForm = types.BoolValue(p.CanViewForm)
	m.CanEditForm = types.BoolValue(p.CanEditForm)
	m.CanViewSubmissions = types.BoolValue(p.CanViewSubmissions)
	m.CanEditSubmissions = types.BoolValue(p.CanEditSubmissions)
	m.CanManageCollaborators = types.BoolValue(p.CanManageCollaborators)
}

func (r *collaboratorResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan collaboratorResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	inv, err := r.client.InviteCollaborator(ctx, plan.FormID.ValueString(), client.InviteCollaboratorRequest{
		Email:                   plan.Email.ValueString(),
		CollaboratorPermissions: plan.perms(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error inviting collaborator", err.Error())
		return
	}

	plan.ID = collaboratorID(plan.FormID.ValueString(), plan.Email.ValueString())
	plan.Status = types.StringValue("Pending")
	plan.UserID = types.StringNull()
	applyPerms(&plan, inv.CollaboratorPermissions)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// collaboratorID is the stable resource identifier. The underlying server ID
// changes across the invitation -> collaborator transition (and on re-invite),
// so the resource keys on the immutable (form_id, email) pair instead.
func collaboratorID(formID, email string) types.String {
	return types.StringValue(formID + ":" + email)
}

func (r *collaboratorResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state collaboratorResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	formID := state.FormID.ValueString()
	email := state.Email.ValueString()

	// Accepted collaborator takes precedence over a lingering invitation.
	collab, err := r.client.FindCollaboratorByEmail(ctx, formID, email)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading collaborator", err.Error())
		return
	}
	state.ID = collaboratorID(formID, email)
	if collab != nil {
		state.Status = types.StringValue("Accepted")
		state.UserID = types.StringValue(collab.UserID)
		applyPerms(&state, collab.CollaboratorPermissions)
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}

	inv, err := r.client.FindInvitationByEmail(ctx, formID, email)
	if err != nil {
		resp.Diagnostics.AddError("Error reading invitation", err.Error())
		return
	}
	if inv == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	state.Status = types.StringValue("Pending")
	state.UserID = types.StringNull()
	applyPerms(&state, inv.CollaboratorPermissions)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *collaboratorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state collaboratorResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	formID := plan.FormID.ValueString()
	collab, err := r.client.FindCollaboratorByEmail(ctx, formID, plan.Email.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error resolving collaborator", err.Error())
		return
	}
	if collab == nil {
		// The invite hasn't been accepted yet and there is no "update invitation"
		// API, so re-issue it: cancel the existing pending invite (if any) and send
		// a fresh one carrying the new permissions. This keeps the invitation in
		// sync with config and avoids a permanent diff.
		if inv, ferr := r.client.FindInvitationByEmail(ctx, formID, plan.Email.ValueString()); ferr == nil && inv != nil {
			if err := r.client.CancelInvitation(ctx, formID, inv.ID); err != nil && !client.IsNotFound(err) {
				resp.Diagnostics.AddError("Error replacing pending invitation", err.Error())
				return
			}
		}
		newInv, err := r.client.InviteCollaborator(ctx, formID, client.InviteCollaboratorRequest{
			Email:                   plan.Email.ValueString(),
			CollaboratorPermissions: plan.perms(),
		})
		if err != nil {
			resp.Diagnostics.AddError("Error re-inviting collaborator", err.Error())
			return
		}
		resp.Diagnostics.AddWarning(
			"Collaborator invitation re-issued",
			"The invitation had not been accepted, so a new invitation was sent with the updated permissions.",
		)
		plan.ID = collaboratorID(formID, plan.Email.ValueString())
		plan.Status = types.StringValue("Pending")
		plan.UserID = types.StringNull()
		applyPerms(&plan, newInv.CollaboratorPermissions)
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		return
	}

	if err := r.client.UpdateCollaborator(ctx, formID, collab.UserID, plan.perms()); err != nil {
		resp.Diagnostics.AddError("Error updating collaborator", err.Error())
		return
	}
	plan.ID = collaboratorID(formID, plan.Email.ValueString())
	plan.Status = types.StringValue("Accepted")
	plan.UserID = types.StringValue(collab.UserID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *collaboratorResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state collaboratorResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	formID := state.FormID.ValueString()
	if state.UserID.IsNull() || state.UserID.ValueString() == "" {
		// Pending: resolve the current invitation by email and cancel it (the
		// stable resource id is form_id:email, not the server invitation id).
		inv, err := r.client.FindInvitationByEmail(ctx, formID, state.Email.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Error resolving invitation", err.Error())
			return
		}
		if inv != nil {
			if err := r.client.CancelInvitation(ctx, formID, inv.ID); err != nil && !client.IsNotFound(err) {
				resp.Diagnostics.AddError("Error cancelling invitation", err.Error())
			}
		}
		return
	}
	if err := r.client.RemoveCollaborator(ctx, formID, state.UserID.ValueString()); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Error removing collaborator", err.Error())
	}
}
