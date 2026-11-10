package provider

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// These tests deliberately manage only fresh workspaces and fresh, disabled,
// zero-limit keys. They never run inference or configure trace forwarding.
func safeExpandedAccPreflight(t *testing.T) {
	t.Helper()
	testAccSkipUnlessEnabled(t)
	if os.Getenv("OPENROUTER_SAFE_EXPANDED_LIVE") != "1" {
		t.Skip("set OPENROUTER_SAFE_EXPANDED_LIVE=1 for isolated management-plane acceptance tests")
	}
	if strings.TrimSpace(os.Getenv("OPENROUTER_BASE_URL")) != "" {
		t.Fatal("unset OPENROUTER_BASE_URL: acceptance verification uses the official API")
	}
	testAccPreCheck(t)
}

func safeExpandedWorkspaceConfig(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf(`%s
resource "openrouter_workspace" "test" {
 name = %q
 slug = %q
}
`, testAccProviderConfig, testAccRandomName("acct-safe"), testAccRandomName("acct-safe"))
}

func safeExpandedImportID(address string, attributes ...string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[address]
		if !ok || rs.Primary == nil {
			return "", fmt.Errorf("missing test resource %s", address)
		}
		parts := make([]string, 0, len(attributes))
		for _, attribute := range attributes {
			value := rs.Primary.Attributes[attribute]
			if attribute == "id" {
				value = rs.Primary.ID
			}
			if value == "" {
				return "", fmt.Errorf("missing import attribute %s", attribute)
			}
			parts = append(parts, value)
		}
		return strings.Join(parts, "/"), nil
	}
}

// Keep the workspace while removing children so a missing workspace cannot
// conceal a child-delete failure. CheckDestroy independently verifies workspace.
func safeExpandedLifecycle(t *testing.T, workspace, initial, updated, address string, importID resource.ImportStateIdFunc, checks ...resource.TestCheckFunc) {
	t.Helper()
	var tracked *terraform.State
	capture := func(s *terraform.State) error { tracked = s.DeepCopy(); return nil }
	check := resource.ComposeAggregateTestCheckFunc(append(checks, capture)...)
	resource.Test(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		TerraformVersionChecks: testAccTerraformVersionChecks(), CheckDestroy: safeExpandedCheckDestroy,
		Steps: []resource.TestStep{
			{Config: workspace + initial, Check: check},
			{Config: workspace + initial, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{ResourceName: address, ImportState: true, ImportStateIdFunc: importID, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"updated_at"}},
			{Config: workspace + updated, Check: capture},
			{Config: workspace + updated, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{Config: workspace, Check: func(_ *terraform.State) error { return safeExpandedChildrenAbsent(tracked) }},
		},
	})
}

func safeExpandedChildrenAbsent(s *terraform.State) error {
	if s == nil {
		return fmt.Errorf("missing child cleanup inventory")
	}
	c := testAccClientFromEnv()
	ctx := context.Background()
	for _, rs := range s.RootModule().Resources {
		if rs.Primary == nil {
			continue
		}
		a := rs.Primary.Attributes
		var err error
		switch rs.Type {
		case "openrouter_workspace_budget":
			budgets, listErr := c.ListWorkspaceBudgets(ctx, a["workspace_id"])
			if listErr != nil {
				if safeExpandedNotFound(listErr) {
					if _, workspaceErr := c.GetWorkspace(ctx, a["workspace_id"]); safeExpandedNotFound(workspaceErr) {
						continue
					}
				}
				return fmt.Errorf("unable to verify workspace budget cleanup")
			}
			for _, budget := range budgets {
				if budget.ResetInterval == a["interval"] {
					return fmt.Errorf("test workspace budget remains")
				}
			}
			continue
		case "openrouter_budget_policy", "openrouter_model_availability_policy":
			kind := strings.TrimSuffix(strings.TrimPrefix(rs.Type, "openrouter_"), "_policy")
			_, err = c.GetPolicy(ctx, kind, a["workspace_id"], rs.Primary.ID)
		case "openrouter_api_key":
			_, err = c.GetAPIKey(ctx, rs.Primary.ID)
		case "openrouter_guardrail":
			_, err = c.GetGuardrail(ctx, rs.Primary.ID)
		default:
			continue
		}
		if err == nil {
			return fmt.Errorf("created %s remains after removal", rs.Type)
		}
		if !safeExpandedNotFound(err) {
			return fmt.Errorf("unable to confirm %s cleanup: non-404 response", rs.Type)
		}
	}
	return nil
}

func TestAccSafeExpandedWorkspaceBudget(t *testing.T) {
	safeExpandedAccPreflight(t)
	config := func(limit int) string {
		return fmt.Sprintf(`
resource "openrouter_workspace_budget" "test" {
 workspace_id = openrouter_workspace.test.id
 interval = "monthly"
 limit_usd = %d
}
`, limit)
	}
	safeExpandedLifecycle(t, safeExpandedWorkspaceConfig(t), config(1), config(2), "openrouter_workspace_budget.test", safeExpandedImportID("openrouter_workspace_budget.test", "workspace_id", "interval"), resource.TestCheckResourceAttr("openrouter_workspace_budget.test", "limit_usd", "1"))
}

