package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Model contains the public metadata needed to select a model without inference.
// Pricing retains the API's decimal strings and their per-token/per-unit units.
type Model struct {
	ID                  string             `json:"id"`
	CanonicalSlug       string             `json:"canonical_slug"`
	Name                string             `json:"name"`
	Description         *string            `json:"description"`
	Created             *int64             `json:"created"`
	ContextLength       *int64             `json:"context_length"`
	ExpirationDate      *string            `json:"expiration_date"`
	KnowledgeCutoff     *string            `json:"knowledge_cutoff"`
	SupportedParameters []string           `json:"supported_parameters"`
	Architecture        *ModelArchitecture `json:"architecture"`
	Pricing             *ModelPricing      `json:"pricing"`
}

type ModelArchitecture struct {
	InputModalities  []string `json:"input_modalities"`
	OutputModalities []string `json:"output_modalities"`
}

type ModelPricing struct {
	Prompt            string  `json:"prompt"`
	Completion        string  `json:"completion"`
	Image             *string `json:"image"`
	ImageOutput       *string `json:"image_output"`
	ImageToken        *string `json:"image_token"`
	Request           *string `json:"request"`
	InputCacheRead    *string `json:"input_cache_read"`
	InputCacheWrite   *string `json:"input_cache_write"`
	InputCacheWrite1h *string `json:"input_cache_write_1h"`
	InputAudioCache   *string `json:"input_audio_cache"`
	InternalReasoning *string `json:"internal_reasoning"`
	Audio             *string `json:"audio"`
	AudioOutput       *string `json:"audio_output"`
	WebSearch         *string `json:"web_search"`
}

// UnmarshalJSON rejects null elements instead of silently coercing them to empty
// strings. Unknown API metadata remains forward-compatible.
func (m *Model) UnmarshalJSON(data []byte) error {
	type plain Model
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	for _, name := range []string{"id", "canonical_slug", "name", "created", "context_length", "architecture", "pricing", "supported_parameters"} {
		v, ok := raw[name]
		if !ok || (name != "context_length" && string(v) == "null") {
			return fmt.Errorf("model response has missing or null %s", name)
		}
	}
	var architecture map[string]json.RawMessage
	if err := json.Unmarshal(raw["architecture"], &architecture); err != nil {
		return err
	}
	for _, pair := range []struct {
		raw  json.RawMessage
		name string
	}{{raw["supported_parameters"], "supported_parameters"}, {architecture["input_modalities"], "input_modalities"}, {architecture["output_modalities"], "output_modalities"}} {
		var values []*string
		if len(pair.raw) == 0 || string(pair.raw) == "null" {
			return fmt.Errorf("model response has missing or null %s", pair.name)
		}
		if err := json.Unmarshal(pair.raw, &values); err != nil {
			return err
		}
		for _, value := range values {
			if value == nil || strings.TrimSpace(*value) == "" {
				return fmt.Errorf("model response has invalid %s element", pair.name)
			}
		}
	}
	*m = Model(decoded)
	return m.Validate()
}

func (m Model) Validate() error {
	if strings.TrimSpace(m.ID) == "" || strings.TrimSpace(m.CanonicalSlug) == "" || strings.TrimSpace(m.Name) == "" || m.Created == nil || *m.Created < 0 || m.Architecture == nil || m.Pricing == nil || m.Pricing.Prompt == "" || m.Pricing.Completion == "" {
		return fmt.Errorf("invalid model response: required metadata or base pricing missing")
	}
	if m.ContextLength != nil && *m.ContextLength < 0 {
		return fmt.Errorf("invalid model response: negative context length")
	}
	return nil
}

func (m ModelPricing) BasePrices() map[string]string {
	result := map[string]string{"prompt": m.Prompt, "completion": m.Completion}
	for k, v := range map[string]*string{
		"image":                m.Image,
		"image_output":         m.ImageOutput,
		"image_token":          m.ImageToken,
		"request":              m.Request,
		"input_cache_read":     m.InputCacheRead,
		"input_cache_write":    m.InputCacheWrite,
		"input_cache_write_1h": m.InputCacheWrite1h,
		"input_audio_cache":    m.InputAudioCache,
		"internal_reasoning":   m.InternalReasoning,
		"audio":                m.Audio,
		"audio_output":         m.AudioOutput,
		"web_search":           m.WebSearch,
	} {
		if v != nil {
			result[k] = *v
		}
	}
	return result
}

func SplitModelSlug(slug string) (string, string, error) {
	parts := strings.Split(slug, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("model slug must be author/model, optionally with a variant suffix")
	}
	for _, part := range parts {
		if strings.TrimSpace(part) == "" || part == "." || part == ".." || strings.ContainsAny(part, "?#\\ \t\r\n") {
			return "", "", fmt.Errorf("model slug must contain two nonempty path segments without query or fragment syntax")
		}
	}
	return parts[0], parts[1], nil
}

func (c *Client) GetModel(ctx context.Context, slug string) (*Model, error) {
	author, model, err := SplitModelSlug(slug)
	if err != nil {
		return nil, err
	}
	var resp singleResponse[Model]
	if err := c.do(ctx, http.MethodGet, "/model/"+url.PathEscape(author)+"/"+url.PathEscape(model), nil, nil, &resp); err != nil {
		return nil, err
	}
	if err := resp.Data.Validate(); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}

type ModelsPageOptions struct {
	Offset, Limit                             int
	Search, InputModalities, OutputModalities string
}
type ModelsPage struct {
	Data       []Model `json:"data"`
	TotalCount *int64  `json:"total_count"`
	Links      *struct {
		Next *string `json:"next"`
	} `json:"links"`
}

// GetModelsPage intentionally reads only one bounded page, never follows API
// supplied URLs, and never forwards credentials to another host.
func (c *Client) GetModelsPage(ctx context.Context, options ModelsPageOptions) (*ModelsPage, error) {
	if options.Offset < 0 || options.Limit < 1 || options.Limit > 1000 {
		return nil, fmt.Errorf("model catalog offset must be nonnegative and limit must be 1..1000")
	}
	query := url.Values{"offset": {strconv.Itoa(options.Offset)}, "limit": {strconv.Itoa(options.Limit)}}
	for k, v := range map[string]string{"q": options.Search, "input_modalities": options.InputModalities, "output_modalities": options.OutputModalities} {
		if v != "" {
			query.Set(k, v)
		}
	}
	var resp ModelsPage
	if err := c.do(ctx, http.MethodGet, "/models", query, nil, &resp); err != nil {
		return nil, err
	}
	if resp.Data == nil || resp.TotalCount == nil || *resp.TotalCount < 0 || resp.Links == nil || len(resp.Data) > options.Limit {
		return nil, fmt.Errorf("invalid model catalog page response")
	}
	seen := map[string]bool{}
	for _, m := range resp.Data {
		if err := m.Validate(); err != nil {
			return nil, err
		}
		if seen[m.ID] {
			return nil, fmt.Errorf("invalid model catalog page: duplicate model identity")
		}
		seen[m.ID] = true
	}
	return &resp, nil
}

func (page *ModelsPage) UnmarshalJSON(data []byte) error {
	type plain ModelsPage
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var raw struct {
		Links map[string]json.RawMessage `json:"links"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if _, ok := raw.Links["next"]; !ok {
		return fmt.Errorf("invalid model catalog page: missing links.next")
	}
	*page = ModelsPage(decoded)
	return nil
}
