package tests

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// Every test sets up what it needs, so any subset can run with --tests.
func BucketTests() []TestDef {
	return []TestDef{
		{"HeadBucket", "bucket", testHeadBucket},
		{"HeadBucket_NoSuchBucket", "bucket", testHeadBucketNoSuchBucket},
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
		{"DeleteBucketPolicy", "bucket", testDeleteBucketPolicy},
		{"GetBucketPolicyStatus", "bucket", testGetBucketPolicyStatus},
		{"PutBucketVersioning", "bucket", testPutBucketVersioning},
		{"PutBucketAccelerateConfiguration", "bucket", testPutBucketAccelerateConfiguration},
		{"GetBucketAccelerateConfiguration", "bucket", testGetBucketAccelerateConfiguration},
		{"GetBucketRequestPayment", "bucket", testGetBucketRequestPayment},
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

// ensureBucket re-creates the test bucket when a bucket-level operation removed
// it (some providers treat DELETE ?tagging as DeleteBucket), so one such
// operation does not turn every later test into NoSuchBucket. It reports
// whether the bucket had vanished; any other HeadBucket error is returned as is.
func ensureBucket(ctx context.Context, env *Env) (bool, error) {
	_, err := env.Client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(env.Bucket)})
	if err == nil {
		return false, nil
	}
	code := errCode(err)
	if httpStatus(err) != 404 && code != "NotFound" && code != "NoSuchBucket" {
		return false, err
	}
	if _, cerr := env.Client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(env.Bucket)}); cerr != nil {
		return true, fmt.Errorf("re-create after it vanished: %w", cerr)
	}
	return true, nil
}

// afterConfigDelete checks the GET that follows a bucket-configuration delete:
// it must fail with the S3 code for a missing configuration, and the bucket
// itself must still exist.
func afterConfigDelete(ctx context.Context, env *Env, err error, what, code string) TestResult {
	vanished, herr := ensureBucket(ctx, env)
	if herr != nil {
		return Fail(fmt.Sprintf("%s: HeadBucket: %v", what, herr))
	}
	if vanished {
		return Fail(what + ": the bucket itself was deleted")
	}
	return expectFailure(err, what, 0, code)
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

// HeadBucket of a bucket that does not exist is 404.
func testHeadBucketNoSuchBucket(ctx context.Context, env *Env) TestResult {
	_, err := env.Client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(env.Bucket + "-does-not-exist"),
	})
	return expectFailure(err, "HeadBucket of a missing bucket", 404, "NotFound", "NoSuchBucket")
}

func testListBuckets(ctx context.Context, env *Env) TestResult {
	out, err := env.Client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return Fail(err.Error())
	}
	for _, b := range out.Buckets {
		if aws.ToString(b.Name) == env.Bucket {
			return Pass()
		}
	}
	return Fail("test bucket not found in ListBuckets response")
}

// GetBucketLocation answers; the constraint value is provider-specific and may
// be empty for the default region, as on AWS us-east-1.
func testGetBucketLocation(ctx context.Context, env *Env) TestResult {
	_, err := env.Client.GetBucketLocation(ctx, &s3.GetBucketLocationInput{
		Bucket: aws.String(env.Bucket),
	})
	if err != nil {
		return Fail(err.Error())
	}
	return Pass()
}

// CreateBucket makes a bucket that HeadBucket then finds.
func testCreateBucket(ctx context.Context, env *Env) TestResult {
	name := env.Bucket + "-create-test"
	_, err := env.Client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(name),
	})
	if err != nil {
		return Fail(err.Error())
	}
	defer func() {
		_, _ = env.Client.DeleteBucket(ctx, &s3.DeleteBucketInput{Bucket: aws.String(name)})
	}()
	if _, err := env.Client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(name)}); err != nil {
		return Fail("HeadBucket after create: " + err.Error())
	}
	return Pass()
}

