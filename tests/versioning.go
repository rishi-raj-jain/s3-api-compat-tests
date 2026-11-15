package tests

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// VersioningTests run against env.VersionedBucket, a bucket the runner created
// with versioning enabled the way the provider supports. They skip when the
// runner could not create one.
func VersioningTests() []TestDef {
	return []TestDef{
		{"GetBucketVersioning", "versioning", testGetBucketVersioning},
		{"PutBucketVersioning_Idempotent", "versioning", testPutBucketVersioningIdempotent},
		{"PutObject_Versioned", "versioning", testPutObjectVersioned},
		{"GetObject_VersionId", "versioning", testGetObjectVersionId},
		{"HeadObject_VersionId", "versioning", testHeadObjectVersionId},
		{"ListObjectVersions", "versioning", testListObjectVersions},
		{"DeleteObject_VersionId", "versioning", testDeleteObjectVersionId},
		{"PutObjectTagging_VersionId", "versioning", testPutObjectTaggingVersionId},
		{"GetObjectTagging_VersionId", "versioning", testGetObjectTaggingVersionId},
		{"DeleteObjectTagging_VersionId", "versioning", testDeleteObjectTaggingVersionId},
	}
}

func needVersioned(env *Env) *TestResult {
	if env.VersionedBucket == "" {
		r := Skip("no versioned bucket: " + env.VersionedBucketErr)
		return &r
	}
	return nil
}

// putTwoVersions writes v1 then v2 of key in the versioned bucket and returns
// both version ids.
func putTwoVersions(ctx context.Context, env *Env, key string) (v1, v2 string, err error) {
	out1, err := putBody(ctx, env, env.VersionedBucket, key, "v1")
	if err != nil {
		return "", "", err
	}
	out2, err := putBody(ctx, env, env.VersionedBucket, key, "v2")
	if err != nil {
		return "", "", err
	}
	v1, v2 = aws.ToString(out1.VersionId), aws.ToString(out2.VersionId)
	if v1 == "" || v2 == "" {
		return v1, v2, fmt.Errorf("PutObject returned no version id (v1=%q, v2=%q)", v1, v2)
	}
	if v1 == v2 {
		return v1, v2, fmt.Errorf("both writes returned version id %q", v1)
	}
	return v1, v2, nil
}

func getVersionBody(ctx context.Context, env *Env, key, versionId string) (string, error) {
	out, err := env.Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket:    aws.String(env.VersionedBucket),
		Key:       aws.String(key),
		VersionId: aws.String(versionId),
	})
	if err != nil {
		return "", err
	}
	defer out.Body.Close()
	b, err := io.ReadAll(out.Body)
	return string(b), err
}

func deleteAllVersions(ctx context.Context, env *Env, key string) {
	out, err := env.Client.ListObjectVersions(ctx, &s3.ListObjectVersionsInput{
		Bucket: aws.String(env.VersionedBucket),
		Prefix: aws.String(key),
	})
	if err != nil {
		return
	}
	for _, v := range out.Versions {
		if aws.ToString(v.Key) == key {
			_, _ = env.Client.DeleteObject(ctx, &s3.DeleteObjectInput{
				Bucket: aws.String(env.VersionedBucket), Key: aws.String(key), VersionId: v.VersionId,
			})
		}
	}
	for _, m := range out.DeleteMarkers {
		if aws.ToString(m.Key) == key {
			_, _ = env.Client.DeleteObject(ctx, &s3.DeleteObjectInput{
				Bucket: aws.String(env.VersionedBucket), Key: aws.String(key), VersionId: m.VersionId,
			})
		}
	}
}

func versionTags(ctx context.Context, env *Env, key, versionId string) (map[string]string, error) {
	in := &s3.GetObjectTaggingInput{Bucket: aws.String(env.VersionedBucket), Key: aws.String(key)}
	if versionId != "" {
		in.VersionId = aws.String(versionId)
	}
	out, err := env.Client.GetObjectTagging(ctx, in)
	if err != nil {
		return nil, err
	}
	tags := map[string]string{}
	for _, t := range out.TagSet {
		tags[aws.ToString(t.Key)] = aws.ToString(t.Value)
	}
	return tags, nil
}

