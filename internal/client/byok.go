package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const maxBYOKPages = 10000

// BYOKCredential is an OpenRouter provider credential. The provider key is
// deliberately absent: OpenRouter accepts it as write-only material and never
// returns it in a credential response.
//
// A nil restriction slice means the API returned null (no restriction); a
// non-nil empty slice means it returned an empty array. This distinction is
// useful to resource callers even though OpenRouter currently rejects an empty
// allowed_api_key_hashes request value.
type BYOKCredential struct {
	ID                  string   `json:"id"`
	Provider            string   `json:"provider"`
	WorkspaceID         string   `json:"workspace_id"`
	Label               string   `json:"label"`
	Name                *string  `json:"name"`
	Disabled            bool     `json:"disabled"`
	IsFallback          bool     `json:"is_fallback"`
	IsRequired          bool     `json:"is_required"`
	IsBYOKOnly          bool     `json:"is_byok_only"`
	DeclaredZDR         *bool    `json:"declared_zdr"`
	AllowedModels       []string `json:"allowed_models"`
	AllowedAPIKeyHashes []string `json:"allowed_api_key_hashes"`
	AllowedUserIDs      []string `json:"allowed_user_ids"`
	SortOrder           int      `json:"sort_order"`
	CreatedAt           string   `json:"created_at"`
}

func (c BYOKCredential) Validate() error {
	if err := requireNonEmpty("BYOK credential", "id", c.ID); err != nil {
		return err
	}
	if err := requireNonEmpty("BYOK credential", "provider", c.Provider); err != nil {
		return err
	}
	if err := requireNonEmpty("BYOK credential", "workspace_id", c.WorkspaceID); err != nil {
		return err
	}
	if err := requireNonEmpty("BYOK credential", "label", c.Label); err != nil {
		return err
	}
	return requireNonEmpty("BYOK credential", "created_at", c.CreatedAt)
}

// BYOKOptionalString serializes either a string or an explicit JSON null. A
// nil pointer to it omits the field, which is essential for partial PATCHes.
type BYOKOptionalString struct {
	Value string
	Null  bool
}

func (v BYOKOptionalString) MarshalJSON() ([]byte, error) {
	if v.Null {
		return []byte("null"), nil
	}
	return json.Marshal(v.Value)
}

// BYOKString returns an explicitly supplied string value.
func BYOKString(value string) *BYOKOptionalString {
	return &BYOKOptionalString{Value: value}
}

// BYOKNullString explicitly clears a nullable string field on PATCH.
func BYOKNullString() *BYOKOptionalString {
	return &BYOKOptionalString{Null: true}
}

// BYOKOptionalBool serializes either a boolean or an explicit JSON null. A
// nil pointer omits the field, allowing a partial PATCH to preserve its value.
type BYOKOptionalBool struct {
	Value bool
	Null  bool
}

func (v BYOKOptionalBool) MarshalJSON() ([]byte, error) {
	if v.Null {
		return []byte("null"), nil
	}
	return json.Marshal(v.Value)
}

// BYOKBool returns an explicitly supplied boolean value.
func BYOKBool(value bool) *BYOKOptionalBool {
	return &BYOKOptionalBool{Value: value}
}

// BYOKNullBool explicitly clears a nullable boolean field on PATCH.
func BYOKNullBool() *BYOKOptionalBool {
	return &BYOKOptionalBool{Null: true}
}

// BYOKOptionalStringSlice serializes either a string array or an explicit JSON
// null. A nil pointer omits the field, so omitted, null, and [] remain distinct.
type BYOKOptionalStringSlice struct {
	Values []string
	Null   bool
}

func (v BYOKOptionalStringSlice) MarshalJSON() ([]byte, error) {
	if v.Null {
		return []byte("null"), nil
	}
	return json.Marshal(v.Values)
}

// BYOKStringSlice returns an explicitly supplied restriction list. It retains
// an empty slice so callers can let the API enforce endpoint-specific rules.
func BYOKStringSlice(values []string) *BYOKOptionalStringSlice {
	if values == nil {
		values = []string{}
	}
	return &BYOKOptionalStringSlice{Values: values}
}

