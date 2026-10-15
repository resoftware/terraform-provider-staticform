package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/resoftware/terraform-provider-staticform/internal/client"
)

func dashboardCondition() *client.ConditionGroup {
	return &client.ConditionGroup{Connector: "Or", Groups: []client.ConditionSubGroup{
		{Connector: "And", Conditions: []client.Condition{{FieldName: "email", Operator: "NotEmpty"}}},
		{Connector: "Or", Conditions: []client.Condition{{FieldName: "email", Operator: "Contains", Value: "@vip.example"}}},
	}}
}

func singleGroupCondition(value string) *client.ConditionGroup {
	return &client.ConditionGroup{Connector: "And", Groups: []client.ConditionSubGroup{
		{Connector: "And", Conditions: []client.Condition{{FieldName: "email", Operator: "Equals", Value: value}}},
	}}
}

func paymentFormModel(condition []runConditionModel) formResourceModel {
	return formResourceModel{
		Name: types.StringValue("form"),
		Payment: []paymentModel{{
			ConnectionID:             types.StringValue("conn-1"),
			Currency:                 types.StringValue("eur"),
			Mode:                     types.StringValue("Fixed"),
			PaymentRequiredCondition: condition,
			FixedRule:                []fixedRuleModel{{AmountCents: types.Int64Value(1000)}},
		}},
	}
}

func requestFor(t *testing.T, m formResourceModel, extras paymentExtras) client.FormRequest {
	t.Helper()
	req, diags := (&formResource{}).toRequest(context.Background(), m, extras)
	if diags.HasError() {
		t.Fatal(diags)
	}
	return req
}

func TestPaymentSettingsJSONRoundTrip(t *testing.T) {
	raw := []byte(`{"enabled":true,"connectionId":"conn-1","currency":"eur","mode":"Fixed",` +
		`"paymentRequiredCondition":{"connector":"Or","groups":[{"connector":"And","conditions":[{"fieldName":"email","operator":"NotEmpty","value":null,"caseSensitive":false}]},` +
		`{"connector":"Or","conditions":[{"fieldName":"email","operator":"Contains","value":"@vip.example","caseSensitive":true}]}]},` +
		`"fixedRules":[{"condition":{"connector":"And","groups":[{"connector":"And","conditions":[{"fieldName":"email","operator":"IsEmpty","value":null,"caseSensitive":false}]}]},"amountCents":500,"description":null}],` +
		`"fieldAmountRules":null,"lineItems":null,"customerEmailFieldName":null}`)
	var got client.PaymentSettings
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	c := got.PaymentRequiredCondition
	if c == nil || c.Connector != "Or" || len(c.Groups) != 2 || c.Groups[1].Connector != "Or" || !c.Groups[1].Conditions[0].CaseSensitive || c.Groups[1].Conditions[0].Value != "@vip.example" {
		t.Fatalf("condition not decoded: %#v", c)
	}
	if got.FixedRules[0].Condition == nil || got.FixedRules[0].Condition.Groups[0].Conditions[0].Operator != "IsEmpty" {
		t.Fatalf("rule condition not decoded: %#v", got.FixedRules[0])
	}
	out, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var again client.PaymentSettings
	if err := json.Unmarshal(out, &again); err != nil {
		t.Fatal(err)
	}
	if len(again.PaymentRequiredCondition.Groups) != 2 || again.PaymentRequiredCondition.Groups[1].Conditions[0].Value != "@vip.example" || again.FixedRules[0].AmountCents != 500 {
		t.Fatalf("condition lost in round trip: %s", out)
	}
}

func TestRequestKeepsServerConditionWhenBlockIsOmitted(t *testing.T) {
	kept := dashboardCondition()
	req := requestFor(t, paymentFormModel(nil), paymentExtras{RequiredCondition: kept})
	got := req.PaymentSettings.PaymentRequiredCondition
	if got != kept || len(got.Groups) != 2 || got.Connector != "Or" {
		t.Fatalf("server condition not sent back: %#v", got)
	}
}

