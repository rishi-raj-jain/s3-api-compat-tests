package tests

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
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
		{"GetObject_NoSuchKey", "object", testGetObjectNoSuchKey},
		{"HeadObject", "object", testHeadObject},
		{"DeleteObject", "object", testDeleteObject},
		{"CopyObject", "object", testCopyObject},
		{"CopyObject_MetadataDirective", "object", testCopyObjectMetadataDirective},
		{"CopyObject_IfMatch", "object", testCopyObjectIfMatch},
		{"CopyObject_MultipartSource", "object", testCopyObjectMultipartSource},
		{"RenameObject", "object", testRenameObject},
		{"RenameObject_IfNoneMatch", "object", testRenameObjectIfNoneMatch},
		{"RenameObject_SourceIsLiteralKey", "object", testRenameObjectSourceIsLiteralKey},
		{"GetObject_Range", "object", testGetObjectRange},
		{"GetObject_IfMatch", "object", testGetObjectIfMatch},
		{"GetObject_IfNoneMatch", "object", testGetObjectIfNoneMatch},
		{"GetObject_IfModifiedSince", "object", testGetObjectIfModifiedSince},
		{"GetObject_IfUnmodifiedSince", "object", testGetObjectIfUnmodifiedSince},
		{"PutObject_IfNoneMatch", "object", testPutObjectIfNoneMatch},
		{"PutObject_IfMatch", "object", testPutObjectIfMatch},
		{"PutObject_Metadata", "object", testPutObjectMetadata},
		{"PutObject_ChecksumSHA256", "object", testPutObjectChecksumSHA256},
		{"PutObject_BadChecksumSHA256", "object", testPutObjectBadChecksumSHA256},
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
		{"ListObjectsV2_StartAfter", "object", testListObjectsV2StartAfter},
		{"DeleteObjects", "object", testDeleteObjects},
		{"DeleteObjects_Quiet", "object", testDeleteObjectsQuiet},
		{"RestoreObject", "object", testRestoreObject},
	}
}

// --- helpers ---

const testBody = "hello s3 compat test"

func md5ETag(body string) string {
	sum := md5.Sum([]byte(body))
	return `"` + hex.EncodeToString(sum[:]) + `"`
}

func putTestObject(ctx context.Context, env *Env, key string) error {
	_, err := env.Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
		Body:   strings.NewReader(testBody),
	})
	return err
}

func putBody(ctx context.Context, env *Env, bucket, key, body string) (*s3.PutObjectOutput, error) {
	return env.Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   strings.NewReader(body),
	})
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

// bodyMD5 returns the hex MD5 of the stored object, read in full.
func bodyMD5(ctx context.Context, env *Env, key string) (string, error) {
	out, err := env.Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return "", err
	}
	defer out.Body.Close()
	h := md5.New()
	if _, err := io.Copy(h, out.Body); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func repeatMD5(fill byte, n int) string {
	h := md5.New()
	chunk := bytes.Repeat([]byte{fill}, 64*1024)
	for n > 0 {
		k := len(chunk)
		if n < k {
			k = n
		}
		_, _ = h.Write(chunk[:k])
		n -= k
	}
	return hex.EncodeToString(h.Sum(nil))
}

func headObject(ctx context.Context, env *Env, key string) (*s3.HeadObjectOutput, error) {
	return env.Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
	})
}

func deleteObject(ctx context.Context, env *Env, key string) {
	_, _ = env.Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
	})
}