// BYOKNullStringSlice explicitly clears a restriction on PATCH.
func BYOKNullStringSlice() *BYOKOptionalStringSlice {
	return &BYOKOptionalStringSlice{Null: true}
}

// BYOKCreateRequest creates an encrypted, write-only provider credential.
// Key must not be persisted or logged by callers.
type BYOKCreateRequest struct {
	Key                 string                   `json:"key"`
	Provider            string                   `json:"provider"`
	Name                *BYOKOptionalString      `json:"name,omitempty"`
	WorkspaceID         *string                  `json:"workspace_id,omitempty"`
	AllowedModels       *BYOKOptionalStringSlice `json:"allowed_models,omitempty"`
	AllowedAPIKeyHashes *BYOKOptionalStringSlice `json:"allowed_api_key_hashes,omitempty"`
	AllowedUserIDs      *BYOKOptionalStringSlice `json:"allowed_user_ids,omitempty"`
	Disabled            *bool                    `json:"disabled,omitempty"`
	IsFallback          *bool                    `json:"is_fallback,omitempty"`
	IsRequired          *bool                    `json:"is_required,omitempty"`
	IsBYOKOnly          *bool                    `json:"is_byok_only,omitempty"`
	DeclaredZDR         *BYOKOptionalBool        `json:"declared_zdr,omitempty"`
}

func (r BYOKCreateRequest) validate() error {
	if strings.TrimSpace(r.Key) == "" {
		return fmt.Errorf("openrouter BYOK create request requires key")
	}
	if strings.TrimSpace(r.Provider) == "" {
		return fmt.Errorf("openrouter BYOK create request requires provider")
	}
	return validateBYOKRestrictionLists(r.AllowedModels, r.AllowedAPIKeyHashes, r.AllowedUserIDs)
}

// BYOKUpdateRequest updates only supplied fields. To clear Name or a
// restriction, use BYOKNullString or BYOKNullStringSlice respectively. Key
// rotates the write-only provider credential in place.
type BYOKUpdateRequest struct {
	Key                 *string                  `json:"key,omitempty"`
	Name                *BYOKOptionalString      `json:"name,omitempty"`
	AllowedModels       *BYOKOptionalStringSlice `json:"allowed_models,omitempty"`
	AllowedAPIKeyHashes *BYOKOptionalStringSlice `json:"allowed_api_key_hashes,omitempty"`
	AllowedUserIDs      *BYOKOptionalStringSlice `json:"allowed_user_ids,omitempty"`
	Disabled            *bool                    `json:"disabled,omitempty"`
	IsFallback          *bool                    `json:"is_fallback,omitempty"`
	IsRequired          *bool                    `json:"is_required,omitempty"`
	IsBYOKOnly          *bool                    `json:"is_byok_only,omitempty"`
	DeclaredZDR         *BYOKOptionalBool        `json:"declared_zdr,omitempty"`
}

type byokDeleteResponse struct {
	Deleted *bool `json:"deleted"`
}

func (r BYOKUpdateRequest) validate() error {
	if r.Key != nil && strings.TrimSpace(*r.Key) == "" {
		return fmt.Errorf("openrouter BYOK update request key must not be empty")
	}
	return validateBYOKRestrictionLists(r.AllowedModels, r.AllowedAPIKeyHashes, r.AllowedUserIDs)
}

func validateBYOKRestrictionLists(models, keyHashes, userIDs *BYOKOptionalStringSlice) error {
	for _, restriction := range []*BYOKOptionalStringSlice{models, userIDs} {
		if restriction != nil && !restriction.Null && len(restriction.Values) > 100 {
			return fmt.Errorf("openrouter BYOK restriction list exceeds 100 entries")
		}
	}
	if keyHashes == nil || keyHashes.Null || keyHashes.Values == nil {
		return nil
	}
	if len(keyHashes.Values) == 0 || len(keyHashes.Values) > 100 {
		return fmt.Errorf("openrouter BYOK allowed_api_key_hashes must contain 1 to 100 hashes")
	}
	for _, hash := range keyHashes.Values {
		if len(hash) != 64 {
			return fmt.Errorf("openrouter BYOK allowed_api_key_hashes contains an invalid hash")
		}
		for _, char := range hash {
			if !(char >= '0' && char <= '9') && !(char >= 'a' && char <= 'f') {
				return fmt.Errorf("openrouter BYOK allowed_api_key_hashes contains an invalid hash")
			}
		}
	}
	return nil
}

