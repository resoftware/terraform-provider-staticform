package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestSensitiveFormFakeAPI(t *testing.T) {
	server, _ := newFakeFormAPI(t, false)
	config := func(policy, name string) string {
		return fmt.Sprintf(`
provider "staticform" {
 api_key = "test-key"
 base_url = %q
}
resource "staticform_form" "test" {
 name = %q
 %s
 field {
  name = "bsn"
  type = "Text"
  %s
 }
 submit_action {
  type = "SendEmailAction"
  name = "Notify"
  recipient = "public@example.test"
  body_template = "{{form.bsn}}"
  %s
 }
}
data "staticform_form" "test" { id = staticform_form.test.id }
`, server.URL, name, map[string]string{"explicit": "confirm_sensitive_fields = true", "remove": "confirm_sensitive_fields = true"}[policy], map[string]string{"explicit": "sensitive = true", "remove": "sensitive = false"}[policy], map[string]string{"explicit": "sensitive_fields = [\"bsn\"]", "remove": "sensitive_fields = []"}[policy])
	}
	checks := resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttr("staticform_form.test", "field.0.sensitive", "true"),
		resource.TestCheckResourceAttr("staticform_form.test", "submit_action.0.sensitive_fields.#", "1"),
		resource.TestCheckResourceAttr("data.staticform_form.test", "sensitive_fields.#", "1"),
	)
	resource.Test(t, resource.TestCase{ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, Steps: []resource.TestStep{
		{Config: config("explicit", "Protected"), Check: checks},
		{Config: config("explicit", "Protected"), PlanOnly: true},
		{ResourceName: "staticform_form.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"confirm_sensitive_fields"}},
		{Config: config("omitted", "Unrelated rename"), Check: checks},
		{Config: config("omitted", "Unrelated rename"), PlanOnly: true},
		{Config: config("remove", "Unrelated rename"), Check: resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr("staticform_form.test", "field.0.sensitive", "false"),
			resource.TestCheckResourceAttr("staticform_form.test", "submit_action.0.sensitive_fields.#", "0"),
			resource.TestCheckResourceAttr("data.staticform_form.test", "sensitive_fields.#", "0"),
		)},
		{Config: config("remove", "Unrelated rename"), PlanOnly: true},
	}})
}
