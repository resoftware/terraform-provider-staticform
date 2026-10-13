package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

const keyID = "c6baa121-6c66-450d-a323-e80557e56f0b"

func TestNativeAPIKeyLifecycle(t *testing.T) {
	permissions := []string{"User.Forms.GetById"}
	formIDs := []string{"bb74b738-2f4d-4295-8e4a-1d1c0e578821"}
	name, deleted := "CI", false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "sf_test_manager" {
			t.Error("management credential was not sent")
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == nativeAPIKeysPath:
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatal(err)
			}
			if req["password"] != "synthetic-password" || req["permissions"] != nil {
				t.Errorf("wrong native create contract")
			}
			if !reflect.DeepEqual(req["scopedPermissions"], []any{"User.Forms.GetById"}) {
				t.Error("legacy scope was not translated")
			}
			fmt.Fprintf(w, `{"apiKeyId":%q,"plainKey":"sf_test_created","keyHint":"created","name":"CI","expiresAtUtc":null,"formIds":[%q]}`, keyID, formIDs[0])
		case r.Method == http.MethodGet && r.URL.Path == nativeAPIKeysPath:
			if r.URL.Query().Get("pageSize") != "100" {
				t.Error("missing native page size")
			}
			if r.URL.Query().Get("page") == "1" && !deleted {
				fmt.Fprint(w, `{"items":[{"id":"earlier-key","name":"Earlier","scopedPermissions":[],"formIds":[]}],"hasNextPage":true}`)
			} else {
				items := []APIKeyListItem{}
				if !deleted {
					items = append(items, APIKeyListItem{ID: keyID, Name: name, KeyHint: "created", IsActive: true, CreatedAtUTC: "2026-10-13T12:00:00Z", Permissions: permissions, FormIDs: formIDs})
				}
				json.NewEncoder(w).Encode(map[string]any{"items": items, "hasNextPage": false})
			}
		case r.Method == http.MethodPut && r.URL.Path == nativeAPIKeysPath+"/"+keyID+"/permissions":
			var req APIKeyUpdateRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(req.Permissions, []string{"User.Submissions.ListAll"}) {
				t.Error("wrong native update scopes")
			}
			if req.FormIDs == nil {
				t.Error("explicit empty form scopes must serialize as []")
			}
			name, permissions, formIDs = req.Name, req.Permissions, req.FormIDs
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && r.URL.Path == nativeAPIKeysPath+"/"+keyID+"/rotate":
			fmt.Fprint(w, `{"plainKey":"sf_test_rotated","keyHint":"rotated"}`)
		case r.Method == http.MethodDelete && r.URL.Path == nativeAPIKeysPath+"/"+keyID:
			deleted = true
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && r.URL.Path == "/identity/v1/users/me/permissions":
			fmt.Fprint(w, `[{"name":"Admin.Users.Delete","isReadOnly":false},{"name":"User.Forms.GetById","isReadOnly":true},{"name":"User.ApiKeys.Create","isReadOnly":false}]`)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := New(srv.URL, "sf_test_manager", WithKeyCreationProof("synthetic-password", ""))
	ctx := context.Background()
	created, err := c.CreateAPIKey(ctx, APIKeyRequest{Name: name, Permissions: []string{"Form.GetById"}, FormIDs: formIDs})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != keyID || created.APIKey != "sf_test_created" || created.CreatedAtUTC != "2026-10-13T12:00:00Z" {
		t.Fatalf("native metadata was not hydrated: %#v", created)
	}
	if !reflect.DeepEqual(created.Permissions, permissions) {
		t.Fatal("permission metadata missing")
	}
	if err := c.UpdateAPIKey(ctx, keyID, APIKeyUpdateRequest{Name: "Renamed", Permissions: []string{"Submission.ListAll"}, FormIDs: []string{}}); err != nil {
		t.Fatal(err)
	}
	item, err := c.GetAPIKey(ctx, keyID)
	if err != nil || item.Name != "Renamed" || len(item.FormIDs) != 0 {
		t.Fatalf("update/read failed: %v", err)
	}
	rotated, err := c.RotateAPIKey(ctx, keyID)
	if err != nil || rotated.APIKey != "sf_test_rotated" || rotated.ID != keyID {
		t.Fatalf("rotation failed: %v", err)
	}
	names, err := c.ListAPIKeyPermissions(ctx)
	if err != nil || !reflect.DeepEqual(names, []string{"User.Forms.GetById", "User.ApiKeys.Create"}) {
		t.Fatalf("native permission discovery failed: %v", err)
	}
	if err := c.RevokeAPIKey(ctx, keyID); err != nil {
		t.Fatal(err)
	}
	if _, err := c.GetAPIKey(ctx, keyID); !IsNotFound(err) {
		t.Fatalf("deleted key still appears: %v", err)
	}
}

