package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func pageParams(t *testing.T, r *http.Request, pageParam string) (page, size int) {
	t.Helper()
	page, err := strconv.Atoi(r.URL.Query().Get(pageParam))
	if err != nil {
		t.Fatalf("missing %s: %s", pageParam, r.URL.RawQuery)
	}
	size, err = strconv.Atoi(r.URL.Query().Get("pageSize"))
	if err != nil || size != 100 {
		t.Fatalf("pageSize must be 100: %s", r.URL.RawQuery)
	}
	return page, size
}

func slice(total, page, size int) (from, to int) {
	from = (page - 1) * size
	if from > total {
		from = total
	}
	to = from + size
	if to > total {
		to = total
	}
	return from, to
}

func TestIntegrationListsFollowEveryPage(t *testing.T) {
	const total = 250
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page, size := pageParams(t, r, "page")
		from, to := slice(total, page, size)
		w.Header().Set("X-Total-Count", strconv.Itoa(total))
		switch r.URL.Path {
		case "/api/v1/integrations/notion/connections":
			items := []NotionConnection{}
			for i := from; i < to; i++ {
				items = append(items, NotionConnection{ID: fmt.Sprint(i), Label: "n"})
			}
			json.NewEncoder(w).Encode(map[string]any{"connections": items})
		case "/api/v1/integrations/stripe/connections":
			items := []StripeConnection{}
			for i := from; i < to; i++ {
				items = append(items, StripeConnection{ID: fmt.Sprint(i)})
			}
			json.NewEncoder(w).Encode(map[string]any{"connections": items})
		case "/api/v1/integrations/google/connections":
			items := []GoogleConnection{}
			for i := from; i < to; i++ {
				items = append(items, GoogleConnection{ID: fmt.Sprint(i)})
			}
			json.NewEncoder(w).Encode(map[string]any{"connections": items})
		case "/api/v1/integrations/smtp/connections":
			items := []SMTPConnection{}
			for i := from; i < to; i++ {
				items = append(items, SMTPConnection{ID: fmt.Sprint(i)})
			}
			json.NewEncoder(w).Encode(map[string]any{"connections": items})
		case "/api/v1/integrations/email-domains":
			items := []EmailDomain{}
			for i := from; i < to; i++ {
				items = append(items, EmailDomain{ID: fmt.Sprint(i)})
			}
			json.NewEncoder(w).Encode(map[string]any{"domains": items})
		default:
			t.Errorf("unexpected request %s", r.URL)
		}
	}))
	defer srv.Close()
	c, ctx := New(srv.URL, "key"), context.Background()

	if got, err := c.ListNotionConnections(ctx); err != nil || len(got) != total {
		t.Fatalf("notion: %d %v", len(got), err)
	}
	if got, err := c.ListStripeConnections(ctx); err != nil || len(got) != total {
		t.Fatalf("stripe: %d %v", len(got), err)
	}
	if got, err := c.ListGoogleConnections(ctx); err != nil || len(got) != total {
		t.Fatalf("google: %d %v", len(got), err)
	}
	if got, err := c.ListSMTPConnections(ctx); err != nil || len(got) != total {
		t.Fatalf("smtp: %d %v", len(got), err)
	}
	if got, err := c.ListEmailDomains(ctx); err != nil || len(got) != total {
		t.Fatalf("domains: %d %v", len(got), err)
	}
	if conn, err := c.GetNotionConnection(ctx, "249"); err != nil || conn.ID != "249" {
		t.Fatalf("lookup on the last page failed: %v", err)
	}
}

