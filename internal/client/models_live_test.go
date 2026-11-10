package client

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// These tests are read-only, explicitly opt-in, and suppress upstream response
// details so neither credentials nor account identifiers enter test logs.
func TestReadOnlyLiveModelsAndActivity(t *testing.T) {
	if os.Getenv("OPENROUTER_READONLY_LIVE") != "1" {
		t.Skip("set OPENROUTER_READONLY_LIVE=1 to enable read-only live API checks")
	}
	key := os.Getenv("OPENROUTER_API_KEY")
	if key == "" {
		t.Fatal("OPENROUTER_API_KEY required for explicitly opted-in live checks")
	}
	c := New(key, DefaultBaseURL, "terraform-provider-openrouter-readonly-test", 30*time.Second)
	t.Run("models", func(t *testing.T) {
		page, err := c.GetModelsPage(context.Background(), ModelsPageOptions{Limit: 1})
		if err != nil {
			readonlyLiveFailure(t, "bounded catalog", err)
		}
		if len(page.Data) == 0 {
			t.Skip("no catalog model available for lookup")
		}
		if _, err := c.GetModel(context.Background(), page.Data[0].ID); err != nil {
			readonlyLiveFailure(t, "model lookup", err)
		}
	})
	t.Run("activity", func(t *testing.T) {
		date := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
		if _, err := c.GetActivity(context.Background(), ActivityOptions{Date: date, MaxItems: 1000}); err != nil {
			readonlyLiveFailure(t, "dated activity", err)
		}
	})
}

func readonlyLiveFailure(t *testing.T, operation string, err error) {
	t.Helper()
	if strings.HasPrefix(err.Error(), "openrouter API error (") {
		var status int
		if _, scanErr := fmt.Sscanf(err.Error(), "openrouter API error (%d)", &status); scanErr == nil && status >= 100 && status <= 599 {
			t.Fatalf("%s failed: HTTP %d; upstream response suppressed", operation, status)
		}
	}
	t.Fatalf("%s failed: transport or contract error; upstream response suppressed", operation)
}
