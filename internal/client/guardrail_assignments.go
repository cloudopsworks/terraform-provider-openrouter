package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// GuardrailKeyAssignment is a key returned by the guardrail key-assignment API.
// The API has returned both hash and key_hash in different responses; accept either
// while exposing a single stable KeyHash value to callers.
type GuardrailKeyAssignment struct {
	Hash    string `json:"hash"`
	KeyHash string `json:"key_hash"`
}

func (a GuardrailKeyAssignment) normalizedKeyHash() string {
	if a.KeyHash != "" {
		return a.KeyHash
	}
	return a.Hash
}

type guardrailKeyAssignmentRequest struct {
	KeyHashes []string `json:"key_hashes"`
}

// GuardrailKeyAssignmentBinding identifies a key's current guardrail assignment.
type GuardrailKeyAssignmentBinding struct {
	GuardrailID string `json:"guardrail_id"`
	KeyHash     string `json:"key_hash"`
}

// ListGuardrailKeyAssignmentBindings returns all guardrail/key bindings. It is
// used as a safety preflight because assigning a key replaces its prior owner.
func (c *Client) ListGuardrailKeyAssignmentBindings(ctx context.Context) ([]GuardrailKeyAssignmentBinding, error) {
	items := make([]GuardrailKeyAssignmentBinding, 0)
	offset := 0
	for page := 0; page < 10000; page++ {
		query := url.Values{}
		query.Set("offset", fmt.Sprintf("%d", offset))
		query.Set("limit", "100")
		var resp listResponse[GuardrailKeyAssignmentBinding]
		if err := c.do(ctx, http.MethodGet, "/guardrails/assignments/keys", query, nil, &resp); err != nil {
			return nil, err
		}
		for _, item := range resp.Data {
			item.GuardrailID = strings.TrimSpace(item.GuardrailID)
			item.KeyHash = strings.TrimSpace(item.KeyHash)
			if item.GuardrailID == "" || item.KeyHash == "" {
				return nil, fmt.Errorf("openrouter guardrail key assignment response missing required fields")
			}
			items = append(items, item)
		}
		if len(resp.Data) == 0 || len(resp.Data) < 100 || (resp.TotalCount > 0 && len(items) >= resp.TotalCount) {
			return items, nil
		}
		offset += len(resp.Data)
	}
	return nil, fmt.Errorf("openrouter guardrail key assignment bindings pagination exceeded 10000 pages")
}

// ListGuardrailKeyAssignments returns every API key currently assigned to a guardrail.
func (c *Client) ListGuardrailKeyAssignments(ctx context.Context, guardrailID string) ([]string, error) {
	items := make([]string, 0)
	offset := 0
	for page := 0; page < 10000; page++ {
		query := url.Values{}
		query.Set("offset", fmt.Sprintf("%d", offset))
		query.Set("limit", "100")

		var resp listResponse[GuardrailKeyAssignment]
		if err := c.do(ctx, http.MethodGet, "/guardrails/"+url.PathEscape(guardrailID)+"/assignments/keys", query, nil, &resp); err != nil {
			return nil, err
		}
		for _, item := range resp.Data {
			hash := strings.TrimSpace(item.normalizedKeyHash())
			if hash == "" {
				return nil, fmt.Errorf("openrouter guardrail key assignment response missing required field key_hash")
			}
			items = append(items, hash)
		}
		if len(resp.Data) == 0 || len(resp.Data) < 100 || (resp.TotalCount > 0 && len(items) >= resp.TotalCount) {
			return items, nil
		}
		offset += len(resp.Data)
	}
	return nil, fmt.Errorf("openrouter guardrail key assignments pagination exceeded 10000 pages")
}

// AssignGuardrailKeys assigns the keys to guardrailID. OpenRouter replaces any
// prior guardrail assignment for a key.
func (c *Client) AssignGuardrailKeys(ctx context.Context, guardrailID string, keyHashes []string) error {
	return c.do(ctx, http.MethodPost, "/guardrails/"+url.PathEscape(guardrailID)+"/assignments/keys", nil, guardrailKeyAssignmentRequest{KeyHashes: keyHashes}, nil)
}

// RemoveGuardrailKeys removes the keys from guardrailID.
func (c *Client) RemoveGuardrailKeys(ctx context.Context, guardrailID string, keyHashes []string) error {
	return c.do(ctx, http.MethodPost, "/guardrails/"+url.PathEscape(guardrailID)+"/assignments/keys/remove", nil, guardrailKeyAssignmentRequest{KeyHashes: keyHashes}, nil)
}
