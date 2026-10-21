package provider

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/cloudopsworks/terraform-provider-openrouter/internal/client"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func budgetTestSchema(t *testing.T) resourceschema.Schema {
	t.Helper()
	var response frameworkresource.SchemaResponse
	NewWorkspaceBudgetResource().Schema(context.Background(), frameworkresource.SchemaRequest{}, &response)
	if response.Diagnostics.HasError() {
		t.Fatal(response.Diagnostics)
	}
	return response.Schema
}

func budgetTestPlan(t *testing.T, schema resourceschema.Schema, workspace, interval string, limit float64) tfsdk.Plan {
	t.Helper()
	plan := tfsdk.Plan{Schema: schema}
	model := workspaceBudgetModel{
		ID:          types.StringUnknown(),
		WorkspaceID: types.StringValue(workspace),
		Interval:    types.StringValue(interval),
		LimitUSD:    types.Float64Value(limit),
		CreatedAt:   types.StringUnknown(),
		UpdatedAt:   types.StringUnknown(),
	}
	if diagnostics := plan.Set(context.Background(), &model); diagnostics.HasError() {
		t.Fatal(diagnostics)
	}
	return plan
}

func budgetTestState(t *testing.T, schema resourceschema.Schema, workspace, interval string, limit float64) tfsdk.State {
	t.Helper()
	state := tfsdk.State{Schema: schema}
	model := workspaceBudgetModel{
		ID:          types.StringValue(budgetID(workspace, interval)),
		WorkspaceID: types.StringValue(workspace),
		Interval:    types.StringValue(interval),
		LimitUSD:    types.Float64Value(limit),
		CreatedAt:   types.StringValue("2026-01-01T00:00:00Z"),
		UpdatedAt:   types.StringValue("2026-01-01T00:00:00Z"),
	}
	if diagnostics := state.Set(context.Background(), &model); diagnostics.HasError() {
		t.Fatal(diagnostics)
	}
	return state
}

func budgetAPI(interval any, limit float64) map[string]any {
	return map[string]any{"id": "budget-uuid", "workspace_id": "canonical-uuid", "reset_interval": interval, "limit_usd": limit, "created_at": "created", "updated_at": "updated"}
}

func TestWorkspaceBudgetCreateCollisionDoesNotOverwrite(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/workspaces/workspace-slug/budgets" {
			t.Fatalf("create collision must only list budgets; got %s %s", request.Method, request.URL.Path)
		}
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{"data": []any{budgetAPI("daily", 10)}})
	}))
	defer server.Close()

	schema := budgetTestSchema(t)
	resource := &workspaceBudgetResource{client: client.New("token", server.URL, "", time.Second)}
	response := frameworkresource.CreateResponse{State: tfsdk.State{Schema: schema}}
	resource.Create(context.Background(), frameworkresource.CreateRequest{Plan: budgetTestPlan(t, schema, "workspace-slug", "daily", 12)}, &response)
	if !response.Diagnostics.HasError() {
		t.Fatal("expected collision diagnostic")
	}
}

func TestWorkspaceBudgetCreatePreservesConfiguredWorkspaceReference(t *testing.T) {
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls = append(calls, request.Method+" "+request.URL.Path)
		writer.Header().Set("Content-Type", "application/json")
		if request.Method == http.MethodGet {
			_ = json.NewEncoder(writer).Encode(map[string]any{"data": []any{}})
			return
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{"data": budgetAPI("daily", 12)})
	}))
	defer server.Close()

	schema := budgetTestSchema(t)
	resource := &workspaceBudgetResource{client: client.New("token", server.URL, "", time.Second)}
	response := frameworkresource.CreateResponse{State: tfsdk.State{Schema: schema}}
	resource.Create(context.Background(), frameworkresource.CreateRequest{Plan: budgetTestPlan(t, schema, "workspace-slug", "daily", 12)}, &response)
	if response.Diagnostics.HasError() {
		t.Fatal(response.Diagnostics)
	}
	var state workspaceBudgetModel
	if diagnostics := response.State.Get(context.Background(), &state); diagnostics.HasError() {
		t.Fatal(diagnostics)
	}
	if state.WorkspaceID.ValueString() != "workspace-slug" || state.ID.ValueString() != "workspace-slug/daily" {
		t.Fatalf("state identity = %q/%q", state.WorkspaceID.ValueString(), state.ID.ValueString())
	}
	if want := []string{"GET /workspaces/workspace-slug/budgets", "PUT /workspaces/workspace-slug/budgets/daily"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
}

