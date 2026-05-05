package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"

	"github.com/cloudopsworks/terraform-provider-openrouter/internal/client"
)

func TestAccAPIKeyResource_basic(t *testing.T) {
	testAccSkipUnlessEnabled(t)

	var apiKey client.APIKey
	workspaceName := testAccRandomName("acct-ak-ws")
	workspaceSlug := testAccRandomName("acct-ak-ws")
	keyName := testAccRandomName("acct-ak")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		TerraformVersionChecks:   testAccTerraformVersionChecks(),
		CheckDestroy:             testAccCheckAPIKeyDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAPIKeyResourceConfig(workspaceName, workspaceSlug, keyName, 10, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAPIKeyExists("openrouter_api_key.test", &apiKey),
					resource.TestCheckResourceAttr("openrouter_api_key.test", "name", keyName),
					resource.TestCheckResourceAttr("openrouter_api_key.test", "limit_reset", "monthly"),
					resource.TestCheckResourceAttr("openrouter_api_key.test", "disabled", "false"),
					resource.TestCheckResourceAttrSet("openrouter_api_key.test", "workspace_id"),
					resource.TestCheckResourceAttrSet("openrouter_api_key.test", "key"),
				),
			},
			{
				PreConfig: func() {
					mutatedLimit := 25.0
					mutatedDisabled := true
					mutatedReset := "monthly"
					_, err := testAccClient(t).UpdateAPIKey(context.Background(), apiKey.Hash, client.APIKeyUpdateRequest{
						Limit:      &mutatedLimit,
						Disabled:   &mutatedDisabled,
						LimitReset: &mutatedReset,
					})
					if err != nil {
						t.Fatalf("mutating api key drift: %v", err)
					}
				},
				Config: testAccAPIKeyResourceConfig(workspaceName, workspaceSlug, keyName, 10, false),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectNonEmptyPlan()},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAPIKeyExists("openrouter_api_key.test", &apiKey),
					resource.TestCheckResourceAttr("openrouter_api_key.test", "disabled", "false"),
					resource.TestCheckResourceAttr("openrouter_api_key.test", "limit", "10"),
				),
			},
			{
				Config: testAccAPIKeyResourceConfig(workspaceName, workspaceSlug, keyName, 15, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAPIKeyExists("openrouter_api_key.test", &apiKey),
					resource.TestCheckResourceAttr("openrouter_api_key.test", "disabled", "true"),
					resource.TestCheckResourceAttr("openrouter_api_key.test", "limit", "15"),
				),
			},
			{
				Config: testAccAPIKeyResourceConfig(workspaceName, workspaceSlug, keyName, 15, true),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				ResourceName:            "openrouter_api_key.test",
				ImportState:             true,
				ImportStateIdFunc:       testAccCompositeImportID("openrouter_api_key.test"),
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"key", "updated_at"},
			},
		},
	})
}

func testAccAPIKeyResourceConfig(workspaceName, workspaceSlug, keyName string, limit int, disabled bool) string {
	return fmt.Sprintf(`
%s

resource "openrouter_workspace" "test" {
  name = %q
  slug = %q
}

resource "openrouter_api_key" "test" {
  name                  = %q
  workspace_id          = openrouter_workspace.test.id
  limit                 = %d
  limit_reset           = "monthly"
  include_byok_in_limit = true
  disabled              = %t
}
`, testAccProviderConfig, workspaceName, workspaceSlug, keyName, limit, disabled)
}
