package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cloudopsworks/terraform-provider-openrouter/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	schema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const obsID = "11111111-2222-3333-4444-555555555555"
const obsWorkspace = "550e8400-e29b-41d4-a716-446655440000"

func obsSchema(t *testing.T) schema.Schema {
	t.Helper()
	var resp resource.SchemaResponse
	NewObservabilityDestinationResource().Schema(context.Background(), resource.SchemaRequest{}, &resp)
	return resp.Schema
}
func obsModel() observabilityDestinationModel {
	regions, _ := types.SetValueFrom(context.Background(), types.StringType, []string{"global"})
	return observabilityDestinationModel{ID: types.StringValue(obsID), WorkspaceID: types.StringValue(obsWorkspace), Type: types.StringValue("langfuse"), Name: types.StringValue("test"), Enabled: types.BoolValue(false), PrivacyMode: types.BoolValue(true), SamplingRate: types.Float64Value(1), Regions: regions, APIKeyHashes: types.SetNull(types.StringType), BroadcastGenerationCost: types.BoolValue(false), BroadcastGenerationIdentity: types.BoolValue(false), BroadcastGenerationRequestContext: types.BoolValue(false), ConfigWO: types.StringNull(), ConfigWORevision: types.Int64Value(1), CreatedAt: types.StringValue("now"), UpdatedAt: types.StringValue("now")}
}
func obsResponse() map[string]any {
	return map[string]any{"id": obsID, "workspace_id": obsWorkspace, "type": "langfuse", "name": "test", "enabled": false, "privacy_mode": true, "sampling_rate": 1, "regions": []string{"global"}, "api_key_hashes": nil, "broadcast_generation_cost": false, "broadcast_generation_identity": false, "broadcast_generation_request_context": false, "created_at": "now", "updated_at": "now", "config": map[string]any{"secretKey": "secret-canary-response"}}
}
func TestObservabilitySchemaAndConfigValidation(t *testing.T) {
	s := obsSchema(t)
	attr := s.Attributes["config_wo"].(schema.StringAttribute)
	if !attr.Optional || !attr.WriteOnly || !attr.Sensitive || attr.Computed {
		t.Fatal("config must be write-only optional sensitive")
	}
	for _, name := range []string{"workspace_id", "type"} {
		if len(s.Attributes[name].(schema.StringAttribute).PlanModifiers) == 0 {
			t.Fatal("immutable identity missing modifier")
		}
	}
	for _, input := range []string{`null`, `[]`, `true`, `{"secret":null}`, `{"nested":{"secret":null}}`, `{"publicKey":"pk-l...EfGh"}`, `{"secretKey":"sk-l...AbCd"}`, `{"secret":`} {
		m := obsModel()
		m.ConfigWO = types.StringValue(input)
		d := validateObservabilityConfig(context.Background(), m)
		if !d.HasError() {
			t.Fatalf("accepted invalid configuration %q", input)
		}
		if strings.Contains(d.Errors()[0].Detail(), "****") {
			t.Fatal("configuration leaked in diagnostics")
		}
	}
	m := obsModel()
	m.ConfigWO = types.StringValue(`{"secret":"canary","headers":{"x":"value"},"flag":true}`)
	if d := validateObservabilityConfig(context.Background(), m); d.HasError() {
		t.Fatal(d)
	}
	m.Type = types.StringValue("s3")
	m.ConfigWO = types.StringValue(`{"authMode":"iam_role","bucketName":"test"}`)
	if !validateObservabilityConfig(context.Background(), m).HasError() {
		t.Fatal("unsupported IAM-role bootstrap accepted")
	}
}

func TestObservabilityConfigurationPunctuationIsNotUniversalMasking(t *testing.T) {
	m := obsModel()
	m.ConfigWO = types.StringValue(`{"headers":{"x-context":"trace...context","x-pattern":"****"},"baseUrl":"https://example.test/path...segment"}`)
	if d := validateObservabilityConfig(context.Background(), m); d.HasError() {
		t.Fatal("ordinary nested strings must not be rejected by an undocumented masking heuristic:", d)
	}
	// Documented Langfuse placeholders have meaning only at their credential fields.
	m.ConfigWO = types.StringValue(`{"headers":{"publicKey":"pk-l...EfGh"}}`)
	if d := validateObservabilityConfig(context.Background(), m); d.HasError() {
		t.Fatal(d)
	}
	m.Type = types.StringValue("webhook")
	m.ConfigWO = types.StringValue(`{"secretKey":"sk-l...AbCd"}`)
	if d := validateObservabilityConfig(context.Background(), m); d.HasError() {
		t.Fatal("do not impose a Langfuse credential rule on another destination type:", d)
	}
}

