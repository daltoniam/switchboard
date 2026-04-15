package freecad

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func measureShape(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
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
obj = doc.getObject(%q)
if obj is None:
    raise ValueError("object not found: %s")
if not hasattr(obj, "Shape") or obj.Shape.isNull():
    raise ValueError("object has no shape: %s")
s = obj.Shape
com = s.CenterOfMass
bb = s.BoundBox
_result_ = {
    "object": obj.Name,
    "shape_type": s.ShapeType,
    "volume_mm3": round(s.Volume, 6),
    "area_mm2": round(s.Area, 6),
    "center_of_mass": {"x": round(com.x, 4), "y": round(com.y, 4), "z": round(com.z, 4)},
    "vertices": len(s.Vertexes),
    "edges": len(s.Edges),
    "faces": len(s.Faces),
    "solids": len(s.Solids),
    "shells": len(s.Shells),
    "wires": len(s.Wires),
    "bounding_box": {
        "x_length": round(bb.XLength, 4),
        "y_length": round(bb.YLength, 4),
        "z_length": round(bb.ZLength, 4),
        "diagonal": round(bb.DiagonalLength, 4)
    }
}
`, fp, objName, objName, objName))
}

func checkGeometry(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
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
obj = doc.getObject(%q)
if obj is None:
    raise ValueError("object not found: %s")
if not hasattr(obj, "Shape") or obj.Shape.isNull():
    raise ValueError("object has no shape: %s")
s = obj.Shape
valid = s.isValid()
closed = s.isClosed() if hasattr(s, "isClosed") else None
checks = {
    "object": obj.Name,
    "is_valid": valid,
    "shape_type": s.ShapeType,
    "solids": len(s.Solids),
    "shells": len(s.Shells),
    "faces": len(s.Faces),
    "edges": len(s.Edges),
    "vertices": len(s.Vertexes)
}
if closed is not None:
    checks["is_closed"] = closed
errors = []
try:
    if not valid:
        errors.append("Shape is not valid (BRep_Builder check failed)")
    for i, face in enumerate(s.Faces):
        if not face.isValid():
            errors.append("Face %%d is invalid" %% i)
    for i, edge in enumerate(s.Edges):
        if not edge.isValid():
            errors.append("Edge %%d is invalid" %% i)
except Exception as e:
    errors.append(str(e))
checks["errors"] = errors
checks["error_count"] = len(errors)
_result_ = checks
`, fp, objName, objName, objName))
}

func boundingBox(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
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
obj = doc.getObject(%q)
if obj is None:
    raise ValueError("object not found: %s")
if not hasattr(obj, "Shape") or obj.Shape.isNull():
    raise ValueError("object has no shape: %s")
bb = obj.Shape.BoundBox
_result_ = {
    "object": obj.Name,
    "x_min": round(bb.XMin, 4), "x_max": round(bb.XMax, 4),
    "y_min": round(bb.YMin, 4), "y_max": round(bb.YMax, 4),
    "z_min": round(bb.ZMin, 4), "z_max": round(bb.ZMax, 4),
    "x_length": round(bb.XLength, 4),
    "y_length": round(bb.YLength, 4),
    "z_length": round(bb.ZLength, 4),
    "diagonal": round(bb.DiagonalLength, 4),
    "center": {"x": round(bb.Center.x, 4), "y": round(bb.Center.y, 4), "z": round(bb.Center.z, 4)}
}
`, fp, objName, objName, objName))
}

func getDocumentErrors(ctx context.Context, f *freecad, _ map[string]any) (*mcp.ToolResult, error) {
	return f.execPython(ctx, `
doc = FreeCAD.ActiveDocument
if doc is None:
    raise ValueError("No active document")
objects = []
for obj in doc.Objects:
    info = {"name": obj.Name, "type": obj.TypeId, "state": list(obj.State) if hasattr(obj, "State") else []}
    try:
        info["valid"] = obj.isValid()
    except:
        info["valid"] = None
    if hasattr(obj, "Shape") and obj.Shape:
        if obj.Shape.isNull():
            info["shape"] = "null"
        else:
            info["shape"] = "valid" if obj.Shape.isValid() else "invalid"
    objects.append(info)
invalid = [o for o in objects if o.get("valid") == False or "Invalid" in o.get("state", []) or o.get("shape") in ("null", "invalid")]
_result_ = {
    "document": doc.Name,
    "total_objects": len(objects),
    "invalid_count": len(invalid),
    "invalid_objects": invalid,
}
`)
}

func getSketchSolverStatus(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	sketchName := r.Str("sketch_name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if sketchName == "" {
		return mcp.ErrResult(fmt.Errorf("sketch_name is required"))
	}
	docName, _ := mcp.ArgStr(args, "doc_name")

	return f.execPython(ctx, fmt.Sprintf(`
import Part, Sketcher
doc = FreeCAD.getDocument(%q) if %q else FreeCAD.ActiveDocument
if doc is None:
    raise ValueError("No document found")
sketch = doc.getObject(%q)
if sketch is None:
    raise ValueError("Sketch not found: " + %q)
result = {
    "name": sketch.Name,
    "geometry_count": sketch.GeometryCount,
    "constraint_count": sketch.ConstraintCount,
    "fully_constrained": sketch.FullyConstrained if hasattr(sketch, "FullyConstrained") else None,
}
try:
    result["dof"] = sketch.DOF
except:
    pass
try:
    result["solver_status"] = sketch.solve()
except:
    pass
if not sketch.FullyConstrained and sketch.GeometryCount > 0:
    dof = result.get("dof", -1)
    result["status"] = "under-constrained"
    result["message"] = "Sketch needs %%d more constraint(s) to be fully defined" %% dof if dof >= 0 else "Sketch is under-constrained"
elif sketch.FullyConstrained:
    result["status"] = "fully-constrained"
    result["message"] = "All geometry is fully defined"
else:
    result["status"] = "empty"
    result["message"] = "Sketch has no geometry"
_result_ = result
`, docName, docName, sketchName, sketchName))
}