// DeleteBucket removes the bucket; HeadBucket is then 404.
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
	_, err = env.Client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(name)})
	return expectFailure(err, "HeadBucket after DeleteBucket", 404, "NotFound", "NoSuchBucket")
}

// --- Tagging ---

func putBucketTags(ctx context.Context, env *Env) error {
	_, err := env.Client.PutBucketTagging(ctx, &s3.PutBucketTaggingInput{
		Bucket: aws.String(env.Bucket),
		Tagging: &types.Tagging{
			TagSet: []types.Tag{
				{Key: aws.String("env"), Value: aws.String("test")},
				{Key: aws.String("suite"), Value: aws.String("s3-compat")},
			},
		},
	})
	return err
}

func bucketTags(ctx context.Context, env *Env) (map[string]string, error) {
	out, err := env.Client.GetBucketTagging(ctx, &s3.GetBucketTaggingInput{Bucket: aws.String(env.Bucket)})
	if err != nil {
		return nil, err
	}
	tags := map[string]string{}
	for _, t := range out.TagSet {
		tags[aws.ToString(t.Key)] = aws.ToString(t.Value)
	}
	return tags, nil
}

func checkBucketTags(ctx context.Context, env *Env) TestResult {
	return eventually(ctx, propagation, func() TestResult {
		tags, err := bucketTags(ctx, env)
		if err != nil {
			return Fail("get tags: " + err.Error())
		}
		if tags["env"] != "test" || tags["suite"] != "s3-compat" || len(tags) != 2 {
			return Fail(fmt.Sprintf("tags not stored as sent: %v", tags))
		}
		return Pass()
	})
}

func testPutBucketTagging(ctx context.Context, env *Env) TestResult {
	if err := putBucketTags(ctx, env); err != nil {
		return Fail(err.Error())
	}
	return checkBucketTags(ctx, env)
}

func testGetBucketTagging(ctx context.Context, env *Env) TestResult {
	if err := putBucketTags(ctx, env); err != nil {
		return Error("setup: " + err.Error())
	}
	return checkBucketTags(ctx, env)
}

// After DeleteBucketTagging, GetBucketTagging is NoSuchTagSet.
func testDeleteBucketTagging(ctx context.Context, env *Env) TestResult {
	if err := putBucketTags(ctx, env); err != nil {
		return Error("setup: " + err.Error())
	}
	_, err := env.Client.DeleteBucketTagging(ctx, &s3.DeleteBucketTaggingInput{
		Bucket: aws.String(env.Bucket),
	})
	if err != nil {
		return Fail(err.Error())
	}
	return eventually(ctx, propagation, func() TestResult {
		_, err := bucketTags(ctx, env)
		return afterConfigDelete(ctx, env, err, "GetBucketTagging after delete", "NoSuchTagSet")
	})
}

// --- CORS ---

func putCors(ctx context.Context, env *Env) error {
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
	return err
}

func checkCors(ctx context.Context, env *Env) TestResult {
	return eventually(ctx, propagation, func() TestResult {
		out, err := env.Client.GetBucketCors(ctx, &s3.GetBucketCorsInput{Bucket: aws.String(env.Bucket)})
		if err != nil {
			return Fail("get cors: " + err.Error())
		}
		if len(out.CORSRules) != 1 || len(out.CORSRules[0].AllowedOrigins) != 1 || out.CORSRules[0].AllowedOrigins[0] != "https://example.com" {
			return Fail(fmt.Sprintf("CORS rule not stored as sent: %+v", out.CORSRules))
		}
		methods := strings.Join(out.CORSRules[0].AllowedMethods, ",")
		if !strings.Contains(methods, "GET") || !strings.Contains(methods, "PUT") {
			return Fail(fmt.Sprintf("allowed methods mismatch: %s", methods))
		}
		return Pass()
	})
}

func testPutBucketCors(ctx context.Context, env *Env) TestResult {
	if err := putCors(ctx, env); err != nil {
		return Fail(err.Error())
	}
	return checkCors(ctx, env)
}

