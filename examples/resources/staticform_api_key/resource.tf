resource "staticform_api_key" "ci" {
  name        = "ci-pipeline"
  expires_at  = "2027-01-01T00:00:00Z"
  permissions = ["Form.GetById", "Submission.ListAll"]
}

output "api_key" {
  value     = staticform_api_key.ci.key
  sensitive = true
}
