terraform {
  required_providers {
    staticform = {
      source = "resoftware/staticform"
    }
  }
}

# The API key can also be supplied via the STATICFORM_API_KEY environment variable.
provider "staticform" {
  api_key = var.staticform_api_key
  # When managing API-key resources, supply a fresh proof via
  # STATICFORM_KEY_CREATION_PASSWORD or STATICFORM_KEY_CREATION_REAUTH_TOKEN.
}

variable "staticform_api_key" {
  type      = string
  sensitive = true
}
