package freecad

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

func getVersion(ctx context.Context, f *freecad, _ map[string]any) (*mcp.ToolResult, error) {
	return jsonScriptResult(ctx, f, `
import FreeCAD
import json
v = FreeCAD.Version()
print(json.dumps({
    "version": v[0] + "." + v[1] + "." + v[2],
    "revision": v[3],
    "build_date": v[5] if len(v) > 5 else "",
    "platform": v[6] if len(v) > 6 else ""
}))
`)
}

func listDocuments(_ context.Context, f *freecad, _ map[string]any) (*mcp.ToolResult, error) {
	entries, err := os.ReadDir(f.dataDir)
	if err != nil {
		return mcp.ErrResult(fmt.Errorf("cannot read data_dir: %w", err))
	}
	var docs []map[string]any
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".fcstd") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		docs = append(docs, map[string]any{
			"name":     e.Name(),
			"path":     filepath.Join(f.dataDir, e.Name()),
			"size":     info.Size(),
			"modified": info.ModTime().Format("2006-01-02T15:04:05Z"),
		})
	}
	if docs == nil {
		docs = []map[string]any{}
	}
	return mcp.JSONResult(docs)
}

func createDocument(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if name == "" {
		return mcp.ErrResult(fmt.Errorf("name is required"))
	}
	label, _ := mcp.ArgStr(args, "label")
	if label == "" {
		label = name
	}

	fp := f.filePath(name + ".FCStd")

	return jsonScriptResult(ctx, f, fmt.Sprintf(`
import FreeCAD
import json
doc = FreeCAD.newDocument(%q)
doc.Label = %q
doc.saveAs(%q)
print(json.dumps({
    "name": doc.Name,
    "label": doc.Label,
    "file_path": %q,
    "objects": len(doc.Objects)
}))
FreeCAD.closeDocument(doc.Name)
`, name, label, fp, fp))
}

func openDocument(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	filePath := r.Str("file_path")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if filePath == "" {
		return mcp.ErrResult(fmt.Errorf("file_path is required"))
	}

	fp := f.filePath(filePath)

	return jsonScriptResult(ctx, f, fmt.Sprintf(`
import FreeCAD
import json
doc = FreeCAD.open(%q)
objects = []
for obj in doc.Objects:
    info = {"name": obj.Name, "label": obj.Label, "type": obj.TypeId}
    if hasattr(obj, "Shape") and obj.Shape and not obj.Shape.isNull():
        info["shape_type"] = obj.Shape.ShapeType
        info["volume"] = round(obj.Shape.Volume, 4)
    objects.append(info)
print(json.dumps({
    "name": doc.Name,
    "label": doc.Label,
    "file_path": %q,
    "object_count": len(doc.Objects),
    "objects": objects
}))
FreeCAD.closeDocument(doc.Name)
`, fp, fp))
}

func getDocument(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	filePath := r.Str("file_path")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if filePath == "" {
		return mcp.ErrResult(fmt.Errorf("file_path is required"))
	}

	fp := f.filePath(filePath)

	return jsonScriptResult(ctx, f, fmt.Sprintf(`
import FreeCAD
import json
doc = FreeCAD.open(%q)
objects = []
for obj in doc.Objects:
    info = {"name": obj.Name, "label": obj.Label, "type": obj.TypeId}
    if hasattr(obj, "Shape") and obj.Shape and not obj.Shape.isNull():
        s = obj.Shape
        info["shape_type"] = s.ShapeType
        info["volume"] = round(s.Volume, 4)
        info["area"] = round(s.Area, 4)
        bb = s.BoundBox
        info["bounding_box"] = {
            "x_min": round(bb.XMin, 4), "x_max": round(bb.XMax, 4),
            "y_min": round(bb.YMin, 4), "y_max": round(bb.YMax, 4),
            "z_min": round(bb.ZMin, 4), "z_max": round(bb.ZMax, 4)
        }
    objects.append(info)
print(json.dumps({
    "name": doc.Name,
    "label": doc.Label,
    "file_path": %q,
    "object_count": len(doc.Objects),
    "objects": objects
}))
FreeCAD.closeDocument(doc.Name)
`, fp, fp))
}
