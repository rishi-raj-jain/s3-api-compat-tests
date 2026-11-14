package tests

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

const partSize = 5 * 1024 * 1024 // 5 MiB minimum part size

func MultipartTests() []TestDef {
	return []TestDef{
		{"CreateMultipartUpload", "multipart", testCreateMultipartUpload},
		{"UploadPart", "multipart", testUploadPart},
		{"ListParts", "multipart", testListParts},
		{"CompleteMultipartUpload", "multipart", testCompleteMultipartUpload},
		{"CompleteMultipartUpload_InvalidPartOrder", "multipart", testCompleteMultipartUploadInvalidPartOrder},
		{"AbortMultipartUpload", "multipart", testAbortMultipartUpload},
		{"ListMultipartUploads", "multipart", testListMultipartUploads},
		{"UploadPartCopy", "multipart", testUploadPartCopy},
	}
}

func abortUpload(ctx context.Context, env *Env, key string, uploadId *string) {
	_, _ = env.Client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(env.Bucket),
		Key:      aws.String(key),
		UploadId: uploadId,
	})
}

func uploadPart(ctx context.Context, env *Env, key string, uploadId *string, n int32, fill byte) (types.CompletedPart, error) {
	part := bytes.Repeat([]byte{fill}, partSize)
	out, err := env.Client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(env.Bucket),
		Key:        aws.String(key),
		UploadId:   uploadId,
		PartNumber: aws.Int32(n),
		Body:       bytes.NewReader(part),
	})
	if err != nil {
		return types.CompletedPart{}, err
	}
	return types.CompletedPart{PartNumber: aws.Int32(n), ETag: out.ETag}, nil
}

// completeTwoPartUpload writes key as a two-part multipart upload and returns
// the object's (composite) ETag.
func completeTwoPartUpload(ctx context.Context, env *Env, key string) (string, error) {
	create, err := env.Client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return "", fmt.Errorf("create: %w", err)
	}
	var parts []types.CompletedPart
	for i, fill := range []byte{'1', '2'} {
		p, err := uploadPart(ctx, env, key, create.UploadId, int32(i+1), fill)
		if err != nil {
			abortUpload(ctx, env, key, create.UploadId)
			return "", fmt.Errorf("upload part %d: %w", i+1, err)
		}
		parts = append(parts, p)
	}
	out, err := env.Client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:          aws.String(env.Bucket),
		Key:             aws.String(key),
		UploadId:        create.UploadId,
		MultipartUpload: &types.CompletedMultipartUpload{Parts: parts},
	})
	if err != nil {
		abortUpload(ctx, env, key, create.UploadId)
		return "", fmt.Errorf("complete: %w", err)
	}
	return aws.ToString(out.ETag), nil
}

// CreateMultipartUpload returns an upload id that ListMultipartUploads shows.
func testCreateMultipartUpload(ctx context.Context, env *Env) TestResult {
	key := "mpu-create"
	out, err := env.Client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return Fail(err.Error())
	}
	defer abortUpload(ctx, env, key, out.UploadId)
	if aws.ToString(out.UploadId) == "" {
		return Fail("empty UploadId")
	}
	list, err := env.Client.ListMultipartUploads(ctx, &s3.ListMultipartUploadsInput{
		Bucket: aws.String(env.Bucket), Prefix: aws.String(key),
	})
	if err != nil {
		return Fail("list uploads: " + err.Error())
	}
	for _, u := range list.Uploads {
		if aws.ToString(u.UploadId) == aws.ToString(out.UploadId) {
			return Pass()
		}
	}
	return Fail("new upload not listed by ListMultipartUploads")
}

// UploadPart returns an ETag for the part.
func testUploadPart(ctx context.Context, env *Env) TestResult {
	key := "mpu-upload-part"
	create, err := env.Client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return Error("create: " + err.Error())
	}
	defer abortUpload(ctx, env, key, create.UploadId)

	p, err := uploadPart(ctx, env, key, create.UploadId, 1, 'a')
	if err != nil {
		return Fail(err.Error())
	}
	if aws.ToString(p.ETag) == "" {
		return Fail("UploadPart returned no ETag")
	}
	return Pass()
}