func TestAPIKeyCreationProof(t *testing.T) {
	for _, useReauth := range []bool{false, true} {
		t.Run(fmt.Sprint(useReauth), func(t *testing.T) {
			posted := false
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					var req APIKeyRequest
					json.NewDecoder(r.Body).Decode(&req)
					if useReauth && (req.ReauthToken != "synthetic-proof" || req.Password != "") {
						t.Error("reauth proof was not forwarded")
					}
					if !useReauth && (req.Password != "synthetic-password" || req.ReauthToken != "") {
						t.Error("password proof was not forwarded")
					}
					posted = true
					fmt.Fprintf(w, `{"apiKeyId":%q,"plainKey":"sf_test_created","formIds":[]}`, keyID)
				} else {
					fmt.Fprintf(w, `{"items":[{"id":%q,"scopedPermissions":[],"formIds":[]}],"hasNextPage":false}`, keyID)
				}
			}))
			defer srv.Close()
			option := WithKeyCreationProof("synthetic-password", "")
			if useReauth {
				option = WithKeyCreationProof("", "synthetic-proof")
			}
			if _, err := New(srv.URL, "sf_test_manager", option).CreateAPIKey(context.Background(), APIKeyRequest{Name: "CI"}); err != nil {
				t.Fatal(err)
			}
			if !posted {
				t.Fatal("creation was not performed")
			}
		})
	}
}

func TestMissingCreationProofDoesNotContactAPI(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests++ }))
	defer srv.Close()
	_, err := New(srv.URL, "sf_test_manager").CreateAPIKey(context.Background(), APIKeyRequest{Name: "CI"})
	if err == nil || requests != 0 {
		t.Fatalf("missing proof was not rejected locally: %v", err)
	}
}

func TestInvalidProofIsNotRetriedOrExposed(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"title":"Reauthentication required","detail":"The proof is invalid or expired."}`)
	}))
	defer srv.Close()
	_, err := New(srv.URL, "sf_test_manager", WithKeyCreationProof("synthetic-password", "synthetic-proof")).CreateAPIKey(context.Background(), APIKeyRequest{Name: "CI"})
	if err == nil || requests != 1 || strings.Contains(err.Error(), "synthetic-") {
		t.Fatalf("invalid proof handling failed: %v", err)
	}
}

func TestCreatedKeySurvivesMetadataRefreshFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			fmt.Fprintf(w, `{"apiKeyId":%q,"plainKey":"sf_test_created","formIds":[]}`, keyID)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer srv.Close()
	key, err := New(srv.URL, "sf_test_manager", WithKeyCreationProof("synthetic-password", "")).CreateAPIKey(context.Background(), APIKeyRequest{Name: "CI"})
	if err == nil || key == nil || key.ID != keyID || key.APIKey != "sf_test_created" {
		t.Fatalf("created key identity/material lost: %v", err)
	}
	if strings.Contains(err.Error(), "sf_test_created") {
		t.Fatal("plaintext leaked into diagnostics")
	}
}

func TestRejectsMalformedNativePagination(t *testing.T) {
	for _, body := range []string{`{"apiKeys":[]}`, `{"items":[],"hasNextPage":true}`} {
		t.Run(body, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
			defer srv.Close()
			if _, err := New(srv.URL, "sf_test_manager").ListAPIKeys(context.Background()); err == nil {
				t.Fatal("malformed native envelope accepted")
			}
		})
	}
}

func TestLegacyPermissionNamesArePreservedOnlyWhenEffectiveScopesMatch(t *testing.T) {
	previous := []string{"Form.GetById", "User.ManageApiKeys"}
	native := NativeAPIKeyPermissions(previous)
	if len(native) != 6 {
		t.Fatal("management permission did not expand to all five native key operations")
	}
	if !reflect.DeepEqual(PreserveAPIKeyPermissionNames(previous, native), previous) {
		t.Fatal("existing Terraform permission representation changed")
	}
	if !reflect.DeepEqual(PreserveAPIKeyPermissionNames(previous, []string{"User.Forms.GetById"}), []string{"User.Forms.GetById"}) {
		t.Fatal("permission removal was hidden")
	}
	if NativeAPIKeyPermissions(nil) != nil {
		t.Fatal("omitted permissions lost inheritance representation")
	}
}
