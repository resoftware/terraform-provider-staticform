resource "staticform_email_domain" "example" {
  domain_name = "example.com"
}

# Publish these DKIM records in DNS to verify the domain.
output "dkim_records" {
  value = staticform_email_domain.example.dkim_records
}