func TestRequestEmptyBlockClearsTheCondition(t *testing.T) {
	req := requestFor(t, paymentFormModel([]runConditionModel{{Connector: types.StringValue("And")}}), paymentExtras{RequiredCondition: dashboardCondition()})
	if req.PaymentSettings.PaymentRequiredCondition != nil {
		t.Fatalf("condition not cleared: %#v", req.PaymentSettings.PaymentRequiredCondition)
	}
}

func TestRequestBlockReplacesTheServerCondition(t *testing.T) {
	block := []runConditionModel{{
		Connector: types.StringValue("Or"),
		Conditions: []conditionModel{{
			FieldName: types.StringValue("email"), Operator: types.StringValue("Equals"), Value: types.StringValue("vip"), CaseSensitive: types.BoolValue(false),
		}},
	}}
	req := requestFor(t, paymentFormModel(block), paymentExtras{RequiredCondition: dashboardCondition()})
	got := req.PaymentSettings.PaymentRequiredCondition
	if got == nil || len(got.Groups) != 1 || got.Groups[0].Connector != "Or" || got.Groups[0].Conditions[0].Value != "vip" {
		t.Fatalf("configured condition not sent: %#v", got)
	}
}

func TestRequestRuleConditionsAreSent(t *testing.T) {
	block := []runConditionModel{{
		Connector:  types.StringValue("And"),
		Conditions: []conditionModel{{FieldName: types.StringValue("email"), Operator: types.StringValue("NotEmpty"), Value: types.StringNull(), CaseSensitive: types.BoolValue(false)}},
	}}
	m := paymentFormModel(nil)
	m.Payment[0].FixedRule[0].Condition = block
	m.Payment[0].FieldAmountRule = []fieldAmountRuleModel{{AmountFieldName: types.StringValue("amount"), Condition: block}}
	m.Payment[0].LineItem = []lineItemModel{{FieldName: types.StringValue("size"), PriceMap: types.MapNull(types.Int64Type), Condition: block}}
	ps := requestFor(t, m, paymentExtras{}).PaymentSettings
	if ps.FixedRules[0].Condition == nil || ps.FieldAmountRules[0].Condition == nil || ps.LineItems[0].Condition == nil {
		t.Fatalf("rule conditions not sent: %#v", ps)
	}
	if ps.PaymentRequiredCondition != nil {
		t.Fatalf("unexpected required condition: %#v", ps.PaymentRequiredCondition)
	}
}

func TestRequestWithoutPaymentBlockKeepsDisabledSettings(t *testing.T) {
	disabled := &client.PaymentSettings{Enabled: false, Currency: "eur", Mode: "Fixed", PaymentRequiredCondition: dashboardCondition(), FixedRules: []client.FixedPaymentRule{{AmountCents: 700}}}
	req := requestFor(t, formResourceModel{Name: types.StringValue("form")}, paymentExtras{RequiredCondition: disabled.PaymentRequiredCondition, Disabled: disabled})
	if req.PaymentSettings != disabled {
		t.Fatalf("disabled settings not sent back: %#v", req.PaymentSettings)
	}
	if req := requestFor(t, formResourceModel{Name: types.StringValue("form")}, paymentExtras{}); req.PaymentSettings != nil {
		t.Fatalf("unexpected payment settings: %#v", req.PaymentSettings)
	}
}

func TestPaymentExtrasFromForm(t *testing.T) {
	enabled := &client.Form{PaymentSettings: &client.PaymentSettings{Enabled: true, PaymentRequiredCondition: dashboardCondition()}}
	if e := paymentExtrasFrom(enabled); e.RequiredCondition == nil || e.Disabled != nil {
		t.Fatalf("enabled settings: %#v", e)
	}
	disabled := &client.Form{PaymentSettings: &client.PaymentSettings{Enabled: false, Currency: "eur"}}
	if e := paymentExtrasFrom(disabled); e.Disabled == nil || e.Disabled.Currency != "eur" {
		t.Fatalf("disabled settings: %#v", e)
	}
	if e := paymentExtrasFrom(&client.Form{}); e.RequiredCondition != nil || e.Disabled != nil {
		t.Fatalf("no settings: %#v", e)
	}
}

