package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type SCIMGroup struct {
	ID             string  `json:"id"`
	OrganizationID string  `json:"organization_id"`
	ExternalID     *string `json:"external_id"`
	DisplayName    string  `json:"display_name"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
}
type SCIMGroupMapping struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organization_id"`
	SCIMGroupID    string `json:"scim_group_id"`
	WorkspaceID    string `json:"workspace_id"`
	Role           string `json:"role"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}
type SCIMGroupMappingCreateRequest struct {
	SCIMGroupID string `json:"scim_group_id"`
	WorkspaceID string `json:"workspace_id"`
	Role        string `json:"role"`
}

func identityPart(value string) bool {
	return value != "" && strings.TrimSpace(value) == value && !strings.ContainsAny(value, "/\\?#")
}
func (g SCIMGroup) validate() error {
	if !identityPart(g.ID) || !identityPart(g.OrganizationID) || strings.TrimSpace(g.DisplayName) == "" || g.CreatedAt == "" || g.UpdatedAt == "" {
		return fmt.Errorf("invalid OpenRouter SCIM group identity")
	}
	return nil
}
func (m SCIMGroupMapping) validate() error {
	if !identityPart(m.ID) || !identityPart(m.OrganizationID) || !identityPart(m.SCIMGroupID) || !identityPart(m.WorkspaceID) || (m.Role != "admin" && m.Role != "member") || m.CreatedAt == "" || m.UpdatedAt == "" {
		return fmt.Errorf("invalid OpenRouter SCIM mapping identity or role")
	}
	return nil
}

// These management lists require complete envelopes. A missing/null list must
// not be mistaken for an empty account when deciding whether Create may adopt it.
func listIdentityItems[T any](ctx context.Context, c *Client, path string, query url.Values, validate func(T) error, identity func(T) string) ([]T, error) {
	if query == nil {
		query = url.Values{}
	}
	items := make([]T, 0)
	seen := map[string]bool{}
	total := -1
	for offset := 0; ; {
		query.Set("offset", strconv.Itoa(offset))
		query.Set("limit", "100")
		var resp struct {
			Data       *[]T `json:"data"`
			TotalCount *int `json:"total_count"`
		}
		if err := c.do(ctx, http.MethodGet, path, query, nil, &resp); err != nil {
			return nil, err
		}
		if resp.Data == nil || resp.TotalCount == nil || *resp.TotalCount < 0 {
			return nil, fmt.Errorf("invalid OpenRouter membership list envelope")
		}
		if total < 0 {
			total = *resp.TotalCount
		} else if total != *resp.TotalCount {
			return nil, fmt.Errorf("OpenRouter membership list changed while paginating; retry refresh")
		}
		if total > 100000 {
			return nil, fmt.Errorf("OpenRouter membership list exceeds safe pagination bound")
		}
		for _, item := range *resp.Data {
			if err := validate(item); err != nil {
				return nil, err
			}
			id := identity(item)
			if seen[id] {
				return nil, fmt.Errorf("duplicate OpenRouter membership identity while paginating")
			}
			seen[id] = true
			items = append(items, item)
		}
		if len(items) > total {
			return nil, fmt.Errorf("OpenRouter membership list exceeds reported total")
		}
		if len(items) == total {
			sort.Slice(items, func(i, j int) bool { return identity(items[i]) < identity(items[j]) })
			return items, nil
		}
		if len(*resp.Data) == 0 {
			return nil, fmt.Errorf("OpenRouter membership list ended before reported total")
		}
		offset += len(*resp.Data)
	}
}
func (c *Client) ListSCIMGroups(ctx context.Context, displayName, externalID string) ([]SCIMGroup, error) {
	q := url.Values{}
	if displayName != "" {
		q.Set("display_name", displayName)
	}
	if externalID != "" {
		q.Set("external_id", externalID)
	}
	return listIdentityItems(ctx, c, "/scim/groups", q, func(v SCIMGroup) error {
		if err := v.validate(); err != nil {
			return err
		}
		if displayName != "" && v.DisplayName != displayName {
			return fmt.Errorf("SCIM group response does not match display_name filter")
		}
		if externalID != "" && (v.ExternalID == nil || *v.ExternalID != externalID) {
			return fmt.Errorf("SCIM group response does not match external_id filter")
		}
		return nil
	}, func(v SCIMGroup) string { return v.ID })
}
func (c *Client) ListSCIMGroupMappings(ctx context.Context) ([]SCIMGroupMapping, error) {
	return listIdentityItems(ctx, c, "/scim/group-mappings", nil, func(v SCIMGroupMapping) error { return v.validate() }, func(v SCIMGroupMapping) string { return v.ID })
}
func (c *Client) GetSCIMGroupMapping(ctx context.Context, id string) (*SCIMGroupMapping, error) {
	if !identityPart(id) {
		return nil, fmt.Errorf("invalid SCIM mapping ID")
	}
	var resp singleResponse[SCIMGroupMapping]
	if err := c.do(ctx, http.MethodGet, "/scim/group-mappings/"+url.PathEscape(id), nil, nil, &resp); err != nil {
		return nil, err
	}
	if err := resp.Data.validate(); err != nil {
		return nil, err
	}
	if resp.Data.ID != id {
		return nil, fmt.Errorf("SCIM mapping response changed requested identity")
	}
	return &resp.Data, nil
}
func (c *Client) CreateSCIMGroupMapping(ctx context.Context, body SCIMGroupMappingCreateRequest) (*SCIMGroupMapping, error) {
	if !identityPart(body.SCIMGroupID) || !identityPart(body.WorkspaceID) || (body.Role != "admin" && body.Role != "member") {
		return nil, fmt.Errorf("invalid SCIM mapping identity or role")
	}
	var resp singleResponse[json.RawMessage]
	if err := c.do(ctx, http.MethodPost, "/scim/group-mappings", nil, body, &resp); err != nil {
		return nil, err
	}
	var mapping SCIMGroupMapping
	if err := json.Unmarshal(resp.Data, &mapping); err != nil || mapping.validate() != nil || mapping.SCIMGroupID != body.SCIMGroupID || mapping.WorkspaceID != body.WorkspaceID || mapping.Role != body.Role {
		// A successful POST may already have granted access. Keep a recoverable
		// identifier in the diagnostic only, never adopt an unvalidated object.
		var candidate struct {
			ID          string `json:"id"`
			SCIMGroupID string `json:"scim_group_id"`
			WorkspaceID string `json:"workspace_id"`
		}
		if err := json.Unmarshal(resp.Data, &candidate); err == nil && scimRecoveryMappingID.MatchString(candidate.ID) && candidate.SCIMGroupID == body.SCIMGroupID && candidate.WorkspaceID == body.WorkspaceID {
			return nil, fmt.Errorf("SCIM mapping creation returned an invalid successful response; candidate mapping ID %q matches the requested group and workspace. Do not retry creation blindly. Inspect and verify this mapping, then import it if appropriate; no mapping was adopted", candidate.ID)
		}
		return nil, fmt.Errorf("SCIM mapping creation returned an invalid successful response without a trustworthy matching candidate ID. Reconcile the requested group/workspace mapping before retrying creation; no mapping was adopted")
	}
	return &mapping, nil
}

// Recovery diagnostics may echo only an API-schema UUID, not arbitrary response
// strings. This is deliberately stricter than ordinary identifier validation.
var scimRecoveryMappingID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func (c *Client) UpdateSCIMGroupMapping(ctx context.Context, id, role string) (*SCIMGroupMapping, error) {
	if !identityPart(id) || (role != "admin" && role != "member") {
		return nil, fmt.Errorf("invalid SCIM mapping ID or role")
	}
	var resp singleResponse[SCIMGroupMapping]
	if err := c.do(ctx, http.MethodPatch, "/scim/group-mappings/"+url.PathEscape(id), nil, struct {
		Role string `json:"role"`
	}{role}, &resp); err != nil {
		return nil, err
	}
	if err := resp.Data.validate(); err != nil {
		return nil, err
	}
	if resp.Data.ID != id || resp.Data.Role != role {
		return nil, fmt.Errorf("SCIM mapping update response does not match request")
	}
	return &resp.Data, nil
}
func (c *Client) DeleteSCIMGroupMapping(ctx context.Context, id string, keepMembers bool) error {
	if !identityPart(id) {
		return fmt.Errorf("invalid SCIM mapping ID")
	}
	q := url.Values{"keep_members": {strconv.FormatBool(keepMembers)}}
	var resp struct {
		Deleted *bool `json:"deleted"`
	}
	if err := c.do(ctx, http.MethodDelete, "/scim/group-mappings/"+url.PathEscape(id), q, nil, &resp); err != nil {
		return err
	}
	if resp.Deleted == nil || !*resp.Deleted {
		return fmt.Errorf("SCIM mapping deletion was not confirmed")
	}
	return nil
}
