package main

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Provider describes one S3-compatible service to test against.
type Provider struct {
	Name      string
	Endpoint  string // empty means default AWS endpoint
	Region    string
	PathStyle bool
	AccessKey string
	SecretKey string
}

// NewClient builds an s3.Client for this provider.
func (p *Provider) NewClient(ctx context.Context) (*s3.Client, error) {
	creds := credentials.NewStaticCredentialsProvider(p.AccessKey, p.SecretKey, "")
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithCredentialsProvider(creds),
		config.WithRegion(p.Region),
	)
	if err != nil {
		return nil, fmt.Errorf("load config for %s: %w", p.Name, err)
	}

	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		if p.Endpoint != "" {
			o.BaseEndpoint = aws.String(p.Endpoint)
		}
		o.UsePathStyle = p.PathStyle
		o.Region = p.Region
	}), nil
}

// NewPresignClient builds a presign client for this provider.
func (p *Provider) NewPresignClient(ctx context.Context) (*s3.PresignClient, error) {
	client, err := p.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	return s3.NewPresignClient(client), nil
}

// LoadProviders reads env vars and returns providers that have credentials configured.
// If names is non-empty, only those named providers are returned.
func LoadProviders(names []string) []Provider {
	all := []Provider{
		{
			Name:      "tigris",
			Endpoint:  "https://t3.storage.dev",
			Region:    "auto",
			PathStyle: false,
			AccessKey: os.Getenv("TIGRIS_ACCESS_KEY_ID"),
			SecretKey: os.Getenv("TIGRIS_SECRET_ACCESS_KEY"),
		},
		{
			Name:      "aws",
			Endpoint:  "",
			Region:    envOr("AWS_S3_REGION", "us-east-1"),
			PathStyle: false,
			AccessKey: os.Getenv("AWS_S3_ACCESS_KEY_ID"),
			SecretKey: os.Getenv("AWS_S3_SECRET_ACCESS_KEY"),
		},
		{
			Name:      "r2",
			Endpoint:  os.Getenv("R2_ENDPOINT"),
			Region:    "auto",
			PathStyle: false,
			AccessKey: os.Getenv("R2_ACCESS_KEY_ID"),
			SecretKey: os.Getenv("R2_SECRET_ACCESS_KEY"),
		},
		{
			Name:      "gcs",
			Endpoint:  "https://storage.googleapis.com",
			Region:    "us-central1",
			PathStyle: true,
			AccessKey: os.Getenv("GCS_ACCESS_KEY_ID"),
			SecretKey: os.Getenv("GCS_SECRET_ACCESS_KEY"),
		},
	}

	// Filter to requested names, or return all configured.
	want := make(map[string]bool, len(names))
	for _, n := range names {
		want[n] = true
	}

	var out []Provider
	for _, p := range all {
		if p.AccessKey == "" || p.SecretKey == "" {
			continue
		}
		if len(want) > 0 && !want[p.Name] {
			continue
		}
		out = append(out, p)
	}
	return out
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
