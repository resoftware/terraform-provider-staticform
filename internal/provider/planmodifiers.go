package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// uploadSiblingChanged reports whether the `source` or `source_hash` sibling of
// the attribute at attrPath differs between plan and state. Used to decide when
// an upload-derived computed value (s3_key, file_name, …) must be recomputed.
func uploadSiblingChanged(ctx context.Context, attrPath path.Path, plan tfsdk.Plan, state tfsdk.State) bool {
	parent := attrPath.ParentPath()
	var planSrc, stateSrc, planHash, stateHash types.String
	plan.GetAttribute(ctx, parent.AtName("source"), &planSrc)
	state.GetAttribute(ctx, parent.AtName("source"), &stateSrc)
	plan.GetAttribute(ctx, parent.AtName("source_hash"), &planHash)
	state.GetAttribute(ctx, parent.AtName("source_hash"), &stateHash)
	return !planSrc.Equal(stateSrc) || !planHash.Equal(stateHash)
}

// uploadComputedString keeps a computed, upload-derived string attribute stable
// across plans (like UseStateForUnknown) but recomputes it (leaves it unknown)
// when the sibling `source`/`source_hash` change. If the user set the attribute
// explicitly (e.g. a pre-existing s3_key), that config value is used unchanged.
type uploadComputedString struct{}

func (uploadComputedString) Description(context.Context) string         { return "" }
func (uploadComputedString) MarkdownDescription(context.Context) string { return "" }

func (uploadComputedString) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if !req.ConfigValue.IsNull() || req.StateValue.IsNull() {
		return
	}
	if uploadSiblingChanged(ctx, req.Path, req.Plan, req.State) {
		return // leave unknown -> recompute (re-upload)
	}
	resp.PlanValue = req.StateValue
}

// uploadComputedInt64 is the int64 counterpart of uploadComputedString.
type uploadComputedInt64 struct{}

func (uploadComputedInt64) Description(context.Context) string         { return "" }
func (uploadComputedInt64) MarkdownDescription(context.Context) string { return "" }

func (uploadComputedInt64) PlanModifyInt64(ctx context.Context, req planmodifier.Int64Request, resp *planmodifier.Int64Response) {
	if !req.ConfigValue.IsNull() || req.StateValue.IsNull() {
		return
	}
	if uploadSiblingChanged(ctx, req.Path, req.Plan, req.State) {
		return
	}
	resp.PlanValue = req.StateValue
}
