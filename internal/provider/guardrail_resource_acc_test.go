package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"

	"github.com/cloudopsworks/terraform-provider-openrouter/internal/client"
)

func TestAccGuardrailResource_basic(t *testing.T) {
	testAccSkipUnlessEnabled(t)

	var guardrail client.Guardrail
	workspaceName := testAccRandomName("acct-gr-ws")
	workspaceSlug := testAccRandomName("acct-gr-ws")
	guardrailName := testAccRandomName("acct-gr")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		TerraformVersionChecks:   testAccTerraformVersionChecks(),
		CheckDestroy:             testAccCheckGuardrailDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccGuardrailResourceConfig(workspaceName, workspaceSlug, guardrailName, 25, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckGuardrailExists("openrouter_guardrail.test", &guardrail),
					resource.TestCheckResourceAttr("openrouter_guardrail.test", "name", guardrailName),
					resource.TestCheckResourceAttr("openrouter_guardrail.test", "reset_interval", "monthly"),
					resource.TestCheckResourceAttr("openrouter_guardrail.test", "limit_usd", "25"),
					resource.TestCheckResourceAttrSet("openrouter_guardrail.test", "workspace_id"),
				),
			},
			{
				PreConfig: func() {
					mutatedLimit := 33.0
					mutatedReset := "monthly"
					ignoredProviders := []string{"mistral"}
					_, err := testAccClient(t).UpdateGuardrail(context.Background(), guardrail.ID, client.GuardrailUpsertRequest{
						Name:             &guardrail.Name,
						LimitUSD:         &mutatedLimit,
						ResetInterval:    &mutatedReset,
						IgnoredProviders: &ignoredProviders,
					})
					if err != nil {
						t.Fatalf("mutating guardrail drift: %v", err)
					}
				},
				Config: testAccGuardrailResourceConfig(workspaceName, workspaceSlug, guardrailName, 25, false),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectNonEmptyPlan()},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckGuardrailExists("openrouter_guardrail.test", &guardrail),
					resource.TestCheckResourceAttr("openrouter_guardrail.test", "limit_usd", "25"),
				),
			},
			{
				Config: testAccGuardrailResourceConfig(workspaceName, workspaceSlug, guardrailName, 30, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckGuardrailExists("openrouter_guardrail.test", &guardrail),
					resource.TestCheckResourceAttr("openrouter_guardrail.test", "limit_usd", "30"),
					resource.TestCheckResourceAttr("openrouter_guardrail.test", "enforce_zdr", "true"),
				),
			},
			{
				Config: testAccGuardrailResourceConfig(workspaceName, workspaceSlug, guardrailName, 30, true),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				ResourceName:      "openrouter_guardrail.test",
				ImportState:       true,
				ImportStateIdFunc: testAccCompositeImportID("openrouter_guardrail.test"),
				ImportStateVerify: true,
			},
		},
	})
}

func testAccGuardrailResourceConfig(workspaceName, workspaceSlug, guardrailName string, limit int, enforceZDR bool) string {
	return fmt.Sprintf(`
%s

resource "openrouter_workspace" "test" {
  name = %q
  slug = %q
}

resource "openrouter_guardrail" "test" {
  name              = %q
  workspace_id      = openrouter_workspace.test.id
  description       = "acceptance guardrail"
  limit_usd         = %d
  reset_interval    = "monthly"
  allowed_providers = ["openai"]
  enforce_zdr       = %t
}
`, testAccProviderConfig, workspaceName, workspaceSlug, guardrailName, limit, enforceZDR)
}
