package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	providerfw "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/resoftware/terraform-provider-staticform/internal/client"
)

func TestProviderCreationProofEnvironmentAndSensitiveSchema(t *testing.T) {
	ctx := context.Background()
	p := &StaticFormProvider{version: "test"}
	sr := providerfw.SchemaResponse{}
	p.Schema(ctx, providerfw.SchemaRequest{}, &sr)
	for _, name := range []string{"key_creation_password", "key_creation_reauth_token"} {
		attr := sr.Schema.Attributes[name].(schema.StringAttribute)
		if !attr.Sensitive || !attr.Optional {
			t.Fatalf("%s must be optional and sensitive", name)
		}
	}
	for _, fromConfig := range []bool{false, true} {
		t.Run(fmt.Sprint(fromConfig), func(t *testing.T) {
			expected := "environment-password"
			t.Setenv("STATICFORM_KEY_CREATION_PASSWORD", expected)
			t.Setenv("STATICFORM_KEY_CREATION_REAUTH_TOKEN", "")
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					var req client.APIKeyRequest
					json.NewDecoder(r.Body).Decode(&req)
					if req.Password != expected {
						t.Error("provider did not forward selected creation proof")
					}
					fmt.Fprint(w, `{"apiKeyId":"key-1","plainKey":"sf_test_new","formIds":[]}`)
				} else {
					fmt.Fprint(w, `{"items":[{"id":"key-1","scopedPermissions":[],"formIds":[]}],"hasNextPage":false}`)
				}
			}))
			defer srv.Close()
			values := map[string]tftypes.Value{
				"api_key": tftypes.NewValue(tftypes.String, "sf_test_manager"), "base_url": tftypes.NewValue(tftypes.String, srv.URL),
				"key_creation_password": tftypes.NewValue(tftypes.String, nil), "key_creation_reauth_token": tftypes.NewValue(tftypes.String, nil),
			}
			if fromConfig {
				expected = "config-password"
				values["key_creation_password"] = tftypes.NewValue(tftypes.String, expected)
			}
			response := providerfw.ConfigureResponse{}
			p.Configure(ctx, providerfw.ConfigureRequest{Config: tfsdk.Config{Schema: sr.Schema, Raw: tftypes.NewValue(sr.Schema.Type().TerraformType(ctx), values)}}, &response)
			if response.Diagnostics.HasError() {
				t.Fatal(response.Diagnostics)
			}
			c := response.ResourceData.(*client.Client)
			if _, err := c.CreateAPIKey(ctx, client.APIKeyRequest{Name: "CI"}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestAPIKeyResourceNativeStateAndImportedIdentity(t *testing.T) {
	for _, metadataFails := range []bool{false, true} {
		t.Run(fmt.Sprint(metadataFails), func(t *testing.T) {
			id := "d894b1db-21d4-4cc9-bc04-209b309f50c8"
			name := "CI"
			revoked := false
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodPost && r.URL.Path == "/identity/v1/api-keys":
					fmt.Fprintf(w, `{"apiKeyId":%q,"plainKey":"sf_test_created","keyHint":"created","name":"CI","formIds":[]}`, id)
				case r.Method == http.MethodGet:
					if metadataFails {
						w.WriteHeader(http.StatusServiceUnavailable)
						return
					}
					fmt.Fprintf(w, `{"items":[{"id":%q,"name":%q,"keyHint":"created","createdAtUtc":"2026-10-13T12:00:00Z","scopedPermissions":["User.Forms.GetById"],"formIds":[]}],"hasNextPage":false}`, id, name)
				case r.Method == http.MethodPut && r.URL.Path == "/identity/v1/api-keys/"+id+"/permissions":
					var req client.APIKeyUpdateRequest
					json.NewDecoder(r.Body).Decode(&req)
					name = req.Name
					w.WriteHeader(http.StatusNoContent)
				case r.Method == http.MethodDelete && r.URL.Path == "/identity/v1/api-keys/"+id:
					revoked = true
					w.WriteHeader(http.StatusNoContent)
				default:
					t.Errorf("unexpected native request %s %s", r.Method, r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			defer srv.Close()
			ctx := context.Background()
			r := &apiKeyResource{client: client.New(srv.URL, "sf_test_manager", client.WithKeyCreationProof("synthetic-password", ""))}
			sr := resource.SchemaResponse{}
			r.Schema(ctx, resource.SchemaRequest{}, &sr)
			model := apiKeyResourceModel{ID: types.StringUnknown(), Name: types.StringValue("CI"), ExpiresAt: types.StringNull(), Permissions: toStringSet([]string{"Form.GetById"}), FormIDs: toStringSet([]string{}), Key: types.StringUnknown(), KeyHint: types.StringUnknown(), CreatedAt: types.StringUnknown()}
			plan := tfsdk.Plan{Schema: sr.Schema}
			if d := plan.Set(ctx, &model); d.HasError() {
				t.Fatal(d)
			}
			created := resource.CreateResponse{State: tfsdk.State{Schema: sr.Schema}}
			r.Create(ctx, resource.CreateRequest{Plan: plan}, &created)
			if created.Diagnostics.HasError() != metadataFails {
				t.Fatal(created.Diagnostics)
			}
			if d := created.State.Get(ctx, &model); d.HasError() {
				t.Fatal(d)
			}
			if model.ID.ValueString() != id || model.Key.ValueString() != "sf_test_created" {
				t.Fatal("created ID/plaintext lost from Terraform state")
			}
			if metadataFails {
				return
			}
			read := resource.ReadResponse{State: created.State}
			r.Read(ctx, resource.ReadRequest{State: created.State}, &read)
			if read.Diagnostics.HasError() {
				t.Fatal(read.Diagnostics)
			}
			if d := read.State.Get(ctx, &model); d.HasError() {
				t.Fatal(d)
			}
			if model.Key.ValueString() != "sf_test_created" {
				t.Fatal("read discarded the one-time key")
			}
			previous, _ := stringSet(ctx, model.Permissions)
			if len(previous) != 1 || previous[0] != "Form.GetById" {
				t.Fatal("read changed historical scope representation")
			}
			model.Name = types.StringValue("Renamed")
			if d := plan.Set(ctx, &model); d.HasError() {
				t.Fatal(d)
			}
			updated := resource.UpdateResponse{State: read.State}
			r.Update(ctx, resource.UpdateRequest{Plan: plan, State: read.State}, &updated)
			if updated.Diagnostics.HasError() {
				t.Fatal(updated.Diagnostics)
			}
			imported := resource.ImportStateResponse{State: tfsdk.State{Schema: sr.Schema}}
			empty := apiKeyResourceModel{ID: types.StringNull(), Name: types.StringNull(), ExpiresAt: types.StringNull(), Permissions: types.SetNull(types.StringType), FormIDs: types.SetNull(types.StringType), Key: types.StringNull(), KeyHint: types.StringNull(), CreatedAt: types.StringNull()}
			if d := imported.State.Set(ctx, &empty); d.HasError() {
				t.Fatal(d)
			}
			r.ImportState(ctx, resource.ImportStateRequest{ID: id}, &imported)
			importedRead := resource.ReadResponse{State: imported.State}
			r.Read(ctx, resource.ReadRequest{State: imported.State}, &importedRead)
			if importedRead.Diagnostics.HasError() {
				t.Fatal(importedRead.Diagnostics)
			}
			var importedModel apiKeyResourceModel
			if d := importedRead.State.Get(ctx, &importedModel); d.HasError() {
				t.Fatal(d)
			}
			if importedModel.ID.ValueString() != id || !importedModel.Key.IsNull() {
				t.Fatal("import must preserve ID and cannot invent plaintext")
			}
			deleted := resource.DeleteResponse{}
			r.Delete(ctx, resource.DeleteRequest{State: updated.State}, &deleted)
			if deleted.Diagnostics.HasError() || !revoked {
				t.Fatal("native deletion failed")
			}
		})
	}
}

func TestExplicitEmptyPermissionsAreRejectedInsteadOfInheritingOwnerAccess(t *testing.T) {
	ctx := context.Background()
	srv, err := testAccProtoV6ProviderFactories["staticform"]()
	if err != nil {
		t.Fatal(err)
	}
	schemaResp, err := srv.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	typ := schemaResp.ResourceSchemas["staticform_api_key"].ValueType().(tftypes.Object)
	for _, explicitEmpty := range []bool{false, true} {
		values := map[string]tftypes.Value{}
		for name, attrType := range typ.AttributeTypes {
			values[name] = tftypes.NewValue(attrType, nil)
		}
		values["name"] = tftypes.NewValue(tftypes.String, "CI")
		if explicitEmpty {
			values["permissions"] = tftypes.NewValue(typ.AttributeTypes["permissions"], []tftypes.Value{})
		}
		dynamic, err := tfprotov6.NewDynamicValue(typ, tftypes.NewValue(typ, values))
		if err != nil {
			t.Fatal(err)
		}
		response, err := srv.ValidateResourceConfig(ctx, &tfprotov6.ValidateResourceConfigRequest{TypeName: "staticform_api_key", Config: &dynamic})
		if err != nil {
			t.Fatal(err)
		}
		hasError := false
		for _, diagnostic := range response.Diagnostics {
			if diagnostic.Severity == tfprotov6.DiagnosticSeverityError {
				hasError = true
			}
		}
		if hasError != explicitEmpty {
			t.Fatalf("explicit empty=%v validation=%v", explicitEmpty, response.Diagnostics)
		}
	}
}
