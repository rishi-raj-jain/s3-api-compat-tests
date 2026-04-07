# S3 API Compatibility Tests

Tests S3 API operations across multiple providers using the AWS Go SDK v2. Produces a structured JSON report showing which operations pass or fail on each provider.

## Supported Providers

| Provider | Endpoint | Auth |
|----------|----------|------|
| **Tigris** | `https://t3.storage.dev` | Access key + secret |
| **AWS S3** | Default | Access key + secret |
| **Cloudflare R2** | `https://<account-id>.r2.cloudflarestorage.com` | Access key + secret |
| **Google Cloud Storage** | `https://storage.googleapis.com` | HMAC key + secret |

## Setup

```bash
cp .env.example .env
# Fill in credentials for the providers you want to test
```

### Environment Variables

```bash
# Tigris
TIGRIS_ACCESS_KEY_ID=
TIGRIS_SECRET_ACCESS_KEY=

# AWS S3
AWS_S3_ACCESS_KEY_ID=
AWS_S3_SECRET_ACCESS_KEY=
AWS_S3_REGION=us-east-1          # optional, defaults to us-east-1

# Cloudflare R2
R2_ACCESS_KEY_ID=
R2_SECRET_ACCESS_KEY=
R2_ENDPOINT=https://<account-id>.r2.cloudflarestorage.com

# Google Cloud Storage (S3-compatible HMAC)
GCS_ACCESS_KEY_ID=               # Create HMAC keys in GCS console
GCS_SECRET_ACCESS_KEY=
```

Only providers with both access key and secret key set will be tested.

## Usage

```bash
# Test all configured providers
go run .

# Test specific providers
go run . --providers=tigris,aws

# Custom output path
go run . --providers=tigris --output=tigris-results.json
```

## Operations Tested

**67 operations** across 4 categories:

### Object (25)
PutObject, GetObject, HeadObject, CopyObject, DeleteObject, DeleteObjects (verbose + quiet), RestoreObject, ListObjects, ListObjectsV2 (basic, pagination, delimiter), object tagging (put/get/delete), object ACL (put/get), conditional GETs (Range, If-Match, If-None-Match, If-Modified-Since, If-Unmodified-Since), PutObject variants (metadata, SHA256 checksum, SSE-S3, storage class)

### Bucket (30)
CreateBucket, DeleteBucket, HeadBucket, ListBuckets, GetBucketLocation, bucket tagging (put/get/delete), CORS (put/get/delete), lifecycle (put/get/delete), ACL (put/get), policy (put/get/status), versioning (put/get), ListObjectVersions, encryption (put/get/delete), ownership controls (put/get/delete), notification configuration (put/get)

### Multipart (7)
CreateMultipartUpload, UploadPart, UploadPartCopy, CompleteMultipartUpload, AbortMultipartUpload, ListMultipartUploads, ListParts

### Presigned URLs (5)
PresignPutObject, PresignGetObject, PresignHeadObject, PresignDeleteObject, PresignUploadPart

## Output Format

```json
{
  "generated_at": "2026-04-07T15:30:00Z",
  "providers": [
    {
      "name": "tigris",
      "endpoint": "https://t3.storage.dev",
      "region": "auto",
      "total": 67,
      "passed": 50,
      "failed": 3,
      "skipped": 2,
      "errors": 1,
      "results": [
        {
          "operation": "PutObject",
          "category": "object",
          "status": "pass",
          "duration_ms": 245
        },
        {
          "operation": "GetBucketVersioning",
          "category": "bucket",
          "status": "fail",
          "duration_ms": 120,
          "error": "NotImplemented"
        }
      ]
    }
  ]
}
```

### Status Values

| Status | Meaning |
|--------|---------|
| `pass` | Operation completed with expected behavior |
| `fail` | Operation returned an unexpected error or wrong behavior |
| `skip` | Operation intentionally skipped (known unsupported) |
| `error` | Infrastructure error (timeout, network, bad credentials) |

## How It Works

1. For each provider, creates a temporary bucket (`s3compat-{provider}-{random}`)
2. Runs all tests sequentially (some tests depend on objects from earlier tests)
3. Cleans up: deletes all objects, aborts multipart uploads, deletes the bucket
4. Writes a JSON report with per-operation results

## Requirements

- Go 1.23.5+
- AWS Go SDK v2
