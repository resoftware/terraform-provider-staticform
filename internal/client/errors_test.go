package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProblemDetailsSurfaceInDiagnostics(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"title":"Validation Error","detail":"One or more validation errors occurred.","errorCode":"ValidationError","errors":{"Name":["MaximumLengthValidator","NotEmptyValidator"],"Fields[0].Name":["FieldNameFormat"]}}`)
	}))
	defer srv.Close()
	err := New(srv.URL, "key").DeleteForm(context.Background(), "form-1")
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %v", err)
	}
	if apiErr.StatusCode != 400 || apiErr.Title != "Validation Error" || apiErr.ErrorCode != "ValidationError" {
		t.Fatalf("problem details not decoded: %#v", apiErr)
	}
	want := "staticform API error (HTTP 400): Validation Error - One or more validation errors occurred. [ValidationError]\n  Fields[0].Name: FieldNameFormat\n  Name: MaximumLengthValidator, NotEmptyValidator"
	if err.Error() != want {
		t.Fatalf("unexpected message:\n%s", err.Error())
	}
}

func TestForbiddenNamesTheMissingPermission(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"title":"Forbidden","status":403,"detail":"Missing permission: User.Integrations.ListNotionConnections","errorCode":"Forbidden"}`)
	}))
	defer srv.Close()
	_, err := New(srv.URL, "key").ListNotionConnections(context.Background())
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.MissingPermission() != "User.Integrations.ListNotionConnections" {
		t.Fatalf("missing permission not reported: %v", err)
	}
	if !strings.Contains(err.Error(), "does not hold the permission User.Integrations.ListNotionConnections") {
		t.Fatalf("diagnostic does not name the permission: %s", err.Error())
	}
}

func TestForbiddenWithoutPermissionNameIsNotMisreported(t *testing.T) {
	cases := map[string]string{
		"scope exceeded": `{"title":"Forbidden","detail":"Missing permission: ApiKeyManagement","errorCode":"ApiKeyScopeExceeded"}`,
		"form scope":     `{"title":"Forbidden","detail":"Missing permission: forms.scope","errorCode":"FormOutsideScope"}`,
		"entitlement":    `{"title":"Forbidden","detail":"Missing entitlement: forms.notion","errorCode":"ForbiddenEntitlement"}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			err := parseAPIError(http.StatusForbidden, []byte(body))
			if err.MissingPermission() != "" {
				t.Fatalf("reported a permission for %s", body)
			}
		})
	}
	scope := parseAPIError(http.StatusForbidden, []byte(cases["scope exceeded"]))
	if !strings.Contains(scope.Error(), "can only create or change keys whose permissions and form scopes it holds itself") {
		t.Fatalf("scope containment hint missing: %s", scope.Error())
	}
}

func TestNonProblemBodiesFallBackToRawText(t *testing.T) {
	err := parseAPIError(http.StatusBadGateway, []byte("<html>Bad gateway</html>"))
	if !strings.Contains(err.Error(), "HTTP 502): <html>Bad gateway</html>") {
		t.Fatalf("raw body missing: %s", err.Error())
	}
	long := parseAPIError(http.StatusBadGateway, []byte(strings.Repeat("x", 2000)))
	if len(long.Raw) > maxRawErrorBody+3 {
		t.Fatalf("raw body not truncated: %d", len(long.Raw))
	}
}

func TestIsNotFoundSeesWrappedErrors(t *testing.T) {
	err := fmt.Errorf("reading: %w", &APIError{StatusCode: http.StatusNotFound})
	if !IsNotFound(err) {
		t.Fatal("wrapped 404 not recognised")
	}
}

func TestUploadErrorsUseProblemDetails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"title":"Validation Error","errorCode":"ValidationError","errors":{"File":["AttachmentMissing"]}}`)
	}))
	defer srv.Close()
	_, err := New(srv.URL, "key").UploadEmailAttachment(context.Background(), "form-1", "a.txt", []byte("x"))
	if err == nil || !strings.Contains(err.Error(), "File: AttachmentMissing") {
		t.Fatalf("upload validation codes not surfaced: %v", err)
	}
}
