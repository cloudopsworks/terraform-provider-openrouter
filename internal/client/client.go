package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const DefaultBaseURL = "https://openrouter.ai/api/v1"

type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	userAgent  string
}

type ErrorResponse struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type listResponse[T any] struct {
	Data       []T `json:"data"`
	TotalCount int `json:"total_count"`
}

type singleResponse[T any] struct {
	Data T `json:"data"`
}

func New(apiKey, baseURL, userAgent string, timeout time.Duration) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &Client{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: timeout,
		},
		userAgent: userAgent,
	}
}

func (c *Client) GetCurrentKey(ctx context.Context) (*CurrentKey, error) {
	var resp singleResponse[CurrentKey]
	if err := c.do(ctx, http.MethodGet, "/key", nil, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) CreateAPIKey(ctx context.Context, body APIKeyCreateRequest) (*APIKeyCreateResponse, error) {
	var resp APIKeyCreateResponse
	if err := c.do(ctx, http.MethodPost, "/keys", nil, body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetAPIKey(ctx context.Context, hash string) (*APIKey, error) {
	var resp singleResponse[APIKey]
	if err := c.do(ctx, http.MethodGet, "/keys/"+url.PathEscape(hash), nil, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) ListAPIKeys(ctx context.Context, workspaceID *string, includeDisabled bool) ([]APIKey, error) {
	query := url.Values{}
	if workspaceID != nil && *workspaceID != "" {
		query.Set("workspace_id", *workspaceID)
	}
	if includeDisabled {
		query.Set("include_disabled", "true")
	}
	var resp listResponse[APIKey]
	if err := c.do(ctx, http.MethodGet, "/keys", query, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *Client) UpdateAPIKey(ctx context.Context, hash string, body APIKeyUpdateRequest) (*APIKey, error) {
	var resp singleResponse[APIKey]
	if err := c.do(ctx, http.MethodPatch, "/keys/"+url.PathEscape(hash), nil, body, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) DeleteAPIKey(ctx context.Context, hash string) error {
	return c.do(ctx, http.MethodDelete, "/keys/"+url.PathEscape(hash), nil, nil, nil)
}

func (c *Client) CreateWorkspace(ctx context.Context, body WorkspaceUpsertRequest) (*Workspace, error) {
	var resp singleResponse[Workspace]
	if err := c.do(ctx, http.MethodPost, "/workspaces", nil, body, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) GetWorkspace(ctx context.Context, idOrSlug string) (*Workspace, error) {
	var resp singleResponse[Workspace]
	if err := c.do(ctx, http.MethodGet, "/workspaces/"+url.PathEscape(idOrSlug), nil, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) ListWorkspaces(ctx context.Context) ([]Workspace, error) {
	var resp listResponse[Workspace]
	if err := c.do(ctx, http.MethodGet, "/workspaces", nil, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *Client) UpdateWorkspace(ctx context.Context, id string, body WorkspaceUpsertRequest) (*Workspace, error) {
	var resp singleResponse[Workspace]
	if err := c.do(ctx, http.MethodPatch, "/workspaces/"+url.PathEscape(id), nil, body, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) DeleteWorkspace(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/workspaces/"+url.PathEscape(id), nil, nil, nil)
}

func (c *Client) CreateGuardrail(ctx context.Context, body GuardrailUpsertRequest) (*Guardrail, error) {
	var resp singleResponse[Guardrail]
	if err := c.do(ctx, http.MethodPost, "/guardrails", nil, body, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) GetGuardrail(ctx context.Context, id string) (*Guardrail, error) {
	var resp singleResponse[Guardrail]
	if err := c.do(ctx, http.MethodGet, "/guardrails/"+url.PathEscape(id), nil, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) ListGuardrails(ctx context.Context) ([]Guardrail, error) {
	items := make([]Guardrail, 0)
	offset := 0
	for {
		query := url.Values{}
		query.Set("offset", fmt.Sprintf("%d", offset))
		query.Set("limit", "100")
		var resp listResponse[Guardrail]
		if err := c.do(ctx, http.MethodGet, "/guardrails", query, nil, &resp); err != nil {
			return nil, err
		}
		items = append(items, resp.Data...)
		if len(resp.Data) == 0 || len(resp.Data) < 100 || (resp.TotalCount > 0 && len(items) >= resp.TotalCount) {
			break
		}
		offset += len(resp.Data)
	}
	return items, nil
}

func (c *Client) UpdateGuardrail(ctx context.Context, id string, body GuardrailUpsertRequest) (*Guardrail, error) {
	var resp singleResponse[Guardrail]
	if err := c.do(ctx, http.MethodPatch, "/guardrails/"+url.PathEscape(id), nil, body, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

func (c *Client) DeleteGuardrail(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/guardrails/"+url.PathEscape(id), nil, nil, nil)
}

func (c *Client) ListOrganizationMembers(ctx context.Context) ([]OrganizationMember, error) {
	items := make([]OrganizationMember, 0)
	offset := 0
	for {
		query := url.Values{}
		query.Set("offset", fmt.Sprintf("%d", offset))
		query.Set("limit", "100")
		var resp listResponse[OrganizationMember]
		if err := c.do(ctx, http.MethodGet, "/organization/members", query, nil, &resp); err != nil {
			return nil, err
		}
		items = append(items, resp.Data...)
		if len(resp.Data) == 0 || len(resp.Data) < 100 || (resp.TotalCount > 0 && len(items) >= resp.TotalCount) {
			break
		}
		offset += len(resp.Data)
	}
	return items, nil
}

func (c *Client) ListProviders(ctx context.Context) ([]ProviderInfo, error) {
	var resp listResponse[ProviderInfo]
	if err := c.do(ctx, http.MethodGet, "/providers", nil, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	var bodyReader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(payload)
	}

	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return err
	}
	if len(query) > 0 {
		u.RawQuery = query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
		var apiErr ErrorResponse
		if err := json.Unmarshal(payload, &apiErr); err == nil && apiErr.Error.Message != "" {
			return fmt.Errorf("openrouter API error (%d): %s", resp.StatusCode, apiErr.Error.Message)
		}
		return fmt.Errorf("openrouter API error (%d): %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}

	if out == nil || len(payload) == 0 {
		return nil
	}
	return json.Unmarshal(payload, out)
}