// GetBucketVersioning reports Enabled on a bucket created with versioning.
func testGetBucketVersioning(ctx context.Context, env *Env) TestResult {
	if r := needVersioned(env); r != nil {
		return *r
	}
	out, err := env.Client.GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{
		Bucket: aws.String(env.VersionedBucket),
	})
	if err != nil {
		return Fail(err.Error())
	}
	if out.Status != types.BucketVersioningStatusEnabled {
		return Fail(fmt.Sprintf("expected status Enabled, got %q", out.Status))
	}
	return Pass()
}

// PutBucketVersioning Enabled on a bucket that already has versioning succeeds.
func testPutBucketVersioningIdempotent(ctx context.Context, env *Env) TestResult {
	if r := needVersioned(env); r != nil {
		return *r
	}
	_, err := env.Client.PutBucketVersioning(ctx, &s3.PutBucketVersioningInput{
		Bucket: aws.String(env.VersionedBucket),
		VersioningConfiguration: &types.VersioningConfiguration{
			Status: types.BucketVersioningStatusEnabled,
		},
	})
	if err != nil {
		return Fail(err.Error())
	}
	return Pass()
}

// Two writes to one key return two version ids, and the current object is v2.
func testPutObjectVersioned(ctx context.Context, env *Env) TestResult {
	if r := needVersioned(env); r != nil {
		return *r
	}
	key := "ver-put"
	defer deleteAllVersions(ctx, env, key)
	_, v2, err := putTwoVersions(ctx, env, key)
	if err != nil {
		return Fail(err.Error())
	}
	out, err := env.Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(env.VersionedBucket), Key: aws.String(key),
	})
	if err != nil {
		return Fail("get current: " + err.Error())
	}
	defer out.Body.Close()
	b, _ := io.ReadAll(out.Body)
	if string(b) != "v2" {
		return Fail(fmt.Sprintf("current version body %q, want v2", string(b)))
	}
	if aws.ToString(out.VersionId) != v2 {
		return Fail(fmt.Sprintf("GetObject of the current version reported x-amz-version-id %q, want %s", aws.ToString(out.VersionId), v2))
	}
	return Pass()
}

func testGetObjectVersionId(ctx context.Context, env *Env) TestResult {
	if r := needVersioned(env); r != nil {
		return *r
	}
	key := "ver-get"
	defer deleteAllVersions(ctx, env, key)
	v1, v2, err := putTwoVersions(ctx, env, key)
	if err != nil {
		return Error("setup: " + err.Error())
	}
	b1, err := getVersionBody(ctx, env, key, v1)
	if err != nil {
		return Fail("get v1: " + err.Error())
	}
	b2, err := getVersionBody(ctx, env, key, v2)
	if err != nil {
		return Fail("get v2: " + err.Error())
	}
	if b1 != "v1" || b2 != "v2" {
		return Fail(fmt.Sprintf("version bodies mismatch: v1=%q v2=%q", b1, b2))
	}
	return Pass()
}

func testHeadObjectVersionId(ctx context.Context, env *Env) TestResult {
	if r := needVersioned(env); r != nil {
		return *r
	}
	key := "ver-head"
	defer deleteAllVersions(ctx, env, key)
	v1, _, err := putTwoVersions(ctx, env, key)
	if err != nil {
		return Error("setup: " + err.Error())
	}
	out, err := env.Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(env.VersionedBucket), Key: aws.String(key), VersionId: aws.String(v1),
	})
	if err != nil {
		return Fail(err.Error())
	}
	if aws.ToString(out.VersionId) != v1 || aws.ToString(out.ETag) != md5ETag("v1") {
		return Fail(fmt.Sprintf("head v1: version=%q etag=%s", aws.ToString(out.VersionId), aws.ToString(out.ETag)))
	}
	return Pass()
}

