package freecad

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func listObjects(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
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
        info["vertices"] = len(s.Vertexes)
        info["edges"] = len(s.Edges)
        info["faces"] = len(s.Faces)
    p = obj.Placement
    info["placement"] = {
        "x": round(p.Base.x, 4), "y": round(p.Base.y, 4), "z": round(p.Base.z, 4)
    }
    objects.append(info)
_result_ = objects
`, fp))
}

func getObject(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
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
info = {"name": obj.Name, "label": obj.Label, "type": obj.TypeId}
p = obj.Placement
info["placement"] = {
    "base": {"x": round(p.Base.x, 4), "y": round(p.Base.y, 4), "z": round(p.Base.z, 4)},
    "rotation": str(p.Rotation)
}
if hasattr(obj, "Shape") and obj.Shape and not obj.Shape.isNull():
    s = obj.Shape
    info["shape_type"] = s.ShapeType
    info["volume"] = round(s.Volume, 4)
    info["area"] = round(s.Area, 4)
    com = s.CenterOfMass
    info["center_of_mass"] = {"x": round(com.x, 4), "y": round(com.y, 4), "z": round(com.z, 4)}
    info["vertices"] = len(s.Vertexes)
    info["edges"] = len(s.Edges)
    info["faces"] = len(s.Faces)
    bb = s.BoundBox
    info["bounding_box"] = {
        "x_min": round(bb.XMin, 4), "x_max": round(bb.XMax, 4),
        "y_min": round(bb.YMin, 4), "y_max": round(bb.YMax, 4),
        "z_min": round(bb.ZMin, 4), "z_max": round(bb.ZMax, 4)
    }
props = {}
for pname in obj.PropertiesList:
    try:
        val = obj.getPropertyByName(pname)
        if isinstance(val, (int, float, str, bool)):
            props[pname] = val
        elif hasattr(val, "Value"):
            props[pname] = val.Value
        else:
            props[pname] = str(val)
    except:
        pass
info["properties"] = props
_result_ = info
`, fp, objName, objName))
}

func deleteObject(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
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
doc.removeObject(%q)
doc.recompute()
doc.save()
_result_ = {"status": "deleted", "object": %q, "remaining_objects": len(doc.Objects)}
`, fp, objName, objName, objName, objName))
}

func setPlacement(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	filePath := r.Str("file_path")
	objName := r.Str("object_name")
	x := r.Float64("x")
	y := r.Float64("y")
	z := r.Float64("z")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if filePath == "" {
		return mcp.ErrResult(fmt.Errorf("file_path is required"))
	}
	if objName == "" {
		return mcp.ErrResult(fmt.Errorf("object_name is required"))
	}
	yaw, _ := mcp.ArgFloat64(args, "yaw")
	pitch, _ := mcp.ArgFloat64(args, "pitch")
	roll, _ := mcp.ArgFloat64(args, "roll")

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
obj.Placement = FreeCAD.Placement(
    FreeCAD.Vector(%f, %f, %f),
    FreeCAD.Rotation(%f, %f, %f)
)
doc.recompute()
doc.save()
p = obj.Placement
_result_ = {
    "status": "placed",
    "object": %q,
    "position": {"x": round(p.Base.x, 4), "y": round(p.Base.y, 4), "z": round(p.Base.z, 4)}
}
`, fp, objName, objName, x, y, z, yaw, pitch, roll, objName))
}

func getProperties(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
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
props = {}
for pname in obj.PropertiesList:
    try:
        val = obj.getPropertyByName(pname)
        if isinstance(val, (int, float, str, bool)):
            props[pname] = val
        elif hasattr(val, "Value"):
            props[pname] = val.Value
        else:
            props[pname] = str(val)
    except:
        pass
_result_ = {"object": obj.Name, "label": obj.Label, "type": obj.TypeId, "properties": props}
`, fp, objName, objName))
}

func setProperty(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	filePath := r.Str("file_path")
	objName := r.Str("object_name")
	prop := r.Str("property")
	value := r.Str("value")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if filePath == "" {
		return mcp.ErrResult(fmt.Errorf("file_path is required"))
	}
	if objName == "" {
		return mcp.ErrResult(fmt.Errorf("object_name is required"))
	}
	if prop == "" {
		return mcp.ErrResult(fmt.Errorf("property is required"))
	}
	if value == "" {
		return mcp.ErrResult(fmt.Errorf("value is required"))
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
prop = %q
val_str = %q
try:
    val = float(val_str)
    setattr(obj, prop, val)
except (ValueError, TypeError):
    setattr(obj, prop, val_str)
doc.recompute()
doc.save()
new_val = obj.getPropertyByName(prop)
if hasattr(new_val, "Value"):
    new_val = new_val.Value
_result_ = {"status": "set", "object": %q, "property": prop, "value": str(new_val)}
`, fp, objName, objName, prop, value, objName))
}
