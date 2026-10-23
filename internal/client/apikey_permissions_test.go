package client

import (
	"os"
	"slices"
	"strings"
	"testing"
)

func catalogPermissions(t *testing.T) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile("testdata/delegable_permissions.txt")
	if err != nil {
		t.Fatal(err)
	}
	catalog := map[string]bool{}
	for _, name := range strings.Fields(string(raw)) {
		catalog[name] = true
	}
	return catalog
}

func TestEveryMappedPermissionExistsInTheCatalog(t *testing.T) {
	catalog := catalogPermissions(t)
	for legacy, targets := range legacyAPIKeyPermissions {
		for _, target := range targets {
			if !catalog[target] {
				t.Errorf("%s maps to %s, which the API does not define", legacy, target)
			}
		}
	}
	for parent, children := range splitAPIKeyPermissions {
		if !catalog[parent] {
			t.Errorf("split parent %s is not in the catalog", parent)
		}
		for _, child := range children {
			if !catalog[child] {
				t.Errorf("%s splits into %s, which the API does not define", parent, child)
			}
		}
	}
}

func TestHistoricalNamesGrantEveryPerEndpointPermission(t *testing.T) {
	cases := map[string][]string{
		"Integrations.ManageNotionConnections": {"User.Integrations.ManageNotionConnections", "User.Integrations.ListNotionConnections", "User.Integrations.DeleteNotionConnection"},
		"Integrations.ManageSmtpConnections":   {"User.Integrations.ManageSmtpConnections", "User.Integrations.ListSmtpConnections", "User.Integrations.DeleteSmtpConnection"},
		"Integrations.ManageStripeConnections": {"User.Integrations.ManageStripeConnections", "User.Integrations.ListStripeConnections", "User.Integrations.DisconnectStripeConnection", "User.Integrations.SetStripeConnectionWebhookSecret"},
		"Integrations.ManageEmailDomains":      {"User.Integrations.ManageEmailDomains", "User.Integrations.ListEmailDomains", "User.Integrations.DeleteEmailDomain"},
		"Integrations.ManageGoogleConnections": {"User.Integrations.ManageGoogleConnections", "User.Integrations.ListGoogleConnections", "User.Integrations.DeleteGoogleConnection"},
		"Form.Update":                          {"User.Forms.Update", "User.Forms.UploadEmailAttachment", "User.Forms.DeleteEmailAttachment"},
		"Form.ListMy":                          {"User.Forms.ListMy", "User.Forms.ListTags"},
		"Form.Delete":                          {"User.Forms.Delete", "User.Forms.TransferOwnership"},
		"Form.GetInsights":                     {"User.Forms.GetInsights", "User.Forms.ListViewRecords"},
		"Form.ManageCollaborators":             {"User.Collaboration.ManageCollaborators", "User.Collaboration.ListInvitations", "User.Collaboration.UpdateCollaborator", "User.Collaboration.RemoveCollaborator", "User.Collaboration.CancelInvitation"},
		"Form.ListCollaborators":               {"User.Collaboration.ListCollaborators", "User.Collaboration.ListCollaboratorSuggestions"},
		"Submission.ListAll":                   {"User.Submissions.ListAll", "User.Submissions.GetUnreadCount"},
		"Submission.ListSpam":                  {"User.Submissions.ListSpam", "User.Submissions.GetSpam"},
		"Form.DeleteSubmissions":               {"User.Submissions.DeleteForForm", "User.Submissions.BatchDelete"},
		"Submission.MarkRead":                  {"User.Submissions.MarkRead", "User.Submissions.BatchMarkRead"},
		"User.GetMyProfile":                    {"User.Account.Get", "User.Billing.GetEntitlements"},
		"Subscription.GetTiers":                {"User.Billing.ListPlans"},
		"Subscription.GetMySubscription":       {"User.Billing.Get"},
		"Payment.CreateCustomerPortalSession":  {"User.Billing.OpenPortal"},
		"Subscription.ChangeSubscription":      {"User.Billing.OpenPortal"},
		"Subscription.ReactivateSubscription":  {"User.Billing.OpenPortal"},
		"Subscription.CreateCheckoutSession":   {"User.Billing.CreateCheckoutSession"},
		"Subscription.CancelSubscription":      {"User.Billing.CancelSubscription"},
		"Subscription.AutoTrial":               {"User.Billing.StartTrial"},
		"Subscription.GetEmailUsage":           {"User.Subscriptions.GetEmailUsage"},
		"User.GetMySettings":                   {"User.Settings.Get", "User.Notifications.GetPreferences"},
		"User.UpdateMySettings":                {"User.Settings.Update", "User.Notifications.UpdatePreferences"},
	}
	for historical, want := range cases {
		got := NativeAPIKeyPermissions([]string{historical})
		for _, permission := range want {
			if !slices.Contains(got, permission) {
				t.Errorf("%s does not grant %s: %v", historical, permission, got)
			}
		}
		if !strings.HasPrefix(historical, "Integrations.") && len(got) != len(want) {
			t.Errorf("%s: got %v, want exactly %v", historical, got, want)
		}
	}
}

func TestIntegrationManagePermissionsCoverEveryIntegrationEndpointPermission(t *testing.T) {
	var granted []string
	for _, historical := range []string{
		"Integrations.ManageGoogleConnections", "Integrations.ManageNotionConnections", "Integrations.ManageEmailDomains",
		"Integrations.ManageSmtpConnections", "Integrations.ManageStripeConnections",
	} {
		granted = append(granted, NativeAPIKeyPermissions([]string{historical})...)
	}
	for permission := range catalogPermissions(t) {
		if strings.HasPrefix(permission, "User.Integrations.") && !slices.Contains(granted, permission) {
			t.Errorf("%s is not granted by any historical integrations permission", permission)
		}
	}
}

func TestNativeNamesAreNotExpanded(t *testing.T) {
	got := NativeAPIKeyPermissions([]string{"User.Integrations.ManageNotionConnections", "User.Forms.Update"})
	if !slices.Equal(got, []string{"User.Integrations.ManageNotionConnections", "User.Forms.Update"}) {
		t.Fatalf("native names were rewritten: %v", got)
	}
}

func TestMappedPermissionsDeduplicate(t *testing.T) {
	got := NativeAPIKeyPermissions([]string{"Subscription.ChangeSubscription", "Payment.CreateCustomerPortalSession", "User.Billing.OpenPortal"})
	if !slices.Equal(got, []string{"User.Billing.OpenPortal"}) {
		t.Fatalf("duplicates kept: %v", got)
	}
}
