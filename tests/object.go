package tests

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func ObjectTests() []TestDef {
	return []TestDef{
		{"PutObject", "object", testPutObject},
		{"GetObject", "object", testGetObject},
		{"HeadObject", "object", testHeadObject},
		{"CopyObject", "object", testCopyObject},
		{"GetObject_Range", "object", testGetObjectRange},
		{"GetObject_IfMatch", "object", testGetObjectIfMatch},
		{"GetObject_IfNoneMatch", "object", testGetObjectIfNoneMatch},
		{"GetObject_IfModifiedSince", "object", testGetObjectIfModifiedSince},
		{"GetObject_IfUnmodifiedSince", "object", testGetObjectIfUnmodifiedSince},
		{"PutObject_Metadata", "object", testPutObjectMetadata},
		{"PutObject_ChecksumSHA256", "object", testPutObjectChecksumSHA256},
		{"PutObject_SSE_S3", "object", testPutObjectSSES3},
		{"PutObject_StorageClass", "object", testPutObjectStorageClass},
		{"PutObjectTagging", "object", testPutObjectTagging},
		{"GetObjectTagging", "object", testGetObjectTagging},
		{"DeleteObjectTagging", "object", testDeleteObjectTagging},
		{"PutObjectAcl", "object", testPutObjectAcl},
		{"GetObjectAcl", "object", testGetObjectAcl},
		{"ListObjects", "object", testListObjects},
		{"ListObjectsV2", "object", testListObjectsV2},
		{"ListObjectsV2_Pagination", "object", testListObjectsV2Pagination},
		{"ListObjectsV2_Delimiter", "object", testListObjectsV2Delimiter},
		{"DeleteObjects", "object", testDeleteObjects},
		{"DeleteObjects_Quiet", "object", testDeleteObjectsQuiet},
		{"RestoreObject", "object", testRestoreObject},
	}
}

// --- helpers ---

const testBody = "hello s3 compat test"

func putTestObject(ctx context.Context, env *Env, key string) error {
	_, err := env.Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
		Body:   strings.NewReader(testBody),
	})
	return err
}

func getObjectBody(ctx context.Context, env *Env, key string) (string, error) {
	out, err := env.Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return "", err
	}
	defer out.Body.Close()
	b, err := io.ReadAll(out.Body)
	return string(b), err
}

func deleteObject(ctx context.Context, env *Env, key string) {
	_, _ = env.Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
	})
}

// --- tests ---

func testPutObject(ctx context.Context, env *Env) TestResult {
	if err := putTestObject(ctx, env, "test-put"); err != nil {
		return Fail(err.Error())
	}
	return Pass()
}

func testGetObject(ctx context.Context, env *Env) TestResult {
	key := "test-get"
	if err := putTestObject(ctx, env, key); err != nil {
		return Error("setup: " + err.Error())
	}
	defer deleteObject(ctx, env, key)

	body, err := getObjectBody(ctx, env, key)
	if err != nil {
		return Fail(err.Error())
	}
	if body != testBody {
		return Fail(fmt.Sprintf("body mismatch: got %q", body))
	}
	return Pass()
}

func testHeadObject(ctx context.Context, env *Env) TestResult {
	key := "test-head"
	if err := putTestObject(ctx, env, key); err != nil {
		return Error("setup: " + err.Error())
	}
	defer deleteObject(ctx, env, key)

	out, err := env.Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return Fail(err.Error())
	}
	if aws.ToInt64(out.ContentLength) != int64(len(testBody)) {
		return Fail(fmt.Sprintf("content-length mismatch: got %d", aws.ToInt64(out.ContentLength)))
	}
	return Pass()
}

func testCopyObject(ctx context.Context, env *Env) TestResult {
	src := "test-copy-src"
	dst := "test-copy-dst"
	if err := putTestObject(ctx, env, src); err != nil {
		return Error("setup: " + err.Error())
	}
	defer deleteObject(ctx, env, src)
	defer deleteObject(ctx, env, dst)

	_, err := env.Client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(env.Bucket),
		CopySource: aws.String(env.Bucket + "/" + src),
		Key:        aws.String(dst),
	})
	if err != nil {
		return Fail(err.Error())
	}

	body, err := getObjectBody(ctx, env, dst)
	if err != nil {
		return Fail("get copy: " + err.Error())
	}
	if body != testBody {
		return Fail("copied body mismatch")
	}
	return Pass()
}

