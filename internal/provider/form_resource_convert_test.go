package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/resoftware/terraform-provider-staticform/internal/client"
)

func TestBoolFromResponse(t *testing.T) {
	tru, fls := true, false
	cases := []struct {
		name     string
		resp     *bool
		prior    types.Bool
		fallback types.Bool
		want     types.Bool
	}{
		{"returned value wins over prior", &fls, types.BoolValue(true), types.BoolNull(), types.BoolValue(false)},
		{"returned value resolves unknown", &tru, types.BoolUnknown(), types.BoolNull(), types.BoolValue(true)},
		{"absent keeps known prior", nil, types.BoolValue(false), types.BoolNull(), types.BoolValue(false)},
		{"absent resolves unknown to fallback", nil, types.BoolUnknown(), types.BoolNull(), types.BoolNull()},
		{"absent resolves null to fallback", nil, types.BoolNull(), types.BoolValue(false), types.BoolValue(false)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := boolFromResponse(c.resp, c.prior, c.fallback); !got.Equal(c.want) {
				t.Errorf("got %s, want %s", got, c.want)
			}
		})
	}
}

func TestListFromResponse(t *testing.T) {
	empty := []string{}
	some := []string{"email", "phone"}
	emptyList := toStringList([]string{})
	cases := []struct {
		name  string
		resp  *[]string
		prior types.List
		want  types.List
	}{
		{"absent keeps known prior", nil, toStringList([]string{"email"}), toStringList([]string{"email"})},
		{"absent keeps null prior", nil, types.ListNull(types.StringType), types.ListNull(types.StringType)},
		{"absent resolves unknown to null", nil, types.ListUnknown(types.StringType), types.ListNull(types.StringType)},
		{"empty response with null prior is null", &empty, types.ListNull(types.StringType), types.ListNull(types.StringType)},
		{"empty response with unknown prior is null", &empty, types.ListUnknown(types.StringType), types.ListNull(types.StringType)},
		{"empty response keeps explicit empty prior", &empty, emptyList, emptyList},
		{"empty response replaces non-empty prior", &empty, toStringList([]string{"email"}), types.ListNull(types.StringType)},
		{"returned values win", &some, types.ListNull(types.StringType), toStringList(some)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := listFromResponse(c.resp, c.prior); !got.Equal(c.want) {
				t.Errorf("got %s, want %s", got, c.want)
			}
		})
	}
}

func TestExplicitList(t *testing.T) {
	if got := explicitList(nil); got == nil || len(*got) != 0 {
		t.Errorf("nil input: got %v, want pointer to empty slice", got)
	}
	if got := explicitList([]string{"a"}); got == nil || len(*got) != 1 || (*got)[0] != "a" {
		t.Errorf("non-empty input: got %v", got)
	}
}

func TestNullIfEmptyPtr(t *testing.T) {
	empty, val := "", "x"
	cases := []struct {
		name string
		in   *string
		want types.String
	}{
		{"nil is null", nil, types.StringNull()},
		{"empty is null", &empty, types.StringNull()},
		{"value is kept", &val, types.StringValue("x")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := nullIfEmptyPtr(c.in); !got.Equal(c.want) {
				t.Errorf("got %s, want %s", got, c.want)
			}
		})
	}
}

func TestKeepEmptyPrior(t *testing.T) {
	empty := types.StringValue("")
	null := types.StringNull()
	cases := []struct {
		name  string
		got   types.String
		prior types.String
		want  types.String
	}{
		{"null response keeps empty prior", null, empty, empty},
		{"empty response keeps empty prior", empty, empty, empty},
		{"empty response keeps null prior", empty, null, null},
		{"null response with null prior", null, null, null},
		{"null response ignores unknown prior", null, types.StringUnknown(), null},
		{"null response replaces non-empty prior", null, types.StringValue("old"), null},
		{"returned value wins over empty prior", types.StringValue("new"), empty, types.StringValue("new")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.got
			keepEmptyPrior(&got, c.prior)
			if !got.Equal(c.want) {
				t.Errorf("got %s, want %s", got, c.want)
			}
		})
	}
}

