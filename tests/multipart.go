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
		{"AbortMultipartUpload", "multipart", testAbortMultipartUpload},
		{"ListMultipartUploads", "multipart", testListMultipartUploads},
		{"UploadPartCopy", "multipart", testUploadPartCopy},
	}
}

func testCreateMultipartUpload(ctx context.Context, env *Env) TestResult {
	key := "mpu-create"
	out, err := env.Client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return Fail(err.Error())
	}
	// Abort to clean up.
	_, _ = env.Client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(env.Bucket),
		Key:      aws.String(key),
		UploadId: out.UploadId,
	})
	return Pass()
}

func testUploadPart(ctx context.Context, env *Env) TestResult {
	key := "mpu-upload-part"
	create, err := env.Client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return Error("create: " + err.Error())
	}
	defer func() {
		_, _ = env.Client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
			Bucket:   aws.String(env.Bucket),
			Key:      aws.String(key),
			UploadId: create.UploadId,
		})
	}()

	part := bytes.Repeat([]byte("a"), partSize)
	_, err = env.Client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(env.Bucket),
		Key:        aws.String(key),
		UploadId:   create.UploadId,
		PartNumber: aws.Int32(1),
		Body:       bytes.NewReader(part),
	})
	if err != nil {
		return Fail(err.Error())
	}
	return Pass()
}

func testListParts(ctx context.Context, env *Env) TestResult {
	key := "mpu-list-parts"
	create, err := env.Client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return Error("create: " + err.Error())
	}
	defer func() {
		_, _ = env.Client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
			Bucket:   aws.String(env.Bucket),
			Key:      aws.String(key),
			UploadId: create.UploadId,
		})
	}()

	part := bytes.Repeat([]byte("b"), partSize)
	_, err = env.Client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(env.Bucket),
		Key:        aws.String(key),
		UploadId:   create.UploadId,
		PartNumber: aws.Int32(1),
		Body:       bytes.NewReader(part),
	})
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
	if len(out.Parts) == 0 {
		return Fail("no parts listed")
	}
	return Pass()
}

func testCompleteMultipartUpload(ctx context.Context, env *Env) TestResult {
	key := "mpu-complete"
	create, err := env.Client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return Error("create: " + err.Error())
	}

	part := bytes.Repeat([]byte("c"), partSize)
	up, err := env.Client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(env.Bucket),
		Key:        aws.String(key),
		UploadId:   create.UploadId,
		PartNumber: aws.Int32(1),
		Body:       bytes.NewReader(part),
	})
	if err != nil {
		_, _ = env.Client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
			Bucket: aws.String(env.Bucket), Key: aws.String(key), UploadId: create.UploadId,
		})
		return Error("upload: " + err.Error())
	}

	_, err = env.Client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(env.Bucket),
		Key:      aws.String(key),
		UploadId: create.UploadId,
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: []types.CompletedPart{
				{PartNumber: aws.Int32(1), ETag: up.ETag},
			},
		},
	})
	if err != nil {
		return Fail(err.Error())
	}

	// Verify object exists.
	_, err = env.Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return Fail("object not found after complete: " + err.Error())
	}

	_, _ = env.Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
	})
	return Pass()
}

func testAbortMultipartUpload(ctx context.Context, env *Env) TestResult {
	key := "mpu-abort"
	create, err := env.Client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return Error("create: " + err.Error())
	}

	_, err = env.Client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(env.Bucket),
		Key:      aws.String(key),
		UploadId: create.UploadId,
	})
	if err != nil {
		return Fail(err.Error())
	}
	return Pass()
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
	defer func() {
		_, _ = env.Client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
			Bucket:   aws.String(env.Bucket),
			Key:      aws.String(key),
			UploadId: create.UploadId,
		})
	}()

	out, err := env.Client.ListMultipartUploads(ctx, &s3.ListMultipartUploadsInput{
		Bucket: aws.String(env.Bucket),
	})
	if err != nil {
		return Fail(err.Error())
	}
	found := false
	for _, u := range out.Uploads {
		if aws.ToString(u.Key) == key {
			found = true
			break
		}
	}
	if !found {
		return Fail("upload not found in list")
	}
	return Pass()
}

func testUploadPartCopy(ctx context.Context, env *Env) TestResult {
	// Need a source object large enough.
	srcKey := "mpu-copy-src"
	body := strings.Repeat("x", partSize)
	_, err := env.Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(srcKey),
		Body:   strings.NewReader(body),
	})
	if err != nil {
		return Error("put source: " + err.Error())
	}
	defer func() {
		_, _ = env.Client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(env.Bucket), Key: aws.String(srcKey),
		})
	}()

	dstKey := "mpu-copy-dst"
	create, err := env.Client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(dstKey),
	})
	if err != nil {
		return Error("create mpu: " + err.Error())
	}
	defer func() {
		_, _ = env.Client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
			Bucket: aws.String(env.Bucket), Key: aws.String(dstKey), UploadId: create.UploadId,
		})
	}()

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
	return Pass()
}