// CreateBYOKCredential creates a provider credential. Sensitive request errors
// are deliberately generic because some proxies echo request material.
func (c *Client) CreateBYOKCredential(ctx context.Context, body BYOKCreateRequest) (*BYOKCredential, error) {
	if err := body.validate(); err != nil {
		return nil, err
	}
	var resp singleResponse[BYOKCredential]
	if err := c.doSensitive(ctx, http.MethodPost, "/byok", nil, body, &resp); err != nil {
		return nil, err
	}
	if err := resp.Data.Validate(); err != nil {
		return &resp.Data, err
	}
	return &resp.Data, nil
}

// GetBYOKCredential returns a credential's metadata, never its raw key.
func (c *Client) GetBYOKCredential(ctx context.Context, id string) (*BYOKCredential, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("openrouter BYOK credential id must not be empty")
	}
	var resp singleResponse[BYOKCredential]
	if err := c.doSensitive(ctx, http.MethodGet, "/byok/"+url.PathEscape(id), nil, nil, &resp); err != nil {
		return nil, err
	}
	if err := resp.Data.Validate(); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

// ListBYOKCredentials returns all credentials matching the optional workspace
// and provider filters. Pagination is bounded to prevent an API regression from
// looping the provider indefinitely.
func (c *Client) ListBYOKCredentials(ctx context.Context, workspaceID, provider *string) ([]BYOKCredential, error) {
	items := make([]BYOKCredential, 0)
	offset := 0
	for page := 0; page < maxBYOKPages; page++ {
		query := url.Values{}
		query.Set("offset", fmt.Sprintf("%d", offset))
		query.Set("limit", "100")
		if workspaceID != nil && strings.TrimSpace(*workspaceID) != "" {
			query.Set("workspace_id", *workspaceID)
		}
		if provider != nil && strings.TrimSpace(*provider) != "" {
			query.Set("provider", *provider)
		}
		var resp listResponse[BYOKCredential]
		if err := c.doSensitive(ctx, http.MethodGet, "/byok", query, nil, &resp); err != nil {
			return nil, err
		}
		for _, item := range resp.Data {
			if err := item.Validate(); err != nil {
				return nil, err
			}
		}
		items = append(items, resp.Data...)
		if len(resp.Data) == 0 || len(resp.Data) < 100 || (resp.TotalCount > 0 && len(items) >= resp.TotalCount) {
			return items, nil
		}
		offset += len(resp.Data)
	}
	return nil, fmt.Errorf("openrouter BYOK credential pagination exceeded %d pages", maxBYOKPages)
}

// UpdateBYOKCredential partially updates a credential and supports rotating its
// raw key without exposing that value in any returned error.
func (c *Client) UpdateBYOKCredential(ctx context.Context, id string, body BYOKUpdateRequest) (*BYOKCredential, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("openrouter BYOK credential id must not be empty")
	}
	if err := body.validate(); err != nil {
		return nil, err
	}
	var resp singleResponse[BYOKCredential]
	if err := c.doSensitive(ctx, http.MethodPatch, "/byok/"+url.PathEscape(id), nil, body, &resp); err != nil {
		return nil, err
	}
	if err := resp.Data.Validate(); err != nil {
		return &resp.Data, err
	}
	return &resp.Data, nil
}

// DeleteBYOKCredential deletes a provider credential by its UUID and validates
// OpenRouter's required deletion confirmation.
func (c *Client) DeleteBYOKCredential(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("openrouter BYOK credential id must not be empty")
	}
	var resp byokDeleteResponse
	if err := c.doSensitive(ctx, http.MethodDelete, "/byok/"+url.PathEscape(id), nil, nil, &resp); err != nil {
		return err
	}
	if resp.Deleted == nil || !*resp.Deleted {
		return fmt.Errorf("openrouter BYOK delete response missing deleted confirmation")
	}
	return nil
}
