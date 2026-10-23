package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/resoftware/terraform-provider-staticform/internal/client"
)

type fakeNotificationDefinition struct {
	key, category string
	optOut, email bool
}

var fakeNotificationCatalog = []fakeNotificationDefinition{
	{"product.EmailQuotaWarning", "Billing", true, true},
	{"product.Mandatory", "Account", false, true},
	{"product.StorageWarning", "Billing", true, true},
	{"product.SubmissionReceived", "Submissions", true, false},
	{"product.SubmitActionFailed", "Attention", true, true},
}

type fakeNotificationAPI struct {
	mu        sync.Mutex
	prefs     client.NotificationPreferencesRequest
	overrides map[string]client.NotificationEventChoice
	puts      int
}

func newFakeNotificationAPI(t *testing.T) (*httptest.Server, *fakeNotificationAPI) {
	api := &fakeNotificationAPI{
		prefs:     client.NotificationPreferencesRequest{EmailEnabled: true, EmailMode: "Immediate", DigestHourLocal: 8, TimeZoneID: "UTC"},
		overrides: map[string]client.NotificationEventChoice{},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/users/me/notification-preferences", api.get)
	mux.HandleFunc("PUT /api/v1/users/me/notification-preferences", api.put)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, api
}

func (a *fakeNotificationAPI) snapshot() client.NotificationPreferences {
	out := client.NotificationPreferences{
		EmailEnabled: a.prefs.EmailEnabled, EmailMode: a.prefs.EmailMode, DigestHourLocal: a.prefs.DigestHourLocal,
		QuietHours: a.prefs.QuietHours, TimeZoneID: a.prefs.TimeZoneID,
	}
	for _, d := range fakeNotificationCatalog {
		o, stored := a.overrides[d.key]
		e := client.NotificationEventPreference{
			Key: d.key, Category: d.category, OptOutAllowed: d.optOut, EmailByDefault: d.email,
			InApp: true, Email: d.email, SampleTitle: "Sample " + d.key,
		}
		if d.optOut && stored {
			e.InApp, e.Email = o.InApp, o.Email
		}
		out.Events = append(out.Events, e)
	}
	return out
}

func (a *fakeNotificationAPI) get(w http.ResponseWriter, _ *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.write(w, http.StatusOK, a.snapshot())
}

func (a *fakeNotificationAPI) put(w http.ResponseWriter, r *http.Request) {
	var req client.NotificationPreferencesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.puts++
	problems := map[string][]string{}
	overrides := map[string]client.NotificationEventChoice{}
	for i, e := range req.Events {
		var def *fakeNotificationDefinition
		for j := range fakeNotificationCatalog {
			if fakeNotificationCatalog[j].key == e.Key {
				def = &fakeNotificationCatalog[j]
			}
		}
		field := fmt.Sprintf("Events[%d]", i)
		switch {
		case def == nil:
			problems[field] = append(problems[field], "UserNotificationUnknownEvent")
		case !def.optOut && !e.InApp:
			problems[field] = append(problems[field], "UserNotificationMandatoryEvent")
		case !e.InApp && e.Email:
			problems[field] = append(problems[field], "UserNotificationEmailRequiresInApp")
		case def.optOut && !(e.InApp && e.Email == def.email):
			overrides[e.Key] = e
		}
	}
	if len(problems) > 0 {
		raw, _ := json.Marshal(map[string]any{"title": "Validation failed", "status": 400, "errors": problems})
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write(raw)
		return
	}
	req.Events = nil
	a.prefs, a.overrides = req, overrides
	a.write(w, http.StatusOK, a.snapshot())
}

func (a *fakeNotificationAPI) write(w http.ResponseWriter, status int, v any) {
	raw, _ := json.Marshal(v)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(raw)
}

func (a *fakeNotificationAPI) event(key string) client.NotificationEventPreference {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, e := range a.snapshot().Events {
		if e.Key == key {
			return e
		}
	}
	return client.NotificationEventPreference{}
}

func notificationConfig(baseURL, body string) string {
	return fmt.Sprintf(`
provider "staticform" {
  base_url = %q
  api_key  = "sf_test_fake"
}

resource "staticform_notification_preferences" "me" {
%s
}

data "staticform_notification_preferences" "me" {
  depends_on = [staticform_notification_preferences.me]
}
`, baseURL, body)
}

func TestNotificationPreferencesResourceFakeAPI(t *testing.T) {
	const res = "staticform_notification_preferences.me"
	const ds = "data.staticform_notification_preferences.me"
	srv, api := newFakeNotificationAPI(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: notificationConfig(srv.URL, `
  email_mode        = "DailyDigest"
  digest_hour_local = 9
  time_zone_id      = "Europe/Amsterdam"

  quiet_hours = {
    start = "22:00"
    end   = "07:00"
  }

  events = {
    "product.StorageWarning"     = { email = false }
    "product.SubmitActionFailed" = { in_app = true, email = true }
    "product.SubmissionReceived" = { in_app = false }
  }
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(res, "id", "notification-preferences"),
					resource.TestCheckResourceAttr(res, "email_enabled", "true"),
					resource.TestCheckResourceAttr(res, "email_mode", "DailyDigest"),
					resource.TestCheckResourceAttr(res, "digest_hour_local", "9"),
					resource.TestCheckResourceAttr(res, "time_zone_id", "Europe/Amsterdam"),
					resource.TestCheckResourceAttr(res, "quiet_hours.start", "22:00"),
					resource.TestCheckResourceAttr(res, "quiet_hours.end", "07:00"),
					resource.TestCheckResourceAttr(res, "events.%", "3"),
					resource.TestCheckResourceAttr(res, "events.product.StorageWarning.in_app", "true"),
					resource.TestCheckResourceAttr(res, "events.product.StorageWarning.email", "false"),
					resource.TestCheckResourceAttr(res, "events.product.SubmitActionFailed.email", "true"),
					resource.TestCheckResourceAttr(res, "events.product.SubmissionReceived.in_app", "false"),
					resource.TestCheckResourceAttr(res, "events.product.SubmissionReceived.email", "false"),
					resource.TestCheckResourceAttr(ds, "email_mode", "DailyDigest"),
					resource.TestCheckResourceAttr(ds, "quiet_hours.start", "22:00"),
					resource.TestCheckResourceAttr(ds, "events.#", "5"),
					resource.TestCheckResourceAttr(ds, "events.0.key", "product.EmailQuotaWarning"),
					resource.TestCheckResourceAttr(ds, "events.0.email_by_default", "true"),
					resource.TestCheckResourceAttr(ds, "events.4.email", "true"),
					func(*terraform.State) error {
						if e := api.event("product.StorageWarning"); !e.InApp || e.Email {
							return fmt.Errorf("server did not store the StorageWarning choice: %#v", e)
						}
						if e := api.event("product.SubmissionReceived"); e.InApp || e.Email {
							return fmt.Errorf("server did not store the SubmissionReceived choice: %#v", e)
						}
						return nil
					},
				),
			},
			{
				PreConfig: func() {
					api.mu.Lock()
					defer api.mu.Unlock()
					api.prefs.EmailEnabled = false
					api.overrides["product.EmailQuotaWarning"] = client.NotificationEventChoice{Key: "product.EmailQuotaWarning", InApp: true, Email: false}
				},
				Config: notificationConfig(srv.URL, `
  email_mode        = "DailyDigest"
  digest_hour_local = 9
  time_zone_id      = "Europe/Amsterdam"

  quiet_hours = {
    start = "22:00"
    end   = "07:00"
  }

  events = {
    "product.StorageWarning" = { email = true }
  }
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(res, "email_enabled", "false"),
					resource.TestCheckResourceAttr(res, "events.%", "1"),
					resource.TestCheckResourceAttr(res, "events.product.StorageWarning.email", "true"),
					func(*terraform.State) error {
						if e := api.event("product.EmailQuotaWarning"); e.Email {
							return fmt.Errorf("an unmanaged dashboard choice was reset: %#v", e)
						}
						if e := api.event("product.SubmissionReceived"); e.InApp {
							return fmt.Errorf("an event dropped from config was reset: %#v", e)
						}
						return nil
					},
				),
			},
			{
				Config: notificationConfig(srv.URL, `
  email_enabled = true
  events        = {}
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(res, "email_enabled", "true"),
					resource.TestCheckResourceAttr(res, "email_mode", "DailyDigest"),
					resource.TestCheckNoResourceAttr(res, "quiet_hours"),
					resource.TestCheckResourceAttr(res, "events.%", "0"),
				),
			},
			{
				ResourceName:            res,
				ImportState:             true,
				ImportStateId:           "notification-preferences",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"events"},
			},
			{
				Config:      notificationConfig(srv.URL, `events = { "product.Nope" = { email = true } }`),
				ExpectError: regexp.MustCompile(`(?s)Unknown notification event.*product\.EmailQuotaWarning`),
			},
			{
				Config:      notificationConfig(srv.URL, `events = { "product.Mandatory" = { email = false } }`),
				ExpectError: regexp.MustCompile("Notification cannot be changed"),
			},
			{
				Config:      notificationConfig(srv.URL, `events = { "product.StorageWarning" = { in_app = false, email = true } }`),
				ExpectError: regexp.MustCompile("Email requires in_app"),
			},
			{
				Config:      notificationConfig(srv.URL, `quiet_hours = { start = "08:00", end = "08:00" }`),
				ExpectError: regexp.MustCompile("Empty quiet hours"),
			},
			{
				Config:      notificationConfig(srv.URL, `quiet_hours = { start = "8am", end = "09:00" }`),
				ExpectError: regexp.MustCompile(`formatted\s+HH:mm`),
			},
			{
				Config:      notificationConfig(srv.URL, `digest_hour_local = 24`),
				ExpectError: regexp.MustCompile("between 0 and 23"),
			},
			{
				Config: notificationConfig(srv.URL, `email_enabled = true`),
				Check:  resource.TestCheckNoResourceAttr(res, "events"),
			},
		},
	})
}

func TestNotificationPreferencesDestroyLeavesServerState(t *testing.T) {
	srv, api := newFakeNotificationAPI(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: notificationConfig(srv.URL, `
  email_mode = "DailyDigest"
  events     = { "product.StorageWarning" = { in_app = false } }
`),
			},
		},
		CheckDestroy: func(*terraform.State) error {
			api.mu.Lock()
			defer api.mu.Unlock()
			if api.prefs.EmailMode != "DailyDigest" || len(api.overrides) != 1 {
				return fmt.Errorf("destroy changed the server preferences: %#v %#v", api.prefs, api.overrides)
			}
			return nil
		},
	})
}

func newNotificationResource(t *testing.T) (*notificationPreferencesResource, *fakeNotificationAPI) {
	srv, api := newFakeNotificationAPI(t)
	return &notificationPreferencesResource{client: client.New(srv.URL, "key")}, api
}

func eventsMap(t *testing.T, in map[string]eventPreferenceModel) types.Map {
	t.Helper()
	m, diags := types.MapValueFrom(context.Background(), types.ObjectType{AttrTypes: eventAttrTypes}, in)
	if diags.HasError() {
		t.Fatal(diags)
	}
	return m
}

func unknownEvent() eventPreferenceModel {
	return eventPreferenceModel{InApp: types.BoolUnknown(), Email: types.BoolUnknown()}
}

func TestDesiredKeepsUnmanagedEventsAndFillsOmittedValues(t *testing.T) {
	r, api := newNotificationResource(t)
	api.overrides["product.EmailQuotaWarning"] = client.NotificationEventChoice{Key: "product.EmailQuotaWarning", InApp: true, Email: false}
	api.prefs.EmailMode = "DailyDigest"
	api.prefs.QuietHours = &client.QuietHours{Start: "22:00:00", End: "07:00:00"}
	plan := notificationPreferencesModel{
		EmailEnabled:    types.BoolUnknown(),
		EmailMode:       types.StringUnknown(),
		DigestHourLocal: types.Int64Value(10),
		TimeZoneID:      types.StringUnknown(),
		QuietHours:      types.ObjectNull(quietHoursAttrTypes),
		Events: eventsMap(t, map[string]eventPreferenceModel{
			"product.StorageWarning":     {InApp: types.BoolValue(false), Email: types.BoolUnknown()},
			"product.SubmitActionFailed": unknownEvent(),
		}),
	}
	req, diags := r.desired(context.Background(), plan)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if !req.EmailEnabled || req.EmailMode != "DailyDigest" || req.DigestHourLocal != 10 || req.TimeZoneID != "UTC" || req.QuietHours != nil {
		t.Fatalf("globals not merged: %#v", req)
	}
	got := map[string]client.NotificationEventChoice{}
	for _, e := range req.Events {
		got[e.Key] = e
	}
	if len(got) != 5 {
		t.Fatalf("expected the whole catalog to be sent, got %d events", len(got))
	}
	if e := got["product.EmailQuotaWarning"]; !e.InApp || e.Email {
		t.Fatalf("unmanaged event changed: %#v", e)
	}
	if e := got["product.StorageWarning"]; e.InApp || e.Email {
		t.Fatalf("in_app=false must switch email off: %#v", e)
	}
	if e := got["product.SubmitActionFailed"]; !e.InApp || !e.Email {
		t.Fatalf("omitted choices must keep the server value: %#v", e)
	}
}

func TestDesiredRejectsInvalidEvents(t *testing.T) {
	r, _ := newNotificationResource(t)
	cases := map[string]struct {
		event eventPreferenceModel
		key   string
		want  string
	}{
		"unknown":   {unknownEvent(), "product.Nope", "Unknown notification event"},
		"mandatory": {eventPreferenceModel{InApp: types.BoolValue(true), Email: types.BoolValue(false)}, "product.Mandatory", "Notification cannot be changed"},
		"email":     {eventPreferenceModel{InApp: types.BoolValue(false), Email: types.BoolValue(true)}, "product.StorageWarning", "Email requires in_app"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			plan := notificationPreferencesModel{
				EmailEnabled: types.BoolUnknown(), EmailMode: types.StringUnknown(), DigestHourLocal: types.Int64Unknown(), TimeZoneID: types.StringUnknown(),
				QuietHours: types.ObjectNull(quietHoursAttrTypes),
				Events:     eventsMap(t, map[string]eventPreferenceModel{tc.key: tc.event}),
			}
			_, diags := r.desired(context.Background(), plan)
			if !diags.HasError() || !strings.Contains(diags[0].Summary(), tc.want) {
				t.Fatalf("want %q, got %v", tc.want, diags)
			}
			if name == "unknown" {
				keys := []string{"product.EmailQuotaWarning", "product.Mandatory", "product.StorageWarning", "product.SubmissionReceived", "product.SubmitActionFailed"}
				sort.Strings(keys)
				if !strings.Contains(diags[0].Detail(), strings.Join(keys, ", ")) {
					t.Fatalf("known events not listed: %s", diags[0].Detail())
				}
			}
		})
	}
}

func TestApplyTracksOnlyManagedEvents(t *testing.T) {
	r, api := newNotificationResource(t)
	api.mu.Lock()
	api.prefs.QuietHours = &client.QuietHours{Start: "22:00:00", End: "07:30:15"}
	api.mu.Unlock()
	prefs, err := r.client.GetNotificationPreferences(context.Background())

	if err != nil {
		t.Fatal(err)
	}

	model := notificationPreferencesModel{Events: types.MapNull(types.ObjectType{AttrTypes: eventAttrTypes})}
	if diags := applyNotificationPreferences(context.Background(), prefs, &model); diags.HasError() {
		t.Fatal(diags)
	}
	if !model.Events.IsNull() {
		t.Fatalf("unmanaged events entered state: %v", model.Events)
	}
	var q quietHoursModel
	if diags := model.QuietHours.As(context.Background(), &q, objectAsOptions); diags.HasError() {
		t.Fatal(diags)
	}
	if q.Start.ValueString() != "22:00" || q.End.ValueString() != "07:30:15" {
		t.Fatalf("quiet hours not normalised: %#v", q)
	}

	model.Events = eventsMap(t, map[string]eventPreferenceModel{"product.StorageWarning": unknownEvent(), "product.Gone": unknownEvent()})
	if diags := applyNotificationPreferences(context.Background(), prefs, &model); diags.HasError() {
		t.Fatal(diags)
	}
	if n := len(model.Events.Elements()); n != 1 {
		t.Fatalf("expected only the known managed event, got %d", n)
	}
}
