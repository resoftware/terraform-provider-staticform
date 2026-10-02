package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/resoftware/terraform-provider-staticform/internal/client"
)

// fakeFormAPI is a minimal in-memory stand-in for the StaticForm forms API.
// With legacy set it behaves like an API version that predates the
// allowed-domain and AI spam review settings: it ignores them on requests and
// omits them from responses.
type fakeFormAPI struct {
	legacy bool
	mu     sync.Mutex
	forms  map[string]*client.Form
	nextID int
}

func newFakeFormAPI(t *testing.T, legacy bool) (*httptest.Server, *fakeFormAPI) {
	api := &fakeFormAPI{legacy: legacy, forms: map[string]*client.Form{}}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/forms", api.create)
	mux.HandleFunc("GET /api/v1/forms/{id}", api.get)
	mux.HandleFunc("PUT /api/v1/forms/{id}", api.update)
	mux.HandleFunc("DELETE /api/v1/forms/{id}", api.delete)
	mux.HandleFunc("PATCH /api/v1/forms/{id}/tags", api.tags)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, api
}

func orEmpty(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

func (a *fakeFormAPI) assignActionIDs(actions []client.SubmitAction) []client.SubmitAction {
	for i := range actions {
		if actions[i].ID == nil {
			a.nextID++
			id := fmt.Sprintf("action-%d", a.nextID)
			actions[i].ID = &id
		}
	}
	return orActions(actions)
}

func orActions(in []client.SubmitAction) []client.SubmitAction {
	if in == nil {
		return []client.SubmitAction{}
	}
	return in
}

func (a *fakeFormAPI) create(w http.ResponseWriter, r *http.Request) {
	var req client.FormRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.nextID++
	f := &client.Form{
		ID:                      fmt.Sprintf("form-%d", a.nextID),
		Name:                    req.Name,
		Fields:                  req.Fields,
		CaptchaType:             "None",
		HoneypotFieldName:       "website_url",
		ExpectedLanguages:       orEmpty(req.ExpectedLanguages),
		LanguageDetectionFields: orEmpty(req.LanguageDetectionFields),
		SubmissionMode:          "ClientSide",
		Tags:                    []string{},
	}
	f.SubmitActions = a.assignActionIDs(req.SubmitActions)
	f.RedirectSettings, f.PaymentSettings = req.RedirectSettings, req.PaymentSettings
	if req.EnableHoneypot != nil {
		f.EnableHoneypot = *req.EnableHoneypot
	}
	if req.EnableLanguageDetection != nil {
		f.EnableLanguageDetection = *req.EnableLanguageDetection
	}
	if req.SubmissionMode != nil {
		f.SubmissionMode = *req.SubmissionMode
	}
	if !a.legacy {
		enableDomains, enableAI := false, true
		domains, excluded := []string{}, []string{}
		if req.EnableAllowedDomains != nil {
			enableDomains = *req.EnableAllowedDomains
		}
		if req.AllowedDomains != nil {
			domains = orEmpty(*req.AllowedDomains)
		}
		if req.EnableAiSpamReview != nil {
			enableAI = *req.EnableAiSpamReview
		}
		if req.AiSpamReviewExcludedFields != nil {
			excluded = orEmpty(*req.AiSpamReviewExcludedFields)
		}
		f.EnableAllowedDomains, f.AllowedDomains = &enableDomains, &domains
		f.EnableAiSpamReview, f.AiSpamReviewExcludedFields = &enableAI, &excluded
	}
	a.forms[f.ID] = f
	a.write(w, f)
}

func (a *fakeFormAPI) get(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	f, ok := a.forms[r.PathValue("id")]
	if !ok {
		http.NotFound(w, r)
		return
	}
	a.write(w, f)
}

func (a *fakeFormAPI) update(w http.ResponseWriter, r *http.Request) {
	var req client.FormRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	f, ok := a.forms[r.PathValue("id")]
	if !ok {
		http.NotFound(w, r)
		return
	}
	f.Name = req.Name
	f.Fields = req.Fields
	f.SubmitActions = a.assignActionIDs(req.SubmitActions)
	f.RedirectSettings, f.PaymentSettings = req.RedirectSettings, req.PaymentSettings
	if !a.legacy {
		if req.EnableAllowedDomains != nil {
			v := *req.EnableAllowedDomains
			f.EnableAllowedDomains = &v
		}
		if req.AllowedDomains != nil {
			v := orEmpty(*req.AllowedDomains)
			f.AllowedDomains = &v
		}
		if req.EnableAiSpamReview != nil {
			v := *req.EnableAiSpamReview
			f.EnableAiSpamReview = &v
		}
		if req.AiSpamReviewExcludedFields != nil {
			v := orEmpty(*req.AiSpamReviewExcludedFields)
			f.AiSpamReviewExcludedFields = &v
		}
	}
	a.write(w, f)
}

func (a *fakeFormAPI) delete(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.forms, r.PathValue("id"))
	w.WriteHeader(http.StatusNoContent)
}

