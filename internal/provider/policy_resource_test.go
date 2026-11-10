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
	fr "github.com/hashicorp/terraform-plugin-framework/resource"
	rs "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func policyTestSchema(t *testing.T, kind string) rs.Schema {
	t.Helper()
	var resp fr.SchemaResponse
	(&policyResource{kind: kind}).Schema(context.Background(), fr.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	return resp.Schema
}
func policyTestModel(t *testing.T, kind, raw string) policyResourceModel {
	t.Helper()
	object, diags := policyConfigState(context.Background(), kind, json.RawMessage(raw))
	if diags.HasError() {
		t.Fatal(diags)
	}
	return policyResourceModel{ID: types.StringValue("policy"), WorkspaceID: types.StringValue("ws"), Name: types.StringValue("policy name"), Config: object}
}
func policyTestState(t *testing.T, schema rs.Schema, model policyResourceModel) tfsdk.State {
	t.Helper()
	state := tfsdk.State{Schema: schema}
	if d := state.Set(context.Background(), model); d.HasError() {
		t.Fatal(d)
	}
	return state
}
func policyTestPlan(t *testing.T, schema rs.Schema, model policyResourceModel) tfsdk.Plan {
	t.Helper()
	plan := tfsdk.Plan{Schema: schema}
	if d := plan.Set(context.Background(), model); d.HasError() {
		t.Fatal(d)
	}
	return plan
}

func TestPolicyResourceLifecycle(t *testing.T) {
	for _, kind := range []string{"budget", "model_availability"} {
		t.Run(kind, func(t *testing.T) {
			initial := `{"limit_usd":25,"reset_interval":"monthly","include_byok_in_budgets":false}`
			updated := `{"limit_usd":null,"reset_interval":null,"include_byok_in_budgets":true}`
			if kind != "budget" {
				initial = `{"allowed_models":["openai/gpt-4o"],"allowed_data_regions":["eu"],"enforce_zdr_xai":false}`
				updated = `{"allowed_models":[],"allowed_data_regions":null,"enforce_zdr_xai":true}`
			}
			var remote map[string]any
			deleted := false
			var writes int
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if (r.Method == http.MethodGet || r.Method == http.MethodDelete) && r.URL.Query().Get("workspace_id") != "ws" {
					t.Error("missing workspace query")
				}
				switch r.Method {
				case http.MethodPost, http.MethodPut:
					writes++
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					for _, k := range []string{"member_ids", "api_key_hashes", "is_workspace_default", "opted_out_api_key_hashes", "opted_out_member_ids"} {
						if _, ok := body[k]; ok {
							t.Errorf("write unexpectedly owns %s", k)
						}
					}
					remote = map[string]any{"id": "policy", "name": body["name"], "policy_type": kind, "config": body["config"], "member_ids": []string{"external-member"}, "api_key_hashes": []string{"external-key"}}
				case http.MethodDelete:
					deleted = true
					w.WriteHeader(http.StatusNoContent)
					return
				case http.MethodGet:
					if deleted {
						http.Error(w, "missing", 404)
						return
					}
				default:
					t.Errorf("unexpected method %s", r.Method)
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"data": remote})
			}))
			defer server.Close()
			schema := policyTestSchema(t, kind)
			r := &policyResource{kind: kind, client: client.New("management", server.URL, "", time.Second)}
			ctx := context.Background()
			model := policyTestModel(t, kind, initial)
			model.ID = types.StringUnknown()
			create := fr.CreateResponse{State: tfsdk.State{Schema: schema}}
			r.Create(ctx, fr.CreateRequest{Plan: policyTestPlan(t, schema, model)}, &create)
			if create.Diagnostics.HasError() {
				t.Fatal(create.Diagnostics)
			}
			var actual policyResourceModel
			if d := create.State.Get(ctx, &actual); d.HasError() {
				t.Fatal(d)
			}
			if actual.ID.ValueString() != "policy" || !actual.Config.Equal(model.Config) {
				t.Fatalf("create model mismatch: %+v", actual)
			}
			read := fr.ReadResponse{State: create.State}
			r.Read(ctx, fr.ReadRequest{State: create.State}, &read)
			if read.Diagnostics.HasError() || !read.State.Raw.Equal(create.State.Raw) {
				t.Fatalf("refresh changed state: %s", read.Diagnostics)
			}
			imported := fr.ImportStateResponse{State: tfsdk.State{Schema: schema}}
			r.ImportState(ctx, fr.ImportStateRequest{ID: "ws/policy"}, &imported)
			if imported.Diagnostics.HasError() || !imported.State.Raw.Equal(create.State.Raw) {
				t.Fatalf("import mismatch: %s", imported.Diagnostics)
			}
			plan := policyTestModel(t, kind, updated)
			update := fr.UpdateResponse{State: create.State}
			r.Update(ctx, fr.UpdateRequest{State: create.State, Plan: policyTestPlan(t, schema, plan)}, &update)
			if update.Diagnostics.HasError() {
				t.Fatal(update.Diagnostics)
			}
			if d := update.State.Get(ctx, &actual); d.HasError() {
				t.Fatal(d)
			}
			if !actual.Config.Equal(plan.Config) {
				t.Fatalf("update config mismatch %s versus %s", actual.Config, plan.Config)
			}
			deletion := fr.DeleteResponse{State: update.State}
			r.Delete(ctx, fr.DeleteRequest{State: update.State}, &deletion)
			if deletion.Diagnostics.HasError() {
				t.Fatal(deletion.Diagnostics)
			}
			missing := fr.ReadResponse{State: update.State}
			r.Read(ctx, fr.ReadRequest{State: update.State}, &missing)
			if missing.Diagnostics.HasError() || !missing.State.Raw.IsNull() {
				t.Fatal("404 did not remove resource")
			}
			if writes != 2 {
				t.Fatalf("writes=%d", writes)
			}
		})
	}
}

