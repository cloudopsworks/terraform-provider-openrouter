package secrets

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

type awsSecretsManagerClient interface {
	GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
}

type AWSResolver struct {
	cfg       AWSConfig
	newClient func(context.Context, AWSConfig) (awsSecretsManagerClient, error)
}

var newSTSAssumeRoleClient = func(cfg aws.Config) stscreds.AssumeRoleAPIClient {
	return sts.NewFromConfig(cfg)
}

func NewAWSResolver(cfg AWSConfig) *AWSResolver {
	return &AWSResolver{cfg: cfg, newClient: newAWSSecretsManagerClient}
}

func NewAWSResolverWithClient(cfg AWSConfig, cl awsSecretsManagerClient) *AWSResolver {
	return &AWSResolver{cfg: cfg, newClient: func(context.Context, AWSConfig) (awsSecretsManagerClient, error) { return cl, nil }}
}

func (r *AWSResolver) Resolve(ctx context.Context) (Settings, error) {
	if r.cfg.SecretID == "" {
		return Settings{}, fmt.Errorf("aws_secrets_manager.secret_id is required")
	}
	cl, err := r.newClient(ctx, r.cfg)
	if err != nil {
		return Settings{}, fmt.Errorf("create AWS Secrets Manager client: %w", err)
	}
	input := &secretsmanager.GetSecretValueInput{SecretId: &r.cfg.SecretID}
	if r.cfg.VersionID != "" {
		input.VersionId = &r.cfg.VersionID
	}
	if r.cfg.VersionStage != "" {
		input.VersionStage = &r.cfg.VersionStage
	}
	out, err := cl.GetSecretValue(ctx, input)
	if err != nil {
		return Settings{}, fmt.Errorf("read AWS Secrets Manager secret: %w", err)
	}
	if out.SecretString != nil {
		return extractSecretSettings(*out.SecretString, r.cfg.JSONKey)
	}
	return bytesSecretSettings(out.SecretBinary, r.cfg.JSONKey)
}

func newAWSSecretsManagerClient(ctx context.Context, cfg AWSConfig) (awsSecretsManagerClient, error) {
	awsCfg, err := loadAWSConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return secretsmanager.NewFromConfig(awsCfg), nil
}

func loadAWSConfig(ctx context.Context, cfg AWSConfig) (aws.Config, error) {
	options := []func(*awsconfig.LoadOptions) error{}
	if cfg.Region != "" {
		options = append(options, awsconfig.WithRegion(cfg.Region))
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, options...)
	if err != nil {
		return aws.Config{}, err
	}
	if cfg.RoleARN != "" {
		assumeRoleOptions := []func(*stscreds.AssumeRoleOptions){
			func(o *stscreds.AssumeRoleOptions) {
				if cfg.RoleSessionName != "" {
					o.RoleSessionName = cfg.RoleSessionName
				}
				if cfg.ExternalID != "" {
					o.ExternalID = aws.String(cfg.ExternalID)
				}
				if cfg.DurationSeconds > 0 {
					o.Duration = time.Duration(cfg.DurationSeconds) * time.Second
				}
			},
		}
		awsCfg.Credentials = aws.NewCredentialsCache(stscreds.NewAssumeRoleProvider(newSTSAssumeRoleClient(awsCfg), cfg.RoleARN, assumeRoleOptions...))
	}
	return awsCfg, nil
}