func getTags(ctx context.Context, env *Env, key string) (map[string]string, error) {
	out, err := env.Client.GetObjectTagging(ctx, &s3.GetObjectTaggingInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	tags := map[string]string{}
	for _, t := range out.TagSet {
		tags[aws.ToString(t.Key)] = aws.ToString(t.Value)
	}
	return tags, nil
}

// --- tests ---

// PutObject stores the body; the response ETag is the quoted MD5 of the body,
// and HeadObject reports the body length.
func testPutObject(ctx context.Context, env *Env) TestResult {
	key := "test-put"
	out, err := putBody(ctx, env, env.Bucket, key, testBody)
	if err != nil {
		return Fail(err.Error())
	}
	defer deleteObject(ctx, env, key)

	if aws.ToString(out.ETag) != md5ETag(testBody) {
		return Fail(fmt.Sprintf("ETag mismatch: got %s, want %s", aws.ToString(out.ETag), md5ETag(testBody)))
	}
	head, err := headObject(ctx, env, key)
	if err != nil {
		return Fail("head: " + err.Error())
	}
	if aws.ToInt64(head.ContentLength) != int64(len(testBody)) {
		return Fail(fmt.Sprintf("content-length mismatch: got %d", aws.ToInt64(head.ContentLength)))
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

// A read of a missing key is 404 NoSuchKey.
func testGetObjectNoSuchKey(ctx context.Context, env *Env) TestResult {
	_, err := env.Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String("test-does-not-exist"),
	})
	return expectFailure(err, "GetObject of a missing key", 404, "NoSuchKey")
}

func testHeadObject(ctx context.Context, env *Env) TestResult {
	key := "test-head"
	if err := putTestObject(ctx, env, key); err != nil {
		return Error("setup: " + err.Error())
	}
	defer deleteObject(ctx, env, key)

	out, err := headObject(ctx, env, key)
	if err != nil {
		return Fail(err.Error())
	}
	if aws.ToInt64(out.ContentLength) != int64(len(testBody)) {
		return Fail(fmt.Sprintf("content-length mismatch: got %d", aws.ToInt64(out.ContentLength)))
	}
	if aws.ToString(out.ETag) != md5ETag(testBody) {
		return Fail(fmt.Sprintf("ETag mismatch: got %s", aws.ToString(out.ETag)))
	}
	return Pass()
}

// DeleteObject removes the key; a later HeadObject is 404. Deleting a key that
// does not exist succeeds, as S3 specifies.
func testDeleteObject(ctx context.Context, env *Env) TestResult {
	key := "test-delete"
	if err := putTestObject(ctx, env, key); err != nil {
		return Error("setup: " + err.Error())
	}

	if _, err := env.Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
	}); err != nil {
		return Fail(err.Error())
	}
	_, err := headObject(ctx, env, key)
	if r := expectFailure(err, "HeadObject after delete", 404, "NotFound", "NoSuchKey"); r.Status != StatusPass {
		return r
	}
	if _, err := env.Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
	}); err != nil {
		return Fail("delete of a missing key must succeed: " + err.Error())
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

	out, err := env.Client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(env.Bucket),
		CopySource: aws.String(env.Bucket + "/" + src),
		Key:        aws.String(dst),
	})
	if err != nil {
		return Fail(err.Error())
	}
	if out.CopyObjectResult == nil || aws.ToString(out.CopyObjectResult.ETag) == "" {
		return Fail("no ETag in CopyObjectResult")
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

// MetadataDirective=REPLACE gives the copy the new metadata and drops the old.
func testCopyObjectMetadataDirective(ctx context.Context, env *Env) TestResult {
	src := "test-copy-md-src"
	dst := "test-copy-md-dst"
	_, err := env.Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:   aws.String(env.Bucket),
		Key:      aws.String(src),
		Body:     strings.NewReader(testBody),
		Metadata: map[string]string{"old": "1"},
	})
	if err != nil {
		return Error("setup: " + err.Error())
	}
	defer deleteObject(ctx, env, src)
	defer deleteObject(ctx, env, dst)

	_, err = env.Client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:            aws.String(env.Bucket),
		CopySource:        aws.String(env.Bucket + "/" + src),
		Key:               aws.String(dst),
		MetadataDirective: types.MetadataDirectiveReplace,
		Metadata:          map[string]string{"new": "2"},
	})
	if err != nil {
		return Fail(err.Error())
	}
	head, err := headObject(ctx, env, dst)
	if err != nil {
		return Fail("head: " + err.Error())
	}
	if head.Metadata["new"] != "2" {
		return Fail(fmt.Sprintf("new metadata missing on copy: %v", head.Metadata))
	}
	if _, ok := head.Metadata["old"]; ok {
		return Fail("REPLACE kept the source metadata")
	}
	return Pass()
}

