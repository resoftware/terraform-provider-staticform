package provider

import (
	"context"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// TestProviderSchema validates every resource, data source, and provider schema
// offline by asking the provider server for its full schema. It catches schema
// definition errors (duplicate attributes, invalid defaults, block conflicts)
// without needing API credentials.
func TestProviderSchema(t *testing.T) {
	srv, err := testAccProtoV6ProviderFactories["staticform"]()
	if err != nil {
		t.Fatalf("building provider server: %s", err)
	}
	resp, err := srv.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatalf("GetProviderSchema: %s", err)
	}
	for _, d := range resp.Diagnostics {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			t.Errorf("schema diagnostic: %s - %s", d.Summary, d.Detail)
		}
	}
	wantResources := []string{
		"staticform_form", "staticform_api_key", "staticform_smtp_connection",
		"staticform_email_domain", "staticform_collaborator",
	}
	for _, name := range wantResources {
		if _, ok := resp.ResourceSchemas[name]; !ok {
			t.Errorf("missing resource schema: %s", name)
		}
	}
	wantData := []string{"staticform_form", "staticform_forms", "staticform_api_key_permissions"}
	for _, name := range wantData {
		if _, ok := resp.DataSourceSchemas[name]; !ok {
			t.Errorf("missing data source schema: %s", name)
		}
	}
}

// testAccProtoV6ProviderFactories wires the provider into the acceptance test
// harness under the "staticform" name.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"staticform": providerserver.NewProtocol6WithError(New("test")()),
}

// testAccPreCheck verifies the credentials needed for acceptance tests are set.
// Acceptance tests hit a real StaticForm account, so use an sf_test_ key and,
// optionally, point STATICFORM_BASE_URL at a non-production environment.
func testAccPreCheck(t *testing.T) {
	if os.Getenv("STATICFORM_API_KEY") == "" {
		t.Fatal("STATICFORM_API_KEY must be set for acceptance tests")
	}
}
