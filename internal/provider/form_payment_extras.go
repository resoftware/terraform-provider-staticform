package provider

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/diag"

	"github.com/resoftware/terraform-provider-staticform/internal/client"
)

const paymentExtrasKey = "payment_extras"

type privateStateGetter interface {
	GetKey(ctx context.Context, key string) ([]byte, diag.Diagnostics)
}

type privateStateSetter interface {
	SetKey(ctx context.Context, key string, value []byte) diag.Diagnostics
}

// paymentExtras is the part of a form's payment settings that the Terraform
// configuration may leave unmanaged, as last read from the API. The update
// endpoint replaces payment settings wholesale, so these are sent back
// whenever the configuration does not set them.
type paymentExtras struct {
	RequiredCondition *client.ConditionGroup  `json:"requiredCondition,omitempty"`
	Disabled          *client.PaymentSettings `json:"disabled,omitempty"`
}

func paymentExtrasFrom(f *client.Form) paymentExtras {
	if f == nil || f.PaymentSettings == nil {
		return paymentExtras{}
	}
	e := paymentExtras{RequiredCondition: f.PaymentSettings.PaymentRequiredCondition}
	if !f.PaymentSettings.Enabled {
		e.Disabled = f.PaymentSettings
	}
	return e
}

func loadPaymentExtras(ctx context.Context, p privateStateGetter) (paymentExtras, diag.Diagnostics) {
	var e paymentExtras
	raw, diags := p.GetKey(ctx, paymentExtrasKey)
	if diags.HasError() || len(raw) == 0 {
		return e, diags
	}
	if err := json.Unmarshal(raw, &e); err != nil {
		diags.AddError("Error reading stored payment settings", err.Error())
	}
	return e, diags
}

func storePaymentExtras(ctx context.Context, p privateStateSetter, e paymentExtras) diag.Diagnostics {
	raw, err := json.Marshal(e)
	if err != nil {
		var diags diag.Diagnostics
		diags.AddError("Error storing payment settings", err.Error())
		return diags
	}
	return p.SetKey(ctx, paymentExtrasKey, raw)
}

// requiredConditionToClient resolves the payment_required_condition sent to the
// API: a block with conditions sets it, an empty block clears it, and an
// omitted block keeps the condition the API holds.
func requiredConditionToClient(blocks []runConditionModel, kept *client.ConditionGroup) *client.ConditionGroup {
	if len(blocks) == 0 {
		return kept
	}
	if len(blocks[0].Conditions) == 0 {
		return nil
	}
	return runConditionToClient(blocks[0])
}

// requiredConditionToModel mirrors the API's payment required condition into
// state only when the configuration manages it. An empty prior block stays while
// the API holds no condition.
func requiredConditionToModel(g *client.ConditionGroup, prior []runConditionModel) []runConditionModel {
	if len(prior) == 0 {
		return nil
	}
	got := conditionToModel(g)
	if len(got) == 0 {
		if len(prior[0].Conditions) == 0 {
			return prior
		}
		return nil
	}
	keepEmptyConditionStrings(got, prior)
	return got
}
