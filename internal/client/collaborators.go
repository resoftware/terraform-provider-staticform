package client

import (
	"context"
	"net/http"
	"strings"
)

// CollaboratorPermissions is the permission set granted to a collaborator or
// pending invitation.
type CollaboratorPermissions struct {
	CanViewForm            bool `json:"canViewForm"`
	CanEditForm            bool `json:"canEditForm"`
	CanViewSubmissions     bool `json:"canViewSubmissions"`
	CanEditSubmissions     bool `json:"canEditSubmissions"`
	CanManageCollaborators bool `json:"canManageCollaborators"`
}

// InviteCollaboratorRequest is the invite body.
type InviteCollaboratorRequest struct {
	Email string `json:"email"`
	CollaboratorPermissions
}

// Invitation is a pending invitation as returned by invite/list.
type Invitation struct {
	ID           string `json:"id"`
	Email        string `json:"email"`
	Status       string `json:"status"`
	ExpiresAtUTC string `json:"expiresAtUtc"`
	CollaboratorPermissions
}

// Collaborator is an accepted collaborator on a form.
type Collaborator struct {
	ID      string `json:"id"`
	UserID  string `json:"userId"`
	Email   string `json:"email"`
	IsOwner bool   `json:"isOwner"`
	CollaboratorPermissions
}

// InviteCollaborator sends a collaboration invitation for a form.
func (c *Client) InviteCollaborator(ctx context.Context, formID string, req InviteCollaboratorRequest) (*Invitation, error) {
	var out Invitation
	if err := c.do(ctx, http.MethodPost, "/api/v1/forms/"+formID+"/collaborators/invite", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListInvitations returns pending invitations for a form.
func (c *Client) ListInvitations(ctx context.Context, formID string) ([]Invitation, error) {
	var out []Invitation
	if err := c.do(ctx, http.MethodGet, "/api/v1/forms/"+formID+"/invitations", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListCollaborators returns accepted collaborators for a form (includes owner).
func (c *Client) ListCollaborators(ctx context.Context, formID string) ([]Collaborator, error) {
	var out []Collaborator
	if err := c.do(ctx, http.MethodGet, "/api/v1/forms/"+formID+"/collaborators", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// FindCollaboratorByEmail returns the accepted collaborator matching email, if any.
func (c *Client) FindCollaboratorByEmail(ctx context.Context, formID, email string) (*Collaborator, error) {
	collabs, err := c.ListCollaborators(ctx, formID)
	if err != nil {
		return nil, err
	}
	for i := range collabs {
		if strings.EqualFold(collabs[i].Email, email) && !collabs[i].IsOwner {
			return &collabs[i], nil
		}
	}
	return nil, nil
}

// FindInvitationByEmail returns a pending invitation matching email, if any.
func (c *Client) FindInvitationByEmail(ctx context.Context, formID, email string) (*Invitation, error) {
	invs, err := c.ListInvitations(ctx, formID)
	if err != nil {
		return nil, err
	}
	for i := range invs {
		if strings.EqualFold(invs[i].Email, email) {
			return &invs[i], nil
		}
	}
	return nil, nil
}

// UpdateCollaborator changes an accepted collaborator's permissions.
func (c *Client) UpdateCollaborator(ctx context.Context, formID, userID string, perms CollaboratorPermissions) error {
	return c.do(ctx, http.MethodPut, "/api/v1/forms/"+formID+"/collaborators/"+userID, perms, nil)
}

// RemoveCollaborator removes an accepted collaborator from a form.
func (c *Client) RemoveCollaborator(ctx context.Context, formID, userID string) error {
	return c.do(ctx, http.MethodDelete, "/api/v1/forms/"+formID+"/collaborators/"+userID, nil, nil)
}

// CancelInvitation cancels a pending invitation.
func (c *Client) CancelInvitation(ctx context.Context, formID, invitationID string) error {
	return c.do(ctx, http.MethodDelete, "/api/v1/forms/"+formID+"/invitations/"+invitationID, nil, nil)
}
