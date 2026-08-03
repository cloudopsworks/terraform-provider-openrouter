package secrets

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
)

type azureSecretClient interface {
	GetSecret(ctx context.Context, name string, version string, options *azsecrets.GetSecretOptions) (azsecrets.GetSecretResponse, error)
}

type AzureResolver struct {
	cfg       AzureConfig
	newClient func(context.Context, AzureConfig) (azureSecretClient, error)
}

func NewAzureResolver(cfg AzureConfig) *AzureResolver {
	return &AzureResolver{cfg: cfg, newClient: newAzureSecretClient}
}

func NewAzureResolverWithClient(cfg AzureConfig, cl azureSecretClient) *AzureResolver {
	return &AzureResolver{cfg: cfg, newClient: func(context.Context, AzureConfig) (azureSecretClient, error) { return cl, nil }}
}

func (r *AzureResolver) Resolve(ctx context.Context) (Settings, error) {
	if r.cfg.VaultURL == "" {
		return Settings{}, fmt.Errorf("azure_key_vault.vault_url is required")
	}
	if r.cfg.SecretName == "" {
		return Settings{}, fmt.Errorf("azure_key_vault.secret_name is required")
	}
	cl, err := r.newClient(ctx, r.cfg)
	if err != nil {
		return Settings{}, fmt.Errorf("create Azure Key Vault client: %w", err)
	}
	out, err := cl.GetSecret(ctx, r.cfg.SecretName, r.cfg.Version, nil)
	if err != nil {
		return Settings{}, fmt.Errorf("read Azure Key Vault secret: %w", err)
	}
	if out.Value == nil {
		return Settings{}, fmt.Errorf("Azure Key Vault secret value is empty")
	}
	return extractSecretSettings(*out.Value, r.cfg.JSONKey)
}

func newAzureSecretClient(_ context.Context, cfg AzureConfig) (azureSecretClient, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, err
	}
	return azsecrets.NewClient(cfg.VaultURL, cred, nil)
}