func TestPolicyResourceUnsafeOwnershipNeverWrites(t *testing.T) {
	for _, marker := range []string{`"reference":"legacy_mapping"`, `"reference":"account_default"`, `"is_workspace_default":true`, `"is_account_policy":true`, `"replaces_legacy_default":true`} {
		t.Run(marker, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("unsafe write %s", r.Method)
				}
				_, _ = w.Write([]byte(`{"data":{"id":"policy","name":"policy name","policy_type":"budget","config":{"include_byok_in_budgets":false},` + marker + `}}`))
			}))
			defer server.Close()
			schema := policyTestSchema(t, "budget")
			model := policyTestModel(t, "budget", `{"include_byok_in_budgets":false}`)
			state := policyTestState(t, schema, model)
			r := &policyResource{kind: "budget", client: client.New("management", server.URL, "", time.Second)}
			ctx := context.Background()
			update := fr.UpdateResponse{State: state}
			r.Update(ctx, fr.UpdateRequest{State: state, Plan: policyTestPlan(t, schema, model)}, &update)
			if !update.Diagnostics.HasError() {
				t.Fatal("unsafe update accepted")
			}
			deletion := fr.DeleteResponse{State: state}
			r.Delete(ctx, fr.DeleteRequest{State: state}, &deletion)
			if !deletion.Diagnostics.HasError() {
				t.Fatal("unsafe delete accepted")
			}
			imported := fr.ImportStateResponse{State: tfsdk.State{Schema: schema}}
			r.ImportState(ctx, fr.ImportStateRequest{ID: "ws/policy"}, &imported)
			if !imported.Diagnostics.HasError() {
				t.Fatal("unsafe import accepted")
			}
		})
	}
}

func TestPolicyResourceValidationAndErrors(t *testing.T) {
	ctx := context.Background()
	for _, test := range []struct{ kind, config string }{
		{"budget", `{"limit_usd":0,"include_byok_in_budgets":false}`},
		{"budget", `{"limit_usd":-1,"include_byok_in_budgets":false}`},
		{"budget", `{"reset_interval":"","include_byok_in_budgets":false}`},
		{"model_availability", `{"allowed_data_regions":[]}`},
	} {
		if d := validatePolicyConfig(ctx, test.kind, policyTestModel(t, test.kind, test.config)); !d.HasError() {
			t.Errorf("invalid config accepted %s", test.config)
		}
	}
	model := policyTestModel(t, "budget", `{"include_byok_in_budgets":false}`)
	for _, name := range []string{"", strings.Repeat("x", 256)} {
		model.Name = types.StringValue(name)
		if d := validatePolicyConfig(ctx, "budget", model); !d.HasError() {
			t.Error("invalid name accepted")
		}
	}
	for _, status := range []int{403, 409, 500} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "upstream mentioned (404)", status) }))
		schema := policyTestSchema(t, "budget")
		model = policyTestModel(t, "budget", `{"include_byok_in_budgets":false}`)
		state := policyTestState(t, schema, model)
		r := &policyResource{kind: "budget", client: client.New("management", server.URL, "", time.Second)}
		response := fr.ReadResponse{State: state}
		r.Read(ctx, fr.ReadRequest{State: state}, &response)
		if !response.Diagnostics.HasError() || response.State.Raw.IsNull() {
			t.Errorf("status %d swallowed or erased state", status)
		}
		server.Close()
	}
	r := &policyResource{kind: "budget"}
	schema := policyTestSchema(t, "budget")
	for _, id := range []string{"", "workspace", "/policy", "ws/", "ws/policy/other"} {
		response := fr.ImportStateResponse{State: tfsdk.State{Schema: schema}}
		r.ImportState(ctx, fr.ImportStateRequest{ID: id}, &response)
		if !response.Diagnostics.HasError() {
			t.Errorf("invalid import accepted %s", id)
		}
	}
}