func testGetBucketCors(ctx context.Context, env *Env) TestResult {
	if err := putCors(ctx, env); err != nil {
		return Error("setup: " + err.Error())
	}
	return checkCors(ctx, env)
}

// After DeleteBucketCors, GetBucketCors is NoSuchCORSConfiguration.
func testDeleteBucketCors(ctx context.Context, env *Env) TestResult {
	if err := putCors(ctx, env); err != nil {
		return Error("setup: " + err.Error())
	}
	_, err := env.Client.DeleteBucketCors(ctx, &s3.DeleteBucketCorsInput{
		Bucket: aws.String(env.Bucket),
	})
	if err != nil {
		return Fail(err.Error())
	}
	return eventually(ctx, propagation, func() TestResult {
		_, err := env.Client.GetBucketCors(ctx, &s3.GetBucketCorsInput{Bucket: aws.String(env.Bucket)})
		return afterConfigDelete(ctx, env, err, "GetBucketCors after delete", "NoSuchCORSConfiguration")
	})
}

// --- Lifecycle ---

func putLifecycle(ctx context.Context, env *Env) error {
	_, err := env.Client.PutBucketLifecycleConfiguration(ctx, &s3.PutBucketLifecycleConfigurationInput{
		Bucket: aws.String(env.Bucket),
		LifecycleConfiguration: &types.BucketLifecycleConfiguration{
			Rules: []types.LifecycleRule{
				{
					ID:     aws.String("expire-test"),
					Status: types.ExpirationStatusEnabled,
					Filter: &types.LifecycleRuleFilter{Prefix: aws.String("expire/")},
					Expiration: &types.LifecycleExpiration{
						Days: aws.Int32(30),
					},
				},
			},
		},
	})
	return err
}

func checkLifecycle(ctx context.Context, env *Env) TestResult {
	return eventually(ctx, propagation, func() TestResult {
		out, err := env.Client.GetBucketLifecycleConfiguration(ctx, &s3.GetBucketLifecycleConfigurationInput{Bucket: aws.String(env.Bucket)})
		if err != nil {
			return Fail("get lifecycle: " + err.Error())
		}
		if len(out.Rules) != 1 {
			return Fail(fmt.Sprintf("expected 1 rule, got %d", len(out.Rules)))
		}
		r := out.Rules[0]
		if aws.ToString(r.ID) != "expire-test" || r.Status != types.ExpirationStatusEnabled || r.Expiration == nil || aws.ToInt32(r.Expiration.Days) != 30 {
			return Fail(fmt.Sprintf("rule not stored as sent: %+v", r))
		}
		if r.Filter == nil || aws.ToString(r.Filter.Prefix) != "expire/" {
			return Fail(fmt.Sprintf("rule prefix not stored: %+v", r.Filter))
		}
		return Pass()
	})
}

func testPutBucketLifecycle(ctx context.Context, env *Env) TestResult {
	if err := putLifecycle(ctx, env); err != nil {
		return Fail(err.Error())
	}
	return checkLifecycle(ctx, env)
}

func testGetBucketLifecycle(ctx context.Context, env *Env) TestResult {
	if err := putLifecycle(ctx, env); err != nil {
		return Error("setup: " + err.Error())
	}
	return checkLifecycle(ctx, env)
}

