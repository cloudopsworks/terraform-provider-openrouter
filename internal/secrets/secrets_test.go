package secrets

import (
	"context"
	"errors"
	"testing"
	"time"

	secretmanagerpb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	ststypes "github.com/aws/aws-sdk-go-v2/service/sts/types"
)

func TestNewResolverExactlyOneSource(t *testing.T) {
	ctx := context.Background()
	resolver, source, err := NewResolver(SourceConfig{Direct: Settings{AdminAPIKey: " sk-admin ", BaseURL: " https://api.example/v1 ", OrganizationID: " org_1 ", ProjectID: " proj_1 "}})
	if err != nil {
		t.Fatalf("NewResolver direct: %v", err)
	}
	if source != "configuration" {
		t.Fatalf("source = %q", source)
	}
	settings, err := resolver.Resolve(ctx)
	if err != nil || settings.AdminAPIKey != "sk-admin" || settings.BaseURL != "https://api.example/v1" || settings.OrganizationID != "org_1" || settings.ProjectID != "proj_1" {
		t.Fatalf("Resolve() = %#v, %v", settings, err)
	}

	if _, _, err := NewResolver(SourceConfig{}); err == nil {
		t.Fatal("expected missing source error")
	}
	resolver, source, err = NewResolver(SourceConfig{Direct: Settings{AdminAPIKey: "sk-direct"}, AWS: &AWSConfig{SecretID: "s"}})
	if err != nil {
		t.Fatalf("direct should take precedence over secret source: %v", err)
	}
	settings, err = resolver.Resolve(ctx)
	if err != nil || source != "configuration" || settings.AdminAPIKey != "sk-direct" {
		t.Fatalf("direct precedence Resolve() = %#v source=%q err=%v", settings, source, err)
	}
	if _, _, err := NewResolver(SourceConfig{AWS: &AWSConfig{SecretID: "s"}, GCP: &GCPConfig{ProjectID: "p", SecretID: "s"}}); err == nil {
		t.Fatal("expected multiple cloud source error")
	}
}

func TestExtractSecretValue(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		jsonKey string
		want    string
		wantErr bool
	}{
		{name: "raw", raw: " sk-raw ", want: "sk-raw"},
		{name: "json top", raw: `{"admin_api_key":"sk-json"}`, jsonKey: "admin_api_key", want: "sk-json"},
		{name: "json nested", raw: `{"openrouter":{"admin":"sk-nested"}}`, jsonKey: "openrouter.admin", want: "sk-nested"},
		{name: "missing", raw: `{}`, jsonKey: "admin_api_key", wantErr: true},
		{name: "not string", raw: `{"admin_api_key":123}`, jsonKey: "admin_api_key", wantErr: true},
		{name: "empty", raw: " ", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractSecretValue(tt.raw, tt.jsonKey)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("extractSecretValue() = %q, %v; want %q", got, err, tt.want)
			}
		})
	}
}

func TestExtractSecretSettings(t *testing.T) {
	settings, err := extractSecretSettings(`{"openrouter_api_key":" sk-json ","base_url":" https://api.example/v1/ "}`, "")
	if err != nil {
		t.Fatalf("extractSecretSettings() error = %v", err)
	}
	if settings.AdminAPIKey != "sk-json" || settings.BaseURL != "https://api.example/v1/" {
		t.Fatalf("settings = %#v", settings)
	}
	settings, err = extractSecretSettings(" sk-plain ", "")
	if err != nil || settings.AdminAPIKey != "sk-plain" {
		t.Fatalf("plaintext settings = %#v, %v", settings, err)
	}
	if _, err := extractSecretSettings(`{"base_url":"https://api.example/v1"}`, ""); err == nil {
		t.Fatal("expected missing admin key error")
	}
	if _, err := extractSecretSettings(`{"api_key":`, ""); err == nil {
		t.Fatal("expected malformed JSON error")
	}
}

type fakeAWSClient struct {
	out *secretsmanager.GetSecretValueOutput
	err error
	in  *secretsmanager.GetSecretValueInput
}

func (f *fakeAWSClient) GetSecretValue(ctx context.Context, in *secretsmanager.GetSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	f.in = in
	return f.out, f.err
}

func TestAWSResolverWithMockedClient(t *testing.T) {
	secret := `{"key":"sk-aws"}`
	version := "v1"
	client := &fakeAWSClient{out: &secretsmanager.GetSecretValueOutput{SecretString: &secret}}
	resolver := NewAWSResolverWithClient(AWSConfig{SecretID: "secret/openrouter", VersionID: version, JSONKey: "key"}, client)
	got, err := resolver.Resolve(context.Background())
	if err != nil || got.AdminAPIKey != "sk-aws" {
		t.Fatalf("Resolve() = %#v, %v", got, err)
	}
	if client.in == nil || *client.in.SecretId != "secret/openrouter" || *client.in.VersionId != version {
		t.Fatalf("unexpected AWS input: %#v", client.in)
	}
}

