package tests

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func BucketTests() []TestDef {
	return []TestDef{
		{"HeadBucket", "bucket", testHeadBucket},
		{"ListBuckets", "bucket", testListBuckets},
		{"GetBucketLocation", "bucket", testGetBucketLocation},
		{"CreateBucket", "bucket", testCreateBucket},
		{"DeleteBucket", "bucket", testDeleteBucket},
		{"PutBucketTagging", "bucket", testPutBucketTagging},
		{"GetBucketTagging", "bucket", testGetBucketTagging},
		{"DeleteBucketTagging", "bucket", testDeleteBucketTagging},
		{"PutBucketCors", "bucket", testPutBucketCors},
		{"GetBucketCors", "bucket", testGetBucketCors},
		{"DeleteBucketCors", "bucket", testDeleteBucketCors},
		{"PutBucketLifecycle", "bucket", testPutBucketLifecycle},
		{"GetBucketLifecycle", "bucket", testGetBucketLifecycle},
		{"DeleteBucketLifecycle", "bucket", testDeleteBucketLifecycle},
		{"PutBucketAcl", "bucket", testPutBucketAcl},
		{"GetBucketAcl", "bucket", testGetBucketAcl},
		{"PutBucketPolicy", "bucket", testPutBucketPolicy},
		{"GetBucketPolicy", "bucket", testGetBucketPolicy},
		{"GetBucketPolicyStatus", "bucket", testGetBucketPolicyStatus},
		{"PutBucketVersioning", "bucket", testPutBucketVersioning},
		{"GetBucketVersioning", "bucket", testGetBucketVersioning},
		{"ListObjectVersions", "bucket", testListObjectVersions},
		{"PutBucketEncryption", "bucket", testPutBucketEncryption},
		{"GetBucketEncryption", "bucket", testGetBucketEncryption},
		{"DeleteBucketEncryption", "bucket", testDeleteBucketEncryption},
		{"PutBucketOwnershipControls", "bucket", testPutBucketOwnershipControls},
		{"GetBucketOwnershipControls", "bucket", testGetBucketOwnershipControls},
		{"DeleteBucketOwnershipControls", "bucket", testDeleteBucketOwnershipControls},
		{"PutBucketNotificationConfiguration", "bucket", testPutBucketNotificationConfiguration},
		{"GetBucketNotificationConfiguration", "bucket", testGetBucketNotificationConfiguration},
	}
}

func testHeadBucket(ctx context.Context, env *Env) TestResult {
	_, err := env.Client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(env.Bucket),
	})
	if err != nil {
		return Fail(err.Error())
	}
	return Pass()
}

func testListBuckets(ctx context.Context, env *Env) TestResult {
	out, err := env.Client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return Fail(err.Error())
	}
	// Verify test bucket appears.
	for _, b := range out.Buckets {
		if aws.ToString(b.Name) == env.Bucket {
			return Pass()
		}
	}
	return Fail("test bucket not found in ListBuckets response")
}

func testGetBucketLocation(ctx context.Context, env *Env) TestResult {
	_, err := env.Client.GetBucketLocation(ctx, &s3.GetBucketLocationInput{
		Bucket: aws.String(env.Bucket),
	})
	if err != nil {
		return Fail(err.Error())
	}
	return Pass()
}

func testCreateBucket(ctx context.Context, env *Env) TestResult {
	name := env.Bucket + "-create-test"
	_, err := env.Client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(name),
	})
	if err != nil {
		return Fail(err.Error())
	}
	// Clean up immediately.
	_, _ = env.Client.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: aws.String(name),
	})
	return Pass()
}

func testDeleteBucket(ctx context.Context, env *Env) TestResult {
	name := env.Bucket + "-delete-test"
	_, err := env.Client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(name),
	})
	if err != nil {
		return Error("setup: " + err.Error())
	}

	_, err = env.Client.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: aws.String(name),
	})
	if err != nil {
		return Fail(err.Error())
	}
	return Pass()
}

// --- Tagging ---

func testPutBucketTagging(ctx context.Context, env *Env) TestResult {
	_, err := env.Client.PutBucketTagging(ctx, &s3.PutBucketTaggingInput{
		Bucket: aws.String(env.Bucket),
		Tagging: &types.Tagging{
			TagSet: []types.Tag{
				{Key: aws.String("env"), Value: aws.String("test")},
				{Key: aws.String("suite"), Value: aws.String("s3-compat")},
			},
		},
	})
	if err != nil {
		return Fail(err.Error())
	}
	return Pass()
}

func testGetBucketTagging(ctx context.Context, env *Env) TestResult {
	out, err := env.Client.GetBucketTagging(ctx, &s3.GetBucketTaggingInput{
		Bucket: aws.String(env.Bucket),
	})
	if err != nil {
		return Fail(err.Error())
	}
	if len(out.TagSet) == 0 {
		return Fail("no tags returned")
	}
	return Pass()
}

func testDeleteBucketTagging(ctx context.Context, env *Env) TestResult {
	_, err := env.Client.DeleteBucketTagging(ctx, &s3.DeleteBucketTaggingInput{
		Bucket: aws.String(env.Bucket),
	})
	if err != nil {
		return Fail(err.Error())
	}
	return Pass()
}