func TestActionToModelNormalizesEmptyStrings(t *testing.T) {
	empty := ""
	m := actionToModel(client.SubmitAction{
		Type:                    client.ActionSendEmail,
		Name:                    "notify",
		Recipient:               "team@example.com",
		ReplyTo:                 &empty,
		Subject:                 &empty,
		BodyBuilderState:        &empty,
		EmailDomainID:           &empty,
		SMTPConnectionID:        &empty,
		FromEmail:               &empty,
		FromName:                &empty,
		GoogleOAuthConnectionID: &empty,
		NotionConnectionID:      &empty,
		CustomEmailTemplate:     &client.EmailTemplateCustomization{HeaderText: &empty},
	})
	for name, v := range map[string]types.String{
		"reply_to":                   m.ReplyTo,
		"subject":                    m.Subject,
		"body_builder_state":         m.BodyBuilderState,
		"email_domain_id":            m.EmailDomainID,
		"smtp_connection_id":         m.SMTPConnectionID,
		"from_email":                 m.FromEmail,
		"from_name":                  m.FromName,
		"google_connection_id":       m.GoogleConnectionID,
		"notion_connection_id":       m.NotionConnectionID,
		"email_template.header_text": m.EmailTemplate[0].HeaderText,
	} {
		if !v.IsNull() {
			t.Errorf("%s: got %s, want null", name, v)
		}
	}
}

// TestApplyKeepsEmptyStringsFromPlan covers the "inconsistent result after
// apply" case: the plan holds "" while the API returns null, with the response
// in a different order than the plan so prior actions must be matched by ID.
func TestApplyKeepsEmptyStringsFromPlan(t *testing.T) {
	empty := types.StringValue("")
	null := types.StringNull()
	plan := formResourceModel{
		SubmitActions: []submitActionModel{
			{
				ID:        types.StringValue("a1"),
				Type:      types.StringValue(client.ActionSendEmail),
				Name:      types.StringValue("first"),
				Recipient: types.StringValue("team@example.com"),
				Subject:   empty,
				ReplyTo:   empty,
				EmailTemplate: []emailTemplateModel{{
					HeaderText: empty,
					FooterText: null,
				}},
				RunCondition: []runConditionModel{{
					Connector:  types.StringValue("And"),
					Conditions: []conditionModel{{FieldName: types.StringValue("email"), Operator: types.StringValue("NotEmpty"), Value: empty}},
				}},
			},
			{
				ID:        types.StringValue("a2"),
				Type:      types.StringValue(client.ActionSendEmail),
				Name:      types.StringValue("second"),
				Recipient: types.StringValue("other@example.com"),
				Subject:   null,
				ReplyTo:   types.StringValue("old@example.com"),
			},
		},
	}
	a1, a2 := "a1", "a2"
	resp := &client.Form{
		ID:   "form-1",
		Name: "form",
		SubmitActions: []client.SubmitAction{
			{ID: &a2, Type: client.ActionSendEmail, Name: "second", Recipient: "other@example.com"},
			{
				ID:                  &a1,
				Type:                client.ActionSendEmail,
				Name:                "first",
				Recipient:           "team@example.com",
				CustomEmailTemplate: &client.EmailTemplateCustomization{},
				RunCondition: &client.ConditionGroup{Connector: "And", Groups: []client.ConditionSubGroup{{
					Connector:  "And",
					Conditions: []client.Condition{{FieldName: "email", Operator: "NotEmpty"}},
				}}},
			},
		},
	}

	if d := (&formResource{}).apply(context.Background(), resp, &plan); d.HasError() {
		t.Fatalf("apply: %v", d)
	}
	first, second := plan.SubmitActions[0], plan.SubmitActions[1]
	checks := []struct {
		name      string
		got, want types.String
	}{
		{"first.subject", first.Subject, empty},
		{"first.reply_to", first.ReplyTo, empty},
		{"first.email_template.header_text", first.EmailTemplate[0].HeaderText, empty},
		{"first.email_template.footer_text", first.EmailTemplate[0].FooterText, null},
		{"first.run_condition.value", first.RunCondition[0].Conditions[0].Value, empty},
		{"second.subject", second.Subject, null},
		{"second.reply_to", second.ReplyTo, null},
	}
	for _, c := range checks {
		if !c.got.Equal(c.want) {
			t.Errorf("%s: got %s, want %s", c.name, c.got, c.want)
		}
	}
}

func TestOrderSubmitActionsPriorIndex(t *testing.T) {
	a1, a3 := "a1", "a3"
	prior := []submitActionModel{
		{ID: types.StringValue("a1"), Name: types.StringValue("one")},
		{ID: types.StringNull(), Name: types.StringValue("two")},
	}
	incoming := []client.SubmitAction{
		{ID: &a3, Name: "three"},
		{Name: "two"},
		{ID: &a1, Name: "one"},
	}
	got, idx := orderSubmitActions(prior, incoming)
	wantNames, wantIdx := []string{"one", "two", "three"}, []int{0, 1, -1}
	for i := range wantNames {
		if got[i].Name != wantNames[i] || idx[i] != wantIdx[i] {
			t.Errorf("position %d: got (%s, %d), want (%s, %d)", i, got[i].Name, idx[i], wantNames[i], wantIdx[i])
		}
	}
}

