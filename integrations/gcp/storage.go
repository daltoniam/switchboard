package gcp

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"

	mcp "github.com/daltoniam/switchboard"
)

const maxGCSGetObjectSize = 10 * 1024 * 1024

func storageListBuckets(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	limit := r.Int("limit")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if limit <= 0 {
		limit = defaultStorageLimit
	}

	var buckets []map[string]any
	it := g.storageClient.Buckets(ctx, g.projectID)
	for i := 0; i < limit; i++ {
		b, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return errResult(err)
		}
		buckets = append(buckets, map[string]any{
			"name":          b.Name,
			"location":      b.Location,
			"storage_class": b.StorageClass,
			"created":       b.Created,
		})
	}
	return mcp.JSONResult(buckets)
}

func storageGetBucket(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	bucketName := r.Str("bucket")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	bucket := g.storageClient.Bucket(bucketName)
	attrs, err := bucket.Attrs(ctx)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(attrs)
}

func storageListObjects(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	query := &storage.Query{}
	if v := r.Str("prefix"); v != "" {
		query.Prefix = v
	}
	if v := r.Str("delimiter"); v != "" {
		query.Delimiter = v
	}

	limit := r.Int("limit")
	bucketName := r.Str("bucket")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if limit <= 0 {
		limit = defaultStorageLimit
	}

	bucket := g.storageClient.Bucket(bucketName)
	var objects []map[string]any
	it := bucket.Objects(ctx, query)
	for i := 0; i < limit; i++ {
		o, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return errResult(err)
		}
		objects = append(objects, map[string]any{
			"name":         o.Name,
			"prefix":       o.Prefix,
			"size":         o.Size,
			"content_type": o.ContentType,
			"updated":      o.Updated,
		})
	}
	return mcp.JSONResult(objects)
}

func storageGetObject(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	bucketName := r.Str("bucket")
	objectName := r.Str("object")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	bucket := g.storageClient.Bucket(bucketName)
	obj := bucket.Object(objectName)

	attrs, err := obj.Attrs(ctx)
	if err != nil {
		return errResult(err)
	}

	reader, err := obj.NewReader(ctx)
	if err != nil {
		return errResult(err)
	}
	defer func() { _ = reader.Close() }()

	data, err := io.ReadAll(io.LimitReader(reader, maxGCSGetObjectSize+1))
	if err != nil {
		return errResult(err)
	}
	if len(data) > maxGCSGetObjectSize {
		return errResult(fmt.Errorf("object exceeds maximum size of %d bytes", maxGCSGetObjectSize))
	}

	type getObjectResult struct {
		Name        string `json:"name"`
		ContentType string `json:"content_type"`
		Size        int64  `json:"size"`
		Updated     string `json:"updated,omitempty"`
		Body        string `json:"body"`
		Encoding    string `json:"encoding"`
	}

	result := getObjectResult{
		Name:        attrs.Name,
		ContentType: attrs.ContentType,
		Size:        attrs.Size,
		Updated:     attrs.Updated.String(),
	}

	ct := attrs.ContentType
	if strings.HasPrefix(ct, "text/") || ct == "application/json" || ct == "application/xml" || ct == "application/yaml" {
		result.Body = string(data)
		result.Encoding = "text"
	} else {
		result.Body = base64.StdEncoding.EncodeToString(data)
		result.Encoding = "base64"
	}
	return mcp.JSONResult(result)
}

func storagePutObject(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	bucketName := r.Str("bucket")
	objectName := r.Str("object")
	contentType := r.Str("content_type")
	body := r.Str("body")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	bucket := g.storageClient.Bucket(bucketName)
	obj := bucket.Object(objectName)

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	writer := obj.NewWriter(ctx)
	writer.ContentType = contentType
	if _, err := io.Copy(writer, strings.NewReader(body)); err != nil {
		_ = writer.Close()
		return errResult(err)
	}
	if err := writer.Close(); err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "success"})
}

func storageDeleteObject(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	bucketName := r.Str("bucket")
	objectName := r.Str("object")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	bucket := g.storageClient.Bucket(bucketName)
	obj := bucket.Object(objectName)
	if err := obj.Delete(ctx); err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "success"})
}

func storageCopyObject(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	srcBucketName := r.Str("source_bucket")
	srcObjectName := r.Str("source_object")
	dstBucketName := r.Str("dest_bucket")
	dstObjectName := r.Str("dest_object")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	srcBucket := g.storageClient.Bucket(srcBucketName)
	srcObj := srcBucket.Object(srcObjectName)

	dstBucket := g.storageClient.Bucket(dstBucketName)
	dstObj := dstBucket.Object(dstObjectName)

	copier := dstObj.CopierFrom(srcObj)
	attrs, err := copier.Run(ctx)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]any{
		"name":    attrs.Name,
		"bucket":  attrs.Bucket,
		"size":    attrs.Size,
		"updated": attrs.Updated,
	})
}