func TestWorkspaceBudgetReadMissingAnd404RemoveState(t *testing.T) {
	for name, handler := range map[string]http.HandlerFunc{
		"absent": func(writer http.ResponseWriter, request *http.Request) {
			writer.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(writer).Encode(map[string]any{"data": []any{}})
		},
		"not found": func(writer http.ResponseWriter, request *http.Request) {
			http.Error(writer, "missing", http.StatusNotFound)
		},
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(handler)
			defer server.Close()
			schema := budgetTestSchema(t)
			state := budgetTestState(t, schema, "workspace", "daily", 10)
			resource := &workspaceBudgetResource{client: client.New("token", server.URL, "", time.Second)}
			response := frameworkresource.ReadResponse{State: state}
			resource.Read(context.Background(), frameworkresource.ReadRequest{State: state}, &response)
			if response.Diagnostics.HasError() || !response.State.Raw.IsNull() {
				t.Fatalf("diagnostics = %s, state null = %t", response.Diagnostics, response.State.Raw.IsNull())
			}
		})
	}
}

func TestWorkspaceBudgetUpdateAndDeletePostconditions(t *testing.T) {
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls = append(calls, request.Method+" "+request.URL.Path)
		writer.Header().Set("Content-Type", "application/json")
		switch request.Method {
		case http.MethodPut:
			_ = json.NewEncoder(writer).Encode(map[string]any{"data": budgetAPI("daily", 20)})
		case http.MethodDelete:
			writer.WriteHeader(http.StatusNoContent)
		case http.MethodGet:
			_ = json.NewEncoder(writer).Encode(map[string]any{"data": []any{}})
		default:
			t.Fatalf("unexpected request: %s", request.Method)
		}
	}))
	defer server.Close()

	schema := budgetTestSchema(t)
	resource := &workspaceBudgetResource{client: client.New("token", server.URL, "", time.Second)}
	state := budgetTestState(t, schema, "workspace", "daily", 10)
	updateResponse := frameworkresource.UpdateResponse{State: tfsdk.State{Schema: schema}}
	resource.Update(context.Background(), frameworkresource.UpdateRequest{State: state, Plan: budgetTestPlan(t, schema, "workspace", "daily", 20)}, &updateResponse)
	if updateResponse.Diagnostics.HasError() {
		t.Fatal(updateResponse.Diagnostics)
	}
	deleteResponse := frameworkresource.DeleteResponse{State: state}
	resource.Delete(context.Background(), frameworkresource.DeleteRequest{State: state}, &deleteResponse)
	if deleteResponse.Diagnostics.HasError() {
		t.Fatal(deleteResponse.Diagnostics)
	}
	want := []string{"PUT /workspaces/workspace/budgets/daily", "DELETE /workspaces/workspace/budgets/daily", "GET /workspaces/workspace/budgets"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
}

func TestWorkspaceBudgetDeleteReportsRemainingBudget(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if request.Method == http.MethodDelete {
			writer.WriteHeader(http.StatusNoContent)
			return
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{"data": []any{budgetAPI("daily", 10)}})
	}))
	defer server.Close()
	schema := budgetTestSchema(t)
	state := budgetTestState(t, schema, "workspace", "daily", 10)
	response := frameworkresource.DeleteResponse{State: state}
	(&workspaceBudgetResource{client: client.New("token", server.URL, "", time.Second)}).Delete(context.Background(), frameworkresource.DeleteRequest{State: state}, &response)
	if !response.Diagnostics.HasError() {
		t.Fatal("expected persisted budget diagnostic")
	}
}

