package freecad

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

// --- Boolean operations ---

func booleanCut(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
	return booleanOp(ctx, f, args, "Cut")
}

func booleanFuse(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
	return booleanOp(ctx, f, args, "Fuse")
}

func booleanCommon(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
	return booleanOp(ctx, f, args, "Common")
}

func booleanOp(ctx context.Context, f *freecad, args map[string]any, op string) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	filePath := r.Str("file_path")
	base := r.Str("base")
	tool := r.Str("tool")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if filePath == "" {
		return mcp.ErrResult(fmt.Errorf("file_path is required"))
	}
	if base == "" {
		return mcp.ErrResult(fmt.Errorf("base is required"))
	}
	if tool == "" {
		return mcp.ErrResult(fmt.Errorf("tool is required"))
	}
	name, _ := mcp.ArgStr(args, "name")
	if name == "" {
		name = op
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
base_obj = doc.getObject(%q)
tool_obj = doc.getObject(%q)
if base_obj is None:
    raise ValueError("base object not found: %s")
if tool_obj is None:
    raise ValueError("tool object not found: %s")
obj = doc.addObject("Part::%s", %q)
obj.Base = base_obj
obj.Tool = tool_obj
doc.recompute()
doc.save()
s = obj.Shape
_result_ = {
    "name": obj.Name, "label": obj.Label, "type": obj.TypeId,
    "volume": round(s.Volume, 4), "area": round(s.Area, 4),
    "vertices": len(s.Vertexes), "edges": len(s.Edges), "faces": len(s.Faces)
}
`, fp, base, tool, base, tool, op, name))
}

// --- Shape operations ---

func fillet(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	filePath := r.Str("file_path")
	objName := r.Str("object_name")
	radius := r.Float64("radius")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if filePath == "" {
		return mcp.ErrResult(fmt.Errorf("file_path is required"))
	}
	if objName == "" {
		return mcp.ErrResult(fmt.Errorf("object_name is required"))
	}
	name, _ := mcp.ArgStr(args, "name")
	if name == "" {
		name = "Fillet"
	}

	fp := f.filePath(filePath)

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
shape = obj.Shape
filleted = shape.makeFillet(%f, shape.Edges)
result = doc.addObject("Part::Feature", %q)
result.Shape = filleted
doc.recompute()
doc.save()
s = result.Shape
_result_ = {
    "name": result.Name, "label": result.Label,
    "volume": round(s.Volume, 4), "area": round(s.Area, 4),
    "faces": len(s.Faces), "edges": len(s.Edges)
}
`, fp, objName, objName, radius, name))
}

func chamfer(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	filePath := r.Str("file_path")
	objName := r.Str("object_name")
	size := r.Float64("size")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if filePath == "" {
		return mcp.ErrResult(fmt.Errorf("file_path is required"))
	}
	if objName == "" {
		return mcp.ErrResult(fmt.Errorf("object_name is required"))
	}
	name, _ := mcp.ArgStr(args, "name")
	if name == "" {
		name = "Chamfer"
	}

	fp := f.filePath(filePath)

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
shape = obj.Shape
chamfered = shape.makeChamfer(%f, shape.Edges)
result = doc.addObject("Part::Feature", %q)
result.Shape = chamfered
doc.recompute()
doc.save()
s = result.Shape
_result_ = {
    "name": result.Name, "label": result.Label,
    "volume": round(s.Volume, 4), "area": round(s.Area, 4),
    "faces": len(s.Faces), "edges": len(s.Edges)
}
`, fp, objName, objName, size, name))
}

func extrude(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	filePath := r.Str("file_path")
	objName := r.Str("object_name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if filePath == "" {
		return mcp.ErrResult(fmt.Errorf("file_path is required"))
	}
	if objName == "" {
		return mcp.ErrResult(fmt.Errorf("object_name is required"))
	}
	dx := optFloat(args, "dx", 0)
	dy := optFloat(args, "dy", 0)
	dz := optFloat(args, "dz", 10)
	name, _ := mcp.ArgStr(args, "name")
	if name == "" {
		name = "Extrude"
	}

	fp := f.filePath(filePath)

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
shape = obj.Shape
extruded = shape.extrude(FreeCAD.Vector(%f, %f, %f))
result = doc.addObject("Part::Feature", %q)
result.Shape = extruded
doc.recompute()
doc.save()
s = result.Shape
_result_ = {
    "name": result.Name, "label": result.Label,
    "shape_type": s.ShapeType,
    "volume": round(s.Volume, 4), "area": round(s.Area, 4)
}
`, fp, objName, objName, dx, dy, dz, name))
}

func mirror(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	filePath := r.Str("file_path")
	objName := r.Str("object_name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if filePath == "" {
		return mcp.ErrResult(fmt.Errorf("file_path is required"))
	}
	if objName == "" {
		return mcp.ErrResult(fmt.Errorf("object_name is required"))
	}
	plane, _ := mcp.ArgStr(args, "plane")
	if plane == "" {
		plane = "yz"
	}
	name, _ := mcp.ArgStr(args, "name")
	if name == "" {
		name = "Mirror"
	}

	// Map plane name to normal vector
	var nx, ny, nz float64
	switch plane {
	case "xy":
		nz = 1
	case "xz":
		ny = 1
	default: // yz
		nx = 1
	}

	fp := f.filePath(filePath)

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
shape = obj.Shape
mirrored = shape.mirror(FreeCAD.Vector(0,0,0), FreeCAD.Vector(%f, %f, %f))
result = doc.addObject("Part::Feature", %q)
result.Shape = mirrored
doc.recompute()
doc.save()
s = result.Shape
_result_ = {
    "name": result.Name, "label": result.Label,
    "volume": round(s.Volume, 4), "area": round(s.Area, 4)
}
`, fp, objName, objName, nx, ny, nz, name))
}
