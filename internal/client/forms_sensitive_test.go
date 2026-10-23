package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSensitiveFormWireContract(t *testing.T) {
	for _, confirmed := range []bool{false, true} {
		t.Run(map[bool]string{false: "unconfirmed", true: "confirmed"}[confirmed], func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if got, _ := body["confirmSensitiveFields"].(bool); got != confirmed {
					t.Errorf("confirmation=%v", got)
				}
				field := body["fields"].([]any)[0].(map[string]any)
				if field["sensitive"] != true {
					t.Errorf("field=%v", field)
				}
				actions := body["submitActions"].([]any)
				for _, a := range actions {
					names := a.(map[string]any)["sensitiveFields"].([]any)
					if len(names) != 1 || names[0] != "bsn" {
						t.Errorf("approval=%v", names)
					}
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"id": "form-1", "fields": body["fields"], "submitActions": body["submitActions"]})
			}))
			defer server.Close()
			c := New(server.URL, "test-key")
			sensitive := true
			request := FormRequest{Name: "Protected", ConfirmSensitiveFields: confirmed, Fields: []Field{{Name: "bsn", Type: "Text", Sensitive: &sensitive}}}
			for _, kind := range []string{ActionSendEmail, ActionTriggerWebhook, ActionSyncGoogleSheets, ActionSyncNotion} {
				request.SubmitActions = append(request.SubmitActions, SubmitAction{Type: kind, SensitiveFields: []string{"bsn"}})
			}
			for _, method := range []string{"create", "update"} {
				var form *Form
				var err error
				if method == "create" {
					form, err = c.CreateForm(context.Background(), request)
				} else {
					form, err = c.UpdateForm(context.Background(), "form-1", request)
				}
				if err != nil {
					t.Fatal(err)
				}
				if form.Fields[0].Sensitive == nil || !*form.Fields[0].Sensitive || form.SubmitActions[0].SensitiveFields[0] != "bsn" {
					t.Fatalf("response lost protection: %#v", form)
				}
			}
		})
	}
}