// TestApplyKeepsEmptyRedirectAndPaymentStrings covers the redirect and payment
// blocks, where the provider also sends "" as null.
func TestApplyKeepsEmptyRedirectAndPaymentStrings(t *testing.T) {
	empty := types.StringValue("")
	null := types.StringNull()
	plan := formResourceModel{
		Redirect: []redirectModel{{
			Success: []redirectBranchModel{{Type: types.StringValue("CustomUrl"), CustomURL: types.StringValue("https://example.com"), Message: empty}},
			Error:   []redirectBranchModel{{Type: types.StringValue("InternalPage"), CustomURL: empty, Message: null}},
		}},
		Payment: []paymentModel{{
			ConnectionID:           types.StringValue("conn-1"),
			Currency:               types.StringValue("eur"),
			Mode:                   types.StringValue("LineItems"),
			CustomerEmailFieldName: empty,
			FixedRule:              []fixedRuleModel{{AmountCents: types.Int64Value(100), Description: empty}},
			FieldAmountRule:        []fieldAmountRuleModel{{AmountFieldName: types.StringValue("amount"), Description: empty}},
			LineItem:               []lineItemModel{{FieldName: types.StringValue("size"), QuantityFieldName: empty, Description: types.StringValue("old")}},
		}},
	}
	url, conn, blank := "https://example.com", "conn-1", ""
	resp := &client.Form{
		ID:   "form-1",
		Name: "form",
		RedirectSettings: &client.RedirectSettings{
			Success: &client.RedirectConfig{Type: "CustomUrl", CustomURL: &url},
			Error:   &client.RedirectConfig{Type: "InternalPage", Message: &blank},
		},
		PaymentSettings: &client.PaymentSettings{
			Enabled:          true,
			ConnectionID:     &conn,
			Currency:         "eur",
			Mode:             "LineItems",
			FixedRules:       []client.FixedPaymentRule{{AmountCents: 100}},
			FieldAmountRules: []client.FieldAmountPaymentRule{{AmountFieldName: "amount", Description: &blank}},
			LineItems:        []client.PaymentLineItem{{FieldName: "size", PriceMap: map[string]int64{"S": 100}}},
		},
	}

	if d := (&formResource{}).apply(context.Background(), resp, &plan); d.HasError() {
		t.Fatalf("apply: %v", d)
	}
	rd, pm := plan.Redirect[0], plan.Payment[0]
	checks := []struct {
		name      string
		got, want types.String
	}{
		{"redirect.success.message", rd.Success[0].Message, empty},
		{"redirect.error.custom_url", rd.Error[0].CustomURL, empty},
		{"redirect.error.message", rd.Error[0].Message, null},
		{"payment.customer_email_field_name", pm.CustomerEmailFieldName, empty},
		{"payment.fixed_rule.description", pm.FixedRule[0].Description, empty},
		{"payment.field_amount_rule.description", pm.FieldAmountRule[0].Description, empty},
		{"payment.line_item.quantity_field_name", pm.LineItem[0].QuantityFieldName, empty},
		{"payment.line_item.description", pm.LineItem[0].Description, null},
	}
	for _, c := range checks {
		if !c.got.Equal(c.want) {
			t.Errorf("%s: got %s, want %s", c.name, c.got, c.want)
		}
	}
}

func TestRedirectAndPaymentToModelNormalizeEmptyStrings(t *testing.T) {
	empty := ""
	rd := redirectBranchToModel(&client.RedirectConfig{Type: "InternalPage", CustomURL: &empty, Message: &empty})
	pm := paymentToModel(&client.PaymentSettings{
		CustomerEmailFieldName: &empty,
		FixedRules:             []client.FixedPaymentRule{{Description: &empty}},
		FieldAmountRules:       []client.FieldAmountPaymentRule{{Description: &empty}},
		LineItems:              []client.PaymentLineItem{{QuantityFieldName: &empty, Description: &empty}},
	})
	for name, v := range map[string]types.String{
		"redirect.custom_url":                   rd.CustomURL,
		"redirect.message":                      rd.Message,
		"payment.customer_email_field_name":     pm.CustomerEmailFieldName,
		"payment.fixed_rule.description":        pm.FixedRule[0].Description,
		"payment.field_amount_rule.description": pm.FieldAmountRule[0].Description,
		"payment.line_item.quantity_field_name": pm.LineItem[0].QuantityFieldName,
		"payment.line_item.description":         pm.LineItem[0].Description,
	} {
		if !v.IsNull() {
			t.Errorf("%s: got %s, want null", name, v)
		}
	}
}
