package client

import "slices"

var legacyAPIKeyPermissions = map[string][]string{
	"User.GetMyProfile":                    {"User.Account.Get", "User.Billing.GetEntitlements"},
	"User.GetMetrics":                      {"User.Metrics.Get"},
	"User.GetMySettings":                   {"User.Settings.Get"},
	"User.UpdateMySettings":                {"User.Settings.Update"},
	"User.ManageApiKeys":                   {"User.ApiKeys.List", "User.ApiKeys.Create", "User.ApiKeys.Rotate", "User.ApiKeys.UpdatePermissions", "User.ApiKeys.Delete"},
	"User.DeleteMyAccount":                 {"User.Account.Delete"},
	"User.ExportMyData":                    {"User.Account.ExportData"},
	"Form.ListMy":                          {"User.Forms.ListMy"},
	"Form.GetById":                         {"User.Forms.GetById"},
	"Form.PreviewTemplate":                 {"User.Forms.PreviewTemplate"},
	"Form.PreviewEmailTemplate":            {"User.Forms.PreviewEmailTemplate"},
	"Form.EvaluateRunCondition":            {"User.Forms.EvaluateRunCondition"},
	"Form.Create":                          {"User.Forms.Create"},
	"Form.Update":                          {"User.Forms.Update"},
	"Form.Delete":                          {"User.Forms.Delete"},
	"Form.Submit":                          {"User.Forms.Submit"},
	"Form.UpdateTags":                      {"User.Forms.UpdateTags"},
	"Form.DeleteTag":                       {"User.Forms.DeleteTag"},
	"Form.RegenerateServerSecret":          {"User.Forms.RegenerateServerSecret"},
	"Form.TestWebhook":                     {"User.Forms.TestWebhook"},
	"Form.GetInsights":                     {"User.Forms.GetInsights"},
	"Form.GetView":                         {"User.Forms.GetView"},
	"Form.SaveView":                        {"User.Forms.SaveView"},
	"Submission.ListAll":                   {"User.Submissions.ListAll"},
	"Submission.GetById":                   {"User.Submissions.GetById"},
	"Form.ListSubmissions":                 {"User.Submissions.ListForForm"},
	"Form.GetSubmissionFileByFieldName":    {"User.Submissions.GetFileByFieldName"},
	"Form.ExportSubmissionsAsCsv":          {"User.Submissions.ExportAsCsv"},
	"Form.ExportSubmissionsAsJson":         {"User.Submissions.ExportAsJson"},
	"Submission.ListSpam":                  {"User.Submissions.ListSpam"},
	"Form.DeleteSubmissions":               {"User.Submissions.DeleteForForm"},
	"Submission.MarkRead":                  {"User.Submissions.MarkRead"},
	"Submission.MarkAsSpam":                {"User.Submissions.MarkAsSpam"},
	"Submission.MarkAsLegitimate":          {"User.Submissions.MarkAsLegitimate"},
	"Submission.RetryAction":               {"User.Submissions.RetryAction"},
	"ExecutionLog.ListAll":                 {"User.ExecutionLogs.ListAll"},
	"ExecutionLog.GetById":                 {"User.ExecutionLogs.GetById"},
	"ExecutionLog.ListForSubmission":       {"User.ExecutionLogs.ListForSubmission"},
	"Form.ListCollaborators":               {"User.Collaboration.ListCollaborators"},
	"Form.ListMyInvitations":               {"User.Collaboration.ListMyInvitations"},
	"Form.ManageCollaborators":             {"User.Collaboration.ManageCollaborators"},
	"Form.LeaveForm":                       {"User.Collaboration.LeaveForm"},
	"Form.AcceptInvitation":                {"User.Collaboration.AcceptInvitation"},
	"Form.DeclineInvitation":               {"User.Collaboration.DeclineInvitation"},
	"Integrations.ManageGoogleConnections": {"User.Integrations.ManageGoogleConnections"},
	"Integrations.ManageNotionConnections": {"User.Integrations.ManageNotionConnections"},
	"Integrations.ManageEmailDomains":      {"User.Integrations.ManageEmailDomains"},
	"Integrations.ManageSmtpConnections":   {"User.Integrations.ManageSmtpConnections"},
	"Integrations.ManageStripeConnections": {"User.Integrations.ManageStripeConnections"},
	"Payment.GetStorageUsage":              {"User.Payments.GetStorageUsage"},
	"Payment.GetPaymentStatus":             {"User.Payments.GetPaymentStatus"},
	"Subscription.GetTiers":                {"User.Billing.ListPlans"},
	"Subscription.GetMySubscription":       {"User.Billing.Get"},
	"Subscription.GetEmailUsage":           {"User.Subscriptions.GetEmailUsage"},
	"Payment.CreateCustomerPortalSession":  {"User.Billing.OpenPortal"},
	"Subscription.CreateCheckoutSession":   {"User.Billing.CreateCheckoutSession"},
	"Subscription.CancelSubscription":      {"User.Billing.CancelSubscription"},
	"Subscription.ChangeSubscription":      {"User.Billing.OpenPortal"},
	"Subscription.ReactivateSubscription":  {"User.Billing.OpenPortal"},
	"Subscription.AutoTrial":               {"User.Billing.StartTrial"},
}

