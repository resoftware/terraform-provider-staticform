resource "staticform_api_key" "ci" {
  name       = "ci-pipeline"
  expires_at = "2027-01-01T00:00:00Z"

  # Every endpoint has its own permission; list each one the key needs.
  permissions = [
    "User.Forms.GetById",
    "User.Submissions.ListAll",
    "User.Submissions.GetUnreadCount",
  ]
}

output "api_key" {
  value     = staticform_api_key.ci.key
  sensitive = true
}