func TestAccSafeExpandedNativePolicies(t *testing.T) {
	safeExpandedAccPreflight(t)
	for _, kind := range []string{"budget", "model_availability"} {
		t.Run(kind, func(t *testing.T) {
			config := func(updated bool) string {
				body := `limit_usd = 1, reset_interval = "monthly", include_byok_in_budgets = false`
				if updated {
					body = `limit_usd = 2, reset_interval = "monthly", include_byok_in_budgets = true`
				}
				if kind == "model_availability" {
					body = `allowed_providers = ["openai"], enforce_zdr_openai = false`
					if updated {
						body = `allowed_providers = ["openai"], enforce_zdr_openai = true`
					}
				}
				return fmt.Sprintf(`
resource "openrouter_%s_policy" "test" {
 workspace_id = openrouter_workspace.test.id
 name = %q
 config = { %s }
}
`, kind, "isolated "+kind, body)
			}
			address := "openrouter_" + kind + "_policy.test"
			safeExpandedLifecycle(t, safeExpandedWorkspaceConfig(t), config(false), config(true), address, safeExpandedImportID(address, "workspace_id", "id"), resource.TestCheckResourceAttrSet(address, "id"))
		})
	}
}

const safeExpandedKeyConfig = `
resource "openrouter_api_key" "test" {
 name = "isolated disabled regression key"
 workspace_id = openrouter_workspace.test.id
 disabled = true
 limit = 0
}
`

func TestAccSafeExpandedAPIKeyPolicyAssignment(t *testing.T) {
	safeExpandedAccPreflight(t)
	workspace := safeExpandedWorkspaceConfig(t)
	deps := safeExpandedKeyConfig + `
resource "openrouter_budget_policy" "test" {
 workspace_id = openrouter_workspace.test.id
 name = "isolated assignment policy"
 config = { limit_usd = 1, reset_interval = "monthly", include_byok_in_budgets = false }
}
`
	assignment := `
resource "openrouter_api_key_policy_assignment" "test" {
 workspace_id = openrouter_workspace.test.id
 policy_type = "budget"
 policy_id = openrouter_budget_policy.test.id
 key_hash = openrouter_api_key.test.id
}
`
	var inventory *terraform.State
	capture := func(s *terraform.State) error { inventory = s.DeepCopy(); return nil }
	address := "openrouter_api_key_policy_assignment.test"
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, TerraformVersionChecks: testAccTerraformVersionChecks(), CheckDestroy: safeExpandedCheckDestroy, Steps: []resource.TestStep{
		{Config: workspace + deps + assignment, Check: resource.ComposeAggregateTestCheckFunc(capture, resource.TestCheckResourceAttr("openrouter_api_key.test", "disabled", "true"), resource.TestCheckResourceAttr("openrouter_api_key.test", "limit", "0"))},
		{Config: workspace + deps + assignment, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		{ResourceName: address, ImportState: true, ImportStateIdFunc: safeExpandedImportID(address, "workspace_id", "policy_type", "key_hash", "policy_id"), ImportStateVerify: true},
		{Config: workspace + deps, Check: func(s *terraform.State) error {
			attrs := inventory.RootModule().Resources[address].Primary.Attributes
			state, err := testAccClientFromEnv().GetPolicyAssignment(context.Background(), "api-keys", attrs["key_hash"], attrs["workspace_id"], "budget")
			if err != nil {
				return fmt.Errorf("unable to verify assignment removal")
			}
			if len(state.PolicyIDs) != 0 || state.OptedOut {
				return fmt.Errorf("test direct assignment remains or changed default opt-out")
			}
			return nil
		}},
		{Config: workspace, Check: func(_ *terraform.State) error { return safeExpandedChildrenAbsent(inventory) }},
	}})
}

func TestAccSafeExpandedReadOnlyDataSources(t *testing.T) {
	safeExpandedAccPreflight(t)
	date := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	workspace := safeExpandedWorkspaceConfig(t)
	data := fmt.Sprintf(`
data "openrouter_workspace_members" "test" { workspace_id = openrouter_workspace.test.id }
data "openrouter_byok_keys" "test" { workspace_id = openrouter_workspace.test.id }
data "openrouter_models" "test" { limit = 1 }
data "openrouter_model" "test" { slug = data.openrouter_models.test.items[0].id }
data "openrouter_activity" "test" {
 date = %q
 workspace_id = openrouter_workspace.test.id
 group_by_workspace = true
 max_items = 1000
}
`, date)
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, TerraformVersionChecks: testAccTerraformVersionChecks(), CheckDestroy: safeExpandedCheckDestroy, Steps: []resource.TestStep{
		{Config: workspace + data, PreventPostDestroyRefresh: true, Check: resource.ComposeAggregateTestCheckFunc(resource.TestCheckResourceAttrSet("data.openrouter_workspace_members.test", "total_count"), resource.TestCheckResourceAttr("data.openrouter_byok_keys.test", "total_count", "0"), resource.TestCheckResourceAttrSet("data.openrouter_model.test", "id"), resource.TestCheckResourceAttr("data.openrouter_models.test", "items.#", "1"), resource.TestCheckResourceAttr("data.openrouter_activity.test", "total_count", "0"))},
		{Config: workspace},
	}})
}