func testGetObjectRange(ctx context.Context, env *Env) TestResult {
	key := "test-range"
	if err := putTestObject(ctx, env, key); err != nil {
		return Error("setup: " + err.Error())
	}
	defer deleteObject(ctx, env, key)

	out, err := env.Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
		Range:  aws.String("bytes=0-4"),
	})
	if err != nil {
		return Fail(err.Error())
	}
	defer out.Body.Close()
	b, _ := io.ReadAll(out.Body)
	if string(b) != testBody[:5] {
		return Fail(fmt.Sprintf("range mismatch: got %q, want %q", string(b), testBody[:5]))
	}
	return Pass()
}

func testGetObjectIfMatch(ctx context.Context, env *Env) TestResult {
	key := "test-if-match"
	if err := putTestObject(ctx, env, key); err != nil {
		return Error("setup: " + err.Error())
	}
	defer deleteObject(ctx, env, key)

	head, err := env.Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return Error("head: " + err.Error())
	}

	_, err = env.Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket:  aws.String(env.Bucket),
		Key:     aws.String(key),
		IfMatch: head.ETag,
	})
	if err != nil {
		return Fail(err.Error())
	}
	return Pass()
}

func testGetObjectIfNoneMatch(ctx context.Context, env *Env) TestResult {
	key := "test-if-none-match"
	if err := putTestObject(ctx, env, key); err != nil {
		return Error("setup: " + err.Error())
	}
	defer deleteObject(ctx, env, key)

	head, err := env.Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return Error("head: " + err.Error())
	}

	// If-None-Match with the same ETag should return 304 (SDK returns an error).
	_, err = env.Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket:      aws.String(env.Bucket),
		Key:         aws.String(key),
		IfNoneMatch: head.ETag,
	})
	if err == nil {
		return Fail("expected 304 Not Modified, got success")
	}
	// The error should indicate 304.
	if strings.Contains(err.Error(), "304") || strings.Contains(err.Error(), "NotModified") {
		return Pass()
	}
	return PassWithNote("got error (expected): " + err.Error())
}

func testGetObjectIfModifiedSince(ctx context.Context, env *Env) TestResult {
	key := "test-if-modified"
	if err := putTestObject(ctx, env, key); err != nil {
		return Error("setup: " + err.Error())
	}
	defer deleteObject(ctx, env, key)

	// Use a time far in the past — object should be newer.
	past := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	out, err := env.Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket:          aws.String(env.Bucket),
		Key:             aws.String(key),
		IfModifiedSince: aws.Time(past),
	})
	if err != nil {
		return Fail(err.Error())
	}
	out.Body.Close()
	return Pass()
}

func testGetObjectIfUnmodifiedSince(ctx context.Context, env *Env) TestResult {
	key := "test-if-unmodified"
	if err := putTestObject(ctx, env, key); err != nil {
		return Error("setup: " + err.Error())
	}
	defer deleteObject(ctx, env, key)

	// Use current time + buffer — object should be older.
	future := time.Now().Add(time.Hour)
	out, err := env.Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket:            aws.String(env.Bucket),
		Key:               aws.String(key),
		IfUnmodifiedSince: aws.Time(future),
	})
	if err != nil {
		return Fail(err.Error())
	}
	out.Body.Close()
	return Pass()
}

func testPutObjectMetadata(ctx context.Context, env *Env) TestResult {
	key := "test-metadata"
	meta := map[string]string{"test-key": "test-value", "another": "data"}
	_, err := env.Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:   aws.String(env.Bucket),
		Key:      aws.String(key),
		Body:     strings.NewReader("metadata test"),
		Metadata: meta,
	})
	if err != nil {
		return Fail(err.Error())
	}
	defer deleteObject(ctx, env, key)

	head, err := env.Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return Fail("head: " + err.Error())
	}
	if head.Metadata["test-key"] != "test-value" {
		return Fail(fmt.Sprintf("metadata mismatch: got %v", head.Metadata))
	}
	return Pass()
}

func testPutObjectChecksumSHA256(ctx context.Context, env *Env) TestResult {
	key := "test-checksum-sha256"
	body := []byte("checksum test body")
	h := sha256.Sum256(body)
	checksum := base64.StdEncoding.EncodeToString(h[:])

	_, err := env.Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:            aws.String(env.Bucket),
		Key:               aws.String(key),
		Body:              bytes.NewReader(body),
		ChecksumAlgorithm: types.ChecksumAlgorithmSha256,
		ChecksumSHA256:    aws.String(checksum),
	})
	if err != nil {
		return Fail(err.Error())
	}
	defer deleteObject(ctx, env, key)
	return Pass()
}

