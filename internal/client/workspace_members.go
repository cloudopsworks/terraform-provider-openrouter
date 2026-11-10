package client

import (
	"context"
	"fmt"
	"net/url"
)

type WorkspaceMember struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	UserID      string `json:"user_id"`
	Role        string `json:"role"`
	CreatedAt   string `json:"created_at"`
}

func (m WorkspaceMember) validate() error {
	if !identityPart(m.ID) || !identityPart(m.WorkspaceID) || !identityPart(m.UserID) || m.CreatedAt == "" {
		return fmt.Errorf("invalid OpenRouter workspace membership identity")
	}
	switch m.Role {
	case "admin", "member", "viewer", "restricted_viewer":
		return nil
	default:
		return fmt.Errorf("invalid OpenRouter workspace membership role")
	}
}
func (c *Client) ListWorkspaceMembers(ctx context.Context, workspaceID string) ([]WorkspaceMember, error) {
	if !identityPart(workspaceID) {
		return nil, fmt.Errorf("invalid workspace ID")
	}
	return listIdentityItems(ctx, c, "/workspaces/"+url.PathEscape(workspaceID)+"/members", nil, func(m WorkspaceMember) error {
		if err := m.validate(); err != nil {
			return err
		}
		if m.WorkspaceID != workspaceID {
			return fmt.Errorf("workspace membership response changed workspace identity")
		}
		return nil
	}, func(m WorkspaceMember) string { return m.UserID })
}
