package main

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// Provider describes one S3-compatible service to test against.
type Provider struct {
	Name      string
	Endpoint  string // empty means default AWS endpoint
	Region    string
	PathStyle bool
	AccessKey string
	SecretKey string

	// StripSDKHeaders removes amz-sdk-* and accept-encoding headers before
	// signing. GCS rejects requests signed with headers it doesn't understand.
	StripSDKHeaders bool
}

// stripSDKHeadersMiddleware removes SDK-specific headers before signing.
// The retry handler adds Amz-Sdk-* headers in the Finalize step, so this
// middleware must also run in Finalize, before the signer.
type stripSDKHeadersMiddleware struct{}

func (stripSDKHeadersMiddleware) ID() string { return "StripSDKHeaders" }

func (stripSDKHeadersMiddleware) HandleFinalize(
	ctx context.Context,
	in middleware.FinalizeInput,
	next middleware.FinalizeHandler,
) (middleware.FinalizeOutput, middleware.Metadata, error) {
	req, ok := in.Request.(*smithyhttp.Request)
	if ok {
		req.Header.Del("Accept-Encoding")
		req.Header.Del("Amz-Sdk-Invocation-Id")
		req.Header.Del("Amz-Sdk-Request")
	}
	return next.HandleFinalize(ctx, in)
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
		o.DisableLogOutputChecksumValidationSkipped = true
		if p.StripSDKHeaders {
			o.APIOptions = append(o.APIOptions, func(stack *middleware.Stack) error {
				// Try to insert before "Signing" (normal client) or just add
				// at the start of Finalize (presign client has no Signing).
				if err := stack.Finalize.Insert(stripSDKHeadersMiddleware{}, "Signing", middleware.Before); err != nil {
					return stack.Finalize.Add(stripSDKHeadersMiddleware{}, middleware.Before)
				}
				return nil
			})
		}
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
			AccessKey: firstEnv("TIGRIS_ACCESS_KEY_ID", "TIGRIS_STORAGE_ACCESS_KEY_ID"),
			SecretKey: firstEnv("TIGRIS_SECRET_ACCESS_KEY", "TIGRIS_STORAGE_SECRET_ACCESS_KEY"),
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
			Endpoint:  firstEnv("R2_ENDPOINT", "CLOUDFLARE_ENDPOINT"),
			Region:    "auto",
			PathStyle: false,
			AccessKey: firstEnv("R2_ACCESS_KEY_ID", "CLOUDFLARE_ACCESS_KEY_ID"),
			SecretKey: firstEnv("R2_SECRET_ACCESS_KEY", "CLOUDFLARE_SECRET_ACCESS_KEY"),
		},
		{
			Name:            "gcs",
			Endpoint:        "https://storage.googleapis.com",
			Region:          "auto",
			PathStyle:       true,
			AccessKey:       os.Getenv("GCS_ACCESS_KEY_ID"),
			SecretKey:       os.Getenv("GCS_SECRET_ACCESS_KEY"),
			StripSDKHeaders: true,
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

// firstEnv returns the value of the first non-empty env var.
func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}
