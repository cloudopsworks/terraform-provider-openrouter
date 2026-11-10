package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// ObservabilityDestination deliberately excludes provider-specific config:
// upstream reads can contain masked or plaintext credentials, neither belongs
// in Terraform state or a later PATCH.
type ObservabilityDestination struct {
	ID                                string   `json:"id"`
	WorkspaceID                       string   `json:"workspace_id"`
	Type                              string   `json:"type"`
	Name                              *string  `json:"name"`
	Enabled                           bool     `json:"enabled"`
	PrivacyMode                       bool     `json:"privacy_mode"`
	SamplingRate                      float64  `json:"sampling_rate"`
	Regions                           []string `json:"regions"`
	APIKeyHashes                      []string `json:"api_key_hashes"`
	BroadcastGenerationCost           bool     `json:"broadcast_generation_cost"`
	BroadcastGenerationIdentity       bool     `json:"broadcast_generation_identity"`
	BroadcastGenerationRequestContext bool     `json:"broadcast_generation_request_context"`
	CreatedAt                         string   `json:"created_at"`
	UpdatedAt                         string   `json:"updated_at"`
}

func (d *ObservabilityDestination) UnmarshalJSON(data []byte) error {
	type plain ObservabilityDestination
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return fmt.Errorf("invalid observability destination response")
	}
	for _, field := range []string{"id", "workspace_id", "type", "name", "enabled", "privacy_mode", "sampling_rate", "regions", "api_key_hashes", "broadcast_generation_cost", "broadcast_generation_identity", "broadcast_generation_request_context", "created_at", "updated_at"} {
		value, ok := fields[field]
		if !ok || (field != "name" && field != "api_key_hashes" && string(value) == "null") {
			return fmt.Errorf("incomplete observability destination response")
		}
	}
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return fmt.Errorf("invalid observability destination metadata")
	}
	*d = ObservabilityDestination(decoded)
	return d.Validate()
}

func (d ObservabilityDestination) Validate() error {
	for _, value := range []string{d.ID, d.WorkspaceID, d.Type, d.CreatedAt, d.UpdatedAt} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("incomplete observability destination metadata")
		}
	}
	if len(d.Regions) == 0 || d.SamplingRate < 0.0001 || d.SamplingRate > 1 {
		return fmt.Errorf("invalid observability destination metadata")
	}
	for _, region := range d.Regions {
		if strings.TrimSpace(region) == "" {
			return fmt.Errorf("invalid observability destination regions")
		}
	}
	for _, hash := range d.APIKeyHashes {
		if strings.TrimSpace(hash) == "" {
			return fmt.Errorf("invalid observability destination key hashes")
		}
	}
	return nil
}

// The provider owns typed outer metadata but config is an explicit, write-only
// merge. Nil Config omits it, while nil APIKeyHashes intentionally clears it.
type ObservabilityDestinationWrite struct {
	WorkspaceID                       string          `json:"workspace_id,omitempty"`
	Type                              string          `json:"type,omitempty"`
	Name                              string          `json:"name"`
	Enabled                           bool            `json:"enabled"`
	PrivacyMode                       bool            `json:"privacy_mode"`
	SamplingRate                      float64         `json:"sampling_rate"`
	Regions                           []string        `json:"regions"`
	APIKeyHashes                      []string        `json:"api_key_hashes"`
	BroadcastGenerationCost           bool            `json:"broadcast_generation_cost"`
	BroadcastGenerationIdentity       bool            `json:"broadcast_generation_identity"`
	BroadcastGenerationRequestContext bool            `json:"broadcast_generation_request_context"`
	Config                            json.RawMessage `json:"config,omitempty"`
}

func (c *Client) CreateObservabilityDestination(ctx context.Context, body ObservabilityDestinationWrite) (*ObservabilityDestination, error) {
	// Decode the successful envelope separately so incomplete metadata cannot
	// discard the identity of an object that the POST already created.
	var resp singleResponse[json.RawMessage]
	if err := c.doSensitive(ctx, http.MethodPost, "/observability/destinations", nil, body, &resp); err != nil {
		return nil, err
	}
	var identity struct {
		ID          string `json:"id"`
		WorkspaceID string `json:"workspace_id"`
		Type        string `json:"type"`
	}
	if err := json.Unmarshal(resp.Data, &identity); err != nil || strings.TrimSpace(identity.ID) == "" || identity.WorkspaceID != body.WorkspaceID || identity.Type != body.Type {
		return nil, fmt.Errorf("observability destination create response identity could not be trusted; manual reconciliation may be required")
	}
	var destination ObservabilityDestination
	if err := json.Unmarshal(resp.Data, &destination); err != nil {
		// Return only the scoped identity, never partial metadata or config.
		return &ObservabilityDestination{ID: identity.ID, WorkspaceID: identity.WorkspaceID, Type: identity.Type}, fmt.Errorf("invalid observability destination create metadata")
	}
	return &destination, nil
}
func (c *Client) GetObservabilityDestination(ctx context.Context, id string) (*ObservabilityDestination, error) {
	var resp singleResponse[ObservabilityDestination]
	if err := c.doSensitive(ctx, http.MethodGet, "/observability/destinations/"+url.PathEscape(id), nil, nil, &resp); err != nil {
		return nil, err
	}
	if err := resp.Data.Validate(); err != nil {
		return nil, err
	}
	if resp.Data.ID != id {
		return nil, fmt.Errorf("observability destination response identity mismatch")
	}
	return &resp.Data, nil
}
func (c *Client) UpdateObservabilityDestination(ctx context.Context, id string, body ObservabilityDestinationWrite) (*ObservabilityDestination, error) {
	// Workspace and type are immutable and not accepted by PATCH.
	body.WorkspaceID = ""
	body.Type = ""
	var resp singleResponse[ObservabilityDestination]
	if err := c.doSensitive(ctx, http.MethodPatch, "/observability/destinations/"+url.PathEscape(id), nil, body, &resp); err != nil {
		return nil, err
	}
	if err := resp.Data.Validate(); err != nil {
		return nil, err
	}
	if resp.Data.ID != id {
		return nil, fmt.Errorf("observability destination response identity mismatch")
	}
	return &resp.Data, nil
}
func (c *Client) DeleteObservabilityDestination(ctx context.Context, id string) error {
	var resp struct {
		Deleted *bool `json:"deleted"`
	}
	if err := c.doSensitive(ctx, http.MethodDelete, "/observability/destinations/"+url.PathEscape(id), nil, nil, &resp); err != nil {
		return err
	}
	if resp.Deleted == nil || !*resp.Deleted {
		return fmt.Errorf("observability destination deletion was not confirmed")
	}
	return nil
}