func TestPolicyResourcePreservesCreatedIdentityWhenRefreshFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			http.Error(w, "unavailable", 500)
			return
		}
		_, _ = w.Write([]byte(`{"data":{"id":"created-policy","name":"policy name","policy_type":"budget","config":{"include_byok_in_budgets":false}}}`))
	}))
	defer server.Close()
	schema := policyTestSchema(t, "budget")
	model := policyTestModel(t, "budget", `{"include_byok_in_budgets":false}`)
	model.ID = types.StringUnknown()
	r := &policyResource{kind: "budget", client: client.New("management", server.URL, "", time.Second)}
	response := fr.CreateResponse{State: tfsdk.State{Schema: schema}}
	r.Create(context.Background(), fr.CreateRequest{Plan: policyTestPlan(t, schema, model)}, &response)
	if !response.Diagnostics.HasError() {
		t.Fatal("refresh failure not reported")
	}
	var actual policyResourceModel
	if d := response.State.Get(context.Background(), &actual); d.HasError() {
		t.Fatal(d)
	}
	if actual.ID.ValueString() != "created-policy" {
		t.Fatal("created identity lost after refresh failure")
	}
}

func TestPolicyResourceUpdateRetainsChangedResponseIdentityForRecovery(t *testing.T) {
	for _, kind := range []string{"budget", "model_availability"} {
		t.Run(kind, func(t *testing.T) {
			writes, reads := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				id := "policy"
				switch r.Method {
				case http.MethodPut:
					writes++
					id = "unexpected-copy"
				case http.MethodGet:
					reads++
				default:
					t.Errorf("unexpected recovery operation %s", r.Method)
				}
				config := map[string]any{}
				if kind == "budget" {
					config["include_byok_in_budgets"] = false
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": id, "name": "policy name", "policy_type": kind, "config": config}})
			}))
			defer server.Close()
			initial, changed := `{"include_byok_in_budgets":false}`, `{"include_byok_in_budgets":true}`
			if kind != "budget" {
				initial = `{"enforce_zdr_xai":false}`
				changed = `{"enforce_zdr_xai":true}`
			}
			schema := policyTestSchema(t, kind)
			model := policyTestModel(t, kind, initial)
			state := policyTestState(t, schema, model)
			plan := policyTestModel(t, kind, changed)
			plan.Name = types.StringValue("requested name")
			r := &policyResource{kind: kind, client: client.New("management", server.URL, "", time.Second)}
			response := fr.UpdateResponse{State: state}
			r.Update(context.Background(), fr.UpdateRequest{State: state, Plan: policyTestPlan(t, schema, plan)}, &response)
			if !response.Diagnostics.HasError() || writes != 1 || reads != 1 {
				t.Fatal("identity-changing update not reported exactly once without retry/compensation")
			}
			if !strings.Contains(response.Diagnostics.Errors()[0].Detail(), "unexpected-copy") {
				t.Fatal("unexpected new identity not reported for reconciliation")
			}
			var actual policyResourceModel
			if d := response.State.Get(context.Background(), &actual); d.HasError() {
				t.Fatal(d)
			}
			if actual.ID.ValueString() != "unexpected-copy" || !actual.WorkspaceID.Equal(plan.WorkspaceID) || !actual.Name.Equal(plan.Name) || !actual.Config.Equal(plan.Config) {
				t.Fatalf("validated new identity and requested state not retained: %+v", actual)
			}
		})
	}
}

