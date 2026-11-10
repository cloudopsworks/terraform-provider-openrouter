package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/cloudopsworks/terraform-provider-openrouter/internal/client"
)

func TestAPIKeyCreateDisabledRetainsCreatedState(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name             string
		createFailure    bool
		patchFailure     bool
		refreshFailure   bool
		patchDisabled    bool
		refreshDisabled  bool
		patchHash        string
		missingPatchHash bool
		refreshHash      string
		wantError        string
		wantDisabled     bool
		wantMethods      string
	}{
		{name: "create failure", createFailure: true, wantError: "Unable to create", wantMethods: "POST"},
		{name: "disable failure retains creation", patchFailure: true, wantError: "Unable to disable newly created", wantMethods: "POST,PATCH"},
		{name: "refresh failure retains patch response", patchDisabled: true, refreshFailure: true, wantError: "Unable to refresh newly created", wantDisabled: true, wantMethods: "POST,PATCH,GET"},
		{name: "refresh is authoritative", patchDisabled: false, refreshDisabled: true, wantDisabled: true, wantMethods: "POST,PATCH,GET"},
		{name: "refresh rejects still enabled key", patchDisabled: true, refreshDisabled: false, wantError: "Unable to disable newly created", wantMethods: "POST,PATCH,GET"},
		{name: "missing patch identity retains creation", missingPatchHash: true, refreshFailure: true, wantError: "Unable to disable newly created", wantMethods: "POST,PATCH"},
		{name: "different patch identity retains creation", patchHash: "other-key", refreshFailure: true, wantError: "Unable to confirm newly created", wantMethods: "POST,PATCH"},
		{name: "different refresh identity retains patch state", patchDisabled: true, refreshHash: "other-key", wantError: "Unable to confirm newly created", wantDisabled: true, wantMethods: "POST,PATCH,GET"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var mu sync.Mutex
			var methods []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				methods = append(methods, r.Method)
				w.Header().Set("Content-Type", "application/json")
				key := client.APIKey{Hash: "created-key-hash", Name: "zero-limit-key"}
				switch r.Method {
				case http.MethodPost:
					if tt.createFailure {
						http.Error(w, "create failed", http.StatusBadRequest)
						return
					}
					_ = json.NewEncoder(w).Encode(client.APIKeyCreateResponse{Data: key, Key: "local-one-time-secret"})
					return
				case http.MethodPatch:
					if r.URL.Path != "/keys/created-key-hash" {
						t.Errorf("disable used wrong identity: %s", r.URL.Path)
					}
					if tt.patchFailure {
						http.Error(w, "disable failed", http.StatusBadRequest)
						return
					}
					key.Disabled = tt.patchDisabled
					if tt.missingPatchHash || tt.patchHash != "" {
						key.Hash = tt.patchHash
					}
				case http.MethodGet:
					if r.URL.Path != "/keys/created-key-hash" {
						t.Errorf("refresh used wrong identity: %s", r.URL.Path)
					}
					if tt.refreshFailure {
						http.Error(w, "refresh failed", http.StatusBadRequest)
						return
					}
					key.Disabled = tt.refreshDisabled
					if tt.refreshHash != "" {
						key.Hash = tt.refreshHash
					}
				default:
					t.Errorf("unexpected operation: %s", r.Method)
					http.Error(w, "unexpected operation", http.StatusMethodNotAllowed)
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"data": key})
			}))
			defer server.Close()
			ctx := context.Background()
			r := &apiKeyResource{client: client.New("local", server.URL, "test", time.Second)}
			var schemaResponse frameworkresource.SchemaResponse
			r.Schema(ctx, frameworkresource.SchemaRequest{}, &schemaResponse)
			plan := tfsdk.Plan{Schema: schemaResponse.Schema}
			model := flattenAPIKeyModel(client.APIKey{Name: "zero-limit-key", Disabled: true})
			model.Limit = types.Float64Value(0)
			if diags := plan.Set(ctx, &model); diags.HasError() {
				t.Fatal(diags)
			}
			resp := frameworkresource.CreateResponse{State: tfsdk.State{Schema: schemaResponse.Schema}}
			r.Create(ctx, frameworkresource.CreateRequest{Plan: plan}, &resp)
			if tt.wantError == "" {
				if resp.Diagnostics.HasError() {
					t.Fatalf("unexpected diagnostics: %s", resp.Diagnostics)
				}
			} else if !resp.Diagnostics.HasError() || !strings.Contains(resp.Diagnostics[0].Summary(), tt.wantError) {
				t.Fatalf("diagnostics=%s, want error containing %q", resp.Diagnostics, tt.wantError)
			}
			if tt.createFailure {
				if !resp.State.Raw.IsNull() {
					t.Fatal("failed POST must not establish resource state")
				}
			} else {
				var state apiKeyResourceModel
				if diags := resp.State.Get(ctx, &state); diags.HasError() {
					t.Fatal(diags)
				}
				if state.ID.ValueString() != "created-key-hash" || state.Key.ValueString() != "local-one-time-secret" {
					t.Fatal("created identity or one-time secret lost after post-create operation")
				}
				if state.Disabled.IsUnknown() || state.Disabled.IsNull() || state.Disabled.ValueBool() != tt.wantDisabled {
					t.Fatalf("disabled=%s, want last observed remote value %t", state.Disabled, tt.wantDisabled)
				}
			}
			mu.Lock()
			defer mu.Unlock()
			if got := strings.Join(methods, ","); got != tt.wantMethods {
				t.Fatalf("API operations=%s, want %s", got, tt.wantMethods)
			}
		})
	}
}

