package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDataSources_basic(t *testing.T) {
	testAccSkipUnlessEnabled(t)

	workspaceName := testAccRandomName("acct-ds-ws")
	workspaceSlug := testAccRandomName("acct-ds-ws")
	keyName := testAccRandomName("acct-ds-ak")
	guardrailName := testAccRandomName("acct-ds-gr")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		TerraformVersionChecks:   testAccTerraformVersionChecks(),
		CheckDestroy:             resource.ComposeAggregateTestCheckFunc(testAccCheckAPIKeyDestroy, testAccCheckGuardrailDestroy, testAccCheckWorkspaceDestroy),
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourcesResourceConfig(workspaceName, workspaceSlug, keyName, guardrailName),
			},
			{
				Config:                    testAccDataSourcesConfig(workspaceName, workspaceSlug, keyName, guardrailName),
				PreventPostDestroyRefresh: true,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.openrouter_workspace.test", "name", workspaceName),
					resource.TestCheckResourceAttr("data.openrouter_workspace.test", "slug", workspaceSlug),
					resource.TestCheckResourceAttrSet("data.openrouter_api_keys.test", "total_count"),
					resource.TestCheckResourceAttrSet("data.openrouter_guardrails.test", "total_count"),
					resource.TestCheckResourceAttrSet("data.openrouter_workspaces.all", "total_count"),
					resource.TestCheckResourceAttrSet("data.openrouter_organization.current", "total_count"),
					resource.TestCheckResourceAttrSet("data.openrouter_providers.all", "total_count"),
				),
			},
		},
	})
}

func testAccDataSourcesResourceConfig(workspaceName, workspaceSlug, keyName, guardrailName string) string {
	return fmt.Sprintf(`
%s

resource "openrouter_workspace" "test" {
  name = %q
  slug = %q
}

resource "openrouter_api_key" "test" {
  name         = %q
  workspace_id = openrouter_workspace.test.id
  limit        = 5
  limit_reset  = "monthly"
  disabled     = false
}

resource "openrouter_guardrail" "test" {
  name           = %q
  workspace_id   = openrouter_workspace.test.id
  limit_usd      = 15
  reset_interval = "monthly"
}
`, testAccProviderConfig, workspaceName, workspaceSlug, keyName, guardrailName)
}

func testAccDataSourcesConfig(workspaceName, workspaceSlug, keyName, guardrailName string) string {
	return fmt.Sprintf(`
%s

resource "openrouter_workspace" "test" {
  name = %q
  slug = %q
}

resource "openrouter_api_key" "test" {
  name         = %q
  workspace_id = openrouter_workspace.test.id
  limit        = 5
  limit_reset  = "monthly"
  disabled     = false
}

resource "openrouter_guardrail" "test" {
  name           = %q
  workspace_id   = openrouter_workspace.test.id
  limit_usd      = 15
  reset_interval = "monthly"
}

data "openrouter_workspace" "test" {
  slug = openrouter_workspace.test.slug
}

data "openrouter_workspaces" "all" {}

data "openrouter_api_keys" "test" {
  workspace_id = openrouter_workspace.test.id
}

data "openrouter_guardrails" "test" {
  workspace_id = openrouter_workspace.test.id
}

data "openrouter_organization" "current" {}

data "openrouter_providers" "all" {}
`, testAccProviderConfig, workspaceName, workspaceSlug, keyName, guardrailName)
}
