resource "staticform_form" "contact" {
  name = "Contact form"

  # Only accept submissions from your own site (and its subdomains).
  enable_allowed_domains = true
  allowed_domains        = ["example.com"]

  # Ask an AI model for a second opinion on doubtful submissions, but never
  # send it the submitter's email address.
  enable_ai_spam_review          = true
  ai_spam_review_excluded_fields = ["email"]

  field {
    name     = "email"
    type     = "Text"
    rule     = "Email"
    required = true
  }

  field {
    name     = "message"
    type     = "Text"
    required = true
  }

  submit_action {
    type      = "SendEmailAction"
    name      = "Notify team"
    recipient = "team@example.com"
    subject   = "New message from {{formName}}"
    body_template = <<-EOT
      From: {{form.email}}

      {{form.message}}
    EOT
  }

  submit_action {
    type        = "TriggerWebhookAction"
    name        = "Forward to Slack"
    webhook_url = "https://hooks.slack.com/services/XXX/YYY/ZZZ"
    http_method = "Post"
    body_template = jsonencode({
      text = "New contact: {{form.email}}"
    })
  }

  redirect {
    success {
      type       = "CustomUrl"
      custom_url = "https://example.com/thank-you"
    }
  }
}

output "form_id" {
  value = staticform_form.contact.id
}
