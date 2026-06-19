package client

import (
	"context"
	"net/http"
)

// ── SMTP connections ────────────────────────────────────────────────────────

// SMTPConnectionRequest is the create body for a BYO-SMTP connection. Provider
// and Security are string enum values (e.g. "Resend", "StartTls").
type SMTPConnectionRequest struct {
	Label     string `json:"label"`
	Provider  string `json:"provider"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Security  string `json:"security"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	FromEmail string `json:"fromEmail"`
	FromName  string `json:"fromName"`
}

// SMTPConnection is the API representation of an SMTP connection. The password
// is never returned.
type SMTPConnection struct {
	ID           string `json:"id"`
	Label        string `json:"label"`
	Provider     string `json:"provider"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	Security     string `json:"security"`
	FromEmail    string `json:"fromEmail"`
	FromName     string `json:"fromName"`
	CreatedAtUTC string `json:"createdAtUtc"`
}

// CreateSMTPConnection creates an SMTP connection (the server live-tests it).
func (c *Client) CreateSMTPConnection(ctx context.Context, req SMTPConnectionRequest) (*SMTPConnection, error) {
	var out SMTPConnection
	if err := c.do(ctx, http.MethodPost, "/api/v1/integrations/smtp/connections", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListSMTPConnections returns all SMTP connections for the user.
func (c *Client) ListSMTPConnections(ctx context.Context) ([]SMTPConnection, error) {
	var env struct {
		Connections []SMTPConnection `json:"connections"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/v1/integrations/smtp/connections", nil, &env); err != nil {
		return nil, err
	}
	return env.Connections, nil
}

// GetSMTPConnection finds a connection by ID via the list endpoint.
func (c *Client) GetSMTPConnection(ctx context.Context, id string) (*SMTPConnection, error) {
	conns, err := c.ListSMTPConnections(ctx)
	if err != nil {
		return nil, err
	}
	for i := range conns {
		if conns[i].ID == id {
			return &conns[i], nil
		}
	}
	return nil, &APIError{StatusCode: http.StatusNotFound, Title: "SMTP connection not found"}
}

// DeleteSMTPConnection deletes an SMTP connection.
func (c *Client) DeleteSMTPConnection(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/api/v1/integrations/smtp/connections/"+id, nil, nil)
}

// ── Email domains ───────────────────────────────────────────────────────────

// DNSRecord is a DNS record the user must publish (DKIM or MAIL FROM).
type DNSRecord struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value string `json:"value"`
}

// EmailDomainRequest is the create body for a managed email domain.
type EmailDomainRequest struct {
	DomainName        string  `json:"domainName"`
	MailFromSubdomain *string `json:"mailFromSubdomain,omitempty"`
}

// EmailDomain is the API representation of a managed sending domain.
type EmailDomain struct {
	ID                   string      `json:"id"`
	DomainName           string      `json:"domainName"`
	Status               string      `json:"status"`
	DkimRecords          []DNSRecord `json:"dkimRecords"`
	MailFromDomain       *string     `json:"mailFromDomain"`
	MailFromRecords      []DNSRecord `json:"mailFromRecords"`
	SuggestedDmarcRecord *string     `json:"suggestedDmarcRecord"`
	DmarcStatus          string      `json:"dmarcStatus"`
	CreatedAtUTC         string      `json:"createdAtUtc"`
}

// CreateEmailDomain registers a sending domain. DKIM records are returned in a
// Pending state; verification happens asynchronously via DNS.
func (c *Client) CreateEmailDomain(ctx context.Context, req EmailDomainRequest) (*EmailDomain, error) {
	var out EmailDomain
	if err := c.do(ctx, http.MethodPost, "/api/v1/integrations/email-domains", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListEmailDomains returns all sending domains for the user.
func (c *Client) ListEmailDomains(ctx context.Context) ([]EmailDomain, error) {
	var env struct {
		Domains []EmailDomain `json:"domains"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/v1/integrations/email-domains", nil, &env); err != nil {
		return nil, err
	}
	return env.Domains, nil
}

// GetEmailDomain finds a domain by ID via the list endpoint.
func (c *Client) GetEmailDomain(ctx context.Context, id string) (*EmailDomain, error) {
	domains, err := c.ListEmailDomains(ctx)
	if err != nil {
		return nil, err
	}
	for i := range domains {
		if domains[i].ID == id {
			return &domains[i], nil
		}
	}
	return nil, &APIError{StatusCode: http.StatusNotFound, Title: "email domain not found"}
}

// DeleteEmailDomain removes a sending domain.
func (c *Client) DeleteEmailDomain(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/api/v1/integrations/email-domains/"+id, nil, nil)
}