// x-amz-copy-source-if-match: a matching ETag copies, a wrong one is 412.
func testCopyObjectIfMatch(ctx context.Context, env *Env) TestResult {
	src := "test-copy-cond-src"
	dst := "test-copy-cond-dst"
	if err := putTestObject(ctx, env, src); err != nil {
		return Error("setup: " + err.Error())
	}
	defer deleteObject(ctx, env, src)
	defer deleteObject(ctx, env, dst)

	_, err := env.Client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:            aws.String(env.Bucket),
		CopySource:        aws.String(env.Bucket + "/" + src),
		Key:               aws.String(dst),
		CopySourceIfMatch: aws.String(md5ETag(testBody)),
	})
	if err != nil {
		return Fail("matching if-match: " + err.Error())
	}
	body, err := getObjectBody(ctx, env, dst)
	if err != nil || body != testBody {
		return Fail(fmt.Sprintf("copy with a matching condition did not store the body: %q %v", body, err))
	}
	_, err = env.Client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:            aws.String(env.Bucket),
		CopySource:        aws.String(env.Bucket + "/" + src),
		Key:               aws.String(dst),
		CopySourceIfMatch: aws.String(`"00000000000000000000000000000000"`),
	})
	return expectFailure(err, "copy with a wrong x-amz-copy-source-if-match", 412, "PreconditionFailed")
}

// A conditional copy of a multipart object: the source ETag is the composite
// "<md5>-<parts>" form, and x-amz-copy-source-if-match must accept it.
func testCopyObjectMultipartSource(ctx context.Context, env *Env) TestResult {
	src := "test-copy-mpu-src"
	dst := "test-copy-mpu-dst"
	etag, err := completeTwoPartUpload(ctx, env, src)
	if err != nil {
		return Error("setup: " + err.Error())
	}
	defer deleteObject(ctx, env, src)
	defer deleteObject(ctx, env, dst)

	if !strings.Contains(etag, "-") {
		return Fail(fmt.Sprintf("multipart ETag is not composite: %s", etag))
	}
	_, err = env.Client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:            aws.String(env.Bucket),
		CopySource:        aws.String(env.Bucket + "/" + src),
		Key:               aws.String(dst),
		CopySourceIfMatch: aws.String(etag),
	})
	if err != nil {
		return Fail(err.Error())
	}
	got, err := bodyMD5(ctx, env, dst)
	if err != nil {
		return Fail("read copy: " + err.Error())
	}
	if got != twoPartMD5() {
		return Fail("copied bytes differ from the source")
	}
	return Pass()
}

// RenameObject moves the key: the source is gone and the destination has the
// body. The SDK sends a client token with the request. x-amz-rename-source is
// the source key alone, URL-encoded.
func testRenameObject(ctx context.Context, env *Env) TestResult {
	src := "test-rename-src"
	dst := "test-rename-dst"
	if err := putTestObject(ctx, env, src); err != nil {
		return Error("setup: " + err.Error())
	}
	defer deleteObject(ctx, env, src)
	defer deleteObject(ctx, env, dst)

	_, err := env.Client.RenameObject(ctx, &s3.RenameObjectInput{
		Bucket:       aws.String(env.Bucket),
		Key:          aws.String(dst),
		RenameSource: aws.String(src),
	})
	if err != nil {
		return Fail(err.Error())
	}
	_, err = headObject(ctx, env, src)
	if r := expectFailure(err, "HeadObject of the rename source", 404, "NotFound", "NoSuchKey"); r.Status != StatusPass {
		return r
	}
	body, err := getObjectBody(ctx, env, dst)
	if err != nil {
		return Fail("get destination: " + err.Error())
	}
	if body != testBody {
		return Fail("renamed body mismatch")
	}
	return Pass()
}