// ListParts lists the uploaded part with its number, size and ETag.
func testListParts(ctx context.Context, env *Env) TestResult {
	key := "mpu-list-parts"
	create, err := env.Client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return Error("create: " + err.Error())
	}
	defer abortUpload(ctx, env, key, create.UploadId)

	p, err := uploadPart(ctx, env, key, create.UploadId, 1, 'b')
	if err != nil {
		return Error("upload: " + err.Error())
	}

	out, err := env.Client.ListParts(ctx, &s3.ListPartsInput{
		Bucket:   aws.String(env.Bucket),
		Key:      aws.String(key),
		UploadId: create.UploadId,
	})
	if err != nil {
		return Fail(err.Error())
	}
	if len(out.Parts) != 1 {
		return Fail(fmt.Sprintf("expected 1 part, got %d", len(out.Parts)))
	}
	got := out.Parts[0]
	if aws.ToInt32(got.PartNumber) != 1 || aws.ToInt64(got.Size) != int64(partSize) || aws.ToString(got.ETag) != aws.ToString(p.ETag) {
		return Fail(fmt.Sprintf("part mismatch: number=%d size=%d etag=%s", aws.ToInt32(got.PartNumber), aws.ToInt64(got.Size), aws.ToString(got.ETag)))
	}
	return Pass()
}

// CompleteMultipartUpload joins the parts: the object has the full length, a
// composite ETag, and the bytes of both parts in order.
func testCompleteMultipartUpload(ctx context.Context, env *Env) TestResult {
	key := "mpu-complete"
	etag, err := completeTwoPartUpload(ctx, env, key)
	if err != nil {
		return Fail(err.Error())
	}
	defer deleteObject(ctx, env, key)

	if !strings.HasSuffix(strings.Trim(etag, `"`), "-2") {
		return Fail(fmt.Sprintf("expected a composite ETag ending in -2, got %s", etag))
	}
	head, err := headObject(ctx, env, key)
	if err != nil {
		return Fail("head: " + err.Error())
	}
	if aws.ToInt64(head.ContentLength) != int64(2*partSize) {
		return Fail(fmt.Sprintf("length mismatch: got %d, want %d", aws.ToInt64(head.ContentLength), 2*partSize))
	}
	// The first byte of part 2 sits at offset partSize.
	out, err := env.Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(env.Bucket), Key: aws.String(key),
		Range: aws.String(fmt.Sprintf("bytes=%d-%d", partSize-1, partSize)),
	})
	if err != nil {
		return Fail("range read: " + err.Error())
	}
	defer out.Body.Close()
	var b [2]byte
	if _, err := out.Body.Read(b[:]); err != nil && b[0] == 0 {
		return Fail("range read body: " + err.Error())
	}
	if b[0] != '1' || b[1] != '2' {
		return Fail(fmt.Sprintf("part boundary bytes %q, want \"12\"", string(b[:])))
	}
	return Pass()
}

// Parts listed out of order are refused with InvalidPartOrder.
func testCompleteMultipartUploadInvalidPartOrder(ctx context.Context, env *Env) TestResult {
	key := "mpu-part-order"
	create, err := env.Client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return Error("create: " + err.Error())
	}
	defer abortUpload(ctx, env, key, create.UploadId)
	defer deleteObject(ctx, env, key)

	p1, err := uploadPart(ctx, env, key, create.UploadId, 1, '1')
	if err != nil {
		return Error("upload part 1: " + err.Error())
	}
	p2, err := uploadPart(ctx, env, key, create.UploadId, 2, '2')
	if err != nil {
		return Error("upload part 2: " + err.Error())
	}
	_, err = env.Client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:          aws.String(env.Bucket),
		Key:             aws.String(key),
		UploadId:        create.UploadId,
		MultipartUpload: &types.CompletedMultipartUpload{Parts: []types.CompletedPart{p2, p1}},
	})
	return expectFailure(err, "complete with parts out of order", 0, "InvalidPartOrder")
}

