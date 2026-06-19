package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccFormResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create + Read.
			{
				Config: `
resource "staticform_form" "test" {
  name = "tf-acc-form"
  tags = ["acc", "test"]

  field {
    name     = "email"
    type     = "Text"
    rule     = "Email"
    required = true
  }

  submit_action {
    type          = "SendEmailAction"
    name          = "notify"
    recipient     = "team@example.com"
    subject       = "New"
    body_template = "{{form.email}}"
  }

  submit_action {
    type          = "TriggerWebhookAction"
    name          = "forward"
    webhook_url   = "https://example.com/hook"
    body_template = "{}"
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("staticform_form.test", "id"),
					resource.TestCheckResourceAttr("staticform_form.test", "name", "tf-acc-form"),
					resource.TestCheckResourceAttr("staticform_form.test", "field.0.name", "email"),
					// The submit action's server-assigned ID must be populated from the create response.
					resource.TestCheckResourceAttrSet("staticform_form.test", "submit_action.0.id"),
					resource.TestCheckResourceAttr("staticform_form.test", "tags.#", "2"),
					// Webhook server defaults must round-trip without drift.
					resource.TestCheckResourceAttr("staticform_form.test", "submit_action.1.http_method", "Post"),
					resource.TestCheckResourceAttr("staticform_form.test", "submit_action.1.content_type", "Json"),
				),
			},
			// Import.
			{
				ResourceName:      "staticform_form.test",
				ImportState:       true,
				ImportStateVerify: true,
				// Server-only and write-only values are not returned identically on import.
				ImportStateVerifyIgnore: []string{"captcha_secret_key"},
			},
			// Update.
			{
				Config: `
resource "staticform_form" "test" {
  name = "tf-acc-form-renamed"

  field {
    name     = "email"
    type     = "Text"
    rule     = "Email"
    required = true
  }

  submit_action {
    type          = "SendEmailAction"
    name          = "notify"
    recipient     = "team@example.com"
    subject       = "Updated"
    body_template = "{{form.email}}"
  }
}
`,
				Check: resource.TestCheckResourceAttr("staticform_form.test", "name", "tf-acc-form-renamed"),
			},
		},
	})
}