// RenameObject with If-None-Match: * refuses to replace an existing destination.
func testRenameObjectIfNoneMatch(ctx context.Context, env *Env) TestResult {
	src := "test-rename-cond-src"
	dst := "test-rename-cond-dst"
	if err := putTestObject(ctx, env, src); err != nil {
		return Error("setup: " + err.Error())
	}
	if _, err := putBody(ctx, env, env.Bucket, dst, "existing"); err != nil {
		return Error("setup: " + err.Error())
	}
	defer deleteObject(ctx, env, src)
	defer deleteObject(ctx, env, dst)

	_, err := env.Client.RenameObject(ctx, &s3.RenameObjectInput{
		Bucket:                 aws.String(env.Bucket),
		Key:                    aws.String(dst),
		RenameSource:           aws.String(src),
		DestinationIfNoneMatch: aws.String("*"),
	})
	if r := expectFailure(err, "rename onto an existing key with If-None-Match: *", 412, "PreconditionFailed"); r.Status != StatusPass {
		return r
	}
	body, err := getObjectBody(ctx, env, dst)
	if err != nil || body != "existing" {
		return Fail("destination changed although the condition failed")
	}
	if _, err := headObject(ctx, env, src); err != nil {
		return Fail("source removed although the condition failed: " + err.Error())
	}
	return Pass()
}

// The decoded x-amz-rename-source is the source key, literally: a leading slash
// or the bucket name in front of it names a different key, which does not exist.
func testRenameObjectSourceIsLiteralKey(ctx context.Context, env *Env) TestResult {
	src := "test-rename-literal-src"
	dst := "test-rename-literal-dst"
	if err := putTestObject(ctx, env, src); err != nil {
		return Error("setup: " + err.Error())
	}
	defer deleteObject(ctx, env, src)
	defer deleteObject(ctx, env, dst)

	for _, source := range []string{"/" + src, env.Bucket + "/" + src, "/" + env.Bucket + "/" + src} {
		_, err := env.Client.RenameObject(ctx, &s3.RenameObjectInput{
			Bucket:       aws.String(env.Bucket),
			Key:          aws.String(dst),
			RenameSource: aws.String(source),
		})
		if r := expectFailure(err, "rename from "+source, 404, "NoSuchKey"); r.Status != StatusPass {
			return r
		}
	}
	if _, err := headObject(ctx, env, src); err != nil {
		return Fail("source removed by a rename that named another key: " + err.Error())
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
	if cr := aws.ToString(out.ContentRange); !strings.HasPrefix(cr, "bytes 0-4/") {
		return Fail(fmt.Sprintf("Content-Range mismatch: got %q", cr))
	}
	return Pass()
}

// If-Match: the right ETag reads, a wrong ETag is 412.
func testGetObjectIfMatch(ctx context.Context, env *Env) TestResult {
	key := "test-if-match"
	if err := putTestObject(ctx, env, key); err != nil {
		return Error("setup: " + err.Error())
	}
	defer deleteObject(ctx, env, key)

	out, err := env.Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket:  aws.String(env.Bucket),
		Key:     aws.String(key),
		IfMatch: aws.String(md5ETag(testBody)),
	})
	if err != nil {
		return Fail("matching If-Match: " + err.Error())
	}
	out.Body.Close()

	_, err = env.Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket:  aws.String(env.Bucket),
		Key:     aws.String(key),
		IfMatch: aws.String(`"00000000000000000000000000000000"`),
	})
	return expectFailure(err, "GET with a wrong If-Match", 412, "PreconditionFailed")
}

// If-None-Match: the current ETag is 304, another ETag reads.
func testGetObjectIfNoneMatch(ctx context.Context, env *Env) TestResult {
	key := "test-if-none-match"
	if err := putTestObject(ctx, env, key); err != nil {
		return Error("setup: " + err.Error())
	}
	defer deleteObject(ctx, env, key)

	_, err := env.Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket:      aws.String(env.Bucket),
		Key:         aws.String(key),
		IfNoneMatch: aws.String(md5ETag(testBody)),
	})
	if r := expectFailure(err, "GET with the current ETag in If-None-Match", 304, "NotModified"); r.Status != StatusPass {
		return r
	}

	out, err := env.Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket:      aws.String(env.Bucket),
		Key:         aws.String(key),
		IfNoneMatch: aws.String(`"00000000000000000000000000000000"`),
	})
	if err != nil {
		return Fail("non-matching If-None-Match must read: " + err.Error())
	}
	out.Body.Close()
	return Pass()
}