// --- CORS ---

func testPutBucketCors(ctx context.Context, env *Env) TestResult {
	_, err := env.Client.PutBucketCors(ctx, &s3.PutBucketCorsInput{
		Bucket: aws.String(env.Bucket),
		CORSConfiguration: &types.CORSConfiguration{
			CORSRules: []types.CORSRule{
				{
					AllowedOrigins: []string{"https://example.com"},
					AllowedMethods: []string{"GET", "PUT"},
					AllowedHeaders: []string{"*"},
					MaxAgeSeconds:  aws.Int32(3600),
				},
			},
		},
	})
	if err != nil {
		return Fail(err.Error())
	}
	return Pass()
}

func testGetBucketCors(ctx context.Context, env *Env) TestResult {
	out, err := env.Client.GetBucketCors(ctx, &s3.GetBucketCorsInput{
		Bucket: aws.String(env.Bucket),
	})
	if err != nil {
		return Fail(err.Error())
	}
	if len(out.CORSRules) == 0 {
		return Fail("no CORS rules returned")
	}
	return Pass()
}

func testDeleteBucketCors(ctx context.Context, env *Env) TestResult {
	_, err := env.Client.DeleteBucketCors(ctx, &s3.DeleteBucketCorsInput{
		Bucket: aws.String(env.Bucket),
	})
	if err != nil {
		return Fail(err.Error())
	}
	return Pass()
}

// --- Lifecycle ---

func testPutBucketLifecycle(ctx context.Context, env *Env) TestResult {
	_, err := env.Client.PutBucketLifecycleConfiguration(ctx, &s3.PutBucketLifecycleConfigurationInput{
		Bucket: aws.String(env.Bucket),
		LifecycleConfiguration: &types.BucketLifecycleConfiguration{
			Rules: []types.LifecycleRule{
				{
					ID:     aws.String("expire-test"),
					Status: types.ExpirationStatusEnabled,
					Filter: &types.LifecycleRuleFilterMemberPrefix{Value: "expire/"},
					Expiration: &types.LifecycleExpiration{
						Days: aws.Int32(30),
					},
				},
			},
		},
	})
	if err != nil {
		return Fail(err.Error())
	}
	return Pass()
}

func testGetBucketLifecycle(ctx context.Context, env *Env) TestResult {
	out, err := env.Client.GetBucketLifecycleConfiguration(ctx, &s3.GetBucketLifecycleConfigurationInput{
		Bucket: aws.String(env.Bucket),
	})
	if err != nil {
		return Fail(err.Error())
	}
	if len(out.Rules) == 0 {
		return Fail("no lifecycle rules returned")
	}
	return Pass()
}

func testDeleteBucketLifecycle(ctx context.Context, env *Env) TestResult {
	_, err := env.Client.DeleteBucketLifecycleConfiguration(ctx, &s3.DeleteBucketLifecycleConfigurationInput{
		Bucket: aws.String(env.Bucket),
	})
	if err != nil {
		return Fail(err.Error())
	}
	return Pass()
}

// --- ACL ---

func testPutBucketAcl(ctx context.Context, env *Env) TestResult {
	_, err := env.Client.PutBucketAcl(ctx, &s3.PutBucketAclInput{
		Bucket: aws.String(env.Bucket),
		ACL:    types.BucketCannedACLPrivate,
	})
	if err != nil {
		return Fail(err.Error())
	}
	return Pass()
}

func testGetBucketAcl(ctx context.Context, env *Env) TestResult {
	out, err := env.Client.GetBucketAcl(ctx, &s3.GetBucketAclInput{
		Bucket: aws.String(env.Bucket),
	})
	if err != nil {
		return Fail(err.Error())
	}
	if out.Owner == nil {
		return Fail("no owner in ACL response")
	}
	return Pass()
}

// --- Policy ---

func testPutBucketPolicy(ctx context.Context, env *Env) TestResult {
	policy := fmt.Sprintf(`{
		"Version": "2012-10-17",
		"Statement": [{
			"Sid": "CompatTest",
			"Effect": "Deny",
			"Principal": "*",
			"Action": "s3:DeleteBucket",
			"Resource": "arn:aws:s3:::%s"
		}]
	}`, env.Bucket)

	_, err := env.Client.PutBucketPolicy(ctx, &s3.PutBucketPolicyInput{
		Bucket: aws.String(env.Bucket),
		Policy: aws.String(policy),
	})
	if err != nil {
		return Fail(err.Error())
	}
	return Pass()
}

func testGetBucketPolicy(ctx context.Context, env *Env) TestResult {
	out, err := env.Client.GetBucketPolicy(ctx, &s3.GetBucketPolicyInput{
		Bucket: aws.String(env.Bucket),
	})
	if err != nil {
		return Fail(err.Error())
	}
	if out.Policy == nil || !strings.Contains(*out.Policy, "CompatTest") {
		return Fail("policy not returned correctly")
	}
	return Pass()
}

