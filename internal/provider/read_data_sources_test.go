package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/resoftware/terraform-provider-staticform/internal/client"
)

func readSubscription(t *testing.T, body string) subscriptionModel {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/billing/me" {
			t.Errorf("unexpected request %s", r.URL)
		}
		fmt.Fprint(w, body)
	}))
	defer srv.Close()
	ctx := context.Background()
	d := &subscriptionDataSource{client: client.New(srv.URL, "key")}
	sr := datasource.SchemaResponse{}
	d.Schema(ctx, datasource.SchemaRequest{}, &sr)
	resp := datasource.ReadResponse{State: tfsdk.State{Schema: sr.Schema}}
	d.Read(ctx, datasource.ReadRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	var state subscriptionModel
	if diags := resp.State.Get(ctx, &state); diags.HasError() {
		t.Fatal(diags)
	}
	return state
}

func TestSubscriptionDataSourceReadsBillingState(t *testing.T) {
	state := readSubscription(t, `{"hasBillingAccount":true,"stripeCustomerId":"cus_1","activeSubscription":{"subscriptionId":"sub-1","stripeProductId":"prod_1","stripePriceId":"price_1","status":"Active","interval":"Year","unitPriceAmount":190,"unitPriceCurrency":"eur","currentPeriodStart":"2026-01-01T00:00:00+00:00","currentPeriodEnd":"2027-01-01T00:00:00+00:00","trialEndUtc":null,"cancelAtPeriodEnd":true,"canceledAtUtc":null},"entitlements":["forms.subscription"]}`)
	if !state.Active.ValueBool() || !state.HasBillingAccount.ValueBool() || state.StripeCustomerID.ValueString() != "cus_1" ||
		state.SubscriptionID.ValueString() != "sub-1" || state.Status.ValueString() != "Active" || state.Interval.ValueString() != "Year" ||
		state.UnitPriceAmount.ValueFloat64() != 190 || state.UnitPriceCurrency.ValueString() != "eur" ||
		state.CurrentPeriodEnd.ValueString() != "2027-01-01T00:00:00+00:00" || !state.CancelAtPeriodEnd.ValueBool() {
		t.Fatalf("subscription not mapped: %#v", state)
	}
	if !state.TrialEnd.IsNull() || !state.CanceledAt.IsNull() {
		t.Fatal("absent timestamps must be null")
	}
	var entitlements []string
	state.Entitlements.ElementsAs(context.Background(), &entitlements, false)
	if len(entitlements) != 1 || entitlements[0] != "forms.subscription" {
		t.Fatalf("entitlements not mapped: %v", entitlements)
	}
}

func TestSubscriptionDataSourceWithoutSubscription(t *testing.T) {
	state := readSubscription(t, `{"hasBillingAccount":false,"stripeCustomerId":null,"activeSubscription":null,"entitlements":[]}`)
	if state.Active.ValueBool() || state.HasBillingAccount.ValueBool() || state.CancelAtPeriodEnd.ValueBool() {
		t.Fatalf("flags wrong: %#v", state)
	}
	if !state.StripeCustomerID.IsNull() || !state.SubscriptionID.IsNull() || !state.Status.IsNull() || !state.UnitPriceAmount.IsNull() || !state.CurrentPeriodEnd.IsNull() {
		t.Fatalf("subscription attributes must be null: %#v", state)
	}
	if state.Entitlements.IsNull() || len(state.Entitlements.Elements()) != 0 {
		t.Fatal("entitlements must be an empty list")
	}
}

