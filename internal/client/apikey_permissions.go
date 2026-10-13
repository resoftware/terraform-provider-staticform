package client

import "slices"

var legacyAPIKeyPermissions = map[string][]string{
	"User.GetMyProfile":                    {"User.Profile.Get"},
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
	"Subscription.GetTiers":                {"User.Subscriptions.GetTiers"},
	"Subscription.GetMySubscription":       {"User.Subscriptions.GetMySubscription"},
	"Subscription.GetEmailUsage":           {"User.Subscriptions.GetEmailUsage"},
	"Payment.CreateCustomerPortalSession":  {"User.Payments.CreateCustomerPortalSession"},
	"Subscription.CreateCheckoutSession":   {"User.Subscriptions.CreateCheckoutSession"},
	"Subscription.CancelSubscription":      {"User.Subscriptions.CancelSubscription"},
	"Subscription.ChangeSubscription":      {"User.Subscriptions.ChangeSubscription"},
	"Subscription.ReactivateSubscription":  {"User.Subscriptions.ReactivateSubscription"},
	"Subscription.AutoTrial":               {"User.Subscriptions.AutoTrial"},
	"Permissions":                          {"PermissionNotHeld"},
}

func NativeAPIKeyPermissions(names []string) []string {
	if names == nil {
		return nil
	}
	out := []string{}
	for _, name := range names {
		mapped, exists := legacyAPIKeyPermissions[name]
		if !exists {
			mapped = []string{name}
		}
		for _, permission := range mapped {
			if !slices.Contains(out, permission) {
				out = append(out, permission)
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