// If-Modified-Since: a past date reads, a future date is 304.
func testGetObjectIfModifiedSince(ctx context.Context, env *Env) TestResult {
	key := "test-if-modified"
	if err := putTestObject(ctx, env, key); err != nil {
		return Error("setup: " + err.Error())
	}
	defer deleteObject(ctx, env, key)

	past := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	out, err := env.Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket:          aws.String(env.Bucket),
		Key:             aws.String(key),
		IfModifiedSince: aws.Time(past),
	})
	if err != nil {
		return Fail("past If-Modified-Since must read: " + err.Error())
	}
	out.Body.Close()

	future := time.Now().Add(time.Hour)
	_, err = env.Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket:          aws.String(env.Bucket),
		Key:             aws.String(key),
		IfModifiedSince: aws.Time(future),
	})
	return expectFailure(err, "GET with a future If-Modified-Since", 304, "NotModified")
}

// If-Unmodified-Since: a future date reads, a past date is 412.
func testGetObjectIfUnmodifiedSince(ctx context.Context, env *Env) TestResult {
	key := "test-if-unmodified"
	if err := putTestObject(ctx, env, key); err != nil {
		return Error("setup: " + err.Error())
	}
	defer deleteObject(ctx, env, key)

	future := time.Now().Add(time.Hour)
	out, err := env.Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket:            aws.String(env.Bucket),
		Key:               aws.String(key),
		IfUnmodifiedSince: aws.Time(future),
	})
	if err != nil {
		return Fail("future If-Unmodified-Since must read: " + err.Error())
	}
	out.Body.Close()

	past := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err = env.Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket:            aws.String(env.Bucket),
		Key:               aws.String(key),
		IfUnmodifiedSince: aws.Time(past),
	})
	return expectFailure(err, "GET with a past If-Unmodified-Since", 412, "PreconditionFailed")
}

// Conditional write: If-None-Match: * creates the key once and refuses the
// second write with 412.
func testPutObjectIfNoneMatch(ctx context.Context, env *Env) TestResult {
	key := "test-put-if-none-match"
	defer deleteObject(ctx, env, key)

	_, err := env.Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(env.Bucket),
		Key:         aws.String(key),
		Body:        strings.NewReader("first"),
		IfNoneMatch: aws.String("*"),
	})
	if err != nil {
		return Fail("create with If-None-Match: *: " + err.Error())
	}
	_, err = env.Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(env.Bucket),
		Key:         aws.String(key),
		Body:        strings.NewReader("second"),
		IfNoneMatch: aws.String("*"),
	})
	if r := expectFailure(err, "second PUT with If-None-Match: *", 412, "PreconditionFailed"); r.Status != StatusPass {
		return r
	}
	body, err := getObjectBody(ctx, env, key)
	if err != nil || body != "first" {
		return Fail("object changed although the condition failed")
	}
	return Pass()
}

// Conditional write: If-Match with the current ETag writes, a wrong ETag is 412.
func testPutObjectIfMatch(ctx context.Context, env *Env) TestResult {
	key := "test-put-if-match"
	if _, err := putBody(ctx, env, env.Bucket, key, "v1"); err != nil {
		return Error("setup: " + err.Error())
	}
	defer deleteObject(ctx, env, key)

	_, err := env.Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:  aws.String(env.Bucket),
		Key:     aws.String(key),
		Body:    strings.NewReader("v2"),
		IfMatch: aws.String(md5ETag("v1")),
	})
	if err != nil {
		return Fail("PUT with the current ETag in If-Match: " + err.Error())
	}
	_, err = env.Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:  aws.String(env.Bucket),
		Key:     aws.String(key),
		Body:    strings.NewReader("v3"),
		IfMatch: aws.String(md5ETag("v1")),
	})
	if r := expectFailure(err, "PUT with a stale If-Match", 412, "PreconditionFailed"); r.Status != StatusPass {
		return r
	}
	body, err := getObjectBody(ctx, env, key)
	if err != nil || body != "v2" {
		return Fail("object changed although the condition failed")
	}
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

	head, err := headObject(ctx, env, key)
	if err != nil {
		return Fail("head: " + err.Error())
	}
	if head.Metadata["test-key"] != "test-value" || head.Metadata["another"] != "data" {
		return Fail(fmt.Sprintf("metadata mismatch: got %v", head.Metadata))
	}
	return Pass()
}