func TestAPIKeyResourceWorkspaceIDSchemaSupportsAPIDefault(t *testing.T) {
	t.Parallel()

	var resp frameworkresource.SchemaResponse
	NewAPIKeyResource().Schema(context.Background(), frameworkresource.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected schema diagnostics: %s", resp.Diagnostics)
	}

	attr, ok := resp.Schema.Attributes["workspace_id"].(resourceschema.StringAttribute)
	if !ok {
		t.Fatalf("workspace_id attribute type = %T, want schema.StringAttribute", resp.Schema.Attributes["workspace_id"])
	}
	if !attr.Optional {
		t.Fatal("workspace_id must remain Optional so users can explicitly target a workspace")
	}
	if !attr.Computed {
		t.Fatal("workspace_id must be Computed so API-defaulted workspace IDs can be recorded in state")
	}

	priorWorkspaceID := "f7574683-5741-5f2a-9704-aef2c3ff2a05"
	plannedWorkspaceID, requiresReplace := applyStringPlanModifiersForTest(
		attr.PlanModifiers,
		types.StringNull(),
		types.StringUnknown(),
		types.StringValue(priorWorkspaceID),
	)
	if requiresReplace {
		t.Fatal("workspace_id must not force replacement when an omitted value is already resolved in state")
	}
	if plannedWorkspaceID.IsUnknown() || plannedWorkspaceID.ValueString() != priorWorkspaceID {
		t.Fatalf("workspace_id planned value = %s, want prior state value %q", plannedWorkspaceID.String(), priorWorkspaceID)
	}

	newWorkspaceID := "a1114683-5741-5f2a-9704-aef2c3ff2a05"
	_, requiresReplace = applyStringPlanModifiersForTest(
		attr.PlanModifiers,
		types.StringValue(newWorkspaceID),
		types.StringValue(newWorkspaceID),
		types.StringValue(priorWorkspaceID),
	)
	if !requiresReplace {
		t.Fatal("workspace_id must continue to force replacement when an explicitly configured value changes")
	}
}

