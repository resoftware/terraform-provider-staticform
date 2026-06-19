package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccAPIKeyResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "staticform_api_key" "test" {
  name = "tf-acc-key"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("staticform_api_key.test", "id"),
					resource.TestCheckResourceAttrSet("staticform_api_key.test", "key"),
					resource.TestCheckResourceAttrSet("staticform_api_key.test", "key_hint"),
					resource.TestCheckResourceAttr("staticform_api_key.test", "name", "tf-acc-key"),
				),
			},
			{
				ResourceName:      "staticform_api_key.test",
				ImportState:       true,
				ImportStateVerify: true,
				// The plaintext key is only returned at creation, never on read/import.
				ImportStateVerifyIgnore: []string{"key"},
			},
		},
	})
}
