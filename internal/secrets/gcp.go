package secrets

import (
	"context"
	"fmt"
	"strings"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	secretmanagerpb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"google.golang.org/api/impersonate"
	"google.golang.org/api/option"
)

type gcpSecretManagerClient interface {
	AccessSecretVersion(ctx context.Context, req *secretmanagerpb.AccessSecretVersionRequest, opts ...any) (*secretmanagerpb.AccessSecretVersionResponse, error)
	Close() error
}

type gcpClientAdapter struct{ *secretmanager.Client }

func (a gcpClientAdapter) AccessSecretVersion(ctx context.Context, req *secretmanagerpb.AccessSecretVersionRequest, _ ...any) (*secretmanagerpb.AccessSecretVersionResponse, error) {
	return a.Client.AccessSecretVersion(ctx, req)
}

type GCPResolver struct {
	cfg       GCPConfig
	newClient func(context.Context, GCPConfig) (gcpSecretManagerClient, error)
}

func NewGCPResolver(cfg GCPConfig) *GCPResolver {
	return &GCPResolver{cfg: cfg, newClient: newGCPSecretManagerClient}
}

func NewGCPResolverWithClient(cfg GCPConfig, cl gcpSecretManagerClient) *GCPResolver {
	return &GCPResolver{cfg: cfg, newClient: func(context.Context, GCPConfig) (gcpSecretManagerClient, error) { return cl, nil }}
}

func (r *GCPResolver) Resolve(ctx context.Context) (Settings, error) {
	if r.cfg.ProjectID == "" {
		return Settings{}, fmt.Errorf("gcp_secret_manager.project_id is required")
	}
	if r.cfg.SecretID == "" {
		return Settings{}, fmt.Errorf("gcp_secret_manager.secret_id is required")
	}
	version := r.cfg.Version
	if version == "" {
		version = "latest"
	}
	cl, err := r.newClient(ctx, r.cfg)
	if err != nil {
		return Settings{}, fmt.Errorf("create GCP Secret Manager client: %w", err)
	}
	defer cl.Close()
	name := fmt.Sprintf("projects/%s/secrets/%s/versions/%s", r.cfg.ProjectID, r.cfg.SecretID, version)
	out, err := cl.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{Name: name})
	if err != nil {
		return Settings{}, fmt.Errorf("read GCP Secret Manager secret: %w", err)
	}
	if out == nil || out.Payload == nil {
		return Settings{}, fmt.Errorf("GCP Secret Manager secret payload is empty")
	}
	return extractSecretSettings(string(out.Payload.Data), r.cfg.JSONKey)
}

func newGCPSecretManagerClient(ctx context.Context, cfg GCPConfig) (gcpSecretManagerClient, error) {
	var opts []option.ClientOption
	scopes := cfg.Scopes
	if len(scopes) == 0 {
		scopes = secretmanager.DefaultAuthScopes()
	} else {
		opts = append(opts, option.WithScopes(scopes...))
	}
	if target := strings.TrimSpace(cfg.ImpersonateServiceAccount); target != "" {
		tokenSource, err := impersonate.CredentialsTokenSource(ctx, impersonate.CredentialsConfig{
			TargetPrincipal: target,
			Scopes:          scopes,
			Delegates:       cfg.Delegates,
		}, opts...)
		if err != nil {
			return nil, err
		}
		opts = []option.ClientOption{option.WithTokenSource(tokenSource)}
	}
	cl, err := secretmanager.NewClient(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return gcpClientAdapter{Client: cl}, nil
}