func TestFlattenAPIKeyModelStoresReturnedWorkspaceID(t *testing.T) {
	t.Parallel()

	workspaceID := "f7574683-5741-5f2a-9704-aef2c3ff2a05"
	model := flattenAPIKeyModel(client.APIKey{
		Hash:        "key-hash",
		Name:        "test-key",
		WorkspaceID: &workspaceID,
	})

	if model.WorkspaceID.IsNull() || model.WorkspaceID.IsUnknown() {
		t.Fatal("workspace_id should be known when the API returns a default workspace ID")
	}
	if got := model.WorkspaceID.ValueString(); got != workspaceID {
		t.Fatalf("workspace_id = %q, want %q", got, workspaceID)
	}
}

func TestAPIKeyResourceCreatorUserIDSchemaSupportsAPIDefault(t *testing.T) {
	t.Parallel()

	var resp frameworkresource.SchemaResponse
	NewAPIKeyResource().Schema(context.Background(), frameworkresource.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected schema diagnostics: %s", resp.Diagnostics)
	}

	attr, ok := resp.Schema.Attributes["creator_user_id"].(resourceschema.StringAttribute)
	if !ok {
		t.Fatalf("creator_user_id attribute type = %T, want schema.StringAttribute", resp.Schema.Attributes["creator_user_id"])
	}
	if !attr.Optional {
		t.Fatal("creator_user_id must remain Optional so users can explicitly select a creator")
	}
	if !attr.Computed {
		t.Fatal("creator_user_id must be Computed so API-defaulted creator IDs can be recorded in state")
	}

	priorCreatorUserID := "user_abc123"
	plannedCreatorUserID, requiresReplace := applyStringPlanModifiersForTest(
		attr.PlanModifiers,
		types.StringNull(),
		types.StringUnknown(),
		types.StringValue(priorCreatorUserID),
	)
	if requiresReplace {
		t.Fatal("creator_user_id must not force replacement when an omitted value is already resolved in state")
	}
	if plannedCreatorUserID.IsUnknown() || plannedCreatorUserID.ValueString() != priorCreatorUserID {
		t.Fatalf("creator_user_id planned value = %s, want prior state value %q", plannedCreatorUserID.String(), priorCreatorUserID)
	}

	newCreatorUserID := "user_def456"
	_, requiresReplace = applyStringPlanModifiersForTest(
		attr.PlanModifiers,
		types.StringValue(newCreatorUserID),
		types.StringValue(newCreatorUserID),
		types.StringValue(priorCreatorUserID),
	)
	if !requiresReplace {
		t.Fatal("creator_user_id must continue to force replacement when an explicitly configured value changes")
	}
}

func TestFlattenAPIKeyModelStoresReturnedCreatorUserID(t *testing.T) {
	t.Parallel()

	creatorUserID := "user_abc123"
	model := flattenAPIKeyModel(client.APIKey{
		Hash:          "key-hash",
		Name:          "test-key",
		CreatorUserID: &creatorUserID,
	})

	if model.CreatorUserID.IsNull() || model.CreatorUserID.IsUnknown() {
		t.Fatal("creator_user_id should be known when the API returns a default creator user ID")
	}
	if got := model.CreatorUserID.ValueString(); got != creatorUserID {
		t.Fatalf("creator_user_id = %q, want %q", got, creatorUserID)
	}
}

func applyStringPlanModifiersForTest(
	modifiers []planmodifier.String,
	configValue types.String,
	planValue types.String,
	stateValue types.String,
) (types.String, bool) {
	rawResourceValue := tftypes.NewValue(
		tftypes.Object{AttributeTypes: map[string]tftypes.Type{}},
		map[string]tftypes.Value{},
	)
	req := planmodifier.StringRequest{
		ConfigValue: configValue,
		PlanValue:   planValue,
		StateValue:  stateValue,
		Plan:        tfsdk.Plan{Raw: rawResourceValue},
		State:       tfsdk.State{Raw: rawResourceValue},
	}

	requiresReplace := false
	for _, modifier := range modifiers {
		resp := &planmodifier.StringResponse{PlanValue: req.PlanValue}
		modifier.PlanModifyString(context.Background(), req, resp)
		req.PlanValue = resp.PlanValue
		requiresReplace = requiresReplace || resp.RequiresReplace
	}

	return req.PlanValue, requiresReplace
}