func TestAccSafeExpandedLegacyGuardrailAssignment(t *testing.T) {
	safeExpandedAccPreflight(t)
	workspace := safeExpandedWorkspaceConfig(t)
	deps := safeExpandedKeyConfig + `
resource "openrouter_guardrail" "first" {
 name = "isolated first guardrail"
 workspace_id = openrouter_workspace.test.id
 limit_usd = 1
 reset_interval = "monthly"
}
resource "openrouter_guardrail" "second" {
 name = "isolated second guardrail"
 workspace_id = openrouter_workspace.test.id
 limit_usd = 1
 reset_interval = "monthly"
}
`
	assignment := func(target string) string {
		return fmt.Sprintf(`
resource "openrouter_guardrail_key_assignment" "test" {
 guardrail_id = openrouter_guardrail.%s.id
 key_hash = openrouter_api_key.test.id
}
`, target)
	}
	var inventory *terraform.State
	capture := func(s *terraform.State) error { inventory = s.DeepCopy(); return nil }
	address := "openrouter_guardrail_key_assignment.test"
	resource.Test(t, resource.TestCase{PreCheck: func() { testAccPreCheck(t) }, ProtoV6ProviderFactories: testAccProtoV6ProviderFactories, TerraformVersionChecks: testAccTerraformVersionChecks(), CheckDestroy: safeExpandedCheckDestroy, Steps: []resource.TestStep{
		{Config: workspace + deps + assignment("first"), Check: resource.ComposeAggregateTestCheckFunc(capture, resource.TestCheckResourceAttr("openrouter_api_key.test", "disabled", "true"), resource.TestCheckResourceAttr("openrouter_api_key.test", "limit", "0"))},
		{ResourceName: address, ImportState: true, ImportStateIdFunc: safeExpandedImportID(address, "guardrail_id", "key_hash"), ImportStateVerify: true},
		{Config: workspace + deps + assignment("second"), Check: capture},
		{Config: workspace + deps + assignment("second"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		{Config: workspace + deps, Check: func(_ *terraform.State) error {
			keyID := inventory.RootModule().Resources["openrouter_api_key.test"].Primary.ID
			for _, name := range []string{"first", "second"} {
				id := inventory.RootModule().Resources["openrouter_guardrail."+name].Primary.ID
				keys, err := testAccClientFromEnv().ListGuardrailKeyAssignments(context.Background(), id)
				if err != nil {
					return fmt.Errorf("unable to verify test guardrail assignment removal")
				}
				for _, key := range keys {
					if key == keyID {
						return fmt.Errorf("test guardrail assignment remains")
					}
				}
			}
			return nil
		}},
		{Config: workspace, Check: func(_ *terraform.State) error { return safeExpandedChildrenAbsent(inventory) }},
	}})
}

// Also check every independently addressable child if a step fails before the
// deliberate child-removal phase. The framework still owns destroy ordering.
func safeExpandedCheckDestroy(s *terraform.State) error {
	return resource.ComposeAggregateTestCheckFunc(safeExpandedChildrenAbsent, safeExpandedWorkspaceAbsent)(s)
}

// Match the client status prefix, never arbitrary upstream message contents.
func safeExpandedNotFound(err error) bool {
	return err != nil && strings.HasPrefix(err.Error(), "openrouter API error (404):")
}

func safeExpandedWorkspaceAbsent(s *terraform.State) error {
	if s == nil || s.RootModule() == nil {
		return fmt.Errorf("missing workspace cleanup inventory")
	}
	c := testAccClientFromEnv()
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "openrouter_workspace" || rs.Primary == nil {
			continue
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("missing created workspace identity")
		}
		_, err := c.GetWorkspace(context.Background(), rs.Primary.ID)
		if err == nil {
			return fmt.Errorf("created test workspace remains")
		}
		if !safeExpandedNotFound(err) {
			return fmt.Errorf("unable to verify test workspace cleanup: non-404 response")
		}
	}
	return nil
}

func TestSafeExpandedNotFound(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil"},
		{name: "404", err: errors.New("openrouter API error (404): missing"), want: true},
		{name: "500 containing 404", err: errors.New("openrouter API error (500): upstream (404) missing")},
		{name: "wrapped text", err: errors.New("proxy: openrouter API error (404): missing")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := safeExpandedNotFound(tc.err); got != tc.want {
				t.Errorf("not found = %t, want %t", got, tc.want)
			}
		})
	}
}