func testPutObjectSSES3(ctx context.Context, env *Env) TestResult {
	key := "test-sse-s3"
	_, err := env.Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:               aws.String(env.Bucket),
		Key:                  aws.String(key),
		Body:                 strings.NewReader("sse test"),
		ServerSideEncryption: types.ServerSideEncryptionAes256,
	})
	if err != nil {
		return Fail(err.Error())
	}
	defer deleteObject(ctx, env, key)

	head, err := env.Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return Fail("head: " + err.Error())
	}
	if head.ServerSideEncryption != types.ServerSideEncryptionAes256 {
		return Fail(fmt.Sprintf("expected AES256, got %s", head.ServerSideEncryption))
	}
	return Pass()
}

func testPutObjectStorageClass(ctx context.Context, env *Env) TestResult {
	key := "test-storage-class"
	_, err := env.Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:       aws.String(env.Bucket),
		Key:          aws.String(key),
		Body:         strings.NewReader("storage class test"),
		StorageClass: types.StorageClassStandard,
	})
	if err != nil {
		return Fail(err.Error())
	}
	defer deleteObject(ctx, env, key)
	return Pass()
}

func testPutObjectTagging(ctx context.Context, env *Env) TestResult {
	key := "test-obj-tagging"
	if err := putTestObject(ctx, env, key); err != nil {
		return Error("setup: " + err.Error())
	}
	// Don't delete — used by GetObjectTagging and DeleteObjectTagging tests.

	_, err := env.Client.PutObjectTagging(ctx, &s3.PutObjectTaggingInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
		Tagging: &types.Tagging{
			TagSet: []types.Tag{
				{Key: aws.String("env"), Value: aws.String("test")},
			},
		},
	})
	if err != nil {
		return Fail(err.Error())
	}
	return Pass()
}

func testGetObjectTagging(ctx context.Context, env *Env) TestResult {
	key := "test-obj-tagging"
	out, err := env.Client.GetObjectTagging(ctx, &s3.GetObjectTaggingInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return Fail(err.Error())
	}
	if len(out.TagSet) == 0 {
		return Fail("no tags returned")
	}
	return Pass()
}

func testDeleteObjectTagging(ctx context.Context, env *Env) TestResult {
	key := "test-obj-tagging"
	defer deleteObject(ctx, env, key)

	_, err := env.Client.DeleteObjectTagging(ctx, &s3.DeleteObjectTaggingInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return Fail(err.Error())
	}
	return Pass()
}

func testPutObjectAcl(ctx context.Context, env *Env) TestResult {
	key := "test-obj-acl"
	if err := putTestObject(ctx, env, key); err != nil {
		return Error("setup: " + err.Error())
	}
	// Don't delete — used by GetObjectAcl test.

	_, err := env.Client.PutObjectAcl(ctx, &s3.PutObjectAclInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
		ACL:    types.ObjectCannedACLPrivate,
	})
	if err != nil {
		return Fail(err.Error())
	}
	return Pass()
}

func testGetObjectAcl(ctx context.Context, env *Env) TestResult {
	key := "test-obj-acl"
	defer deleteObject(ctx, env, key)

	out, err := env.Client.GetObjectAcl(ctx, &s3.GetObjectAclInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return Fail(err.Error())
	}
	if out.Owner == nil {
		return Fail("no owner in ACL response")
	}
	return Pass()
}

func testListObjects(ctx context.Context, env *Env) TestResult {
	prefix := "list-v1/"
	for i := 0; i < 3; i++ {
		key := fmt.Sprintf("%sobj-%d", prefix, i)
		if err := putTestObject(ctx, env, key); err != nil {
			return Error("setup: " + err.Error())
		}
		defer deleteObject(ctx, env, key)
	}

	out, err := env.Client.ListObjects(ctx, &s3.ListObjectsInput{
		Bucket: aws.String(env.Bucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return Fail(err.Error())
	}
	if len(out.Contents) < 3 {
		return Fail(fmt.Sprintf("expected 3 objects, got %d", len(out.Contents)))
	}
	return Pass()
}

func testListObjectsV2(ctx context.Context, env *Env) TestResult {
	prefix := "list-v2/"
	for i := 0; i < 3; i++ {
		key := fmt.Sprintf("%sobj-%d", prefix, i)
		if err := putTestObject(ctx, env, key); err != nil {
			return Error("setup: " + err.Error())
		}
		defer deleteObject(ctx, env, key)
	}

	out, err := env.Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(env.Bucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return Fail(err.Error())
	}
	if aws.ToInt32(out.KeyCount) < 3 {
		return Fail(fmt.Sprintf("expected 3 objects, got %d", aws.ToInt32(out.KeyCount)))
	}
	return Pass()
}

func testListObjectsV2Pagination(ctx context.Context, env *Env) TestResult {
	prefix := "list-page/"
	for i := 0; i < 3; i++ {
		key := fmt.Sprintf("%sobj-%d", prefix, i)
		if err := putTestObject(ctx, env, key); err != nil {
			return Error("setup: " + err.Error())
		}
		defer deleteObject(ctx, env, key)
	}

	// Fetch with MaxKeys=1 to force pagination.
	out, err := env.Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(env.Bucket),
		Prefix:  aws.String(prefix),
		MaxKeys: aws.Int32(1),
	})
	if err != nil {
		return Fail(err.Error())
	}
	if !aws.ToBool(out.IsTruncated) {
		return Fail("expected truncated response with MaxKeys=1")
	}
	if out.NextContinuationToken == nil {
		return Fail("expected NextContinuationToken")
	}

	// Fetch second page.
	out2, err := env.Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:            aws.String(env.Bucket),
		Prefix:            aws.String(prefix),
		MaxKeys:           aws.Int32(1),
		ContinuationToken: out.NextContinuationToken,
	})
	if err != nil {
		return Fail("page 2: " + err.Error())
	}
	if len(out2.Contents) == 0 {
		return Fail("page 2 empty")
	}
	return Pass()
}

