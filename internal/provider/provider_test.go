package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestResolveAPIKey(t *testing.T) {
	tests := []struct {
		name   string
		env    string
		config types.String
		want   string
		source string
	}{
		{
			name:   "uses environment variable when config is null",
			env:    "env-key",
			config: types.StringNull(),
			want:   "env-key",
			source: "environment",
		},
		{
			name:   "uses environment variable when config is empty",
			env:    "env-key",
			config: types.StringValue(""),
			want:   "env-key",
			source: "environment",
		},
		{
			name:   "trims and uses environment variable when config is whitespace",
			env:    "  env-key\n",
			config: types.StringValue("   "),
			want:   "env-key",
			source: "environment",
		},
		{
			name:   "prefers non-empty config over environment variable",
			env:    "env-key",
			config: types.StringValue("config-key"),
			want:   "config-key",
			source: "configuration",
		},
		{
			name:   "returns none when no configuration is available",
			env:    "   ",
			config: types.StringNull(),
			want:   "",
			source: "none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("OPENROUTER_API_KEY", tt.env)

			got, source := resolveAPIKey(tt.config)
			if got != tt.want {
				t.Fatalf("resolveAPIKey() = %q, want %q", got, tt.want)
			}
			if source != tt.source {
				t.Fatalf("resolveAPIKey() source = %q, want %q", source, tt.source)
			}
		})
	}
}
