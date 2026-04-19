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

	if objName != "" {
		return f.execPython(ctx, fmt.Sprintf(`
import os
import Part
fp = %q
doc = None
for d in FreeCAD.listDocuments().values():
    if d.FileName == fp:
        doc = d
        break
if doc is None:
    doc = FreeCAD.open(fp)
obj = doc.getObject(%q)
if obj is None:
    raise ValueError("object not found: %s")
obj.Shape.exportStep(%q)
_result_ = {"status": "exported", "format": "STEP", "file": %q, "size": os.path.getsize(%q)}
`, fp, objName, objName, op, op, op))
	}

	return f.execPython(ctx, fmt.Sprintf(`
import os
import Part
fp = %q
doc = None
for d in FreeCAD.listDocuments().values():
    if d.FileName == fp:
        doc = d
        break
if doc is None:
    doc = FreeCAD.open(fp)
shapes = [obj.Shape for obj in doc.Objects if hasattr(obj, "Shape") and obj.Shape and not obj.Shape.isNull()]
if not shapes:
    raise ValueError("no shapes to export")
if len(shapes) == 1:
    shapes[0].exportStep(%q)
else:
    compound = Part.makeCompound(shapes)
    compound.exportStep(%q)
_result_ = {"status": "exported", "format": "STEP", "file": %q, "size": os.path.getsize(%q), "shapes_exported": len(shapes)}
`, fp, op, op, op, op))
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

	if objName != "" {
		return f.execPython(ctx, fmt.Sprintf(`
import os
import Mesh
import MeshPart
fp = %q
doc = None
for d in FreeCAD.listDocuments().values():
    if d.FileName == fp:
        doc = d
        break
if doc is None:
    doc = FreeCAD.open(fp)
obj = doc.getObject(%q)
if obj is None:
    raise ValueError("object not found: %s")
mesh = MeshPart.meshFromShape(obj.Shape, LinearDeflection=%f)
mesh.write(%q)
_result_ = {"status": "exported", "format": "STL", "file": %q, "size": os.path.getsize(%q), "triangles": mesh.CountFacets, "vertices": mesh.CountPoints}
`, fp, objName, objName, tolerance, op, op, op))
	}

	return f.execPython(ctx, fmt.Sprintf(`
import os
import Part
import Mesh
import MeshPart
fp = %q
doc = None
for d in FreeCAD.listDocuments().values():
    if d.FileName == fp:
        doc = d
        break
if doc is None:
    doc = FreeCAD.open(fp)
shapes = [obj.Shape for obj in doc.Objects if hasattr(obj, "Shape") and obj.Shape and not obj.Shape.isNull()]
if not shapes:
    raise ValueError("no shapes to export")
if len(shapes) == 1:
    shape = shapes[0]
else:
    shape = Part.makeCompound(shapes)
mesh = MeshPart.meshFromShape(shape, LinearDeflection=%f)
mesh.write(%q)
_result_ = {"status": "exported", "format": "STL", "file": %q, "size": os.path.getsize(%q), "triangles": mesh.CountFacets, "vertices": mesh.CountPoints}
`, fp, tolerance, op, op, op))
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

	if objName != "" {
		return f.execPython(ctx, fmt.Sprintf(`
import os
import Part
fp = %q
doc = None
for d in FreeCAD.listDocuments().values():
    if d.FileName == fp:
        doc = d
        break
if doc is None:
    doc = FreeCAD.open(fp)
obj = doc.getObject(%q)
if obj is None:
    raise ValueError("object not found: %s")
obj.Shape.exportBrep(%q)
_result_ = {"status": "exported", "format": "BRep", "file": %q, "size": os.path.getsize(%q)}
`, fp, objName, objName, op, op, op))
	}

	return f.execPython(ctx, fmt.Sprintf(`
import os
import Part
fp = %q
doc = None
for d in FreeCAD.listDocuments().values():
    if d.FileName == fp:
        doc = d
        break
if doc is None:
    doc = FreeCAD.open(fp)
shapes = [obj.Shape for obj in doc.Objects if hasattr(obj, "Shape") and obj.Shape and not obj.Shape.isNull()]
if not shapes:
    raise ValueError("no shapes to export")
if len(shapes) == 1:
    shapes[0].exportBrep(%q)
else:
    compound = Part.makeCompound(shapes)
    compound.exportBrep(%q)
_result_ = {"status": "exported", "format": "BRep", "file": %q, "size": os.path.getsize(%q), "shapes_exported": len(shapes)}
`, fp, op, op, op, op))
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

	return f.execPython(ctx, fmt.Sprintf(`
import os
import Part
fp = %q
doc = None
for d in FreeCAD.listDocuments().values():
    if d.FileName == fp:
        doc = d
        break
if doc is None:
    doc = FreeCAD.open(fp)
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
_result_ = {
    "status": "imported",
    "source": %q,
    "new_objects": after - before,
    "total_objects": after,
    "objects": new_objects
}
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

	fullScript := ""
	if filePath != "" {
		fp := f.filePath(filePath)
		fullScript += fmt.Sprintf(`
import os
fp = %q
doc = None
for d in FreeCAD.listDocuments().values():
    if d.FileName == fp:
        doc = d
        break
if doc is None:
    doc = FreeCAD.open(fp)
`, fp)
	}
	fullScript += script

	return f.execPython(ctx, fullScript)
}

func getScreenshot(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
	width := optFloat(args, "width", 800)
	height := optFloat(args, "height", 600)
	viewAngle, _ := mcp.ArgStr(args, "view_angle")
	if viewAngle == "" {
		viewAngle = "Isometric"
	}

	return f.execPython(ctx, fmt.Sprintf(`
import base64, tempfile, os

if not FreeCAD.GuiUp:
    raise RuntimeError("Screenshot requires FreeCAD GUI mode")

doc = FreeCAD.ActiveDocument
if doc is None:
    raise ValueError("No active document")

view = None
views_3d = FreeCADGui.ActiveDocument.mdiViewsOfType("Gui::View3DInventor")
if views_3d:
    view = views_3d[0]
if view is None:
    view = FreeCADGui.ActiveDocument.ActiveView

view_class = type(view).__name__
if view_class not in ("View3DInventor", "View3DInventorPy"):
    raise ValueError("No 3D viewport available (active view is " + view_class + ")")

angle = %q
if angle == "FitAll":
    view.fitAll()
elif angle == "Isometric":
    view.viewIsometric()
elif angle == "Front":
    view.viewFront()
elif angle == "Back":
    view.viewRear()
elif angle == "Top":
    view.viewTop()
elif angle == "Bottom":
    view.viewBottom()
elif angle == "Left":
    view.viewLeft()
elif angle == "Right":
    view.viewRight()

with tempfile.NamedTemporaryFile(suffix=".png", delete=False) as tmp:
    tmp_path = tmp.name

view.saveImage(tmp_path, int(%d), int(%d), "Current")

with open(tmp_path, "rb") as img:
    image_data = base64.b64encode(img.read()).decode("utf-8")

file_size = os.path.getsize(tmp_path)
os.unlink(tmp_path)

_result_ = {
    "format": "png",
    "width": int(%d),
    "height": int(%d),
    "size_bytes": file_size,
    "data": image_data
}
`, viewAngle, int(width), int(height), int(width), int(height)))
}
