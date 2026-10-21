package client

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"
)

type WorkspaceBudget struct {
	ID            string  `json:"id"`
	WorkspaceID   string  `json:"workspace_id"`
	LimitUSD      float64 `json:"limit_usd"`
	ResetInterval string  `json:"reset_interval"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
}

type workspaceBudgetRequest struct {
	LimitUSD float64 `json:"limit_usd"`
}

// UnmarshalJSON normalizes OpenRouter's explicit null lifetime reset_interval
// to the provider's canonical lifetime identifier. Other intervals remain strings.
func (b *WorkspaceBudget) UnmarshalJSON(data []byte) error {
	var raw struct {
		ID          string  `json:"id"`
		WorkspaceID string  `json:"workspace_id"`
		LimitUSD    float64 `json:"limit_usd"`
		CreatedAt   string  `json:"created_at"`
		UpdatedAt   string  `json:"updated_at"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	intervalJSON, ok := fields["reset_interval"]
	if !ok {
		return fmt.Errorf("openrouter workspace budget response missing reset_interval")
	}
	interval := "lifetime"
	if string(intervalJSON) != "null" {
		if err := json.Unmarshal(intervalJSON, &interval); err != nil {
			return fmt.Errorf("decode workspace budget reset_interval: %w", err)
		}
	}
	*b = WorkspaceBudget{
		ID: raw.ID, WorkspaceID: raw.WorkspaceID, LimitUSD: raw.LimitUSD,
		ResetInterval: interval, CreatedAt: raw.CreatedAt, UpdatedAt: raw.UpdatedAt,
	}
	return nil
}

func (b WorkspaceBudget) Validate() error {
	if strings.TrimSpace(b.ID) == "" || strings.TrimSpace(b.WorkspaceID) == "" || strings.TrimSpace(b.ResetInterval) == "" {
		return fmt.Errorf("openrouter workspace budget response missing required fields")
	}
	if b.LimitUSD <= 0 || math.IsNaN(b.LimitUSD) || math.IsInf(b.LimitUSD, 0) {
		return fmt.Errorf("openrouter workspace budget response has invalid limit_usd")
	}
	if !isWorkspaceBudgetInterval(b.ResetInterval) {
		return fmt.Errorf("openrouter workspace budget response has unknown reset_interval %q", b.ResetInterval)
	}
	return nil
}

func isWorkspaceBudgetInterval(interval string) bool {
	switch interval {
	case "daily", "weekly", "monthly", "lifetime":
		return true
	default:
		return false
	}
}

func (c *Client) ListWorkspaceBudgets(ctx context.Context, workspace string) ([]WorkspaceBudget, error) {
	var response listResponse[WorkspaceBudget]
	if err := c.do(ctx, http.MethodGet, "/workspaces/"+url.PathEscape(workspace)+"/budgets", nil, nil, &response); err != nil {
		return nil, err
	}
	for _, budget := range response.Data {
		if err := budget.Validate(); err != nil {
			return nil, err
		}
	}
	return response.Data, nil
}

func (c *Client) PutWorkspaceBudget(ctx context.Context, workspace, interval string, limit float64) (*WorkspaceBudget, error) {
	var response singleResponse[WorkspaceBudget]
	if err := c.do(ctx, http.MethodPut, "/workspaces/"+url.PathEscape(workspace)+"/budgets/"+url.PathEscape(interval), nil, workspaceBudgetRequest{LimitUSD: limit}, &response); err != nil {
		return nil, err
	}
	if err := response.Data.Validate(); err != nil {
		return &response.Data, err
	}
	if response.Data.ResetInterval != interval {
		return &response.Data, fmt.Errorf("openrouter workspace budget response interval %q does not match requested %q", response.Data.ResetInterval, interval)
	}
	if response.Data.LimitUSD != limit {
		return &response.Data, fmt.Errorf("openrouter workspace budget response limit_usd %v does not match requested %v", response.Data.LimitUSD, limit)
	}
	return &response.Data, nil
}

func (c *Client) DeleteWorkspaceBudget(ctx context.Context, workspace, interval string) error {
	return c.do(ctx, http.MethodDelete, "/workspaces/"+url.PathEscape(workspace)+"/budgets/"+url.PathEscape(interval), nil, nil, nil)
}