func (a *fakeFormAPI) tags(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Tags []string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	f, ok := a.forms[r.PathValue("id")]
	if !ok {
		http.NotFound(w, r)
		return
	}
	f.Tags = orEmpty(req.Tags)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"formId": f.ID, "tags": f.Tags})
}

func (a *fakeFormAPI) write(w http.ResponseWriter, f *client.Form) {
	raw, _ := json.Marshal(f)
	if a.legacy {
		var m map[string]any
		_ = json.Unmarshal(raw, &m)
		for _, k := range []string{"enableAllowedDomains", "allowedDomains", "enableAiSpamReview", "aiSpamReviewExcludedFields"} {
			delete(m, k)
		}
		raw, _ = json.Marshal(m)
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(raw)
}

func fakeAPIFormConfig(baseURL, settings string) string {
	return fmt.Sprintf(`
provider "staticform" {
  base_url = %q
  api_key  = "sf_test_fake"
}

resource "staticform_form" "test" {
  name = "fake-api-form"
%s
  field {
    name = "email"
    type = "Text"
    rule = "Email"
  }
}

data "staticform_form" "test" {
  id         = staticform_form.test.id
  depends_on = [staticform_form.test]
}
`, baseURL, settings)
}

const fakeAPISettings = `
  enable_allowed_domains         = true
  allowed_domains                = ["example.com", "example.org"]
  enable_ai_spam_review          = false
  ai_spam_review_excluded_fields = ["email"]
`

const fakeAPIEmptyStrings = `
  submit_action {
    type          = "SendEmailAction"
    name          = "notify"
    recipient     = "team@example.com"
    subject       = ""
    reply_to      = ""
    body_template = "{{form.email}}"

    email_template {
      header_text = ""
      footer_text = "Thanks"
    }

    run_condition {
      condition {
        field_name = "email"
        operator   = "NotEmpty"
        value      = ""
      }
    }
  }

  redirect {
    success {
      type       = "CustomUrl"
      custom_url = "https://example.com/thanks"
      message    = ""
    }
    error {
      type       = "InternalPage"
      custom_url = ""
      message    = "Payment failed"
    }
  }

  payment {
    connection_id             = "conn-1"
    currency                  = "eur"
    mode                      = "Fixed"
    customer_email_field_name = ""

    fixed_rule {
      amount_cents = 1000
      description  = ""
    }
  }
`

// TestFormResourceFakeAPI runs plan/apply/refresh cycles for the allowed-domain
// and AI spam review settings against an in-process fake API, both one that
// supports them and a legacy one that omits them. It needs no credentials, but
// like other terraform-plugin-testing tests only runs with TF_ACC set.
func TestFormResourceFakeAPI(t *testing.T) {
	const res = "staticform_form.test"
	const ds = "data.staticform_form.test"

	t.Run("current", func(t *testing.T) {
		srv, _ := newFakeFormAPI(t, false)
		resource.Test(t, resource.TestCase{
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			Steps: []resource.TestStep{
				{
					Config: fakeAPIFormConfig(srv.URL, `
  enable_allowed_domains = true
`),
					ExpectError: regexp.MustCompile("Allowed domains required"),
				},
				{
					Config: fakeAPIFormConfig(srv.URL, `
  allowed_domains = ["https://Example.com/contact"]
`),
					ExpectError: regexp.MustCompile("must be a bare lowercase hostname"),
				},
				{
					// Unset: the server default (AI review on) is adopted without a diff.
					Config: fakeAPIFormConfig(srv.URL, ""),
					Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckResourceAttr(res, "enable_ai_spam_review", "true"),
						resource.TestCheckNoResourceAttr(res, "ai_spam_review_excluded_fields"),
						resource.TestCheckResourceAttr(res, "enable_allowed_domains", "false"),
						resource.TestCheckNoResourceAttr(res, "allowed_domains"),
						resource.TestCheckResourceAttr(ds, "enable_ai_spam_review", "true"),
						resource.TestCheckResourceAttr(ds, "ai_spam_review_excluded_fields.#", "0"),
					),
				},
				{
					Config: fakeAPIFormConfig(srv.URL, fakeAPISettings),
					Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckResourceAttr(res, "enable_ai_spam_review", "false"),
						resource.TestCheckResourceAttr(res, "ai_spam_review_excluded_fields.#", "1"),
						resource.TestCheckResourceAttr(res, "ai_spam_review_excluded_fields.0", "email"),
						resource.TestCheckResourceAttr(res, "enable_allowed_domains", "true"),
						resource.TestCheckResourceAttr(res, "allowed_domains.#", "2"),
						resource.TestCheckResourceAttr(ds, "enable_ai_spam_review", "false"),
						resource.TestCheckResourceAttr(ds, "ai_spam_review_excluded_fields.0", "email"),
					),
				},
				{
					ResourceName:            res,
					ImportState:             true,
					ImportStateVerify:       true,
					ImportStateVerifyIgnore: []string{"captcha_secret_key"},
				},
				{
					// Explicit empty list round-trips.
					Config: fakeAPIFormConfig(srv.URL, `
  ai_spam_review_excluded_fields = []
`),
					Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckResourceAttr(res, "ai_spam_review_excluded_fields.#", "0"),
						resource.TestCheckResourceAttr(res, "enable_ai_spam_review", "false"),
					),
				},
				{
					// Removing everything: lists and the allowed-domain toggle are
					// cleared, the AI toggle keeps its last server value.
					Config: fakeAPIFormConfig(srv.URL, ""),
					Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckResourceAttr(res, "enable_ai_spam_review", "false"),
						resource.TestCheckNoResourceAttr(res, "ai_spam_review_excluded_fields"),
						resource.TestCheckResourceAttr(res, "enable_allowed_domains", "false"),
						resource.TestCheckNoResourceAttr(res, "allowed_domains"),
					),
				},
			},
		})
	})

	t.Run("empty strings", func(t *testing.T) {
		srv, api := newFakeFormAPI(t, false)
		resource.Test(t, resource.TestCase{
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			Steps: []resource.TestStep{
				{
					// The provider sends "" as null and the API echoes null; the
					// configured "" must survive apply and refresh without a diff.
					Config: fakeAPIFormConfig(srv.URL, fakeAPIEmptyStrings),
					Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckResourceAttr(res, "submit_action.0.subject", ""),
						resource.TestCheckResourceAttr(res, "submit_action.0.reply_to", ""),
						resource.TestCheckResourceAttr(res, "submit_action.0.email_template.0.header_text", ""),
						resource.TestCheckResourceAttr(res, "submit_action.0.run_condition.0.condition.0.value", ""),
						resource.TestCheckResourceAttr(res, "redirect.0.success.0.message", ""),
						resource.TestCheckResourceAttr(res, "redirect.0.error.0.custom_url", ""),
						resource.TestCheckResourceAttr(res, "payment.0.customer_email_field_name", ""),
						resource.TestCheckResourceAttr(res, "payment.0.fixed_rule.0.description", ""),
					),
				},
				{
					// A form saved from the dashboard can store "" instead of null.
					// Import must read those back as null, so generated config does
					// not contain `subject = ""`.
					PreConfig: func() {
						api.mu.Lock()
						defer api.mu.Unlock()
						empty := ""
						for _, f := range api.forms {
							for i := range f.SubmitActions {
								f.SubmitActions[i].Subject = &empty
								f.SubmitActions[i].ReplyTo = &empty
								f.SubmitActions[i].CustomEmailTemplate.HeaderText = &empty
							}
							f.RedirectSettings.Success.Message = &empty
							f.RedirectSettings.Error.CustomURL = &empty
							f.PaymentSettings.CustomerEmailFieldName = &empty
							f.PaymentSettings.FixedRules[0].Description = &empty
						}
					},
					ResourceName: res,
					ImportState:  true,
					ImportStateCheck: func(states []*terraform.InstanceState) error {
						if len(states) != 1 {
							return fmt.Errorf("expected 1 imported state, got %d", len(states))
						}
						for _, k := range []string{
							"submit_action.0.subject",
							"submit_action.0.reply_to",
							"submit_action.0.email_template.0.header_text",
							"submit_action.0.run_condition.0.condition.0.value",
							"redirect.0.success.0.message",
							"redirect.0.error.0.custom_url",
							"payment.0.customer_email_field_name",
							"payment.0.fixed_rule.0.description",
						} {
							if v, ok := states[0].Attributes[k]; ok {
								return fmt.Errorf("%s: imported as %q, want null", k, v)
							}
						}
						return nil
					},
				},
				{
					// Refreshing an API "" against a configured "" keeps the config value.
					Config: fakeAPIFormConfig(srv.URL, fakeAPIEmptyStrings),
					Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckResourceAttr(res, "submit_action.0.subject", ""),
						resource.TestCheckResourceAttr(res, "submit_action.0.reply_to", ""),
						resource.TestCheckResourceAttr(res, "redirect.0.success.0.message", ""),
						resource.TestCheckResourceAttr(res, "payment.0.fixed_rule.0.description", ""),
					),
				},
			},
		})
	})

	t.Run("legacy", func(t *testing.T) {
		srv, _ := newFakeFormAPI(t, true)
		resource.Test(t, resource.TestCase{
			ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
			Steps: []resource.TestStep{
				{
					Config: fakeAPIFormConfig(srv.URL, ""),
					Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckNoResourceAttr(res, "enable_ai_spam_review"),
						resource.TestCheckNoResourceAttr(res, "ai_spam_review_excluded_fields"),
						resource.TestCheckResourceAttr(res, "enable_allowed_domains", "false"),
						resource.TestCheckNoResourceAttr(ds, "enable_ai_spam_review"),
						resource.TestCheckNoResourceAttr(ds, "ai_spam_review_excluded_fields"),
					),
				},
				{
					// Configured values are kept in state although the API does not echo them.
					Config: fakeAPIFormConfig(srv.URL, fakeAPISettings),
					Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckResourceAttr(res, "enable_ai_spam_review", "false"),
						resource.TestCheckResourceAttr(res, "ai_spam_review_excluded_fields.0", "email"),
						resource.TestCheckResourceAttr(res, "enable_allowed_domains", "true"),
						resource.TestCheckResourceAttr(res, "allowed_domains.#", "2"),
					),
				},
				{
					Config: fakeAPIFormConfig(srv.URL, ""),
					Check: resource.ComposeAggregateTestCheckFunc(
						resource.TestCheckNoResourceAttr(res, "ai_spam_review_excluded_fields"),
						resource.TestCheckResourceAttr(res, "enable_allowed_domains", "false"),
						resource.TestCheckNoResourceAttr(res, "allowed_domains"),
					),
				},
			},
		})
	})
}
