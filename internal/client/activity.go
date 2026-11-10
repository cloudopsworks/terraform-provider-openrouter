package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

type ActivityOptions struct {
	MaxItems                              int
	Date, WorkspaceID, APIKeyHash, UserID string
	GroupByWorkspace                      bool
}
type ActivityItem struct {
	Date               string      `json:"date"`
	Model              string      `json:"model"`
	ModelPermaslug     string      `json:"model_permaslug"`
	EndpointID         string      `json:"endpoint_id"`
	ProviderName       string      `json:"provider_name"`
	WorkspaceID        *string     `json:"workspace_id"`
	Usage              json.Number `json:"usage"`
	BYOKUsageInference json.Number `json:"byok_usage_inference"`
	Requests           *int64      `json:"requests"`
	PromptTokens       *int64      `json:"prompt_tokens"`
	CompletionTokens   *int64      `json:"completion_tokens"`
	ReasoningTokens    *int64      `json:"reasoning_tokens"`
	CachedTokens       *int64      `json:"cached_tokens"`
}

func ValidateActivityDate(value string) error {
	date, err := time.Parse("2006-01-02", value)
	if err != nil || date.Format("2006-01-02") != value {
		return fmt.Errorf("activity date must be a real UTC date in YYYY-MM-DD format")
	}
	return nil
}

func (c *Client) GetActivity(ctx context.Context, options ActivityOptions) ([]ActivityItem, error) {
	if err := ValidateActivityDate(options.Date); err != nil {
		return nil, err
	}
	if options.MaxItems == 0 {
		options.MaxItems = 1000
	}
	if options.MaxItems < 1 || options.MaxItems > 10000 {
		return nil, fmt.Errorf("activity max_items must be 1..10000")
	}
	query := url.Values{"date": {options.Date}}
	for k, v := range map[string]string{"workspace_id": options.WorkspaceID, "api_key_hash": options.APIKeyHash, "user_id": options.UserID} {
		if v != "" {
			query.Set(k, v)
		}
	}
	if options.GroupByWorkspace {
		query.Set("group_by", "workspace")
	}
	var resp struct {
		Data []ActivityItem `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/activity", query, nil, &resp); err != nil {
		return nil, err
	}
	if len(resp.Data) > options.MaxItems {
		return nil, fmt.Errorf("activity response exceeds max_items; narrow the workspace/key/user filters or increase the bounded limit")
	}
	if resp.Data == nil {
		return nil, fmt.Errorf("invalid activity response: missing or null data")
	}
	for _, item := range resp.Data {
		if item.Date != options.Date || strings.TrimSpace(item.Model) == "" || strings.TrimSpace(item.ModelPermaslug) == "" || strings.TrimSpace(item.EndpointID) == "" || strings.TrimSpace(item.ProviderName) == "" || item.Usage.String() == "" || item.BYOKUsageInference.String() == "" {
			return nil, fmt.Errorf("invalid activity response: missing metadata or date outside requested window")
		}
		for _, n := range []*int64{item.Requests, item.PromptTokens, item.CompletionTokens, item.ReasoningTokens, item.CachedTokens} {
			if n == nil || *n < 0 {
				return nil, fmt.Errorf("invalid activity response: missing or negative counter")
			}
		}
		if options.GroupByWorkspace && (item.WorkspaceID == nil || *item.WorkspaceID == "") {
			return nil, fmt.Errorf("invalid activity response: grouped row missing workspace_id")
		}
		if options.WorkspaceID != "" && item.WorkspaceID != nil && *item.WorkspaceID != options.WorkspaceID {
			return nil, fmt.Errorf("invalid activity response: unexpected workspace_id")
		}
	}
	sort.SliceStable(resp.Data, func(i, j int) bool {
		a, b := resp.Data[i], resp.Data[j]
		ak, bk := "", ""
		if a.WorkspaceID != nil {
			ak = *a.WorkspaceID
		}
		if b.WorkspaceID != nil {
			bk = *b.WorkspaceID
		}
		return strings.Join([]string{a.Date, ak, a.Model, a.EndpointID, a.ModelPermaslug, a.ProviderName}, "\x00") < strings.Join([]string{b.Date, bk, b.Model, b.EndpointID, b.ModelPermaslug, b.ProviderName}, "\x00")
	})
	return resp.Data, nil
}

func (item *ActivityItem) UnmarshalJSON(data []byte) error {
	type plain ActivityItem
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	for _, name := range []string{"usage", "byok_usage_inference"} {
		value := raw[name]
		if len(value) == 0 || string(value) == "null" || value[0] == '"' {
			return fmt.Errorf("invalid activity response: %s must be a JSON number", name)
		}
	}
	*item = ActivityItem(decoded)
	return nil
}