// After DeleteBucketLifecycle, the GET is NoSuchLifecycleConfiguration.
func testDeleteBucketLifecycle(ctx context.Context, env *Env) TestResult {
	if err := putLifecycle(ctx, env); err != nil {
		return Error("setup: " + err.Error())
	}
	_, err := env.Client.DeleteBucketLifecycle(ctx, &s3.DeleteBucketLifecycleInput{
		Bucket: aws.String(env.Bucket),
	})
	if err != nil {
		return Fail(err.Error())
	}
	return eventually(ctx, propagation, func() TestResult {
		_, err := env.Client.GetBucketLifecycleConfiguration(ctx, &s3.GetBucketLifecycleConfigurationInput{Bucket: aws.String(env.Bucket)})
		return afterConfigDelete(ctx, env, err, "GetBucketLifecycleConfiguration after delete", "NoSuchLifecycleConfiguration")
	})
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
	return eventually(ctx, propagation, func() TestResult {
		out, err := env.Client.GetBucketAcl(ctx, &s3.GetBucketAclInput{Bucket: aws.String(env.Bucket)})
		if err != nil {
			return Fail("get acl: " + err.Error())
		}
		if len(out.Grants) == 0 {
			return Fail("no grants after PutBucketAcl private")
		}
		for _, g := range out.Grants {
			if g.Grantee != nil && g.Grantee.Type == types.TypeGroup && strings.Contains(aws.ToString(g.Grantee.URI), "AllUsers") {
				return Fail("private ACL still grants AllUsers")
			}
		}
		return Pass()
	})
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
	if len(out.Grants) == 0 {
		return Fail("no grants in ACL response")
	}
	return Pass()
}

// --- Policy ---

func putPolicy(ctx context.Context, env *Env) error {
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
	return err
}

func checkPolicy(ctx context.Context, env *Env) TestResult {
	return eventually(ctx, propagation, func() TestResult {
		out, err := env.Client.GetBucketPolicy(ctx, &s3.GetBucketPolicyInput{Bucket: aws.String(env.Bucket)})
		if err != nil {
			return Fail(err.Error())
		}
		if out.Policy == nil || !strings.Contains(*out.Policy, "CompatTest") {
			return Fail("policy not returned as sent")
		}
		return Pass()
	})
}

func testPutBucketPolicy(ctx context.Context, env *Env) TestResult {
	if err := putPolicy(ctx, env); err != nil {
		return Fail(err.Error())
	}
	return checkPolicy(ctx, env)
}

func testGetBucketPolicy(ctx context.Context, env *Env) TestResult {
	if err := putPolicy(ctx, env); err != nil {
		return Error("setup: " + err.Error())
	}
	return checkPolicy(ctx, env)
}

// After DeleteBucketPolicy, GetBucketPolicy is NoSuchBucketPolicy.
func testDeleteBucketPolicy(ctx context.Context, env *Env) TestResult {
	if err := putPolicy(ctx, env); err != nil {
		return Error("setup: " + err.Error())
	}
	_, err := env.Client.DeleteBucketPolicy(ctx, &s3.DeleteBucketPolicyInput{
		Bucket: aws.String(env.Bucket),
	})
	if err != nil {
		return Fail(err.Error())
	}
	return eventually(ctx, propagation, func() TestResult {
		_, err := env.Client.GetBucketPolicy(ctx, &s3.GetBucketPolicyInput{Bucket: aws.String(env.Bucket)})
		return afterConfigDelete(ctx, env, err, "GetBucketPolicy after delete", "NoSuchBucketPolicy")
	})
}

// A fresh private bucket is not public.
func testGetBucketPolicyStatus(ctx context.Context, env *Env) TestResult {
	out, err := env.Client.GetBucketPolicyStatus(ctx, &s3.GetBucketPolicyStatusInput{
		Bucket: aws.String(env.Bucket),
	})
	if err != nil {
		return Fail(err.Error())
	}
	if out.PolicyStatus == nil {
		return Fail("no PolicyStatus in response")
	}
	if aws.ToBool(out.PolicyStatus.IsPublic) {
		return Fail("fresh bucket reported as public")
	}
	return Pass()
}

// --- Versioning on an existing bucket ---

// Turning versioning on for a bucket created without it. Providers where
// versioning is a creation-time property fail here; see the versioning
// category for the operations on a bucket that has it.
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
	return eventually(ctx, propagation, func() TestResult {
		out, err := env.Client.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{Bucket: aws.String(env.Bucket)})
		if err != nil {
			return Fail("get versioning: " + err.Error())
		}
		if out.Status != types.BucketVersioningStatusEnabled {
			return Fail(fmt.Sprintf("PutBucketVersioning succeeded but status is %q", out.Status))
		}
		return Pass()
	})
}