// ListObjectVersions lists both versions, newest first, with IsLatest set once.
func testListObjectVersions(ctx context.Context, env *Env) TestResult {
	if r := needVersioned(env); r != nil {
		return *r
	}
	key := "ver-list"
	defer deleteAllVersions(ctx, env, key)
	v1, v2, err := putTwoVersions(ctx, env, key)
	if err != nil {
		return Error("setup: " + err.Error())
	}
	out, err := env.Client.ListObjectVersions(ctx, &s3.ListObjectVersionsInput{
		Bucket: aws.String(env.VersionedBucket), Prefix: aws.String(key),
	})
	if err != nil {
		return Fail(err.Error())
	}
	var ids []string
	latest := map[string]bool{}
	for _, v := range out.Versions {
		if aws.ToString(v.Key) != key {
			continue
		}
		ids = append(ids, aws.ToString(v.VersionId))
		latest[aws.ToString(v.VersionId)] = aws.ToBool(v.IsLatest)
	}
	if len(ids) != 2 {
		return Fail(fmt.Sprintf("expected 2 versions, got %v", ids))
	}
	if ids[0] != v2 || ids[1] != v1 {
		return Fail(fmt.Sprintf("versions not newest-first: got %v, want [%s %s]", ids, v2, v1))
	}
	if !latest[v2] || latest[v1] {
		return Fail(fmt.Sprintf("IsLatest must be set on %s only: v2=%v v1=%v", v2, latest[v2], latest[v1]))
	}
	return Pass()
}

// Deleting one version by id removes that version only.
func testDeleteObjectVersionId(ctx context.Context, env *Env) TestResult {
	if r := needVersioned(env); r != nil {
		return *r
	}
	key := "ver-delete"
	defer deleteAllVersions(ctx, env, key)
	v1, _, err := putTwoVersions(ctx, env, key)
	if err != nil {
		return Error("setup: " + err.Error())
	}
	if _, err := env.Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(env.VersionedBucket), Key: aws.String(key), VersionId: aws.String(v1),
	}); err != nil {
		return Fail(err.Error())
	}
	_, err = getVersionBody(ctx, env, key, v1)
	if r := expectFailure(err, "GET of the deleted version", 404, "NoSuchVersion", "NoSuchKey"); r.Status != StatusPass {
		return r
	}
	out, err := env.Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(env.VersionedBucket), Key: aws.String(key),
	})
	if err != nil {
		return Fail("current version gone after deleting an old one: " + err.Error())
	}
	defer out.Body.Close()
	b, _ := io.ReadAll(out.Body)
	if string(b) != "v2" {
		return Fail(fmt.Sprintf("current body %q after deleting v1, want v2", string(b)))
	}
	return Pass()
}

// Tagging a version tags that version only; the current version keeps its
// tags, and no new version is created.
func testPutObjectTaggingVersionId(ctx context.Context, env *Env) TestResult {
	if r := needVersioned(env); r != nil {
		return *r
	}
	key := "ver-tag-put"
	defer deleteAllVersions(ctx, env, key)
	v1, _, err := putTwoVersions(ctx, env, key)
	if err != nil {
		return Error("setup: " + err.Error())
	}
	out, err := env.Client.PutObjectTagging(ctx, &s3.PutObjectTaggingInput{
		Bucket:    aws.String(env.VersionedBucket),
		Key:       aws.String(key),
		VersionId: aws.String(v1),
		Tagging:   &types.Tagging{TagSet: []types.Tag{{Key: aws.String("gen"), Value: aws.String("one")}}},
	})
	if err != nil {
		return Fail(err.Error())
	}
	if aws.ToString(out.VersionId) != v1 {
		return Fail(fmt.Sprintf("response version %q, want %s", aws.ToString(out.VersionId), v1))
	}
	tagsV1, err := versionTags(ctx, env, key, v1)
	if err != nil {
		return Fail("get v1 tags: " + err.Error())
	}
	if tagsV1["gen"] != "one" {
		return Fail(fmt.Sprintf("v1 tags not stored: %v", tagsV1))
	}
	tagsCur, err := versionTags(ctx, env, key, "")
	if err != nil {
		return Fail("get current tags: " + err.Error())
	}
	if len(tagsCur) != 0 {
		return Fail(fmt.Sprintf("tagging v1 changed the current version's tags: %v", tagsCur))
	}
	list, err := env.Client.ListObjectVersions(ctx, &s3.ListObjectVersionsInput{
		Bucket: aws.String(env.VersionedBucket), Prefix: aws.String(key),
	})
	if err != nil {
		return Fail("ListObjectVersions after tagging: " + err.Error())
	}
	n := 0
	for _, v := range list.Versions {
		if aws.ToString(v.Key) == key {
			n++
		}
	}
	if n != 2 {
		return Fail(fmt.Sprintf("tagging created a version: %d versions, want 2", n))
	}
	return Pass()
}

