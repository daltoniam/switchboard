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
	return jsonResult(out)
}

func s3ListObjects(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(argStr(args, "bucket")),
	}
	if v := argStr(args, "prefix"); v != "" {
		input.Prefix = aws.String(v)
	}
	if v := argInt32(args, "max_keys"); v > 0 {
		input.MaxKeys = aws.Int32(v)
	}
	if v := argStr(args, "continuation_token"); v != "" {
		input.ContinuationToken = aws.String(v)
	}
	out, err := a.s3Client.ListObjectsV2(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}

func s3GetObject(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	out, err := a.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(argStr(args, "bucket")),
		Key:    aws.String(argStr(args, "key")),
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
	return jsonResult(result)
}

func s3PutObject(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	contentType := argStr(args, "content_type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	body := argStr(args, "body")
	_, err := a.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(argStr(args, "bucket")),
		Key:         aws.String(argStr(args, "key")),
		Body:        strings.NewReader(body),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return errResult(err)
	}
	return jsonResult(map[string]string{"status": "success"})
}

func s3DeleteObject(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	_, err := a.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(argStr(args, "bucket")),
		Key:    aws.String(argStr(args, "key")),
	})
	if err != nil {
		return errResult(err)
	}
	return jsonResult(map[string]string{"status": "success"})
}

func s3HeadObject(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	out, err := a.s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(argStr(args, "bucket")),
		Key:    aws.String(argStr(args, "key")),
	})
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}

func s3CopyObject(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	source := argStr(args, "source_bucket") + "/" + url.PathEscape(argStr(args, "source_key"))
	out, err := a.s3Client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(argStr(args, "dest_bucket")),
		Key:        aws.String(argStr(args, "dest_key")),
		CopySource: aws.String(source),
	})
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}
