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
	return mcp.JSONResult(collections)
}

func firestoreListDocuments(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	collection := r.Str("collection")
	limit := r.Int("limit")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
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
	return mcp.JSONResult(results)
}

func firestoreGetDocument(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	path := r.Str("path")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	doc, err := g.firestoreClient.Doc(path).Get(ctx)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]any{
		"id":          doc.Ref.ID,
		"path":        doc.Ref.Path,
		"data":        doc.Data(),
		"create_time": doc.CreateTime,
		"update_time": doc.UpdateTime,
	})
}

func firestoreSetDocument(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	dataStr := r.Str("data")
	path := r.Str("path")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
		return errResult(err)
	}

	_, err := g.firestoreClient.Doc(path).Set(ctx, data)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "success"})
}

func firestoreDeleteDocument(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	path := r.Str("path")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	_, err := g.firestoreClient.Doc(path).Delete(ctx)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(map[string]string{"status": "success"})
}

func firestoreQuery(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	collection := r.Str("collection")
	whereField := r.Str("where_field")
	whereOp := r.Str("where_op")
	whereValue := r.Str("where_value")
	orderBy := r.Str("order_by")
	orderDir := r.Str("order_dir")
	limit := r.Int("limit")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	q := g.firestoreClient.Collection(collection).Query

	if whereField != "" {
		var value any
		if err := json.Unmarshal([]byte(whereValue), &value); err != nil {
			value = whereValue
		}
		q = q.Where(whereField, whereOp, value)
	}

	if orderBy != "" {
		dir := firestore.Asc
		if orderDir == "desc" {
			dir = firestore.Desc
		}
		q = q.OrderBy(orderBy, dir)
	}

	if limit > 0 {
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
	return mcp.JSONResult(results)
}