// --- Accelerate and request payment ---

func putAccelerateSuspended(ctx context.Context, env *Env) error {
	_, err := env.Client.PutBucketAccelerateConfiguration(ctx, &s3.PutBucketAccelerateConfigurationInput{
		Bucket:                  aws.String(env.Bucket),
		AccelerateConfiguration: &types.AccelerateConfiguration{Status: types.BucketAccelerateStatusSuspended},
	})
	return err
}

func checkAccelerateSuspended(ctx context.Context, env *Env) TestResult {
	return eventually(ctx, propagation, func() TestResult {
		out, err := env.Client.GetBucketAccelerateConfiguration(ctx, &s3.GetBucketAccelerateConfigurationInput{Bucket: aws.String(env.Bucket)})
		if err != nil {
			return Fail(err.Error())
		}
		if out.Status != types.BucketAccelerateStatusSuspended {
			return Fail(fmt.Sprintf("expected Suspended after the Put, got %q", out.Status))
		}
		return Pass()
	})
}

func testPutBucketAccelerateConfiguration(ctx context.Context, env *Env) TestResult {
	if err := putAccelerateSuspended(ctx, env); err != nil {
		return Fail(err.Error())
	}
	return checkAccelerateSuspended(ctx, env)
}

func testGetBucketAccelerateConfiguration(ctx context.Context, env *Env) TestResult {
	if err := putAccelerateSuspended(ctx, env); err != nil {
		return Error("setup: " + err.Error())
	}
	return checkAccelerateSuspended(ctx, env)
}

func testGetBucketRequestPayment(ctx context.Context, env *Env) TestResult {
	out, err := env.Client.GetBucketRequestPayment(ctx, &s3.GetBucketRequestPaymentInput{
		Bucket: aws.String(env.Bucket),
	})
	if err != nil {
		return Fail(err.Error())
	}
	if out.Payer != types.PayerBucketOwner {
		return Fail(fmt.Sprintf("expected Payer BucketOwner, got %q", out.Payer))
	}
	return Pass()
}

// --- Encryption ---

func putEncryption(ctx context.Context, env *Env) error {
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
	return err
}

func checkEncryption(ctx context.Context, env *Env) TestResult {
	return eventually(ctx, propagation, func() TestResult {
		out, err := env.Client.GetBucketEncryption(ctx, &s3.GetBucketEncryptionInput{Bucket: aws.String(env.Bucket)})
		if err != nil {
			return Fail(err.Error())
		}
		if out.ServerSideEncryptionConfiguration == nil || len(out.ServerSideEncryptionConfiguration.Rules) == 0 {
			return Fail("no encryption rules returned")
		}
		d := out.ServerSideEncryptionConfiguration.Rules[0].ApplyServerSideEncryptionByDefault
		if d == nil || d.SSEAlgorithm != types.ServerSideEncryptionAes256 {
			return Fail(fmt.Sprintf("expected AES256 default, got %+v", d))
		}
		return Pass()
	})
}

func testPutBucketEncryption(ctx context.Context, env *Env) TestResult {
	if err := putEncryption(ctx, env); err != nil {
		return Fail(err.Error())
	}
	return checkEncryption(ctx, env)
}

func testGetBucketEncryption(ctx context.Context, env *Env) TestResult {
	if err := putEncryption(ctx, env); err != nil {
		return Error("setup: " + err.Error())
	}
	return checkEncryption(ctx, env)
}

