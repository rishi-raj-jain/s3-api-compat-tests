package tests

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func PresignedTests() []TestDef {
	return []TestDef{
		{"PresignPutObject", "presigned", testPresignPutObject},
		{"PresignGetObject", "presigned", testPresignGetObject},
		{"PresignHeadObject", "presigned", testPresignHeadObject},
		{"PresignDeleteObject", "presigned", testPresignDeleteObject},
		{"PresignUploadPart", "presigned", testPresignUploadPart},
	}
}

var httpClient = &http.Client{Timeout: 30 * time.Second}

func testPresignPutObject(ctx context.Context, env *Env) TestResult {
	key := "presign-put"
	presigned, err := env.PresignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(5*time.Minute))
	if err != nil {
		return Fail(err.Error())
	}

	body := "presigned put content"
	req, _ := http.NewRequestWithContext(ctx, presigned.Method, presigned.URL, strings.NewReader(body))
	for k, v := range presigned.SignedHeader {
		for _, vv := range v {
			req.Header.Set(k, vv)
		}
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return Fail("http: " + err.Error())
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return Fail(fmt.Sprintf("http status %d", resp.StatusCode))
	}

	// Don't delete — used by PresignGetObject.
	return Pass()
}

func testPresignGetObject(ctx context.Context, env *Env) TestResult {
	key := "presign-put" // Reuse object from PresignPutObject.

	presigned, err := env.PresignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(5*time.Minute))
	if err != nil {
		return Fail(err.Error())
	}

	req, _ := http.NewRequestWithContext(ctx, presigned.Method, presigned.URL, nil)
	resp, err := httpClient.Do(req)
	if err != nil {
		return Fail("http: " + err.Error())
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return Fail(fmt.Sprintf("http status %d", resp.StatusCode))
	}
	b, _ := io.ReadAll(resp.Body)
	if string(b) != "presigned put content" {
		return Fail(fmt.Sprintf("body mismatch: got %q", string(b)))
	}
	return Pass()
}

func testPresignHeadObject(ctx context.Context, env *Env) TestResult {
	key := "presign-put" // Reuse object.

	presigned, err := env.PresignClient.PresignHeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(5*time.Minute))
	if err != nil {
		return Fail(err.Error())
	}

	req, _ := http.NewRequestWithContext(ctx, presigned.Method, presigned.URL, nil)
	resp, err := httpClient.Do(req)
	if err != nil {
		return Fail("http: " + err.Error())
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return Fail(fmt.Sprintf("http status %d", resp.StatusCode))
	}
	return Pass()
}

func testPresignDeleteObject(ctx context.Context, env *Env) TestResult {
	key := "presign-put" // Delete the object we created.

	presigned, err := env.PresignClient.PresignDeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(5*time.Minute))
	if err != nil {
		return Fail(err.Error())
	}

	req, _ := http.NewRequestWithContext(ctx, presigned.Method, presigned.URL, nil)
	resp, err := httpClient.Do(req)
	if err != nil {
		return Fail("http: " + err.Error())
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return Fail(fmt.Sprintf("http status %d", resp.StatusCode))
	}
	return Pass()
}

func testPresignUploadPart(ctx context.Context, env *Env) TestResult {
	key := "presign-mpu"
	create, err := env.Client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return Error("create mpu: " + err.Error())
	}
	defer func() {
		_, _ = env.Client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
			Bucket:   aws.String(env.Bucket),
			Key:      aws.String(key),
			UploadId: create.UploadId,
		})
	}()

	presigned, err := env.PresignClient.PresignUploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(env.Bucket),
		Key:        aws.String(key),
		UploadId:   create.UploadId,
		PartNumber: aws.Int32(1),
	}, s3.WithPresignExpires(5*time.Minute))
	if err != nil {
		return Fail(err.Error())
	}

	part := bytes.Repeat([]byte("p"), partSize)
	req, _ := http.NewRequestWithContext(ctx, presigned.Method, presigned.URL, bytes.NewReader(part))
	for k, v := range presigned.SignedHeader {
		for _, vv := range v {
			req.Header.Set(k, vv)
		}
	}
	req.ContentLength = int64(len(part))

	resp, err := httpClient.Do(req)
	if err != nil {
		return Fail("http: " + err.Error())
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return Fail(fmt.Sprintf("http status %d", resp.StatusCode))
	}
	return Pass()
}