type memoryPrivateState map[string][]byte

func (m memoryPrivateState) GetKey(_ context.Context, key string) ([]byte, diag.Diagnostics) {
	return m[key], nil
}

func (m memoryPrivateState) SetKey(_ context.Context, key string, value []byte) diag.Diagnostics {
	m[key] = value
	return nil
}

func TestPaymentExtrasSurvivePrivateState(t *testing.T) {
	ctx, store := context.Background(), memoryPrivateState{}
	if e, diags := loadPaymentExtras(ctx, store); diags.HasError() || e.RequiredCondition != nil || e.Disabled != nil {
		t.Fatalf("empty store: %#v %v", e, diags)
	}
	want := paymentExtras{RequiredCondition: dashboardCondition(), Disabled: &client.PaymentSettings{Currency: "eur", Mode: "Fixed"}}
	if diags := storePaymentExtras(ctx, store, want); diags.HasError() {
		t.Fatal(diags)
	}
	got, diags := loadPaymentExtras(ctx, store)
	if diags.HasError() || len(got.RequiredCondition.Groups) != 2 || got.RequiredCondition.Groups[1].Conditions[0].Value != "@vip.example" || got.Disabled.Currency != "eur" {
		t.Fatalf("extras not restored: %#v %v", got, diags)
	}
}

func TestApplyTracksTheRequiredConditionOnlyWhenManaged(t *testing.T) {
	form := func() *client.Form {
		conn := "conn-1"
		return &client.Form{ID: "form-1", Name: "form", PaymentSettings: &client.PaymentSettings{
			Enabled: true, ConnectionID: &conn, Currency: "eur", Mode: "Fixed",
			PaymentRequiredCondition: singleGroupCondition("vip"),
			FixedRules:               []client.FixedPaymentRule{{AmountCents: 1000}},
		}}
	}
	t.Run("unmanaged", func(t *testing.T) {
		plan := paymentFormModel(nil)
		if d := (&formResource{}).apply(context.Background(), form(), &plan); d.HasError() {
			t.Fatal(d)
		}
		if len(plan.Payment[0].PaymentRequiredCondition) != 0 {
			t.Fatalf("an unmanaged condition entered state: %#v", plan.Payment[0].PaymentRequiredCondition)
		}
	})
	t.Run("managed", func(t *testing.T) {
		prior := []runConditionModel{{Connector: types.StringValue("And"), Conditions: []conditionModel{{
			FieldName: types.StringValue("email"), Operator: types.StringValue("Equals"), Value: types.StringValue("vip"), CaseSensitive: types.BoolValue(false),
		}}}}
		plan := paymentFormModel(prior)
		if d := (&formResource{}).apply(context.Background(), form(), &plan); d.HasError() {
			t.Fatal(d)
		}
		got := plan.Payment[0].PaymentRequiredCondition
		if len(got) != 1 || got[0].Conditions[0].Value.ValueString() != "vip" {
			t.Fatalf("managed condition not mirrored: %#v", got)
		}
	})
	t.Run("explicit clear keeps the empty block", func(t *testing.T) {
		f := form()
		f.PaymentSettings.PaymentRequiredCondition = nil
		plan := paymentFormModel([]runConditionModel{{Connector: types.StringValue("And")}})
		if d := (&formResource{}).apply(context.Background(), f, &plan); d.HasError() {
			t.Fatal(d)
		}
		if got := plan.Payment[0].PaymentRequiredCondition; len(got) != 1 || len(got[0].Conditions) != 0 {
			t.Fatalf("empty block not kept: %#v", got)
		}
	})
	t.Run("managed condition removed on the server shows as drift", func(t *testing.T) {
		f := form()
		f.PaymentSettings.PaymentRequiredCondition = nil
		prior := []runConditionModel{{Connector: types.StringValue("And"), Conditions: []conditionModel{{
			FieldName: types.StringValue("email"), Operator: types.StringValue("Equals"), Value: types.StringValue("vip"), CaseSensitive: types.BoolValue(false),
		}}}}
		plan := paymentFormModel(prior)
		if d := (&formResource{}).apply(context.Background(), f, &plan); d.HasError() {
			t.Fatal(d)
		}
		if len(plan.Payment[0].PaymentRequiredCondition) != 0 {
			t.Fatalf("drift hidden: %#v", plan.Payment[0].PaymentRequiredCondition)
		}
	})
	t.Run("rule conditions are mirrored", func(t *testing.T) {
		f := form()
		f.PaymentSettings.FixedRules[0].Condition = singleGroupCondition("gold")
		plan := paymentFormModel(nil)
		if d := (&formResource{}).apply(context.Background(), f, &plan); d.HasError() {
			t.Fatal(d)
		}
		got := plan.Payment[0].FixedRule[0].Condition
		if len(got) != 1 || got[0].Conditions[0].Value.ValueString() != "gold" {
			t.Fatalf("rule condition not mirrored: %#v", got)
		}
	})
}