// AbortMultipartUpload ends the upload; ListParts is then NoSuchUpload.
func testAbortMultipartUpload(ctx context.Context, env *Env) TestResult {
	key := "mpu-abort"
	create, err := env.Client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return Error("create: " + err.Error())
	}
	if _, err := uploadPart(ctx, env, key, create.UploadId, 1, 'x'); err != nil {
		abortUpload(ctx, env, key, create.UploadId)
		return Error("upload: " + err.Error())
	}

	_, err = env.Client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(env.Bucket),
		Key:      aws.String(key),
		UploadId: create.UploadId,
	})
	if err != nil {
		return Fail(err.Error())
	}
	_, err = env.Client.ListParts(ctx, &s3.ListPartsInput{
		Bucket: aws.String(env.Bucket), Key: aws.String(key), UploadId: create.UploadId,
	})
	return expectFailure(err, "ListParts after abort", 404, "NoSuchUpload")
}

func testListMultipartUploads(ctx context.Context, env *Env) TestResult {
	key := "mpu-list-uploads"
	create, err := env.Client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return Error("create: " + err.Error())
	}
	defer abortUpload(ctx, env, key, create.UploadId)

	out, err := env.Client.ListMultipartUploads(ctx, &s3.ListMultipartUploadsInput{
		Bucket: aws.String(env.Bucket),
	})
	if err != nil {
		return Fail(err.Error())
	}
	for _, u := range out.Uploads {
		if aws.ToString(u.Key) == key && aws.ToString(u.UploadId) == aws.ToString(create.UploadId) {
			return Pass()
		}
	}
	return Fail("upload not found in list")
}

// UploadPartCopy copies a part from an existing object; the completed object
// has the source bytes.
func testUploadPartCopy(ctx context.Context, env *Env) TestResult {
	srcKey := "mpu-copy-src"
	body := strings.Repeat("x", partSize)
	if _, err := putBody(ctx, env, env.Bucket, srcKey, body); err != nil {
		return Error("put source: " + err.Error())
	}
	defer deleteObject(ctx, env, srcKey)

	dstKey := "mpu-copy-dst"
	create, err := env.Client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(dstKey),
	})
	if err != nil {
		return Error("create mpu: " + err.Error())
	}
	defer abortUpload(ctx, env, dstKey, create.UploadId)
	defer deleteObject(ctx, env, dstKey)

	copyOut, err := env.Client.UploadPartCopy(ctx, &s3.UploadPartCopyInput{
		Bucket:     aws.String(env.Bucket),
		Key:        aws.String(dstKey),
		UploadId:   create.UploadId,
		PartNumber: aws.Int32(1),
		CopySource: aws.String(fmt.Sprintf("%s/%s", env.Bucket, srcKey)),
	})
	if err != nil {
		return Fail(err.Error())
	}
	if copyOut.CopyPartResult == nil || copyOut.CopyPartResult.ETag == nil {
		return Fail("no ETag in copy result")
	}
	_, err = env.Client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(env.Bucket),
		Key:      aws.String(dstKey),
		UploadId: create.UploadId,
		MultipartUpload: &types.CompletedMultipartUpload{Parts: []types.CompletedPart{
			{PartNumber: aws.Int32(1), ETag: copyOut.CopyPartResult.ETag},
		}},
	})
	if err != nil {
		return Fail("complete: " + err.Error())
	}
	head, err := headObject(ctx, env, dstKey)
	if err != nil {
		return Fail("head: " + err.Error())
	}
	if aws.ToInt64(head.ContentLength) != int64(partSize) {
		return Fail(fmt.Sprintf("copied part length %d, want %d", aws.ToInt64(head.ContentLength), partSize))
	}
	return Pass()
}
