package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

const formWithPaymentCondition = `{"id":"form-1","name":"f","fields":[],"submitActions":[],"captchaType":"None","enableHoneypot":false,"honeypotFieldName":"","enableLanguageDetection":false,"expectedLanguages":[],"languageDetectionFields":[],` +
	`"paymentSettings":{"enabled":true,"connectionId":"conn-1","currency":"eur","mode":"Fixed",` +
	`"paymentRequiredCondition":{"connector":"Or","groups":[{"connector":"And","conditions":[{"fieldName":"plan","operator":"Equals","value":"pro","caseSensitive":true}]},{"connector":"Or","conditions":[{"fieldName":"vip","operator":"NotEmpty","value":null,"caseSensitive":false}]}]},` +
	`"fixedRules":[{"condition":null,"amountCents":900,"description":null}],"fieldAmountRules":null,"lineItems":null,"customerEmailFieldName":null},"submissionMode":"ClientSide","tags":[]}`

func TestGetFormDecodesPaymentRequiredCondition(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, formWithPaymentCondition)
	}))
	defer srv.Close()
	f, err := New(srv.URL, "key").GetForm(context.Background(), "form-1")
	if err != nil {
		t.Fatal(err)
	}
	c := f.PaymentSettings.PaymentRequiredCondition
	if c == nil || c.Connector != "Or" || len(c.Groups) != 2 || c.Groups[0].Conditions[0].Value != "pro" || !c.Groups[0].Conditions[0].CaseSensitive || c.Groups[1].Connector != "Or" {
		t.Fatalf("payment required condition not decoded: %#v", c)
	}
}

func TestGetFormWithoutPaymentRequiredCondition(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":"form-1","name":"f","paymentSettings":{"enabled":true,"connectionId":null,"currency":"eur","mode":"Fixed","paymentRequiredCondition":null,"fixedRules":[],"fieldAmountRules":null,"lineItems":null,"customerEmailFieldName":null}}`)
	}))
	defer srv.Close()
	f, err := New(srv.URL, "key").GetForm(context.Background(), "form-1")
	if err != nil || f.PaymentSettings.PaymentRequiredCondition != nil {
		t.Fatalf("null condition not decoded: %#v %v", f, err)
	}
}

func TestUpdateFormSendsPaymentRequiredCondition(t *testing.T) {
	var sent map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/v1/forms/form-1" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
		}
		_ = json.NewDecoder(r.Body).Decode(&sent)
		fmt.Fprint(w, formWithPaymentCondition)
	}))
	defer srv.Close()
	condition := &ConditionGroup{Connector: "Or", Groups: []ConditionSubGroup{
		{Connector: "And", Conditions: []Condition{{FieldName: "plan", Operator: "Equals", Value: "pro", CaseSensitive: true}}},
		{Connector: "Or", Conditions: []Condition{{FieldName: "vip", Operator: "NotEmpty"}}},
	}}
	_, err := New(srv.URL, "key").UpdateForm(context.Background(), "form-1", FormRequest{
		Name:            "f",
		PaymentSettings: &PaymentSettings{Enabled: true, Currency: "eur", Mode: "Fixed", PaymentRequiredCondition: condition},
	})
	if err != nil {
		t.Fatal(err)
	}
	payment, _ := sent["paymentSettings"].(map[string]any)
	got, _ := payment["paymentRequiredCondition"].(map[string]any)
	groups, _ := got["groups"].([]any)
	if got["connector"] != "Or" || len(groups) != 2 {
		t.Fatalf("condition not sent: %v", payment)
	}
	first, _ := groups[0].(map[string]any)
	conditions, _ := first["conditions"].([]any)
	condition0, _ := conditions[0].(map[string]any)
	if condition0["fieldName"] != "plan" || condition0["value"] != "pro" || condition0["caseSensitive"] != true {
		t.Fatalf("condition body not sent: %v", condition0)
	}
}

func TestUpdateFormOmitsAClearedPaymentRequiredCondition(t *testing.T) {
	var sent map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&sent)
		fmt.Fprint(w, formWithPaymentCondition)
	}))
	defer srv.Close()
	_, err := New(srv.URL, "key").UpdateForm(context.Background(), "form-1", FormRequest{
		Name:            "f",
		PaymentSettings: &PaymentSettings{Enabled: true, Currency: "eur", Mode: "Fixed"},
	})
	if err != nil {
		t.Fatal(err)
	}
	payment, _ := sent["paymentSettings"].(map[string]any)
	if _, present := payment["paymentRequiredCondition"]; present {
		t.Fatalf("a cleared condition must not be sent: %v", payment)
	}
}