func TestCollaboratorListsFollowEveryPage(t *testing.T) {
	const total = 120
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page, size := pageParams(t, r, "page")
		from, to := slice(total, page, size)
		w.Header().Set("X-Total-Count", strconv.Itoa(total))
		switch r.URL.Path {
		case "/api/v1/forms/f1/collaborators":
			items := []Collaborator{}
			for i := from; i < to; i++ {
				items = append(items, Collaborator{ID: fmt.Sprint(i), Email: fmt.Sprintf("u%d@example.com", i)})
			}
			json.NewEncoder(w).Encode(items)
		case "/api/v1/forms/f1/invitations":
			items := []Invitation{}
			for i := from; i < to; i++ {
				items = append(items, Invitation{ID: fmt.Sprint(i), Email: fmt.Sprintf("i%d@example.com", i)})
			}
			json.NewEncoder(w).Encode(items)
		default:
			t.Errorf("unexpected request %s", r.URL)
		}
	}))
	defer srv.Close()
	c, ctx := New(srv.URL, "key"), context.Background()

	collaborator, err := c.FindCollaboratorByEmail(ctx, "f1", "U119@example.com")
	if err != nil || collaborator == nil || collaborator.ID != "119" {
		t.Fatalf("collaborator on the second page not found: %v", err)
	}
	invitation, err := c.FindInvitationByEmail(ctx, "f1", "i110@example.com")
	if err != nil || invitation == nil || invitation.ID != "110" {
		t.Fatalf("invitation on the second page not found: %v", err)
	}
}

func TestListsWithoutTotalHeaderStopAtAShortPage(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		page, size := pageParams(t, r, "page")
		from, to := slice(130, page, size)
		items := []Collaborator{}
		for i := from; i < to; i++ {
			items = append(items, Collaborator{ID: fmt.Sprint(i)})
		}
		json.NewEncoder(w).Encode(items)
	}))
	defer srv.Close()
	got, err := New(srv.URL, "key").ListCollaborators(context.Background(), "f1")
	if err != nil || len(got) != 130 || requests != 2 {
		t.Fatalf("got %d items in %d requests: %v", len(got), requests, err)
	}
}

func TestPaginationRejectsAnEmptyPageBeforeTheReportedTotal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Total-Count", "500")
		json.NewEncoder(w).Encode(map[string]any{"connections": []NotionConnection{}})
	}))
	defer srv.Close()
	if _, err := New(srv.URL, "key").ListNotionConnections(context.Background()); err == nil {
		t.Fatal("empty page before the reported total accepted")
	}
}

func TestFormsSubmissionsAndExecutionLogsFollowEveryPage(t *testing.T) {
	const total = 205
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/forms":
			page, size := pageParams(t, r, "pageNumber")
			from, to := slice(total, page, size)
			items := []FormListItem{}
			for i := from; i < to; i++ {
				items = append(items, FormListItem{ID: fmt.Sprint(i)})
			}
			json.NewEncoder(w).Encode(map[string]any{"data": items, "totalCount": total, "pageNumber": page, "pageSize": size})
		case "/api/v1/forms/f1/submissions":
			page, size := pageParams(t, r, "pageNumber")
			from, to := slice(total, page, size)
			items := []Submission{}
			for i := from; i < to; i++ {
				items = append(items, Submission{ID: fmt.Sprint(i)})
			}
			json.NewEncoder(w).Encode(map[string]any{"data": items, "totalCount": total, "pageNumber": page, "pageSize": size})
		case "/api/v1/execution-logs":
			page, size := pageParams(t, r, "page")
			if r.URL.Query().Get("formId") != "f 1" {
				t.Errorf("form filter not sent: %s", r.URL.RawQuery)
			}
			from, to := slice(total, page, size)
			items := []ExecutionLog{}
			for i := from; i < to; i++ {
				items = append(items, ExecutionLog{ID: fmt.Sprint(i)})
			}
			json.NewEncoder(w).Encode(map[string]any{"logs": items, "totalCount": total})
		default:
			t.Errorf("unexpected request %s", r.URL)
		}
	}))
	defer srv.Close()
	c, ctx := New(srv.URL, "key"), context.Background()

	if got, err := c.ListForms(ctx); err != nil || len(got) != total {
		t.Fatalf("forms: %d %v", len(got), err)
	}
	if got, err := c.ListSubmissions(ctx, "f1"); err != nil || len(got) != total {
		t.Fatalf("submissions: %d %v", len(got), err)
	}
	if got, err := c.ListExecutionLogs(ctx, "f 1"); err != nil || len(got) != total {
		t.Fatalf("logs: %d %v", len(got), err)
	}
}
