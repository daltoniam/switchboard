package aws

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	mcp "github.com/daltoniam/switchboard"
)

const maxS3GetObjectSize = 10 * 1024 * 1024

func s3ListBuckets(ctx context.Context, a *integration, _ map[string]any) (*mcp.ToolResult, error) {
	out, err := a.s3Client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}

func s3ListObjects(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(r.Str("bucket")),
	}
	if v := r.Str("prefix"); v != "" {
		input.Prefix = aws.String(v)
	}
	if v := r.Int32("max_keys"); v > 0 {
		input.MaxKeys = aws.Int32(v)
	}
	if v := r.Str("continuation_token"); v != "" {
		input.ContinuationToken = aws.String(v)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	out, err := a.s3Client.ListObjectsV2(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}

func s3GetObject(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	bucket := r.Str("bucket")
	key := r.Str("key")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	out, err := a.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return errResult(err)
	}
	defer func() { _ = out.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(out.Body, maxS3GetObjectSize+1))
	if err != nil {
		return errResult(err)
	}
	if len(data) > maxS3GetObjectSize {
		return errResult(fmt.Errorf("object exceeds maximum size of %d bytes", maxS3GetObjectSize))
	}

	ct := ""
	if out.ContentType != nil {
		ct = *out.ContentType
	}

	type getObjectResult struct {
		ContentType   string `json:"content_type"`
		ContentLength int64  `json:"content_length"`
		ETag          string `json:"etag,omitempty"`
		LastModified  string `json:"last_modified,omitempty"`
		Body          string `json:"body"`
		Encoding      string `json:"encoding"`
	}

	result := getObjectResult{
		ContentType:   ct,
		ContentLength: int64(len(data)),
	}
	if out.ETag != nil {
		result.ETag = *out.ETag
	}
	if out.LastModified != nil {
		result.LastModified = out.LastModified.String()
	}

	if strings.HasPrefix(ct, "text/") || ct == "application/json" || ct == "application/xml" || ct == "application/yaml" {
		result.Body = string(data)
		result.Encoding = "text"
	} else {
		result.Body = base64.StdEncoding.EncodeToString(data)
		result.Encoding = "base64"
	}
	return mcp.JSONResult(result)
}

func s3PutObject(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	contentType := r.Str("content_type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	body := r.Str("body")
	bucket := r.Str("bucket")
	key := r.Str("key")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	_, err := a.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        strings.NewReader(body),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "success"})
}

func s3DeleteObject(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	bucket := r.Str("bucket")
	key := r.Str("key")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	_, err := a.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "success"})
}

func s3HeadObject(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	bucket := r.Str("bucket")
	key := r.Str("key")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	out, err := a.s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}

func s3CopyObject(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	source := r.Str("source_bucket") + "/" + url.PathEscape(r.Str("source_key"))
	destBucket := r.Str("dest_bucket")
	destKey := r.Str("dest_key")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	out, err := a.s3Client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(destBucket),
		Key:        aws.String(destKey),
		CopySource: aws.String(source),
	})
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}
