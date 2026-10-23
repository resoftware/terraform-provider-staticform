package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

const notificationPreferencesBody = `{"emailEnabled":true,"emailMode":"DailyDigest","digestHourLocal":8,"quietHours":{"start":"22:00:00","end":"07:00:00"},"timeZoneId":"Europe/Amsterdam","events":[` +
	`{"key":"product.StorageWarning","category":"Billing","optOutAllowed":true,"emailByDefault":true,"inApp":true,"email":false,"sampleTitle":"Your storage is running low"},` +
	`{"key":"product.SubmissionReceived","category":"Submissions","optOutAllowed":true,"emailByDefault":false,"inApp":true,"email":false,"sampleTitle":"New submission"}]}`

func TestGetNotificationPreferencesDecodesTheContract(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/users/me/notification-preferences" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
		}
		fmt.Fprint(w, notificationPreferencesBody)
	}))
	defer srv.Close()
	p, err := New(srv.URL, "key").GetNotificationPreferences(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !p.EmailEnabled || p.EmailMode != "DailyDigest" || p.DigestHourLocal != 8 || p.TimeZoneID != "Europe/Amsterdam" ||
		p.QuietHours == nil || p.QuietHours.Start != "22:00:00" || p.QuietHours.End != "07:00:00" || len(p.Events) != 2 {
		t.Fatalf("preferences not decoded: %#v", p)
	}
	e, ok := p.Event("product.StorageWarning")
	if !ok || e.Category != "Billing" || !e.OptOutAllowed || !e.EmailByDefault || !e.InApp || e.Email || e.SampleTitle != "Your storage is running low" {
		t.Fatalf("event not decoded: %#v", e)
	}
	if _, ok := p.Event("product.Missing"); ok {
		t.Fatal("found an event that is not in the catalog")
	}
}

func TestGetNotificationPreferencesWithoutQuietHours(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"emailEnabled":false,"emailMode":"Immediate","digestHourLocal":8,"quietHours":null,"timeZoneId":"UTC","events":[]}`)
	}))
	defer srv.Close()
	p, err := New(srv.URL, "key").GetNotificationPreferences(context.Background())
	if err != nil || p.QuietHours != nil || p.EmailEnabled {
		t.Fatalf("preferences not decoded: %#v %v", p, err)
	}
}

func TestUpdateNotificationPreferencesSendsTheFullRequest(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/v1/users/me/notification-preferences" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Error(err)
		}
		fmt.Fprint(w, notificationPreferencesBody)
	}))
	defer srv.Close()
	req := NotificationPreferencesRequest{
		EmailEnabled: true, EmailMode: "DailyDigest", DigestHourLocal: 8, TimeZoneID: "Europe/Amsterdam",
		QuietHours: &QuietHours{Start: "22:00:00", End: "07:00:00"},
		Events:     []NotificationEventChoice{{Key: "product.StorageWarning", InApp: true, Email: false}},
	}
	out, err := New(srv.URL, "key").UpdateNotificationPreferences(context.Background(), req)
	if err != nil || out.EmailMode != "DailyDigest" {
		t.Fatalf("update failed: %#v %v", out, err)
	}
	events, _ := got["events"].([]any)
	first, _ := events[0].(map[string]any)
	quiet, _ := got["quietHours"].(map[string]any)
	if got["emailEnabled"] != true || got["emailMode"] != "DailyDigest" || got["digestHourLocal"] != float64(8) || got["timeZoneId"] != "Europe/Amsterdam" ||
		quiet["start"] != "22:00:00" || len(events) != 1 || first["key"] != "product.StorageWarning" || first["inApp"] != true || first["email"] != false {
		t.Fatalf("unexpected request body: %v", got)
	}
}

func TestUpdateNotificationPreferencesSendsNullQuietHoursAndEmptyEvents(t *testing.T) {
	var raw map[string]json.RawMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&raw)
		fmt.Fprint(w, notificationPreferencesBody)
	}))
	defer srv.Close()
	_, err := New(srv.URL, "key").UpdateNotificationPreferences(context.Background(), NotificationPreferencesRequest{EmailMode: "Immediate", TimeZoneID: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	if string(raw["quietHours"]) != "null" || string(raw["events"]) != "[]" {
		t.Fatalf("quietHours %s events %s", raw["quietHours"], raw["events"])
	}
}

func TestRequestReproducesCurrentChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, notificationPreferencesBody)
	}))
	defer srv.Close()
	p, err := New(srv.URL, "key").GetNotificationPreferences(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	req := p.Request()
	if req.EmailMode != "DailyDigest" || req.TimeZoneID != "Europe/Amsterdam" || req.QuietHours == nil || len(req.Events) != 2 ||
		req.Events[0] != (NotificationEventChoice{Key: "product.StorageWarning", InApp: true, Email: false}) {
		t.Fatalf("request does not reproduce the preferences: %#v", req)
	}
}

func TestNotificationPreferencesProblemDetails(t *testing.T) {
	cases := map[string]func(*Client) error{
		"get": func(c *Client) error {
			_, err := c.GetNotificationPreferences(context.Background())
			return err
		},
		"put": func(c *Client) error {
			_, err := c.UpdateNotificationPreferences(context.Background(), NotificationPreferencesRequest{})
			return err
		},
	}
	for name, call := range cases {
		t.Run(name+" forbidden", func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/problem+json")
				w.WriteHeader(http.StatusForbidden)
				fmt.Fprint(w, `{"title":"Forbidden","status":403,"detail":"Missing permission: User.Notifications.GetPreferences","errorCode":"Forbidden"}`)
			}))
			defer srv.Close()
			var apiErr *APIError
			if err := call(New(srv.URL, "key")); !errors.As(err, &apiErr) || apiErr.MissingPermission() != "User.Notifications.GetPreferences" {
				t.Fatalf("missing permission not reported: %v", err)
			}
		})
	}
	t.Run("validation", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"title":"Validation failed","status":400,"errors":{"Events[0]":["UserNotificationUnknownEvent"],"TimeZoneId":["TimeZoneInvalid"]}}`)
		}))
		defer srv.Close()
		err := cases["put"](New(srv.URL, "key"))
		var apiErr *APIError
		if !errors.As(err, &apiErr) || apiErr.StatusCode != 400 || apiErr.Errors["Events[0]"][0] != "UserNotificationUnknownEvent" || apiErr.Errors["TimeZoneId"][0] != "TimeZoneInvalid" {
			t.Fatalf("validation problem not decoded: %v", err)
		}
	})
}