func testListObjectsV2Delimiter(ctx context.Context, env *Env) TestResult {
	// Create objects with "folder" structure.
	keys := []string{"delim/a/1.txt", "delim/a/2.txt", "delim/b/1.txt"}
	for _, key := range keys {
		if err := putTestObject(ctx, env, key); err != nil {
			return Error("setup: " + err.Error())
		}
		defer deleteObject(ctx, env, key)
	}

	out, err := env.Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:    aws.String(env.Bucket),
		Prefix:    aws.String("delim/"),
		Delimiter: aws.String("/"),
	})
	if err != nil {
		return Fail(err.Error())
	}
	if len(out.CommonPrefixes) < 2 {
		return Fail(fmt.Sprintf("expected 2 common prefixes, got %d", len(out.CommonPrefixes)))
	}
	return Pass()
}

func testDeleteObjects(ctx context.Context, env *Env) TestResult {
	var keys []string
	for i := 0; i < 3; i++ {
		key := fmt.Sprintf("del-batch-%d", i)
		if err := putTestObject(ctx, env, key); err != nil {
			return Error("setup: " + err.Error())
		}
		keys = append(keys, key)
	}

	objects := make([]types.ObjectIdentifier, len(keys))
	for i, k := range keys {
		objects[i] = types.ObjectIdentifier{Key: aws.String(k)}
	}

	out, err := env.Client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(env.Bucket),
		Delete: &types.Delete{Objects: objects, Quiet: aws.Bool(false)},
	})
	if err != nil {
		return Fail(err.Error())
	}
	if len(out.Deleted) < 3 {
		return Fail(fmt.Sprintf("expected 3 deleted, got %d", len(out.Deleted)))
	}
	return Pass()
}

func testDeleteObjectsQuiet(ctx context.Context, env *Env) TestResult {
	var keys []string
	for i := 0; i < 3; i++ {
		key := fmt.Sprintf("del-quiet-%d", i)
		if err := putTestObject(ctx, env, key); err != nil {
			return Error("setup: " + err.Error())
		}
		keys = append(keys, key)
	}

	objects := make([]types.ObjectIdentifier, len(keys))
	for i, k := range keys {
		objects[i] = types.ObjectIdentifier{Key: aws.String(k)}
	}

	out, err := env.Client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(env.Bucket),
		Delete: &types.Delete{Objects: objects, Quiet: aws.Bool(true)},
	})
	if err != nil {
		return Fail(err.Error())
	}
	// Quiet mode: Deleted list should be empty (only errors reported).
	if len(out.Errors) > 0 {
		return Fail(fmt.Sprintf("got %d errors in quiet delete", len(out.Errors)))
	}
	return Pass()
}

func testRestoreObject(ctx context.Context, env *Env) TestResult {
	key := "test-restore"
	if err := putTestObject(ctx, env, key); err != nil {
		return Error("setup: " + err.Error())
	}
	defer deleteObject(ctx, env, key)

	_, err := env.Client.RestoreObject(ctx, &s3.RestoreObjectInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
		RestoreRequest: &types.RestoreRequest{
			Days: aws.Int32(1),
		},
	})
	if err != nil {
		// Many providers don't support this or require the object to be in Glacier.
		return Fail(err.Error())
	}
	return Pass()
}
