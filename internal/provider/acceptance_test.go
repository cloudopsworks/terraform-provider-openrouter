package provider

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"

	"github.com/cloudopsworks/terraform-provider-openrouter/internal/client"
)

const testAccProviderConfig = `provider "openrouter" {}`

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"openrouter": providerserver.NewProtocol6WithError(New("test")()),
}

func testAccSkipUnlessEnabled(t *testing.T) {
	t.Helper()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
}

func testAccPreCheck(t *testing.T) {
	t.Helper()
	if strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY")) == "" {
		t.Fatal("OPENROUTER_API_KEY must be set for acceptance tests")
	}
}

func testAccTerraformVersionChecks() []tfversion.TerraformVersionCheck {
	return []tfversion.TerraformVersionCheck{
		tfversion.SkipBelow(tfversion.Version1_7_0),
	}
}

func testAccClient(t *testing.T) *client.Client {
	t.Helper()
	return client.New(
		strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY")),
		client.DefaultBaseURL,
		"terraform-provider-openrouter/testacc",
		30*time.Second,
	)
}

func testAccRandomName(prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, strings.ToLower(acctest.RandStringFromCharSet(10, acctest.CharSetAlphaNum)))
}

func testAccResourceState(t *testing.T, s *terraform.State, resourceName string) *terraform.ResourceState {
	t.Helper()
	if s == nil || s.RootModule() == nil {
		t.Fatalf("terraform state is nil")
	}
	resourceState, ok := s.RootModule().Resources[resourceName]
	if !ok || resourceState == nil || resourceState.Primary == nil {
		t.Fatalf("resource %s not found in state", resourceName)
	}
	return resourceState
}

func testAccCompositeImportID(resourceName string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		resourceState, ok := s.RootModule().Resources[resourceName]
		if !ok || resourceState == nil || resourceState.Primary == nil {
			return "", fmt.Errorf("resource %s not found in state", resourceName)
		}
		workspaceID := resourceState.Primary.Attributes["workspace_id"]
		name := resourceState.Primary.Attributes["name"]
		if workspaceID == "" || name == "" {
			return "", fmt.Errorf("resource %s missing workspace_id or name in state", resourceName)
		}
		return workspaceID + "_" + name, nil
	}
}

func testAccWorkspaceImportID(resourceName string) resource.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		resourceState, ok := s.RootModule().Resources[resourceName]
		if !ok || resourceState == nil || resourceState.Primary == nil {
			return "", fmt.Errorf("resource %s not found in state", resourceName)
		}
		workspaceID := resourceState.Primary.ID
		name := resourceState.Primary.Attributes["name"]
		if workspaceID == "" || name == "" {
			return "", fmt.Errorf("resource %s missing id or name in state", resourceName)
		}
		return workspaceID + "_" + name, nil
	}
}

func testAccCheckWorkspaceExists(resourceName string, workspace *client.Workspace) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		resourceState, ok := s.RootModule().Resources[resourceName]
		if !ok || resourceState == nil || resourceState.Primary == nil {
			return fmt.Errorf("resource %s not found in state", resourceName)
		}
		found, err := testAccClientFromEnv().GetWorkspace(context.Background(), resourceState.Primary.ID)
		if err != nil {
			return err
		}
		*workspace = *found
		return nil
	}
}

func testAccCheckAPIKeyExists(resourceName string, apiKey *client.APIKey) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		resourceState, ok := s.RootModule().Resources[resourceName]
		if !ok || resourceState == nil || resourceState.Primary == nil {
			return fmt.Errorf("resource %s not found in state", resourceName)
		}
		found, err := testAccClientFromEnv().GetAPIKey(context.Background(), resourceState.Primary.ID)
		if err != nil {
			return err
		}
		*apiKey = *found
		return nil
	}
}

func testAccCheckGuardrailExists(resourceName string, guardrail *client.Guardrail) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		resourceState, ok := s.RootModule().Resources[resourceName]
		if !ok || resourceState == nil || resourceState.Primary == nil {
			return fmt.Errorf("resource %s not found in state", resourceName)
		}
		found, err := testAccClientFromEnv().GetGuardrail(context.Background(), resourceState.Primary.ID)
		if err != nil {
			return err
		}
		*guardrail = *found
		return nil
	}
}

func testAccCheckWorkspaceDestroy(s *terraform.State) error {
	c := testAccClientFromEnv()
	for name, resourceState := range s.RootModule().Resources {
		if resourceState.Type != "openrouter_workspace" || resourceState.Primary == nil {
			continue
		}
		_, err := c.GetWorkspace(context.Background(), resourceState.Primary.ID)
		if err == nil {
			return fmt.Errorf("workspace %s (%s) still exists", name, resourceState.Primary.ID)
		}
		if !strings.Contains(err.Error(), "(404)") {
			return err
		}
	}
	return nil
}

func testAccCheckAPIKeyDestroy(s *terraform.State) error {
	c := testAccClientFromEnv()
	for name, resourceState := range s.RootModule().Resources {
		if resourceState.Type != "openrouter_api_key" || resourceState.Primary == nil {
			continue
		}
		_, err := c.GetAPIKey(context.Background(), resourceState.Primary.ID)
		if err == nil {
			return fmt.Errorf("api key %s (%s) still exists", name, resourceState.Primary.ID)
		}
		if !strings.Contains(err.Error(), "(404)") {
			return err
		}
	}
	return nil
}

func testAccCheckGuardrailDestroy(s *terraform.State) error {
	c := testAccClientFromEnv()
	for name, resourceState := range s.RootModule().Resources {
		if resourceState.Type != "openrouter_guardrail" || resourceState.Primary == nil {
			continue
		}
		_, err := c.GetGuardrail(context.Background(), resourceState.Primary.ID)
		if err == nil {
			return fmt.Errorf("guardrail %s (%s) still exists", name, resourceState.Primary.ID)
		}
		if !strings.Contains(err.Error(), "(404)") {
			return err
		}
	}
	return nil
}

func testAccClientFromEnv() *client.Client {
	return client.New(
		strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY")),
		client.DefaultBaseURL,
		"terraform-provider-openrouter/testacc",
		30*time.Second,
	)
}