func TestWorkspaceBudgetImportAndValidation(t *testing.T) {
	schema := budgetTestSchema(t)
	resource := &workspaceBudgetResource{}
	for _, id := range []string{"workspace/daily", "workspace/lifetime"} {
		response := frameworkresource.ImportStateResponse{State: tfsdk.State{Schema: schema}}
		resource.ImportState(context.Background(), frameworkresource.ImportStateRequest{ID: id}, &response)
		if response.Diagnostics.HasError() {
			t.Fatalf("import %q: %s", id, response.Diagnostics)
		}
	}
	for _, id := range []string{"bad", "workspace/yearly", "workspace/daily/extra", "/daily"} {
		response := frameworkresource.ImportStateResponse{State: tfsdk.State{Schema: schema}}
		resource.ImportState(context.Background(), frameworkresource.ImportStateRequest{ID: id}, &response)
		if !response.Diagnostics.HasError() {
			t.Fatalf("expected malformed import %q diagnostic", id)
		}
	}
	for _, model := range []workspaceBudgetModel{
		{WorkspaceID: types.StringValue(""), Interval: types.StringValue("daily"), LimitUSD: types.Float64Value(1)},
		{WorkspaceID: types.StringValue("workspace"), Interval: types.StringValue("yearly"), LimitUSD: types.Float64Value(1)},
		{WorkspaceID: types.StringValue("workspace"), Interval: types.StringValue("daily"), LimitUSD: types.Float64Value(math.Inf(1))},
	} {
		if err := validBudget(model); err == nil {
			t.Fatal("expected invalid budget")
		}
	}
}

func TestWorkspaceBudgetCreatePreservesIdentityAfterInvalidPutResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if request.Method == http.MethodGet {
			_ = json.NewEncoder(writer).Encode(map[string]any{"data": []any{}})
			return
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{"data": budgetAPI("daily", 99)})
	}))
	defer server.Close()

	schema := budgetTestSchema(t)
	response := frameworkresource.CreateResponse{State: tfsdk.State{Schema: schema}}
	(&workspaceBudgetResource{client: client.New("token", server.URL, "", time.Second)}).Create(
		context.Background(), frameworkresource.CreateRequest{Plan: budgetTestPlan(t, schema, "workspace-slug", "daily", 12)}, &response,
	)
	if !response.Diagnostics.HasError() {
		t.Fatal("expected response mismatch diagnostic")
	}
	var state workspaceBudgetModel
	if diagnostics := response.State.Get(context.Background(), &state); diagnostics.HasError() {
		t.Fatal(diagnostics)
	}
	if state.ID.ValueString() != "workspace-slug/daily" || state.WorkspaceID.ValueString() != "workspace-slug" || state.LimitUSD.ValueFloat64() != 12 {
		t.Fatalf("recovery state = %#v", state)
	}
}

func TestWorkspaceBudgetCreateRejectsInvalidConfigBeforeRequests(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		t.Fatalf("invalid config must not make requests: %s %s", request.Method, request.URL.Path)
	}))
	defer server.Close()

	schema := budgetTestSchema(t)
	for _, input := range []struct {
		interval string
		limit    float64
	}{
		{interval: "yearly", limit: 1},
		{interval: "daily", limit: 0},
	} {
		response := frameworkresource.CreateResponse{State: tfsdk.State{Schema: schema}}
		(&workspaceBudgetResource{client: client.New("token", server.URL, "", time.Second)}).Create(
			context.Background(), frameworkresource.CreateRequest{Plan: budgetTestPlan(t, schema, "workspace", input.interval, input.limit)}, &response,
		)
		if !response.Diagnostics.HasError() {
			t.Fatalf("expected invalid config diagnostic for %#v", input)
		}
	}
}