func TestAWSResolverErrors(t *testing.T) {
	if _, err := NewAWSResolverWithClient(AWSConfig{}, &fakeAWSClient{}).Resolve(context.Background()); err == nil {
		t.Fatal("expected missing secret_id error")
	}
	boom := errors.New("boom")
	if _, err := NewAWSResolverWithClient(AWSConfig{SecretID: "s"}, &fakeAWSClient{err: boom}).Resolve(context.Background()); err == nil {
		t.Fatal("expected SDK error")
	}
}

func TestAWSSecretsManagerClientUsesDefaultConfigWithoutRole(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test-access-key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret-key")
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")

	client, err := newAWSSecretsManagerClient(context.Background(), AWSConfig{SecretID: "s"})
	if err != nil {
		t.Fatalf("newAWSSecretsManagerClient() error = %v", err)
	}
	if client == nil {
		t.Fatal("newAWSSecretsManagerClient() returned nil client")
	}
}

type fakeSTSClient struct {
	in *sts.AssumeRoleInput
}

func (f *fakeSTSClient) AssumeRole(ctx context.Context, in *sts.AssumeRoleInput, _ ...func(*sts.Options)) (*sts.AssumeRoleOutput, error) {
	f.in = in
	return &sts.AssumeRoleOutput{
		Credentials: &ststypes.Credentials{
			AccessKeyId:     aws.String("assumed-key"),
			SecretAccessKey: aws.String("assumed-secret"),
			SessionToken:    aws.String("assumed-token"),
			Expiration:      aws.Time(time.Now().Add(time.Hour)),
		},
	}, nil
}

func TestLoadAWSConfigAssumeRoleUsesDefaultConfigAsSource(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "base-key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "base-secret")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")

	originalNewSTSClient := newSTSAssumeRoleClient
	defer func() { newSTSAssumeRoleClient = originalNewSTSClient }()

	var captured aws.Config
	stsClient := &fakeSTSClient{}
	newSTSAssumeRoleClient = func(cfg aws.Config) stscreds.AssumeRoleAPIClient {
		captured = cfg
		return stsClient
	}

	got, err := loadAWSConfig(context.Background(), AWSConfig{
		Region:          "us-east-1",
		RoleARN:         "arn:aws:iam::123456789012:role/openrouter-secrets",
		RoleSessionName: "terraform-provider-openrouter",
		ExternalID:      "external-1",
		DurationSeconds: 900,
	})
	if err != nil {
		t.Fatalf("loadAWSConfig() error = %v", err)
	}
	if captured.Region != "us-east-1" {
		t.Fatalf("captured base region = %q", captured.Region)
	}
	baseCreds, err := captured.Credentials.Retrieve(context.Background())
	if err != nil {
		t.Fatalf("captured base credentials retrieve: %v", err)
	}
	if baseCreds.AccessKeyID != "base-key" {
		t.Fatalf("base credential access key = %q", baseCreds.AccessKeyID)
	}
	if _, ok := got.Credentials.(*aws.CredentialsCache); !ok {
		t.Fatalf("assume-role config credentials type = %T, want *aws.CredentialsCache", got.Credentials)
	}
	assumedCreds, err := got.Credentials.Retrieve(context.Background())
	if err != nil {
		t.Fatalf("assume-role credentials retrieve: %v", err)
	}
	if assumedCreds.AccessKeyID != "assumed-key" {
		t.Fatalf("assume-role access key = %q", assumedCreds.AccessKeyID)
	}
	if stsClient.in == nil {
		t.Fatal("expected STS AssumeRole call")
	}
	if got := aws.ToString(stsClient.in.RoleArn); got != "arn:aws:iam::123456789012:role/openrouter-secrets" {
		t.Fatalf("AssumeRole RoleArn = %q", got)
	}
	if got := aws.ToString(stsClient.in.RoleSessionName); got != "terraform-provider-openrouter" {
		t.Fatalf("AssumeRole RoleSessionName = %q", got)
	}
	if got := aws.ToString(stsClient.in.ExternalId); got != "external-1" {
		t.Fatalf("AssumeRole ExternalId = %q", got)
	}
	if stsClient.in.DurationSeconds == nil || *stsClient.in.DurationSeconds != 900 {
		t.Fatalf("AssumeRole DurationSeconds = %v, want 900", stsClient.in.DurationSeconds)
	}
}

func TestLoadAWSConfigWithoutRoleARNDoesNotCreateSTSClient(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "base-key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "base-secret")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")

	originalNewSTSClient := newSTSAssumeRoleClient
	defer func() { newSTSAssumeRoleClient = originalNewSTSClient }()

	called := false
	newSTSAssumeRoleClient = func(cfg aws.Config) stscreds.AssumeRoleAPIClient {
		called = true
		return &fakeSTSClient{}
	}

	if _, err := loadAWSConfig(context.Background(), AWSConfig{Region: "us-east-1"}); err != nil {
		t.Fatalf("loadAWSConfig() error = %v", err)
	}
	if called {
		t.Fatal("STS client was created without role_arn")
	}
}

type fakeGCPClient struct {
	out    *secretmanagerpb.AccessSecretVersionResponse
	err    error
	closed bool
	name   string
}

