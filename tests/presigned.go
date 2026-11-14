package tests

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
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
		{"PresignPostObject", "presigned", testPresignPostObject},
	}
}

var httpClient = &http.Client{Timeout: 30 * time.Second}

const presignBody = "presigned put content"

func doPresigned(ctx context.Context, p *v4PresignedHTTPRequest, body io.Reader, length int64) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, p.Method, p.URL, body)
	if err != nil {
		return nil, err
	}
	for k, v := range p.SignedHeader {
		for _, vv := range v {
			req.Header.Set(k, vv)
		}
	}
	if length >= 0 {
		req.ContentLength = length
	}
	return httpClient.Do(req)
}

// v4PresignedHTTPRequest holds the parts of a presigned request the tests send.
type v4PresignedHTTPRequest struct {
	URL          string
	Method       string
	SignedHeader http.Header
}

func testPresignPutObject(ctx context.Context, env *Env) TestResult {
	key := "presign-put"
	presigned, err := env.PresignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(5*time.Minute))
	if err != nil {
		return Fail(err.Error())
	}

	resp, err := doPresigned(ctx, &v4PresignedHTTPRequest{presigned.URL, presigned.Method, presigned.SignedHeader}, strings.NewReader(presignBody), int64(len(presignBody)))
	if err != nil {
		return Fail("http: " + err.Error())
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return Fail(fmt.Sprintf("http status %d", resp.StatusCode))
	}
	body, err := getObjectBody(ctx, env, key)
	if err != nil || body != presignBody {
		return Fail(fmt.Sprintf("object not stored through the presigned PUT: %q %v", body, err))
	}
	// Don't delete — used by PresignGetObject, PresignHeadObject, PresignDeleteObject.
	return Pass()
}

func testPresignGetObject(ctx context.Context, env *Env) TestResult {
	key := "presign-put"
	presigned, err := env.PresignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(5*time.Minute))
	if err != nil {
		return Fail(err.Error())
	}

	resp, err := doPresigned(ctx, &v4PresignedHTTPRequest{presigned.URL, presigned.Method, presigned.SignedHeader}, nil, -1)
	if err != nil {
		return Fail("http: " + err.Error())
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return Fail(fmt.Sprintf("http status %d", resp.StatusCode))
	}
	b, _ := io.ReadAll(resp.Body)
	if string(b) != presignBody {
		return Fail(fmt.Sprintf("body mismatch: got %q", string(b)))
	}
	return Pass()
}

func testPresignHeadObject(ctx context.Context, env *Env) TestResult {
	key := "presign-put"
	presigned, err := env.PresignClient.PresignHeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(5*time.Minute))
	if err != nil {
		return Fail(err.Error())
	}

	resp, err := doPresigned(ctx, &v4PresignedHTTPRequest{presigned.URL, presigned.Method, presigned.SignedHeader}, nil, -1)
	if err != nil {
		return Fail("http: " + err.Error())
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return Fail(fmt.Sprintf("http status %d", resp.StatusCode))
	}
	if resp.ContentLength != int64(len(presignBody)) {
		return Fail(fmt.Sprintf("Content-Length %d, want %d", resp.ContentLength, len(presignBody)))
	}
	return Pass()
}

func testPresignDeleteObject(ctx context.Context, env *Env) TestResult {
	key := "presign-put"
	presigned, err := env.PresignClient.PresignDeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(5*time.Minute))
	if err != nil {
		return Fail(err.Error())
	}

	resp, err := doPresigned(ctx, &v4PresignedHTTPRequest{presigned.URL, presigned.Method, presigned.SignedHeader}, nil, -1)
	if err != nil {
		return Fail("http: " + err.Error())
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return Fail(fmt.Sprintf("http status %d", resp.StatusCode))
	}
	_, err = headObject(ctx, env, key)
	return expectFailure(err, "HeadObject after the presigned DELETE", 404, "NotFound", "NoSuchKey")
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
	defer abortUpload(ctx, env, key, create.UploadId)

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
	resp, err := doPresigned(ctx, &v4PresignedHTTPRequest{presigned.URL, presigned.Method, presigned.SignedHeader}, bytes.NewReader(part), int64(len(part)))
	if err != nil {
		return Fail("http: " + err.Error())
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return Fail(fmt.Sprintf("http status %d", resp.StatusCode))
	}
	if resp.Header.Get("ETag") == "" {
		return Fail("presigned UploadPart returned no ETag")
	}
	list, err := env.Client.ListParts(ctx, &s3.ListPartsInput{
		Bucket: aws.String(env.Bucket), Key: aws.String(key), UploadId: create.UploadId,
	})
	if err != nil || len(list.Parts) != 1 {
		return Fail(fmt.Sprintf("part not listed after the presigned upload: %v", err))
	}
	return Pass()
}

// A presigned POST (browser form upload) stores the object.
func testPresignPostObject(ctx context.Context, env *Env) TestResult {
	key := "presign-post"
	presigned, err := env.PresignClient.PresignPostObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(env.Bucket),
		Key:    aws.String(key),
	}, func(o *s3.PresignPostOptions) { o.Expires = 5 * time.Minute })
	if err != nil {
		return Fail(err.Error())
	}
	defer deleteObject(ctx, env, key)

	var form bytes.Buffer
	w := multipart.NewWriter(&form)
	for k, v := range presigned.Values {
		if err := w.WriteField(k, v); err != nil {
			return Error("form: " + err.Error())
		}
	}
	fw, err := w.CreateFormFile("file", "upload.txt")
	if err != nil {
		return Error("form: " + err.Error())
	}
	_, _ = fw.Write([]byte("presigned post content"))
	_ = w.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, presigned.URL, &form)
	if err != nil {
		return Error(err.Error())
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := httpClient.Do(req)
	if err != nil {
		return Fail("http: " + err.Error())
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return Fail(fmt.Sprintf("http status %d: %s", resp.StatusCode, strings.TrimSpace(string(b))))
	}
	body, err := getObjectBody(ctx, env, key)
	if err != nil || body != "presigned post content" {
		return Fail(fmt.Sprintf("object not stored through the presigned POST: %q %v", body, err))
	}
	return Pass()
}
