package client

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetBillingDecodesTheBillingContract(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/billing/me" {
			t.Errorf("unexpected request %s", r.URL)
		}
		fmt.Fprint(w, `{"hasBillingAccount":true,"stripeCustomerId":"cus_1","activeSubscription":{"subscriptionId":"0d1ff5dd-7b2c-4b6a-8a58-d2ba6a1c24f2","stripeProductId":"prod_1","stripePriceId":"price_1","status":"Trialing","interval":"Month","unitPriceAmount":19.5,"unitPriceCurrency":"eur","currentPeriodStart":"2026-10-01T00:00:00+00:00","currentPeriodEnd":"2026-11-01T00:00:00+00:00","trialEndUtc":"2026-10-15T00:00:00+00:00","cancelAtPeriodEnd":true,"canceledAtUtc":null},"entitlements":["forms.subscription","forms.notion"]}`)
	}))
	defer srv.Close()
	b, err := New(srv.URL, "key").GetBilling(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	s := b.ActiveSubscription
	if !b.HasBillingAccount || *b.StripeCustomerID != "cus_1" || s == nil || s.Status != "Trialing" || s.Interval != "Month" ||
		s.UnitPriceAmount != 19.5 || s.UnitPriceCurrency != "eur" || *s.TrialEndUTC != "2026-10-15T00:00:00+00:00" ||
		!s.CancelAtPeriodEnd || s.CanceledAtUTC != nil || len(b.Entitlements) != 2 {
		t.Fatalf("billing not decoded: %#v %#v", b, s)
	}
}

func TestGetBillingWithoutSubscription(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"hasBillingAccount":false,"stripeCustomerId":null,"activeSubscription":null,"entitlements":[]}`)
	}))
	defer srv.Close()
	b, err := New(srv.URL, "key").GetBilling(context.Background())
	if err != nil || b.ActiveSubscription != nil || b.HasBillingAccount {
		t.Fatalf("empty billing state not decoded: %#v %v", b, err)
	}
}

func TestProfileAndEntitlements(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/users/me":
			fmt.Fprint(w, `{"id":"u1","email":"a@example.com","isSuperAdmin":true,"permissions":["User.Account.Get"]}`)
		case "/api/v1/billing/entitlements":
			fmt.Fprint(w, `{"featureKeys":["forms.google-sheets","forms.payments"],"limits":{"forms.storage-bytes":1}}`)
		default:
			t.Errorf("unexpected request %s", r.URL)
		}
	}))
	defer srv.Close()
	c, ctx := New(srv.URL, "key"), context.Background()
	p, err := c.GetProfile(ctx)
	if err != nil || p.ID != "u1" || !p.IsSuperAdmin || len(p.Permissions) != 1 {
		t.Fatalf("profile not decoded: %#v %v", p, err)
	}
	e, err := c.GetEntitlements(ctx)
	if err != nil || !e.Has(EntitlementGoogleSheets) || !e.Has(EntitlementPaymentForms) || e.Has(EntitlementNotion) {
		t.Fatalf("entitlements not decoded: %#v %v", e, err)
	}
}

func TestUserSettingsRoundTrip(t *testing.T) {
	frequency := "Weekly"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/users/me/settings" {
			t.Errorf("unexpected request %s", r.URL)
		}
		if r.Method == http.MethodPut {
			frequency = "Monthly"
		}
		fmt.Fprintf(w, `{"submissionOverviewEmailFrequency":%q}`, frequency)
	}))
	defer srv.Close()
	c, ctx := New(srv.URL, "key"), context.Background()
	got, err := c.GetUserSettings(ctx)
	if err != nil || got.SubmissionOverviewEmailFrequency != "Weekly" {
		t.Fatalf("settings not read: %#v %v", got, err)
	}
	updated, err := c.UpdateUserSettings(ctx, UserSettings{SubmissionOverviewEmailFrequency: "Monthly"})
	if err != nil || updated.SubmissionOverviewEmailFrequency != "Monthly" {
		t.Fatalf("settings not updated: %#v %v", updated, err)
	}
}
