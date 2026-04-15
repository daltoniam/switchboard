package freecad

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func exportSTEP(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	filePath := r.Str("file_path")
	outputPath := r.Str("output_path")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if filePath == "" {
		return mcp.ErrResult(fmt.Errorf("file_path is required"))
	}
	if outputPath == "" {
		return mcp.ErrResult(fmt.Errorf("output_path is required"))
	}
	objName, _ := mcp.ArgStr(args, "object_name")

	fp := f.filePath(filePath)
	op := f.filePath(outputPath)

	script := fmt.Sprintf(`
import FreeCAD
import Part
import json
import os
doc = FreeCAD.open(%q)
`, fp)

	if objName != "" {
		script += fmt.Sprintf(`
obj = doc.getObject(%q)
if obj is None:
    print(json.dumps({"error": "object not found: %s"}))
else:
    obj.Shape.exportStep(%q)
    print(json.dumps({"status": "exported", "format": "STEP", "file": %q, "size": os.path.getsize(%q)}))
`, objName, objName, op, op, op)
	} else {
		script += fmt.Sprintf(`
shapes = [obj.Shape for obj in doc.Objects if hasattr(obj, "Shape") and obj.Shape and not obj.Shape.isNull()]
if not shapes:
    print(json.dumps({"error": "no shapes to export"}))
else:
    if len(shapes) == 1:
        shapes[0].exportStep(%q)
    else:
        compound = Part.makeCompound(shapes)
        compound.exportStep(%q)
    print(json.dumps({"status": "exported", "format": "STEP", "file": %q, "size": os.path.getsize(%q), "shapes_exported": len(shapes)}))
`, op, op, op, op)
	}

	script += "\nFreeCAD.closeDocument(doc.Name)"
	return jsonScriptResult(ctx, f, script)
}

func exportSTL(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	filePath := r.Str("file_path")
	outputPath := r.Str("output_path")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if filePath == "" {
		return mcp.ErrResult(fmt.Errorf("file_path is required"))
	}
	if outputPath == "" {
		return mcp.ErrResult(fmt.Errorf("output_path is required"))
	}
	objName, _ := mcp.ArgStr(args, "object_name")
	tolerance := optFloat(args, "mesh_tolerance", 0.1)

	fp := f.filePath(filePath)
	op := f.filePath(outputPath)

	script := fmt.Sprintf(`
import FreeCAD
import Mesh
import MeshPart
import json
import os
doc = FreeCAD.open(%q)
`, fp)

	if objName != "" {
		script += fmt.Sprintf(`
obj = doc.getObject(%q)
if obj is None:
    print(json.dumps({"error": "object not found: %s"}))
else:
    mesh = MeshPart.meshFromShape(obj.Shape, LinearDeflection=%f)
    mesh.write(%q)
    print(json.dumps({"status": "exported", "format": "STL", "file": %q, "size": os.path.getsize(%q), "triangles": mesh.CountFacets, "vertices": mesh.CountPoints}))
`, objName, objName, tolerance, op, op, op)
	} else {
		script += fmt.Sprintf(`
shapes = [obj.Shape for obj in doc.Objects if hasattr(obj, "Shape") and obj.Shape and not obj.Shape.isNull()]
if not shapes:
    print(json.dumps({"error": "no shapes to export"}))
else:
    import Part
    if len(shapes) == 1:
        shape = shapes[0]
    else:
        shape = Part.makeCompound(shapes)
    mesh = MeshPart.meshFromShape(shape, LinearDeflection=%f)
    mesh.write(%q)
    print(json.dumps({"status": "exported", "format": "STL", "file": %q, "size": os.path.getsize(%q), "triangles": mesh.CountFacets, "vertices": mesh.CountPoints}))
`, tolerance, op, op, op)
	}

	script += "\nFreeCAD.closeDocument(doc.Name)"
	return jsonScriptResult(ctx, f, script)
}

func exportBRep(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	filePath := r.Str("file_path")
	outputPath := r.Str("output_path")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if filePath == "" {
		return mcp.ErrResult(fmt.Errorf("file_path is required"))
	}
	if outputPath == "" {
		return mcp.ErrResult(fmt.Errorf("output_path is required"))
	}
	objName, _ := mcp.ArgStr(args, "object_name")

	fp := f.filePath(filePath)
	op := f.filePath(outputPath)

	script := fmt.Sprintf(`
import FreeCAD
import Part
import json
import os
doc = FreeCAD.open(%q)
`, fp)

	if objName != "" {
		script += fmt.Sprintf(`
obj = doc.getObject(%q)
if obj is None:
    print(json.dumps({"error": "object not found: %s"}))
else:
    obj.Shape.exportBrep(%q)
    print(json.dumps({"status": "exported", "format": "BRep", "file": %q, "size": os.path.getsize(%q)}))
`, objName, objName, op, op, op)
	} else {
		script += fmt.Sprintf(`
shapes = [obj.Shape for obj in doc.Objects if hasattr(obj, "Shape") and obj.Shape and not obj.Shape.isNull()]
if not shapes:
    print(json.dumps({"error": "no shapes to export"}))
else:
    if len(shapes) == 1:
        shapes[0].exportBrep(%q)
    else:
        compound = Part.makeCompound(shapes)
        compound.exportBrep(%q)
    print(json.dumps({"status": "exported", "format": "BRep", "file": %q, "size": os.path.getsize(%q), "shapes_exported": len(shapes)}))
`, op, op, op, op)
	}

	script += "\nFreeCAD.closeDocument(doc.Name)"
	return jsonScriptResult(ctx, f, script)
}

func importFile(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	filePath := r.Str("file_path")
	importPath := r.Str("import_path")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if filePath == "" {
		return mcp.ErrResult(fmt.Errorf("file_path is required"))
	}
	if importPath == "" {
		return mcp.ErrResult(fmt.Errorf("import_path is required"))
	}

	fp := f.filePath(filePath)
	ip := f.filePath(importPath)

	return jsonScriptResult(ctx, f, fmt.Sprintf(`
import FreeCAD
import Part
import json
doc = FreeCAD.open(%q)
before = len(doc.Objects)
Part.insert(%q, doc.Name)
doc.recompute()
doc.save()
after = len(doc.Objects)
new_objects = []
for obj in doc.Objects[before:]:
    info = {"name": obj.Name, "label": obj.Label, "type": obj.TypeId}
    if hasattr(obj, "Shape") and obj.Shape and not obj.Shape.isNull():
        info["volume"] = round(obj.Shape.Volume, 4)
        info["shape_type"] = obj.Shape.ShapeType
    new_objects.append(info)
print(json.dumps({
    "status": "imported",
    "source": %q,
    "new_objects": after - before,
    "total_objects": after,
    "objects": new_objects
}))
FreeCAD.closeDocument(doc.Name)
`, fp, ip, ip))
}

func runUserScript(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	script := r.Str("script")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if script == "" {
		return mcp.ErrResult(fmt.Errorf("script is required"))
	}
	filePath, _ := mcp.ArgStr(args, "file_path")

	fullScript := "import FreeCAD\nimport Part\nimport Mesh\nimport json\n"
	if filePath != "" {
		fp := f.filePath(filePath)
		fullScript += fmt.Sprintf("doc = FreeCAD.open(%q)\n", fp)
	}
	fullScript += script
	if filePath != "" {
		fullScript += "\nif FreeCAD.ActiveDocument:\n    FreeCAD.closeDocument(FreeCAD.ActiveDocument.Name)"
	}

	return scriptResult(ctx, f, fullScript)
}