const fakePaymentRedirect = `
  redirect {
    success {
      type       = "CustomUrl"
      custom_url = "https://example.com/thanks"
    }
    error {
      type    = "InternalPage"
      message = "Payment failed"
    }
  }
`

func fakePaymentSettings(description, extra, ruleCondition string) string {
	return fmt.Sprintf(`%s
  payment {
    connection_id = "conn-1"
    currency      = "eur"
    mode          = "Fixed"
%s
    fixed_rule {
      amount_cents = 1000
      description  = %q
%s
    }
  }
`, fakePaymentRedirect, extra, description, ruleCondition)
}

const fakeVIPCondition = `
    payment_required_condition {
      connector = "And"
      condition {
        field_name = "email"
        operator   = "Equals"
        value      = "vip@example.com"
      }
    }
`

const fakeRuleCondition = `
      condition {
        condition {
          field_name = "email"
          operator   = "NotEmpty"
        }
      }
`

func TestFormResourcePaymentConditionFakeAPI(t *testing.T) {
	const res = "staticform_form.test"
	srv, api := newFakeFormAPI(t, false)

	serverSettings := func() *client.PaymentSettings {
		api.mu.Lock()
		defer api.mu.Unlock()
		for _, f := range api.forms {
			return f.PaymentSettings
		}
		return nil
	}
	setServerSettings := func(update func(*client.PaymentSettings)) {
		api.mu.Lock()
		defer api.mu.Unlock()
		for _, f := range api.forms {
			update(f.PaymentSettings)
		}
	}
	serverCondition := func(want *client.ConditionGroup) resource.TestCheckFunc {
		return func(*terraform.State) error {
			got := serverSettings().PaymentRequiredCondition
			switch {
			case want == nil && got != nil:
				return fmt.Errorf("server holds a condition: %#v", got)
			case want == nil:
				return nil
			case got == nil:
				return fmt.Errorf("server condition was dropped")
			case got.Connector != want.Connector || len(got.Groups) != len(want.Groups):
				return fmt.Errorf("server condition changed: %#v", got)
			}
			return nil
		}
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fakeAPIFormConfig(srv.URL, fakePaymentSettings("one", fakeVIPCondition, fakeRuleCondition)),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(res, "payment.0.payment_required_condition.0.connector", "And"),
					resource.TestCheckResourceAttr(res, "payment.0.payment_required_condition.0.condition.0.field_name", "email"),
					resource.TestCheckResourceAttr(res, "payment.0.payment_required_condition.0.condition.0.value", "vip@example.com"),
					resource.TestCheckResourceAttr(res, "payment.0.fixed_rule.0.condition.0.condition.0.operator", "NotEmpty"),
					serverCondition(singleGroupCondition("vip@example.com")),
					func(*terraform.State) error {
						if serverSettings().FixedRules[0].Condition == nil {
							return fmt.Errorf("rule condition was not sent")
						}
						return nil
					},
				),
			},
			{
				PreConfig: func() {
					setServerSettings(func(p *client.PaymentSettings) { p.PaymentRequiredCondition = dashboardCondition() })
				},
				Config: fakeAPIFormConfig(srv.URL, fakePaymentSettings("two", "", fakeRuleCondition)),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(res, "payment.0.fixed_rule.0.description", "two"),
					resource.TestCheckResourceAttr(res, "payment.0.payment_required_condition.#", "0"),
					serverCondition(dashboardCondition()),
				),
			},
			{
				PreConfig: func() {
					setServerSettings(func(p *client.PaymentSettings) { p.PaymentRequiredCondition = singleGroupCondition("changed") })
				},
				Config: fakeAPIFormConfig(srv.URL, fakePaymentSettings("three", "", fakeRuleCondition)),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(res, "payment.0.fixed_rule.0.description", "three"),
					serverCondition(singleGroupCondition("changed")),
				),
			},
			{
				Config: fakeAPIFormConfig(srv.URL, fakePaymentSettings("four", "    payment_required_condition {}\n", fakeRuleCondition)),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(res, "payment.0.payment_required_condition.#", "1"),
					resource.TestCheckResourceAttr(res, "payment.0.payment_required_condition.0.condition.#", "0"),
					serverCondition(nil),
				),
			},
			{
				Config: fakeAPIFormConfig(srv.URL, fakePaymentSettings("five", fakeVIPCondition, fakeRuleCondition)),
				Check:  serverCondition(singleGroupCondition("vip@example.com")),
			},
			{
				PreConfig: func() {
					setServerSettings(func(p *client.PaymentSettings) { p.PaymentRequiredCondition = nil })
				},
				Config: fakeAPIFormConfig(srv.URL, fakePaymentSettings("five", fakeVIPCondition, fakeRuleCondition)),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(res, "payment.0.payment_required_condition.0.condition.0.value", "vip@example.com"),
					serverCondition(singleGroupCondition("vip@example.com")),
				),
			},
			{
				ResourceName:            res,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"captcha_secret_key", "payment.0.payment_required_condition"},
			},
			{
				PreConfig: func() {
					setServerSettings(func(p *client.PaymentSettings) {
						p.Enabled = false
						p.PaymentRequiredCondition = dashboardCondition()
					})
				},
				Config: fakeAPIFormConfig(srv.URL, "  enable_honeypot = false\n"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(res, "payment.#", "0"),
					resource.TestCheckResourceAttr(res, "enable_honeypot", "false"),
					func(*terraform.State) error {
						p := serverSettings()
						if p == nil || p.Enabled || p.Currency != "eur" || len(p.FixedRules) != 1 {
							return fmt.Errorf("disabled payment settings were dropped: %#v", p)
						}
						return serverCondition(dashboardCondition())(nil)
					},
				),
			},
			{
				Config: fakeAPIFormConfig(srv.URL, fakePaymentSettings("six", "", "")),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(res, "payment.0.fixed_rule.0.description", "six"),
					serverCondition(dashboardCondition()),
				),
			},
			{
				Config: fakeAPIFormConfig(srv.URL, fakePaymentSettings("six", `
    payment_required_condition {
      condition {
        field_name = "email"
        operator   = "Nope"
      }
    }
`, "")),
				ExpectError: regexp.MustCompile("Equals"),
			},
			{
				Config: fakeAPIFormConfig(srv.URL, ""),
				Check:  resource.TestCheckResourceAttr(res, "payment.#", "0"),
			},
		},
	})
}
