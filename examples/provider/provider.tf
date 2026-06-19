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
}

variable "staticform_api_key" {
  type      = string
  sensitive = true
}
