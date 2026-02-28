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

func storageListBuckets(ctx context.Context, g *integration, _ map[string]any) (*mcp.ToolResult, error) {
	var buckets []map[string]any
	it := g.storageClient.Buckets(ctx, g.projectID)
	for {
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
	return jsonResult(buckets)
}

func storageGetBucket(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	bucket := g.storageClient.Bucket(argStr(args, "bucket"))
	attrs, err := bucket.Attrs(ctx)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(attrs)
}

func storageListObjects(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	query := &storage.Query{}
	if v := argStr(args, "prefix"); v != "" {
		query.Prefix = v
	}
	if v := argStr(args, "delimiter"); v != "" {
		query.Delimiter = v
	}

	bucket := g.storageClient.Bucket(argStr(args, "bucket"))
	var objects []map[string]any
	it := bucket.Objects(ctx, query)
	for i := 0; i < 1000; i++ {
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
	return jsonResult(objects)
}

func storageGetObject(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	bucket := g.storageClient.Bucket(argStr(args, "bucket"))
	obj := bucket.Object(argStr(args, "object"))

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
	return jsonResult(result)
}

func storagePutObject(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	bucket := g.storageClient.Bucket(argStr(args, "bucket"))
	obj := bucket.Object(argStr(args, "object"))

	contentType := argStr(args, "content_type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	writer := obj.NewWriter(ctx)
	writer.ContentType = contentType
	body := argStr(args, "body")
	if _, err := io.Copy(writer, strings.NewReader(body)); err != nil {
		_ = writer.Close()
		return errResult(err)
	}
	if err := writer.Close(); err != nil {
		return errResult(err)
	}
	return jsonResult(map[string]string{"status": "success"})
}

func storageDeleteObject(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	bucket := g.storageClient.Bucket(argStr(args, "bucket"))
	obj := bucket.Object(argStr(args, "object"))
	if err := obj.Delete(ctx); err != nil {
		return errResult(err)
	}
	return jsonResult(map[string]string{"status": "success"})
}

func storageCopyObject(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	srcBucket := g.storageClient.Bucket(argStr(args, "source_bucket"))
	srcObj := srcBucket.Object(argStr(args, "source_object"))

	dstBucket := g.storageClient.Bucket(argStr(args, "dest_bucket"))
	dstObj := dstBucket.Object(argStr(args, "dest_object"))

	copier := dstObj.CopierFrom(srcObj)
	attrs, err := copier.Run(ctx)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(map[string]any{
		"name":    attrs.Name,
		"bucket":  attrs.Bucket,
		"size":    attrs.Size,
		"updated": attrs.Updated,
	})
}
