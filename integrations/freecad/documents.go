package freecad

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func getVersion(ctx context.Context, f *freecad, _ map[string]any) (*mcp.ToolResult, error) {
	return f.execPython(ctx, `
v = FreeCAD.Version()
_result_ = {"version": v[0] + "." + v[1] + "." + v[2], "revision": v[3], "build_date": v[5] if len(v) > 5 else "", "platform": v[6] if len(v) > 6 else ""}
`)
}

func listDocuments(ctx context.Context, f *freecad, _ map[string]any) (*mcp.ToolResult, error) {
	return f.execPython(ctx, `
import os
docs = []
for doc in FreeCAD.listDocuments().values():
    info = {"name": doc.Name, "label": doc.Label, "path": doc.FileName or "", "objects": len(doc.Objects), "modified": doc.Modified if hasattr(doc, "Modified") else False}
    docs.append(info)
_result_ = docs
`)
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

	return f.execPython(ctx, fmt.Sprintf(`
doc = FreeCAD.newDocument(%q)
doc.Label = %q
doc.saveAs(%q)
_result_ = {
    "name": doc.Name,
    "label": doc.Label,
    "file_path": %q,
    "objects": len(doc.Objects)
}
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

	return f.execPython(ctx, fmt.Sprintf(`
import os
fp = %q
doc = None
for d in FreeCAD.listDocuments().values():
    if d.FileName == fp:
        doc = d
        break
if doc is None:
    doc = FreeCAD.open(fp)
objects = []
for obj in doc.Objects:
    info = {"name": obj.Name, "label": obj.Label, "type": obj.TypeId}
    if hasattr(obj, "Shape") and obj.Shape and not obj.Shape.isNull():
        info["shape_type"] = obj.Shape.ShapeType
        info["volume"] = round(obj.Shape.Volume, 4)
    objects.append(info)
_result_ = {
    "name": doc.Name,
    "label": doc.Label,
    "file_path": fp,
    "object_count": len(doc.Objects),
    "objects": objects
}
`, fp))
}

func saveDocument(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
	docName, _ := mcp.ArgStr(args, "doc_name")
	filePath, _ := mcp.ArgStr(args, "file_path")

	return f.execPython(ctx, fmt.Sprintf(`
doc_name = %q
file_path = %q
if doc_name:
    doc = FreeCAD.getDocument(doc_name)
else:
    doc = FreeCAD.ActiveDocument
if doc is None:
    raise ValueError("No document found")
if file_path:
    doc.saveAs(file_path)
else:
    doc.save()
_result_ = {"status": "saved", "name": doc.Name, "path": doc.FileName}
`, docName, filePath))
}

func closeDocument(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
	docName, _ := mcp.ArgStr(args, "doc_name")

	return f.execPython(ctx, fmt.Sprintf(`
doc_name = %q
if doc_name:
    FreeCAD.closeDocument(doc_name)
else:
    doc = FreeCAD.ActiveDocument
    if doc:
        FreeCAD.closeDocument(doc.Name)
    else:
        raise ValueError("No active document")
_result_ = {"status": "closed"}
`, docName))
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

	return f.execPython(ctx, fmt.Sprintf(`
import os
fp = %q
doc = None
for d in FreeCAD.listDocuments().values():
    if d.FileName == fp:
        doc = d
        break
if doc is None:
    doc = FreeCAD.open(fp)
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
_result_ = {
    "name": doc.Name,
    "label": doc.Label,
    "file_path": fp,
    "object_count": len(doc.Objects),
    "objects": objects
}
`, fp))
}