// After DeleteBucketEncryption, the GET is ServerSideEncryptionConfigurationNotFoundError.
func testDeleteBucketEncryption(ctx context.Context, env *Env) TestResult {
	if err := putEncryption(ctx, env); err != nil {
		return Error("setup: " + err.Error())
	}
	_, err := env.Client.DeleteBucketEncryption(ctx, &s3.DeleteBucketEncryptionInput{
		Bucket: aws.String(env.Bucket),
	})
	if err != nil {
		return Fail(err.Error())
	}
	return eventually(ctx, propagation, func() TestResult {
		_, err := env.Client.GetBucketEncryption(ctx, &s3.GetBucketEncryptionInput{Bucket: aws.String(env.Bucket)})
		return afterConfigDelete(ctx, env, err, "GetBucketEncryption after delete", "ServerSideEncryptionConfigurationNotFoundError")
	})
}

// --- Ownership Controls ---

func putOwnership(ctx context.Context, env *Env) error {
	_, err := env.Client.PutBucketOwnershipControls(ctx, &s3.PutBucketOwnershipControlsInput{
		Bucket: aws.String(env.Bucket),
		OwnershipControls: &types.OwnershipControls{
			Rules: []types.OwnershipControlsRule{
				{ObjectOwnership: types.ObjectOwnershipBucketOwnerEnforced},
			},
		},
	})
	return err
}

func checkOwnership(ctx context.Context, env *Env) TestResult {
	return eventually(ctx, propagation, func() TestResult {
		out, err := env.Client.GetBucketOwnershipControls(ctx, &s3.GetBucketOwnershipControlsInput{Bucket: aws.String(env.Bucket)})
		if err != nil {
			return Fail(err.Error())
		}
		if out.OwnershipControls == nil || len(out.OwnershipControls.Rules) == 0 {
			return Fail("no ownership rules returned")
		}
		if out.OwnershipControls.Rules[0].ObjectOwnership != types.ObjectOwnershipBucketOwnerEnforced {
			return Fail(fmt.Sprintf("expected BucketOwnerEnforced, got %q", out.OwnershipControls.Rules[0].ObjectOwnership))
		}
		return Pass()
	})
}

func testPutBucketOwnershipControls(ctx context.Context, env *Env) TestResult {
	if err := putOwnership(ctx, env); err != nil {
		return Fail(err.Error())
	}
	return checkOwnership(ctx, env)
}

func testGetBucketOwnershipControls(ctx context.Context, env *Env) TestResult {
	if err := putOwnership(ctx, env); err != nil {
		return Error("setup: " + err.Error())
	}
	return checkOwnership(ctx, env)
}

// After DeleteBucketOwnershipControls, the GET is OwnershipControlsNotFoundError.
func testDeleteBucketOwnershipControls(ctx context.Context, env *Env) TestResult {
	if err := putOwnership(ctx, env); err != nil {
		return Error("setup: " + err.Error())
	}
	_, err := env.Client.DeleteBucketOwnershipControls(ctx, &s3.DeleteBucketOwnershipControlsInput{
		Bucket: aws.String(env.Bucket),
	})
	if err != nil {
		return Fail(err.Error())
	}
	return eventually(ctx, propagation, func() TestResult {
		_, err := env.Client.GetBucketOwnershipControls(ctx, &s3.GetBucketOwnershipControlsInput{Bucket: aws.String(env.Bucket)})
		return afterConfigDelete(ctx, env, err, "GetBucketOwnershipControls after delete", "OwnershipControlsNotFoundError")
	})
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

// GetBucketNotificationConfiguration on a bucket with no notifications returns
// an empty configuration.
func testGetBucketNotificationConfiguration(ctx context.Context, env *Env) TestResult {
	return eventually(ctx, propagation, func() TestResult {
		out, err := env.Client.GetBucketNotificationConfiguration(ctx, &s3.GetBucketNotificationConfigurationInput{
			Bucket: aws.String(env.Bucket),
		})
		if err != nil {
			return Fail(err.Error())
		}
		if len(out.TopicConfigurations)+len(out.QueueConfigurations)+len(out.LambdaFunctionConfigurations) != 0 {
			return Fail("expected an empty notification configuration")
		}
		return Pass()
	})
}
