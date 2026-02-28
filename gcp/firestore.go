package gcp

import (
	"context"
	"encoding/json"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"

	mcp "github.com/daltoniam/switchboard"
)

func firestoreListCollections(ctx context.Context, g *integration, _ map[string]any) (*mcp.ToolResult, error) {
	var collections []string
	it := g.firestoreClient.Collections(ctx)
	for {
		col, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return errResult(err)
		}
		collections = append(collections, col.ID)
	}
	return jsonResult(collections)
}

func firestoreListDocuments(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	collection := argStr(args, "collection")
	limit := argInt(args, "limit")
	if limit <= 0 {
		limit = 100
	}

	docs := g.firestoreClient.Collection(collection).Limit(limit).Documents(ctx)
	defer docs.Stop()

	var results []map[string]any
	for {
		doc, err := docs.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return errResult(err)
		}
		results = append(results, map[string]any{
			"id":   doc.Ref.ID,
			"path": doc.Ref.Path,
			"data": doc.Data(),
		})
	}
	return jsonResult(results)
}

func firestoreGetDocument(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	doc, err := g.firestoreClient.Doc(argStr(args, "path")).Get(ctx)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(map[string]any{
		"id":          doc.Ref.ID,
		"path":        doc.Ref.Path,
		"data":        doc.Data(),
		"create_time": doc.CreateTime,
		"update_time": doc.UpdateTime,
	})
}

func firestoreSetDocument(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	dataStr := argStr(args, "data")
	var data map[string]any
	if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
		return errResult(err)
	}

	_, err := g.firestoreClient.Doc(argStr(args, "path")).Set(ctx, data)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(map[string]string{"status": "success"})
}

func firestoreDeleteDocument(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	_, err := g.firestoreClient.Doc(argStr(args, "path")).Delete(ctx)
	if err != nil {
		return errResult(err)
	}
	return jsonResult(map[string]string{"status": "success"})
}

func firestoreQuery(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	collection := argStr(args, "collection")
	q := g.firestoreClient.Collection(collection).Query

	if field := argStr(args, "where_field"); field != "" {
		op := argStr(args, "where_op")
		valueStr := argStr(args, "where_value")
		var value any
		if err := json.Unmarshal([]byte(valueStr), &value); err != nil {
			value = valueStr
		}
		q = q.Where(field, op, value)
	}

	if orderBy := argStr(args, "order_by"); orderBy != "" {
		dir := firestore.Asc
		if argStr(args, "order_dir") == "desc" {
			dir = firestore.Desc
		}
		q = q.OrderBy(orderBy, dir)
	}

	if limit := argInt(args, "limit"); limit > 0 {
		q = q.Limit(limit)
	}

	docs := q.Documents(ctx)
	defer docs.Stop()

	var results []map[string]any
	for {
		doc, err := docs.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return errResult(err)
		}
		results = append(results, map[string]any{
			"id":   doc.Ref.ID,
			"path": doc.Ref.Path,
			"data": doc.Data(),
		})
	}
	return jsonResult(results)
}