// A correct SHA-256 checksum is accepted and echoed in the response.
func testPutObjectChecksumSHA256(ctx context.Context, env *Env) TestResult {
	key := "test-checksum-sha256"
	body := []byte("checksum test body")
	h := sha256.Sum256(body)
	checksum := base64.StdEncoding.EncodeToString(h[:])

	out, err := env.Client.PutObject(ctx, &s3.PutObjectInput{
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
	if aws.ToString(out.ChecksumSHA256) != checksum {
		return Fail(fmt.Sprintf("checksum not echoed: got %q, want %q", aws.ToString(out.ChecksumSHA256), checksum))
	}
	return Pass()
}

// A wrong SHA-256 checksum must be rejected, not stored.
func testPutObjectBadChecksumSHA256(ctx context.Context, env *Env) TestResult {
	key := "test-bad-checksum-sha256"
	wrong := sha256.Sum256([]byte("a different body"))
	defer deleteObject(ctx, env, key)

	_, err := env.Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:            aws.String(env.Bucket),
		Key:               aws.String(key),
		Body:              strings.NewReader("checksum test body"),
		ChecksumAlgorithm: types.ChecksumAlgorithmSha256,
		ChecksumSHA256:    aws.String(base64.StdEncoding.EncodeToString(wrong[:])),
	})
	if r := expectFailure(err, "PUT with a wrong x-amz-checksum-sha256", 400, "BadDigest", "InvalidRequest", "XAmzContentSHA256Mismatch"); r.Status != StatusPass {
		return r
	}
	_, err = headObject(ctx, env, key)
	return expectFailure(err, "HeadObject after a rejected PUT", 404, "NotFound", "NoSuchKey")
}

// The SSE-S3 header is honoured when HeadObject reports it back.
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

	head, err := headObject(ctx, env, key)
	if err != nil {
		return Fail("head: " + err.Error())
	}
	if head.ServerSideEncryption != types.ServerSideEncryptionAes256 {
		return Fail(fmt.Sprintf("expected AES256, got %s", head.ServerSideEncryption))
	}
	return Pass()
}

// A non-default storage class is stored and reported by HeadObject. STANDARD is
// not used because S3 omits the header for it, which would hide a provider that
// ignores the class.
func testPutObjectStorageClass(ctx context.Context, env *Env) TestResult {
	key := "test-storage-class"
	_, err := env.Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:       aws.String(env.Bucket),
		Key:          aws.String(key),
		Body:         strings.NewReader("storage class test"),
		StorageClass: types.StorageClassStandardIa,
	})
	if err != nil {
		return Fail(err.Error())
	}
	defer deleteObject(ctx, env, key)

	head, err := headObject(ctx, env, key)
	if err != nil {
		return Fail("head: " + err.Error())
	}
	if head.StorageClass != types.StorageClassStandardIa {
		return Fail(fmt.Sprintf("expected STANDARD_IA, got %q", head.StorageClass))
	}
	return Pass()
}

func putTag(ctx context.Context, env *Env, key string) error {
	_, err := env.Client.PutObjectTagging(ctx, &s3.PutObjectTaggingInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
		Tagging: &types.Tagging{
			TagSet: []types.Tag{
				{Key: aws.String("env"), Value: aws.String("test")},
			},
		},
	})
	return err
}

func testPutObjectTagging(ctx context.Context, env *Env) TestResult {
	key := "test-obj-tagging-put"
	if err := putTestObject(ctx, env, key); err != nil {
		return Error("setup: " + err.Error())
	}
	defer deleteObject(ctx, env, key)

	if err := putTag(ctx, env, key); err != nil {
		return Fail(err.Error())
	}
	tags, err := getTags(ctx, env, key)
	if err != nil {
		return Fail("get tags: " + err.Error())
	}
	if tags["env"] != "test" || len(tags) != 1 {
		return Fail(fmt.Sprintf("tags not stored: got %v", tags))
	}
	return Pass()
}

func testGetObjectTagging(ctx context.Context, env *Env) TestResult {
	key := "test-obj-tagging-get"
	if err := putTestObject(ctx, env, key); err != nil {
		return Error("setup: " + err.Error())
	}
	defer deleteObject(ctx, env, key)
	if err := putTag(ctx, env, key); err != nil {
		return Error("setup tags: " + err.Error())
	}
	tags, err := getTags(ctx, env, key)
	if err != nil {
		return Fail(err.Error())
	}
	if tags["env"] != "test" {
		return Fail(fmt.Sprintf("expected env=test, got %v", tags))
	}
	return Pass()
}