func testGetBucketPolicyStatus(ctx context.Context, env *Env) TestResult {
	_, err := env.Client.GetBucketPolicyStatus(ctx, &s3.GetBucketPolicyStatusInput{
		Bucket: aws.String(env.Bucket),
	})
	if err != nil {
		return Fail(err.Error())
	}
	return Pass()
}

// --- Versioning ---

func testPutBucketVersioning(ctx context.Context, env *Env) TestResult {
	_, err := env.Client.PutBucketVersioning(ctx, &s3.PutBucketVersioningInput{
		Bucket: aws.String(env.Bucket),
		VersioningConfiguration: &types.VersioningConfiguration{
			Status: types.BucketVersioningStatusEnabled,
		},
	})
	if err != nil {
		return Fail(err.Error())
	}
	return Pass()
}

func testGetBucketVersioning(ctx context.Context, env *Env) TestResult {
	out, err := env.Client.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{
		Bucket: aws.String(env.Bucket),
	})
	if err != nil {
		return Fail(err.Error())
	}
	_ = out.Status // may be empty if never enabled
	return Pass()
}

func testListObjectVersions(ctx context.Context, env *Env) TestResult {
	_, err := env.Client.ListObjectVersions(ctx, &s3.ListObjectVersionsInput{
		Bucket: aws.String(env.Bucket),
	})
	if err != nil {
		return Fail(err.Error())
	}
	return Pass()
}

// --- Encryption ---

func testPutBucketEncryption(ctx context.Context, env *Env) TestResult {
	_, err := env.Client.PutBucketEncryption(ctx, &s3.PutBucketEncryptionInput{
		Bucket: aws.String(env.Bucket),
		ServerSideEncryptionConfiguration: &types.ServerSideEncryptionConfiguration{
			Rules: []types.ServerSideEncryptionRule{
				{
					ApplyServerSideEncryptionByDefault: &types.ServerSideEncryptionByDefault{
						SSEAlgorithm: types.ServerSideEncryptionAes256,
					},
				},
			},
		},
	})
	if err != nil {
		return Fail(err.Error())
	}
	return Pass()
}

func testGetBucketEncryption(ctx context.Context, env *Env) TestResult {
	out, err := env.Client.GetBucketEncryption(ctx, &s3.GetBucketEncryptionInput{
		Bucket: aws.String(env.Bucket),
	})
	if err != nil {
		return Fail(err.Error())
	}
	if len(out.ServerSideEncryptionConfiguration.Rules) == 0 {
		return Fail("no encryption rules returned")
	}
	return Pass()
}

func testDeleteBucketEncryption(ctx context.Context, env *Env) TestResult {
	_, err := env.Client.DeleteBucketEncryption(ctx, &s3.DeleteBucketEncryptionInput{
		Bucket: aws.String(env.Bucket),
	})
	if err != nil {
		return Fail(err.Error())
	}
	return Pass()
}

// --- Ownership Controls ---

func testPutBucketOwnershipControls(ctx context.Context, env *Env) TestResult {
	_, err := env.Client.PutBucketOwnershipControls(ctx, &s3.PutBucketOwnershipControlsInput{
		Bucket: aws.String(env.Bucket),
		OwnershipControls: &types.OwnershipControls{
			Rules: []types.OwnershipControlsRule{
				{ObjectOwnership: types.ObjectOwnershipBucketOwnerEnforced},
			},
		},
	})
	if err != nil {
		return Fail(err.Error())
	}
	return Pass()
}

func testGetBucketOwnershipControls(ctx context.Context, env *Env) TestResult {
	out, err := env.Client.GetBucketOwnershipControls(ctx, &s3.GetBucketOwnershipControlsInput{
		Bucket: aws.String(env.Bucket),
	})
	if err != nil {
		return Fail(err.Error())
	}
	if len(out.OwnershipControls.Rules) == 0 {
		return Fail("no ownership rules returned")
	}
	return Pass()
}

func testDeleteBucketOwnershipControls(ctx context.Context, env *Env) TestResult {
	_, err := env.Client.DeleteBucketOwnershipControls(ctx, &s3.DeleteBucketOwnershipControlsInput{
		Bucket: aws.String(env.Bucket),
	})
	if err != nil {
		return Fail(err.Error())
	}
	return Pass()
}

// --- Notifications ---

func testPutBucketNotificationConfiguration(ctx context.Context, env *Env) TestResult {
	// Put an empty notification config (valid — clears all notifications).
	_, err := env.Client.PutBucketNotificationConfiguration(ctx, &s3.PutBucketNotificationConfigurationInput{
		Bucket:                    aws.String(env.Bucket),
		NotificationConfiguration: &types.NotificationConfiguration{},
	})
	if err != nil {
		return Fail(err.Error())
	}
	return Pass()
}

func testGetBucketNotificationConfiguration(ctx context.Context, env *Env) TestResult {
	_, err := env.Client.GetBucketNotificationConfiguration(ctx, &s3.GetBucketNotificationConfigurationInput{
		Bucket: aws.String(env.Bucket),
	})
	if err != nil {
		return Fail(err.Error())
	}
	return Pass()
}