func TestUserDataSourceDerivesCapabilitiesFromEntitlements(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/users/me":
			fmt.Fprint(w, `{"id":"u1","email":"a@example.com","isSuperAdmin":false,"permissions":["User.Account.Get"]}`)
		case "/api/v1/billing/entitlements":
			fmt.Fprint(w, `{"featureKeys":["forms.subscription","forms.notion","forms.payments"],"limits":{}}`)
		default:
			t.Errorf("unexpected request %s", r.URL)
		}
	}))
	defer srv.Close()
	ctx := context.Background()
	d := &userDataSource{client: client.New(srv.URL, "key")}
	sr := datasource.SchemaResponse{}
	d.Schema(ctx, datasource.SchemaRequest{}, &sr)
	resp := datasource.ReadResponse{State: tfsdk.State{Schema: sr.Schema}}
	d.Read(ctx, datasource.ReadRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	var state userModel
	if diags := resp.State.Get(ctx, &state); diags.HasError() {
		t.Fatal(diags)
	}
	if state.ID.ValueString() != "u1" || state.IsAdmin.ValueBool() || !state.CanUseNotion.ValueBool() || !state.CanUsePaymentForms.ValueBool() ||
		state.CanUseGoogleSheets.ValueBool() || state.CanUseEmailAttachments.ValueBool() || state.CanUseConditionalSubmitActions.ValueBool() {
		t.Fatalf("capabilities not derived from entitlements: %#v", state)
	}
}

func TestUserDataSourceReportsMissingPermission(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/billing/entitlements" {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, `{"title":"Forbidden","detail":"Missing permission: User.Billing.GetEntitlements","errorCode":"Forbidden"}`)
			return
		}
		fmt.Fprint(w, `{"id":"u1","email":"a@example.com","isSuperAdmin":false,"permissions":[]}`)
	}))
	defer srv.Close()
	ctx := context.Background()
	d := &userDataSource{client: client.New(srv.URL, "key")}
	sr := datasource.SchemaResponse{}
	d.Schema(ctx, datasource.SchemaRequest{}, &sr)
	resp := datasource.ReadResponse{State: tfsdk.State{Schema: sr.Schema}}
	d.Read(ctx, datasource.ReadRequest{}, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("missing permission was not reported")
	}
	if detail := resp.Diagnostics[0].Detail(); !strings.Contains(detail, "User.Billing.GetEntitlements") {
		t.Fatalf("diagnostic does not name the permission: %s", detail)
	}
}

func TestUserSettingsResourceKeepsUnsetFrequencyAndUpdatesSet(t *testing.T) {
	frequency := "Weekly"
	var puts []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/users/me/settings" {
			t.Errorf("unexpected request %s", r.URL)
		}
		if r.Method == http.MethodPut {
			var body client.UserSettings
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			frequency = body.SubmissionOverviewEmailFrequency
			puts = append(puts, frequency)
		}
		fmt.Fprintf(w, `{"submissionOverviewEmailFrequency":%q}`, frequency)
	}))
	defer srv.Close()
	ctx := context.Background()
	r := &userSettingsResource{client: client.New(srv.URL, "key")}
	sr := resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, &sr)

	for _, planned := range []types.String{types.StringUnknown(), types.StringValue("Monthly")} {
		plan := tfsdk.Plan{Schema: sr.Schema}
		model := userSettingsModel{ID: types.StringUnknown(), SubmissionOverviewEmailFrequency: planned}
		if diags := plan.Set(ctx, &model); diags.HasError() {
			t.Fatal(diags)
		}
		created := resource.CreateResponse{State: tfsdk.State{Schema: sr.Schema}}
		r.Create(ctx, resource.CreateRequest{Plan: plan}, &created)
		if created.Diagnostics.HasError() {
			t.Fatal(created.Diagnostics)
		}
		var state userSettingsModel
		if diags := created.State.Get(ctx, &state); diags.HasError() {
			t.Fatal(diags)
		}
		want := "Weekly"
		if !planned.IsUnknown() {
			want = "Monthly"
		}
		if state.ID.ValueString() != "settings" || state.SubmissionOverviewEmailFrequency.ValueString() != want {
			t.Fatalf("planned %v produced %#v", planned, state)
		}
	}
	if len(puts) != 2 || puts[0] != "Weekly" || puts[1] != "Monthly" {
		t.Fatalf("unexpected updates: %v", puts)
	}
}
