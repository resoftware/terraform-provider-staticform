resource "staticform_smtp_connection" "resend" {
  label      = "Resend"
  provider_preset = "Resend"
  host       = "smtp.resend.com"
  port       = 465
  security   = "SslOnConnect"
  username   = "resend"
  password   = var.resend_api_key
  from_email = "forms@example.com"
  from_name  = "Example Forms"
}

variable "resend_api_key" {
  type      = string
  sensitive = true
}
