package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/resoftware/terraform-provider-staticform/internal/client"
)

func TestSensitivePolicySurvivesRefreshAndUnrelatedUpdate(t *testing.T) {
	sensitive := true
	r := &formResource{}
	var state formResourceModel
	state.ConfirmSensitiveFields = types.BoolNull()
	r.apply(context.Background(), &client.Form{ID: "form-1", Name: "Protected", Fields: []client.Field{{Name: "bsn", Type: "Text", Sensitive: &sensitive}}, SubmitActions: []client.SubmitAction{{Type: client.ActionSendEmail, Name: "Notify", SensitiveFields: []string{"bsn"}}}}, &state)
	request, diags := r.toRequest(context.Background(), state, paymentExtras{})
	if diags.HasError() {
		t.Fatal(diags)
	}
	if request.Fields[0].Sensitive == nil || !*request.Fields[0].Sensitive || len(request.SubmitActions[0].SensitiveFields) != 1 {
		t.Fatalf("policy lost: %#v", request)
	}
	if request.ConfirmSensitiveFields {
		t.Fatal("refresh must not manufacture confirmation")
	}
	state.ConfirmSensitiveFields = types.BoolValue(true)
	request, diags = r.toRequest(context.Background(), state, paymentExtras{})
	if diags.HasError() || !request.ConfirmSensitiveFields {
		t.Fatalf("explicit confirmation lost: %v", diags)
	}
	state.SubmitActions[0].SensitiveFields = toStringSet([]string{})
	request, diags = r.toRequest(context.Background(), state, paymentExtras{})
	if diags.HasError() || request.SubmitActions[0].SensitiveFields == nil || len(request.SubmitActions[0].SensitiveFields) != 0 {
		t.Fatal("explicit empty set must revoke approval")
	}
}