func TestPolicyResourceMalformedUpdateResponseDoesNotChangeTrackedIdentity(t *testing.T) {
	for _, body := range []string{
		`{"data":{"id":"unvalidated-copy","name":"policy name","policy_type":"moderation","config":{}}}`,
		`{"data":{"id":"unvalidated-copy","name":"policy name","policy_type":"budget","config":{"limit_usd":"invalid"}}}`,
		`{"data":{"id":"unvalidated-copy","name":"policy name","policy_type":"budget","config":[]}}`,
	} {
		t.Run(body, func(t *testing.T) {
			writes := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPut {
					writes++
					_, _ = w.Write([]byte(body))
					return
				}
				if r.Method != http.MethodGet {
					t.Errorf("unexpected request %s", r.Method)
				}
				_, _ = w.Write([]byte(`{"data":{"id":"policy","name":"policy name","policy_type":"budget","config":{"include_byok_in_budgets":false}}}`))
			}))
			defer server.Close()
			schema := policyTestSchema(t, "budget")
			model := policyTestModel(t, "budget", `{"include_byok_in_budgets":false}`)
			state := policyTestState(t, schema, model)
			r := &policyResource{kind: "budget", client: client.New("management", server.URL, "", time.Second)}
			response := fr.UpdateResponse{State: state}
			r.Update(context.Background(), fr.UpdateRequest{State: state, Plan: policyTestPlan(t, schema, model)}, &response)
			if !response.Diagnostics.HasError() || writes != 1 {
				t.Fatal("malformed update not rejected exactly once")
			}
			if !response.State.Raw.Equal(state.Raw) {
				t.Fatal("unvalidated response overwrote tracked state")
			}
		})
	}
}

func TestPolicyResourceMalformedWireConfigRetainsOriginalIdentity(t *testing.T) {
	tests := []struct{ kind, config string }{{"budget", `{"include_byok_in_budgets":null}`}}
	for _, field := range []string{"allowed_data_regions", "allowed_models", "allowed_providers", "ignored_models", "ignored_providers", "platform_banned_authors", "platform_banned_models", "platform_banned_providers"} {
		tests = append(tests, struct{ kind, config string }{"model_availability", `{"` + field + `":["valid",null]}`})
	}
	for _, test := range tests {
		t.Run(test.kind+test.config, func(t *testing.T) {
			writes, reads := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				id, config := "policy", json.RawMessage(`{}`)
				switch r.Method {
				case http.MethodPut:
					writes++
					id = "malformed-copy"
					config = json.RawMessage(test.config)
				case http.MethodGet:
					reads++
				default:
					t.Errorf("unexpected operation %s", r.Method)
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": id, "name": "policy name", "policy_type": test.kind, "config": config}})
			}))
			defer server.Close()
			initial := `{"include_byok_in_budgets":false}`
			if test.kind != "budget" {
				initial = `{"enforce_zdr_xai":true}`
			}
			schema := policyTestSchema(t, test.kind)
			model := policyTestModel(t, test.kind, initial)
			state := policyTestState(t, schema, model)
			r := &policyResource{kind: test.kind, client: client.New("management", server.URL, "", time.Second)}
			response := fr.UpdateResponse{State: state}
			r.Update(context.Background(), fr.UpdateRequest{State: state, Plan: policyTestPlan(t, schema, model)}, &response)
			if !response.Diagnostics.HasError() || writes != 1 || reads != 1 {
				t.Fatal("malformed response not rejected exactly once without retries")
			}
			if !response.State.Raw.Equal(state.Raw) {
				t.Fatal("malformed config overwrote original identity/state")
			}
		})
	}
}

func TestPolicyResourceNullOwnershipFlagsNeverAuthorizeMutation(t *testing.T) {
	for _, field := range []string{"is_workspace_default", "is_account_policy", "replaces_legacy_default"} {
		t.Run(field, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("null ownership flag authorized mutation %s", r.Method)
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"id": "policy", "name": "policy name", "policy_type": "budget", "config": map[string]any{}, field: nil}})
			}))
			defer server.Close()
			schema := policyTestSchema(t, "budget")
			model := policyTestModel(t, "budget", `{"include_byok_in_budgets":false}`)
			state := policyTestState(t, schema, model)
			r := &policyResource{kind: "budget", client: client.New("management", server.URL, "", time.Second)}
			ctx := context.Background()
			update := fr.UpdateResponse{State: state}
			r.Update(ctx, fr.UpdateRequest{State: state, Plan: policyTestPlan(t, schema, model)}, &update)
			if !update.Diagnostics.HasError() || !update.State.Raw.Equal(state.Raw) {
				t.Fatal("null ownership flag allowed update or changed state")
			}
			deletion := fr.DeleteResponse{State: state}
			r.Delete(ctx, fr.DeleteRequest{State: state}, &deletion)
			if !deletion.Diagnostics.HasError() {
				t.Fatal("null ownership flag allowed delete")
			}
			imported := fr.ImportStateResponse{State: tfsdk.State{Schema: schema}}
			r.ImportState(ctx, fr.ImportStateRequest{ID: "ws/policy"}, &imported)
			if !imported.Diagnostics.HasError() {
				t.Fatal("null ownership flag allowed import")
			}
		})
	}
}