var splitAPIKeyPermissions = map[string][]string{
	"User.Forms.ListMy":                      {"User.Forms.ListTags"},
	"User.Forms.GetInsights":                 {"User.Forms.ListViewRecords"},
	"User.Forms.Delete":                      {"User.Forms.TransferOwnership"},
	"User.Forms.Update":                      {"User.Forms.UploadEmailAttachment", "User.Forms.DeleteEmailAttachment"},
	"User.Collaboration.ListCollaborators":   {"User.Collaboration.ListCollaboratorSuggestions"},
	"User.Collaboration.ManageCollaborators": {"User.Collaboration.ListInvitations", "User.Collaboration.UpdateCollaborator", "User.Collaboration.RemoveCollaborator", "User.Collaboration.CancelInvitation"},
	"User.Submissions.ListSpam":              {"User.Submissions.GetSpam"},
	"User.Submissions.ListAll":               {"User.Submissions.GetUnreadCount"},
	"User.Submissions.DeleteForForm":         {"User.Submissions.BatchDelete"},
	"User.Submissions.MarkRead":              {"User.Submissions.BatchMarkRead"},
	"User.Settings.Get":                      {"User.Notifications.GetPreferences"},
	"User.Settings.Update":                   {"User.Notifications.UpdatePreferences"},
	"User.Integrations.ManageGoogleConnections": {
		"User.Integrations.ListGoogleConnections", "User.Integrations.DeleteGoogleConnection", "User.Integrations.ListGoogleConnectionUsages",
		"User.Integrations.DisableGoogleConnectionUsage", "User.Integrations.DeleteGoogleConnectionUsage",
		"User.Integrations.DisableAllGoogleConnectionUsages", "User.Integrations.DeleteAllGoogleConnectionUsages",
		"User.Integrations.IssueGooglePickerToken", "User.Integrations.ListGoogleSheetTabs", "User.Integrations.TestGoogleSheetsAppend",
	},
	"User.Integrations.ManageNotionConnections": {
		"User.Integrations.ListNotionConnections", "User.Integrations.DeleteNotionConnection", "User.Integrations.ListNotionConnectionUsages",
		"User.Integrations.DisableAllNotionConnectionUsages", "User.Integrations.DeleteAllNotionConnectionUsages",
		"User.Integrations.ListNotionDatabases", "User.Integrations.GetNotionDatabaseSchema",
		"User.Integrations.CreateNotionDatabaseProperties", "User.Integrations.TestNotionDatabaseInsert",
	},
	"User.Integrations.ManageSmtpConnections": {
		"User.Integrations.TestSmtpConnection", "User.Integrations.ListSmtpConnections", "User.Integrations.DeleteSmtpConnection",
		"User.Integrations.ListSmtpConnectionUsages", "User.Integrations.DisableAllSmtpConnectionUsages",
		"User.Integrations.DeleteAllSmtpConnectionUsages", "User.Integrations.SendSmtpConnectionTestEmail",
	},
	"User.Integrations.ManageEmailDomains": {
		"User.Integrations.ListEmailDomains", "User.Integrations.DeleteEmailDomain", "User.Integrations.ListEmailDomainUsages",
		"User.Integrations.DisableAllEmailDomainUsages", "User.Integrations.DeleteAllEmailDomainUsages",
		"User.Integrations.RecheckEmailDomain", "User.Integrations.SendEmailDomainTestEmail",
	},
	"User.Integrations.ManageStripeConnections": {
		"User.Integrations.ListStripeConnections", "User.Integrations.DisconnectStripeConnection", "User.Integrations.ListStripeConnectionUsages",
		"User.Integrations.DisableAllStripeConnectionUsages", "User.Integrations.DeleteAllStripeConnectionUsages",
		"User.Integrations.SetStripeConnectionWebhookSecret",
	},
}

// NativeAPIKeyPermissions translates historical StaticForm permission names to
// the per-endpoint permissions that replace them. Names that are not
// historical are used as given.
func NativeAPIKeyPermissions(names []string) []string {
	if names == nil {
		return nil
	}
	out := []string{}
	add := func(permission string) {
		if !slices.Contains(out, permission) {
			out = append(out, permission)
		}
	}
	for _, name := range names {
		mapped, historical := legacyAPIKeyPermissions[name]
		if !historical {
			add(name)
			continue
		}
		for _, permission := range mapped {
			add(permission)
			for _, split := range splitAPIKeyPermissions[permission] {
				add(split)
			}
		}
	}
	return out
}

func PreserveAPIKeyPermissionNames(previous, actual []string) []string {
	expected := NativeAPIKeyPermissions(previous)
	if len(expected) == len(actual) {
		matches := true
		for _, name := range expected {
			if !slices.Contains(actual, name) {
				matches = false
				break
			}
		}
		if matches && previous != nil {
			return previous
		}
	}
	return actual
}