func TestObservabilityValidationDefersUnknownSetElements(t *testing.T) {
	for _, field := range []string{"regions", "api_key_hashes"} {
		t.Run(field, func(t *testing.T) {
			value, diags := types.SetValue(types.StringType, []attr.Value{types.StringUnknown()})
			if diags.HasError() {
				t.Fatal(diags)
			}
			m := obsModel()
			if field == "regions" {
				m.Regions = value
			} else {
				m.APIKeyHashes = value
			}
			if diags := validateObservabilityConfig(context.Background(), m); diags.HasError() {
				t.Fatal("computed values must be allowed during planning:", diags)
			}
			for _, invalid := range []types.String{types.StringNull(), types.StringValue("invalid")} {
				mixed, diags := types.SetValue(types.StringType, []attr.Value{types.StringUnknown(), invalid})
				if diags.HasError() {
					t.Fatal(diags)
				}
				if field == "regions" {
					m.Regions = mixed
				} else {
					m.APIKeyHashes = mixed
				}
				if diags := validateObservabilityConfig(context.Background(), m); !diags.HasError() {
					t.Fatal("unknown elements must not suppress validation of invalid known or null elements")
				}
			}
		})
	}
}
func TestObservabilityIdentityPreflightPreventsMutations(t *testing.T) {
	for _, method := range []string{"update", "delete"} {
		t.Run(method, func(t *testing.T) {
			writes := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					writes++
				}
				m := obsResponse()
				m["workspace_id"] = "other"
				_ = json.NewEncoder(w).Encode(map[string]any{"data": m})
			}))
			defer server.Close()
			r := &observabilityDestinationResource{client: client.New("local", server.URL, "test", time.Second)}
			ctx := context.Background()
			s := obsSchema(t)
			m := obsModel()
			state := tfsdk.State{Schema: s}
			if d := state.Set(ctx, &m); d.HasError() {
				t.Fatal(d)
			}
			if method == "update" {
				plan := tfsdk.Plan{Schema: s}
				_ = plan.Set(ctx, &m)
				config := tfsdk.Config{Schema: s, Raw: plan.Raw}
				var resp resource.UpdateResponse
				r.Update(ctx, resource.UpdateRequest{State: state, Plan: plan, Config: config}, &resp)
				if !resp.Diagnostics.HasError() {
					t.Fatal("identity mismatch accepted")
				}
			} else {
				var resp resource.DeleteResponse
				r.Delete(ctx, resource.DeleteRequest{State: state}, &resp)
				if !resp.Diagnostics.HasError() {
					t.Fatal("identity mismatch accepted")
				}
			}
			if writes != 0 {
				t.Fatal("mutation performed before identity verified")
			}
		})
	}
}
func TestObservabilityReadOnlyRemovesReal404(t *testing.T) {
	for _, status := range []int{404, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"error":{"message":"secret (404)"}}`))
			}))
			defer server.Close()
			ctx := context.Background()
			state := tfsdk.State{Schema: obsSchema(t)}
			m := obsModel()
			_ = state.Set(ctx, &m)
			resp := resource.ReadResponse{State: state}
			(&observabilityDestinationResource{client: client.New("local", server.URL, "test", time.Second)}).Read(ctx, resource.ReadRequest{State: state}, &resp)
			if status == 404 {
				if !resp.State.Raw.IsNull() {
					t.Fatal("404 did not remove state")
				}
			} else if !resp.Diagnostics.HasError() || resp.State.Raw.IsNull() {
				t.Fatal("non404 error swallowed")
			}
		})
	}
}

func TestObservabilityCreateMalformedMetadataPreservesTrustedIdentity(t *testing.T) {
	for _, test := range []struct {
		name    string
		mutate  func(map[string]any)
		trusted bool
	}{
		{"missing metadata", func(m map[string]any) { delete(m, "enabled") }, true},
		{"null metadata", func(m map[string]any) { m["privacy_mode"] = nil }, true},
		{"wrong workspace", func(m map[string]any) { m["workspace_id"] = "other" }, false},
		{"wrong type", func(m map[string]any) { m["type"] = "webhook" }, false},
		{"invalid id", func(m map[string]any) { m["id"] = "invalid"; delete(m, "enabled") }, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			posts := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				posts++
				m := obsResponse()
				test.mutate(m)
				_ = json.NewEncoder(w).Encode(map[string]any{"data": m})
			}))
			defer server.Close()
			ctx := context.Background()
			schema := obsSchema(t)
			m := obsModel()
			m.ID = types.StringUnknown()
			m.CreatedAt, m.UpdatedAt = types.StringUnknown(), types.StringUnknown()
			plan := tfsdk.Plan{Schema: schema}
			if d := plan.Set(ctx, &m); d.HasError() {
				t.Fatal(d)
			}
			m.ConfigWO = types.StringValue(`{"secretKey":"submitted-secret-canary"}`)
			configPlan := tfsdk.Plan{Schema: schema}
			if d := configPlan.Set(ctx, &m); d.HasError() {
				t.Fatal(d)
			}
			resp := resource.CreateResponse{State: tfsdk.State{Schema: schema}}
			(&observabilityDestinationResource{client: client.New("local", server.URL, "test", time.Second)}).Create(ctx, resource.CreateRequest{Plan: plan, Config: tfsdk.Config{Schema: schema, Raw: configPlan.Raw}}, &resp)
			if !resp.Diagnostics.HasError() || posts != 1 {
				t.Fatalf("diagnostics=%v requests=%d", resp.Diagnostics, posts)
			}
			if test.trusted {
				var recovered observabilityDestinationModel
				if d := resp.State.Get(ctx, &recovered); d.HasError() {
					t.Fatal(d)
				}
				if recovered.ID.ValueString() != obsID || !recovered.ConfigWO.IsNull() || !recovered.CreatedAt.IsNull() || !resp.State.Raw.IsFullyKnown() {
					t.Fatal("invalid recovery state")
				}
				if !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), "invalid observability destination create metadata") {
					t.Fatal("original create failure lost")
				}
			} else if !resp.State.Raw.IsNull() {
				t.Fatal("untrusted identity adopted")
			}
			text := resp.State.Raw.String()
			for _, diagnostic := range resp.Diagnostics {
				text += diagnostic.Detail()
			}
			if strings.Contains(text, "secret-canary") || strings.Contains(text, "secret-canary-response") {
				t.Fatal("secret response or configuration leaked")
			}
		})
	}
}
