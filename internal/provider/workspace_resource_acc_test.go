package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"

	"github.com/cloudopsworks/terraform-provider-openrouter/internal/client"
)

func TestAccWorkspaceResource_basic(t *testing.T) {
	testAccSkipUnlessEnabled(t)

	var workspace client.Workspace
	name := testAccRandomName("acct-ws")
	slugA := testAccRandomName("acct-ws")
	slugB := testAccRandomName("acct-ws")
	descriptionA := "acceptance workspace A"
	descriptionB := "acceptance workspace B"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		TerraformVersionChecks:   testAccTerraformVersionChecks(),
		CheckDestroy:             testAccCheckWorkspaceDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccWorkspaceResourceConfig(name, slugA, descriptionA),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckWorkspaceExists("openrouter_workspace.test", &workspace),
					resource.TestCheckResourceAttr("openrouter_workspace.test", "name", name),
					resource.TestCheckResourceAttr("openrouter_workspace.test", "slug", slugA),
					resource.TestCheckResourceAttr("openrouter_workspace.test", "description", descriptionA),
				),
			},
			{
				PreConfig: func() {
					mutatedDescription := "drifted description"
					_, err := testAccClient(t).UpdateWorkspace(context.Background(), workspace.ID, client.WorkspaceUpsertRequest{
						Name:        &workspace.Name,
						Slug:        &workspace.Slug,
						Description: &mutatedDescription,
					})
					if err != nil {
						t.Fatalf("mutating workspace drift: %v", err)
					}
				},
				Config: testAccWorkspaceResourceConfig(name, slugA, descriptionA),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectNonEmptyPlan()},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckWorkspaceExists("openrouter_workspace.test", &workspace),
					resource.TestCheckResourceAttr("openrouter_workspace.test", "description", descriptionA),
				),
			},
			{
				Config: testAccWorkspaceResourceConfig(name, slugB, descriptionB),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckWorkspaceExists("openrouter_workspace.test", &workspace),
					resource.TestCheckResourceAttr("openrouter_workspace.test", "slug", slugB),
					resource.TestCheckResourceAttr("openrouter_workspace.test", "description", descriptionB),
				),
			},
			{
				Config: testAccWorkspaceResourceConfig(name, slugB, descriptionB),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				ResourceName:            "openrouter_workspace.test",
				ImportState:             true,
				ImportStateIdFunc:       testAccWorkspaceImportID("openrouter_workspace.test"),
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"updated_at"},
			},
		},
	})
}

func testAccWorkspaceResourceConfig(name, slug, description string) string {
	return fmt.Sprintf(`
%s

resource "openrouter_workspace" "test" {
  name                            = %q
  slug                            = %q
  description                     = %q
  default_text_model              = "openai/gpt-4o-mini"
  default_provider_sort           = "price"
  io_logging_sampling_rate        = 1
  is_data_discount_logging_enabled = true
}
`, testAccProviderConfig, name, slug, description)
}