func testGetObjectTaggingVersionId(ctx context.Context, env *Env) TestResult {
	if r := needVersioned(env); r != nil {
		return *r
	}
	key := "ver-tag-get"
	defer deleteAllVersions(ctx, env, key)
	v1, v2, err := putTwoVersions(ctx, env, key)
	if err != nil {
		return Error("setup: " + err.Error())
	}
	for _, p := range []struct{ id, val string }{{v1, "one"}, {v2, "two"}} {
		if _, err := env.Client.PutObjectTagging(ctx, &s3.PutObjectTaggingInput{
			Bucket: aws.String(env.VersionedBucket), Key: aws.String(key), VersionId: aws.String(p.id),
			Tagging: &types.Tagging{TagSet: []types.Tag{{Key: aws.String("gen"), Value: aws.String(p.val)}}},
		}); err != nil {
			return Error("setup tags: " + err.Error())
		}
	}
	t1, err := versionTags(ctx, env, key, v1)
	if err != nil {
		return Fail(err.Error())
	}
	t2, err := versionTags(ctx, env, key, v2)
	if err != nil {
		return Fail(err.Error())
	}
	if t1["gen"] != "one" || t2["gen"] != "two" {
		return Fail(fmt.Sprintf("per-version tags mismatch: v1=%v v2=%v", t1, t2))
	}
	return Pass()
}

func testDeleteObjectTaggingVersionId(ctx context.Context, env *Env) TestResult {
	if r := needVersioned(env); r != nil {
		return *r
	}
	key := "ver-tag-delete"
	defer deleteAllVersions(ctx, env, key)
	v1, v2, err := putTwoVersions(ctx, env, key)
	if err != nil {
		return Error("setup: " + err.Error())
	}
	for _, id := range []string{v1, v2} {
		if _, err := env.Client.PutObjectTagging(ctx, &s3.PutObjectTaggingInput{
			Bucket: aws.String(env.VersionedBucket), Key: aws.String(key), VersionId: aws.String(id),
			Tagging: &types.Tagging{TagSet: []types.Tag{{Key: aws.String("gen"), Value: aws.String("x")}}},
		}); err != nil {
			return Error("setup tags: " + err.Error())
		}
	}
	if _, err := env.Client.DeleteObjectTagging(ctx, &s3.DeleteObjectTaggingInput{
		Bucket: aws.String(env.VersionedBucket), Key: aws.String(key), VersionId: aws.String(v1),
	}); err != nil {
		return Fail(err.Error())
	}
	t1, err := versionTags(ctx, env, key, v1)
	if err != nil {
		return Fail(err.Error())
	}
	t2, err := versionTags(ctx, env, key, v2)
	if err != nil {
		return Fail(err.Error())
	}
	if len(t1) != 0 {
		return Fail(fmt.Sprintf("v1 tags remain after delete: %v", t1))
	}
	if t2["gen"] != "x" {
		return Fail(fmt.Sprintf("deleting v1 tags changed v2 tags: %v", t2))
	}
	return Pass()
}

var _ = strings.TrimSpace