func (f *fakeGCPClient) AccessSecretVersion(ctx context.Context, req *secretmanagerpb.AccessSecretVersionRequest, _ ...any) (*secretmanagerpb.AccessSecretVersionResponse, error) {
	f.name = req.Name
	return f.out, f.err
}
func (f *fakeGCPClient) Close() error { f.closed = true; return nil }

func TestGCPResolverWithMockedClient(t *testing.T) {
	client := &fakeGCPClient{out: &secretmanagerpb.AccessSecretVersionResponse{Payload: &secretmanagerpb.SecretPayload{Data: []byte(`{"key":"sk-gcp"}`)}}}
	resolver := NewGCPResolverWithClient(GCPConfig{ProjectID: "p", SecretID: "openrouter", JSONKey: "key"}, client)
	got, err := resolver.Resolve(context.Background())
	if err != nil || got.AdminAPIKey != "sk-gcp" {
		t.Fatalf("Resolve() = %#v, %v", got, err)
	}
	if client.name != "projects/p/secrets/openrouter/versions/latest" || !client.closed {
		t.Fatalf("unexpected GCP request: name=%q closed=%v", client.name, client.closed)
	}
}

func TestGCPResolverErrors(t *testing.T) {
	if _, err := NewGCPResolverWithClient(GCPConfig{SecretID: "s"}, &fakeGCPClient{}).Resolve(context.Background()); err == nil {
		t.Fatal("expected missing project_id")
	}
	if _, err := NewGCPResolverWithClient(GCPConfig{ProjectID: "p"}, &fakeGCPClient{}).Resolve(context.Background()); err == nil {
		t.Fatal("expected missing secret_id")
	}
}

type fakeAzureClient struct {
	out     azsecrets.GetSecretResponse
	err     error
	name    string
	version string
}

func (f *fakeAzureClient) GetSecret(ctx context.Context, name string, version string, options *azsecrets.GetSecretOptions) (azsecrets.GetSecretResponse, error) {
	f.name = name
	f.version = version
	return f.out, f.err
}

func TestAzureResolverWithMockedClient(t *testing.T) {
	value := `{"key":"sk-azure"}`
	client := &fakeAzureClient{out: azsecrets.GetSecretResponse{Secret: azsecrets.Secret{Value: &value}}}
	resolver := NewAzureResolverWithClient(AzureConfig{VaultURL: "https://vault.example", SecretName: "openrouter", Version: "42", JSONKey: "key"}, client)
	got, err := resolver.Resolve(context.Background())
	if err != nil || got.AdminAPIKey != "sk-azure" {
		t.Fatalf("Resolve() = %#v, %v", got, err)
	}
	if client.name != "openrouter" || client.version != "42" {
		t.Fatalf("unexpected Azure request: %q %q", client.name, client.version)
	}
}

func TestAzureResolverErrors(t *testing.T) {
	if _, err := NewAzureResolverWithClient(AzureConfig{SecretName: "s"}, &fakeAzureClient{}).Resolve(context.Background()); err == nil {
		t.Fatal("expected missing vault_url")
	}
	if _, err := NewAzureResolverWithClient(AzureConfig{VaultURL: "https://vault.example"}, &fakeAzureClient{}).Resolve(context.Background()); err == nil {
		t.Fatal("expected missing secret_name")
	}
}

func TestResolverConstructorsAndBinarySecret(t *testing.T) {
	if NewAWSResolver(AWSConfig{SecretID: "s"}) == nil {
		t.Fatal("NewAWSResolver returned nil")
	}
	if NewGCPResolver(GCPConfig{ProjectID: "p", SecretID: "s"}) == nil {
		t.Fatal("NewGCPResolver returned nil")
	}
	if NewAzureResolver(AzureConfig{VaultURL: "https://vault.example", SecretName: "s"}) == nil {
		t.Fatal("NewAzureResolver returned nil")
	}
	got, err := bytesSecretValue([]byte("secret"), "")
	if err != nil || got == "" {
		t.Fatalf("bytesSecretValue() = %q, %v", got, err)
	}
	if _, err := bytesSecretValue(nil, ""); err == nil {
		t.Fatal("expected empty binary error")
	}
}

func TestNewResolverCloudSourceNames(t *testing.T) {
	cases := []struct {
		name string
		cfg  SourceConfig
		want string
	}{
		{name: "aws", cfg: SourceConfig{AWS: &AWSConfig{SecretID: "s"}}, want: "aws_secrets_manager"},
		{name: "gcp", cfg: SourceConfig{GCP: &GCPConfig{ProjectID: "p", SecretID: "s"}}, want: "gcp_secret_manager"},
		{name: "azure", cfg: SourceConfig{Azure: &AzureConfig{VaultURL: "https://vault.example", SecretName: "s"}}, want: "azure_key_vault"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			resolver, source, err := NewResolver(tt.cfg)
			if err != nil {
				t.Fatalf("NewResolver() error = %v", err)
			}
			if resolver == nil || source != tt.want {
				t.Fatalf("resolver/source = %#v/%q", resolver, source)
			}
		})
	}
}