func testDeleteObjectTagging(ctx context.Context, env *Env) TestResult {
	key := "test-obj-tagging-delete"
	if err := putTestObject(ctx, env, key); err != nil {
		return Error("setup: " + err.Error())
	}
	defer deleteObject(ctx, env, key)
	if err := putTag(ctx, env, key); err != nil {
		return Error("setup tags: " + err.Error())
	}

	_, err := env.Client.DeleteObjectTagging(ctx, &s3.DeleteObjectTaggingInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return Fail(err.Error())
	}
	tags, err := getTags(ctx, env, key)
	if err != nil {
		return Fail("get tags after delete: " + err.Error())
	}
	if len(tags) != 0 {
		return Fail(fmt.Sprintf("tags remain after delete: %v", tags))
	}
	return Pass()
}

func testPutObjectAcl(ctx context.Context, env *Env) TestResult {
	key := "test-obj-acl-put"
	if err := putTestObject(ctx, env, key); err != nil {
		return Error("setup: " + err.Error())
	}
	defer deleteObject(ctx, env, key)

	_, err := env.Client.PutObjectAcl(ctx, &s3.PutObjectAclInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
		ACL:    types.ObjectCannedACLPrivate,
	})
	if err != nil {
		return Fail(err.Error())
	}
	out, err := env.Client.GetObjectAcl(ctx, &s3.GetObjectAclInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return Fail("get acl: " + err.Error())
	}
	if len(out.Grants) == 0 {
		return Fail("no grants after PutObjectAcl private")
	}
	return Pass()
}

func testGetObjectAcl(ctx context.Context, env *Env) TestResult {
	key := "test-obj-acl-get"
	if err := putTestObject(ctx, env, key); err != nil {
		return Error("setup: " + err.Error())
	}
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
	if len(out.Grants) == 0 {
		return Fail("no grants in ACL response")
	}
	return Pass()
}

func putListFixtures(ctx context.Context, env *Env, prefix string, n int) ([]string, error) {
	var keys []string
	for i := 0; i < n; i++ {
		key := fmt.Sprintf("%sobj-%d", prefix, i)
		if err := putTestObject(ctx, env, key); err != nil {
			return keys, err
		}
		keys = append(keys, key)
	}
	return keys, nil
}

func testListObjects(ctx context.Context, env *Env) TestResult {
	prefix := "list-v1/"
	keys, err := putListFixtures(ctx, env, prefix, 3)
	for _, k := range keys {
		defer deleteObject(ctx, env, k)
	}
	if err != nil {
		return Error("setup: " + err.Error())
	}

	out, err := env.Client.ListObjects(ctx, &s3.ListObjectsInput{
		Bucket: aws.String(env.Bucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return Fail(err.Error())
	}
	if len(out.Contents) != 3 {
		return Fail(fmt.Sprintf("expected 3 objects, got %d", len(out.Contents)))
	}
	for i, c := range out.Contents {
		if aws.ToString(c.Key) != keys[i] {
			return Fail(fmt.Sprintf("keys not in lexical order: %v", out.Contents))
		}
	}
	return Pass()
}

func testListObjectsV2(ctx context.Context, env *Env) TestResult {
	prefix := "list-v2/"
	keys, err := putListFixtures(ctx, env, prefix, 3)
	for _, k := range keys {
		defer deleteObject(ctx, env, k)
	}
	if err != nil {
		return Error("setup: " + err.Error())
	}

	out, err := env.Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(env.Bucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return Fail(err.Error())
	}
	if aws.ToInt32(out.KeyCount) != 3 || len(out.Contents) != 3 {
		return Fail(fmt.Sprintf("expected 3 objects, got KeyCount=%d len=%d", aws.ToInt32(out.KeyCount), len(out.Contents)))
	}
	for _, c := range out.Contents {
		if aws.ToInt64(c.Size) != int64(len(testBody)) || aws.ToString(c.ETag) != md5ETag(testBody) {
			return Fail(fmt.Sprintf("listing entry mismatch: %s size=%d etag=%s", aws.ToString(c.Key), aws.ToInt64(c.Size), aws.ToString(c.ETag)))
		}
	}
	return Pass()
}

func testListObjectsV2Pagination(ctx context.Context, env *Env) TestResult {
	prefix := "list-page/"
	keys, err := putListFixtures(ctx, env, prefix, 3)
	for _, k := range keys {
		defer deleteObject(ctx, env, k)
	}
	if err != nil {
		return Error("setup: " + err.Error())
	}

	var seen []string
	var token *string
	for page := 0; page < 5; page++ {
		out, err := env.Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(env.Bucket),
			Prefix:            aws.String(prefix),
			MaxKeys:           aws.Int32(1),
			ContinuationToken: token,
		})
		if err != nil {
			return Fail(fmt.Sprintf("page %d: %v", page+1, err))
		}
		if len(out.Contents) != 1 {
			return Fail(fmt.Sprintf("page %d: expected 1 key with MaxKeys=1, got %d", page+1, len(out.Contents)))
		}
		seen = append(seen, aws.ToString(out.Contents[0].Key))
		if !aws.ToBool(out.IsTruncated) {
			break
		}
		if out.NextContinuationToken == nil {
			return Fail("truncated page without NextContinuationToken")
		}
		token = out.NextContinuationToken
	}
	if strings.Join(seen, ",") != strings.Join(keys, ",") {
		return Fail(fmt.Sprintf("pagination returned %v, want %v", seen, keys))
	}
	return Pass()
}

