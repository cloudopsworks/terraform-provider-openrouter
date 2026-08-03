package secrets

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

type Resolver interface {
	Resolve(ctx context.Context) (Settings, error)
}

type ResolverFunc func(context.Context) (Settings, error)

func (f ResolverFunc) Resolve(ctx context.Context) (Settings, error) { return f(ctx) }

type Settings struct {
	AdminAPIKey    string
	BaseURL        string
	OrganizationID string
	ProjectID      string
}

type SourceConfig struct {
	Direct Settings
	AWS    *AWSConfig
	GCP    *GCPConfig
	Azure  *AzureConfig
}

type AWSConfig struct {
	Region          string
	SecretID        string
	VersionID       string
	VersionStage    string
	JSONKey         string
	RoleARN         string
	RoleSessionName string
	ExternalID      string
	DurationSeconds int64
}

type GCPConfig struct {
	ProjectID                 string
	SecretID                  string
	Version                   string
	JSONKey                   string
	ImpersonateServiceAccount string
	Delegates                 []string
	Scopes                    []string
}

type AzureConfig struct {
	VaultURL   string
	SecretName string
	Version    string
	JSONKey    string
}

func NewResolver(cfg SourceConfig) (Resolver, string, error) {
	if strings.TrimSpace(cfg.Direct.AdminAPIKey) != "" {
		settings := cfg.Direct.Trimmed()
		return ResolverFunc(func(context.Context) (Settings, error) { return settings, nil }), "configuration", nil
	}

	sources := configuredSources(cfg)
	if len(sources) == 0 {
		return nil, "none", fmt.Errorf("missing OpenRouter API key source: set api_key or configure exactly one of aws_secrets_manager, gcp_secret_manager, or azure_key_vault")
	}
	if len(sources) > 1 {
		return nil, "multiple", fmt.Errorf("ambiguous OpenRouter API key source: configure exactly one source, got %s", strings.Join(sources, ", "))
	}

	switch sources[0] {
	case "aws_secrets_manager":
		return NewAWSResolver(*cfg.AWS), "aws_secrets_manager", nil
	case "gcp_secret_manager":
		return NewGCPResolver(*cfg.GCP), "gcp_secret_manager", nil
	case "azure_key_vault":
		return NewAzureResolver(*cfg.Azure), "azure_key_vault", nil
	default:
		return nil, "unknown", fmt.Errorf("unsupported OpenRouter API key source %q", sources[0])
	}
}

func configuredSources(cfg SourceConfig) []string {
	var sources []string
	if strings.TrimSpace(cfg.Direct.AdminAPIKey) != "" {
		sources = append(sources, "configuration")
	}
	if cfg.AWS != nil {
		sources = append(sources, "aws_secrets_manager")
	}
	if cfg.GCP != nil {
		sources = append(sources, "gcp_secret_manager")
	}
	if cfg.Azure != nil {
		sources = append(sources, "azure_key_vault")
	}
	return sources
}

func (s Settings) Trimmed() Settings {
	return Settings{
		AdminAPIKey:    strings.TrimSpace(s.AdminAPIKey),
		BaseURL:        strings.TrimSpace(s.BaseURL),
		OrganizationID: strings.TrimSpace(s.OrganizationID),
		ProjectID:      strings.TrimSpace(s.ProjectID),
	}
}

func (s Settings) MergeFallback(fallback Settings) Settings {
	s = s.Trimmed()
	fallback = fallback.Trimmed()
	if s.AdminAPIKey == "" {
		s.AdminAPIKey = fallback.AdminAPIKey
	}
	if s.BaseURL == "" {
		s.BaseURL = fallback.BaseURL
	}
	if s.OrganizationID == "" {
		s.OrganizationID = fallback.OrganizationID
	}
	if s.ProjectID == "" {
		s.ProjectID = fallback.ProjectID
	}
	return s
}

func extractSecretSettings(raw, jsonKey string) (Settings, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return Settings{}, fmt.Errorf("secret value is empty")
	}
	if strings.TrimSpace(jsonKey) != "" {
		apiKey, err := extractSecretValue(value, jsonKey)
		if err != nil {
			return Settings{}, err
		}
		return Settings{AdminAPIKey: apiKey}.Trimmed(), nil
	}

	if strings.HasPrefix(value, "{") {
		var decoded map[string]any
		if err := json.Unmarshal([]byte(value), &decoded); err != nil {
			return Settings{}, fmt.Errorf("secret value is not valid JSON: %w", err)
		}
		settings := Settings{
			AdminAPIKey:    firstStringValue(decoded, "api_key", "openrouter_api_key", "openrouter_admin_key", "openrouter_admin_api_key", "admin_api_key", "OPENROUTER_API_KEY"),
			BaseURL:        firstStringValue(decoded, "base_url", "openrouter_base_url", "OPENROUTER_BASE_URL"),
			OrganizationID: firstStringValue(decoded, "organization_id", "org_id", "organization", "openrouter_org_id", "OPENROUTER_ORG_ID"),
			ProjectID:      firstStringValue(decoded, "project_id", "project", "openrouter_project_id", "OPENROUTER_PROJECT_ID"),
		}.Trimmed()
		if settings.AdminAPIKey == "" {
			return Settings{}, fmt.Errorf("secret JSON object does not contain a non-empty api_key/openrouter_api_key field")
		}
		return settings, nil
	}
	if strings.HasPrefix(value, "[") {
		return Settings{}, fmt.Errorf("secret value JSON must be an object when json_key is not configured")
	}

	return Settings{AdminAPIKey: value}.Trimmed(), nil
}

func firstStringValue(object map[string]any, keys ...string) string {
	for _, key := range keys {
		raw, ok := object[key]
		if !ok {
			continue
		}
		value, ok := raw.(string)
		if !ok {
			continue
		}
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func extractSecretValue(raw, jsonKey string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", fmt.Errorf("secret value is empty")
	}
	if strings.TrimSpace(jsonKey) == "" {
		return value, nil
	}

	var decoded any
	if err := json.Unmarshal([]byte(value), &decoded); err != nil {
		return "", fmt.Errorf("secret value is not valid JSON for json_key %q: %w", jsonKey, err)
	}
	current := decoded
	for _, part := range strings.Split(jsonKey, ".") {
		part = strings.TrimSpace(part)
		if part == "" {
			return "", fmt.Errorf("json_key %q contains an empty path segment", jsonKey)
		}
		object, ok := current.(map[string]any)
		if !ok {
			return "", fmt.Errorf("json_key %q did not resolve to a string value", jsonKey)
		}
		var exists bool
		current, exists = object[part]
		if !exists {
			return "", fmt.Errorf("json_key %q was not found in secret value", jsonKey)
		}
	}
	resolved, ok := current.(string)
	if !ok || strings.TrimSpace(resolved) == "" {
		return "", fmt.Errorf("json_key %q did not resolve to a non-empty string", jsonKey)
	}
	return strings.TrimSpace(resolved), nil
}

func bytesSecretValue(data []byte, jsonKey string) (string, error) {
	settings, err := bytesSecretSettings(data, jsonKey)
	if err != nil {
		return "", err
	}
	return settings.AdminAPIKey, nil
}

func bytesSecretSettings(data []byte, jsonKey string) (Settings, error) {
	if len(data) == 0 {
		return Settings{}, fmt.Errorf("secret binary value is empty")
	}
	decoded := make([]byte, base64.StdEncoding.EncodedLen(len(data)))
	base64.StdEncoding.Encode(decoded, data)
	return extractSecretSettings(string(decoded), jsonKey)
}