func testListObjectsV2Delimiter(ctx context.Context, env *Env) TestResult {
	keys := []string{"delim/a/1.txt", "delim/a/2.txt", "delim/b/1.txt", "delim/top.txt"}
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
	var prefixes []string
	for _, p := range out.CommonPrefixes {
		prefixes = append(prefixes, aws.ToString(p.Prefix))
	}
	if strings.Join(prefixes, ",") != "delim/a/,delim/b/" {
		return Fail(fmt.Sprintf("expected common prefixes [delim/a/ delim/b/], got %v", prefixes))
	}
	if len(out.Contents) != 1 || aws.ToString(out.Contents[0].Key) != "delim/top.txt" {
		return Fail(fmt.Sprintf("expected only delim/top.txt in Contents, got %v", out.Contents))
	}
	return Pass()
}

// start-after resumes a listing after the given key.
func testListObjectsV2StartAfter(ctx context.Context, env *Env) TestResult {
	prefix := "list-start/"
	keys, err := putListFixtures(ctx, env, prefix, 3)
	for _, k := range keys {
		defer deleteObject(ctx, env, k)
	}
	if err != nil {
		return Error("setup: " + err.Error())
	}

	out, err := env.Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:     aws.String(env.Bucket),
		Prefix:     aws.String(prefix),
		StartAfter: aws.String(keys[0]),
	})
	if err != nil {
		return Fail(err.Error())
	}
	var got []string
	for _, c := range out.Contents {
		got = append(got, aws.ToString(c.Key))
	}
	if strings.Join(got, ",") != strings.Join(keys[1:], ",") {
		return Fail(fmt.Sprintf("expected %v, got %v", keys[1:], got))
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
	if len(out.Deleted) != 3 {
		return Fail(fmt.Sprintf("expected 3 deleted, got %d", len(out.Deleted)))
	}
	for _, k := range keys {
		_, err := headObject(ctx, env, k)
		if r := expectFailure(err, "HeadObject of "+k+" after DeleteObjects", 404, "NotFound", "NoSuchKey"); r.Status != StatusPass {
			return r
		}
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
	// Quiet mode: only errors are reported.
	if len(out.Errors) > 0 {
		return Fail(fmt.Sprintf("got %d errors in quiet delete", len(out.Errors)))
	}
	if len(out.Deleted) > 0 {
		return Fail(fmt.Sprintf("quiet delete listed %d deleted keys", len(out.Deleted)))
	}
	for _, k := range keys {
		_, err := headObject(ctx, env, k)
		if r := expectFailure(err, "HeadObject of "+k+" after quiet DeleteObjects", 404, "NotFound", "NoSuchKey"); r.Status != StatusPass {
			return r
		}
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
