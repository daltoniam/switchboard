package freecad

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func createBody(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
	docName, _ := mcp.ArgStr(args, "doc_name")
	name, _ := mcp.ArgStr(args, "name")
	if name == "" {
		name = "Body"
	}

	return f.execPython(ctx, fmt.Sprintf(`
doc = FreeCAD.getDocument(%q) if %q else FreeCAD.ActiveDocument
if doc is None:
    raise ValueError("No document found")
body = doc.addObject("PartDesign::Body", %q)
doc.recompute()
_result_ = {"name": body.Name, "label": body.Label, "type": body.TypeId}
`, docName, docName, name))
}

func pad(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	sketchName := r.Str("sketch_name")
	length := r.Float64("length")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if sketchName == "" {
		return mcp.ErrResult(fmt.Errorf("sketch_name is required"))
	}
	docName, _ := mcp.ArgStr(args, "doc_name")
	bodyName, _ := mcp.ArgStr(args, "body_name")
	name, _ := mcp.ArgStr(args, "name")
	if name == "" {
		name = "Pad"
	}
	symmetric, _ := mcp.ArgBool(args, "symmetric")
	reversed, _ := mcp.ArgBool(args, "reversed")

	return f.execPython(ctx, fmt.Sprintf(`
doc = FreeCAD.getDocument(%q) if %q else FreeCAD.ActiveDocument
if doc is None:
    raise ValueError("No document found")
sketch = doc.getObject(%q)
if sketch is None:
    raise ValueError("Sketch not found: " + %q)
body_name = %q
if body_name:
    body = doc.getObject(body_name)
else:
    body = sketch.getParentGeoFeatureGroup()
if body is None:
    raise ValueError("No body found. Create a body first or specify body_name.")
pad = body.newObject("PartDesign::Pad", %q)
pad.Profile = sketch
pad.Length = %f
pad.Midplane = %s
pad.Reversed = %s
doc.recompute()
s = pad.Shape
_result_ = {"name": pad.Name, "length": pad.Length.Value, "volume": round(s.Volume, 4) if s and not s.isNull() else 0, "area": round(s.Area, 4) if s and not s.isNull() else 0}
`, docName, docName, sketchName, sketchName, bodyName, name, length, pyBool(symmetric), pyBool(reversed)))
}

func pocket(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	sketchName := r.Str("sketch_name")
	length := r.Float64("length")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if sketchName == "" {
		return mcp.ErrResult(fmt.Errorf("sketch_name is required"))
	}
	docName, _ := mcp.ArgStr(args, "doc_name")
	bodyName, _ := mcp.ArgStr(args, "body_name")
	name, _ := mcp.ArgStr(args, "name")
	if name == "" {
		name = "Pocket"
	}
	throughAll, _ := mcp.ArgBool(args, "through_all")

	return f.execPython(ctx, fmt.Sprintf(`
doc = FreeCAD.getDocument(%q) if %q else FreeCAD.ActiveDocument
if doc is None:
    raise ValueError("No document found")
sketch = doc.getObject(%q)
if sketch is None:
    raise ValueError("Sketch not found: " + %q)
body_name = %q
if body_name:
    body = doc.getObject(body_name)
else:
    body = sketch.getParentGeoFeatureGroup()
if body is None:
    raise ValueError("No body found")
pocket = body.newObject("PartDesign::Pocket", %q)
pocket.Profile = sketch
if %s:
    pocket.Type = 1
else:
    pocket.Length = %f
doc.recompute()
s = pocket.Shape
_result_ = {"name": pocket.Name, "volume": round(s.Volume, 4) if s and not s.isNull() else 0}
`, docName, docName, sketchName, sketchName, bodyName, name, pyBool(throughAll), length))
}

func revolution(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	sketchName := r.Str("sketch_name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if sketchName == "" {
		return mcp.ErrResult(fmt.Errorf("sketch_name is required"))
	}
	angle := optFloat(args, "angle", 360)
	docName, _ := mcp.ArgStr(args, "doc_name")
	bodyName, _ := mcp.ArgStr(args, "body_name")
	name, _ := mcp.ArgStr(args, "name")
	if name == "" {
		name = "Revolution"
	}
	axis, _ := mcp.ArgStr(args, "axis")
	if axis == "" {
		axis = "V_Axis"
	}

	return f.execPython(ctx, fmt.Sprintf(`
doc = FreeCAD.getDocument(%q) if %q else FreeCAD.ActiveDocument
if doc is None:
    raise ValueError("No document found")
sketch = doc.getObject(%q)
if sketch is None:
    raise ValueError("Sketch not found: " + %q)
body_name = %q
if body_name:
    body = doc.getObject(body_name)
else:
    body = sketch.getParentGeoFeatureGroup()
if body is None:
    raise ValueError("No body found")
rev = body.newObject("PartDesign::Revolution", %q)
rev.Profile = sketch
axis_name = %q
if axis_name in ("V_Axis", "H_Axis", "N_Axis"):
    rev.ReferenceAxis = (sketch, [axis_name])
else:
    rev.ReferenceAxis = (sketch, ["V_Axis"])
rev.Angle = %f
doc.recompute()
s = rev.Shape
_result_ = {"name": rev.Name, "angle": rev.Angle.Value, "volume": round(s.Volume, 4) if s and not s.isNull() else 0}
`, docName, docName, sketchName, sketchName, bodyName, name, axis, angle))
}

func groove(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	sketchName := r.Str("sketch_name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if sketchName == "" {
		return mcp.ErrResult(fmt.Errorf("sketch_name is required"))
	}
	angle := optFloat(args, "angle", 360)
	docName, _ := mcp.ArgStr(args, "doc_name")
	bodyName, _ := mcp.ArgStr(args, "body_name")
	name, _ := mcp.ArgStr(args, "name")
	if name == "" {
		name = "Groove"
	}
	axis, _ := mcp.ArgStr(args, "axis")
	if axis == "" {
		axis = "V_Axis"
	}

	return f.execPython(ctx, fmt.Sprintf(`
doc = FreeCAD.getDocument(%q) if %q else FreeCAD.ActiveDocument
if doc is None:
    raise ValueError("No document found")
sketch = doc.getObject(%q)
if sketch is None:
    raise ValueError("Sketch not found: " + %q)
body_name = %q
if body_name:
    body = doc.getObject(body_name)
else:
    body = sketch.getParentGeoFeatureGroup()
if body is None:
    raise ValueError("No body found")
groove = body.newObject("PartDesign::Groove", %q)
groove.Profile = sketch
axis_name = %q
if axis_name in ("V_Axis", "H_Axis", "N_Axis"):
    groove.ReferenceAxis = (sketch, [axis_name])
else:
    groove.ReferenceAxis = (sketch, ["V_Axis"])
groove.Angle = %f
doc.recompute()
s = groove.Shape
_result_ = {"name": groove.Name, "angle": groove.Angle.Value, "volume": round(s.Volume, 4) if s and not s.isNull() else 0}
`, docName, docName, sketchName, sketchName, bodyName, name, axis, angle))
}

func pdFillet(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	radius := r.Float64("radius")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	docName, _ := mcp.ArgStr(args, "doc_name")
	bodyName, _ := mcp.ArgStr(args, "body_name")
	edgeIndices, _ := mcp.ArgStr(args, "edges")
	name, _ := mcp.ArgStr(args, "name")
	if name == "" {
		name = "Fillet"
	}

	return f.execPython(ctx, fmt.Sprintf(`
import json as _json
doc = FreeCAD.getDocument(%q) if %q else FreeCAD.ActiveDocument
if doc is None:
    raise ValueError("No document found")
body_name = %q
if body_name:
    body = doc.getObject(body_name)
else:
    for obj in doc.Objects:
        if obj.TypeId == "PartDesign::Body":
            body = obj
            break
    else:
        raise ValueError("No body found")
tip = body.Tip
if tip is None:
    raise ValueError("Body has no tip feature")
edges_str = %q
fillet = body.newObject("PartDesign::Fillet", %q)
fillet.Base = (tip, [])
fillet.Radius = %f
if edges_str:
    edge_indices = _json.loads(edges_str)
    refs = ["Edge" + str(i) for i in edge_indices]
    fillet.Base = (tip, refs)
else:
    all_edges = ["Edge" + str(i+1) for i in range(len(tip.Shape.Edges))]
    fillet.Base = (tip, all_edges)
doc.recompute()
s = fillet.Shape
_result_ = {"name": fillet.Name, "radius": fillet.Radius.Value, "volume": round(s.Volume, 4) if s and not s.isNull() else 0}
`, docName, docName, bodyName, edgeIndices, name, radius))
}

func pdChamfer(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	size := r.Float64("size")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	docName, _ := mcp.ArgStr(args, "doc_name")
	bodyName, _ := mcp.ArgStr(args, "body_name")
	edgeIndices, _ := mcp.ArgStr(args, "edges")
	name, _ := mcp.ArgStr(args, "name")
	if name == "" {
		name = "Chamfer"
	}

	return f.execPython(ctx, fmt.Sprintf(`
import json as _json
doc = FreeCAD.getDocument(%q) if %q else FreeCAD.ActiveDocument
if doc is None:
    raise ValueError("No document found")
body_name = %q
if body_name:
    body = doc.getObject(body_name)
else:
    for obj in doc.Objects:
        if obj.TypeId == "PartDesign::Body":
            body = obj
            break
    else:
        raise ValueError("No body found")
tip = body.Tip
if tip is None:
    raise ValueError("Body has no tip feature")
edges_str = %q
chamfer = body.newObject("PartDesign::Chamfer", %q)
chamfer.Size = %f
if edges_str:
    edge_indices = _json.loads(edges_str)
    refs = ["Edge" + str(i) for i in edge_indices]
    chamfer.Base = (tip, refs)
else:
    all_edges = ["Edge" + str(i+1) for i in range(len(tip.Shape.Edges))]
    chamfer.Base = (tip, all_edges)
doc.recompute()
s = chamfer.Shape
_result_ = {"name": chamfer.Name, "size": chamfer.Size.Value, "volume": round(s.Volume, 4) if s and not s.isNull() else 0}
`, docName, docName, bodyName, edgeIndices, name, size))
}

func pdMirror(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
	docName, _ := mcp.ArgStr(args, "doc_name")
	bodyName, _ := mcp.ArgStr(args, "body_name")
	plane, _ := mcp.ArgStr(args, "plane")
	if plane == "" {
		plane = "XY_Plane"
	}
	name, _ := mcp.ArgStr(args, "name")
	if name == "" {
		name = "Mirror"
	}

	return f.execPython(ctx, fmt.Sprintf(`
doc = FreeCAD.getDocument(%q) if %q else FreeCAD.ActiveDocument
if doc is None:
    raise ValueError("No document found")
body_name = %q
if body_name:
    body = doc.getObject(body_name)
else:
    for obj in doc.Objects:
        if obj.TypeId == "PartDesign::Body":
            body = obj
            break
    else:
        raise ValueError("No body found")
tip = body.Tip
if tip is None:
    raise ValueError("Body has no tip feature")
mirror = body.newObject("PartDesign::Mirrored", %q)
mirror.Originals = [tip]
plane_name = %q
origin = body.Origin if hasattr(body, "Origin") else None
if origin:
    for p in origin.OriginFeatures:
        if p.Name == plane_name or plane_name in p.Label:
            mirror.MirrorPlane = (p, [""])
            break
doc.recompute()
s = mirror.Shape
_result_ = {"name": mirror.Name, "volume": round(s.Volume, 4) if s and not s.isNull() else 0}
`, docName, docName, bodyName, name, plane))
}

func linearPattern(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	occurrences := r.Int("occurrences")
	length := r.Float64("length")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	docName, _ := mcp.ArgStr(args, "doc_name")
	bodyName, _ := mcp.ArgStr(args, "body_name")
	_, _ = mcp.ArgStr(args, "direction") // reserved for future axis selection
	name, _ := mcp.ArgStr(args, "name")
	if name == "" {
		name = "LinearPattern"
	}

	return f.execPython(ctx, fmt.Sprintf(`
doc = FreeCAD.getDocument(%q) if %q else FreeCAD.ActiveDocument
if doc is None:
    raise ValueError("No document found")
body_name = %q
if body_name:
    body = doc.getObject(body_name)
else:
    for obj in doc.Objects:
        if obj.TypeId == "PartDesign::Body":
            body = obj
            break
    else:
        raise ValueError("No body found")
tip = body.Tip
if tip is None:
    raise ValueError("Body has no tip feature")
pattern = body.newObject("PartDesign::LinearPattern", %q)
pattern.Originals = [tip]
pattern.Length = %f
pattern.Occurrences = %d
doc.recompute()
s = pattern.Shape
_result_ = {"name": pattern.Name, "occurrences": pattern.Occurrences, "length": pattern.Length.Value, "volume": round(s.Volume, 4) if s and not s.isNull() else 0}
`, docName, docName, bodyName, name, length, occurrences))
}

func polarPattern(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	occurrences := r.Int("occurrences")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	angle := optFloat(args, "angle", 360)
	docName, _ := mcp.ArgStr(args, "doc_name")
	bodyName, _ := mcp.ArgStr(args, "body_name")
	name, _ := mcp.ArgStr(args, "name")
	if name == "" {
		name = "PolarPattern"
	}

	return f.execPython(ctx, fmt.Sprintf(`
doc = FreeCAD.getDocument(%q) if %q else FreeCAD.ActiveDocument
if doc is None:
    raise ValueError("No document found")
body_name = %q
if body_name:
    body = doc.getObject(body_name)
else:
    for obj in doc.Objects:
        if obj.TypeId == "PartDesign::Body":
            body = obj
            break
    else:
        raise ValueError("No body found")
tip = body.Tip
if tip is None:
    raise ValueError("Body has no tip feature")
pattern = body.newObject("PartDesign::PolarPattern", %q)
pattern.Originals = [tip]
pattern.Angle = %f
pattern.Occurrences = %d
doc.recompute()
s = pattern.Shape
_result_ = {"name": pattern.Name, "occurrences": pattern.Occurrences, "angle": pattern.Angle.Value if hasattr(pattern.Angle, "Value") else pattern.Angle, "volume": round(s.Volume, 4) if s and not s.isNull() else 0}
`, docName, docName, bodyName, name, angle, occurrences))
}

func pdHole(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	sketchName := r.Str("sketch_name")
	diameter := r.Float64("diameter")
	depth := r.Float64("depth")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if sketchName == "" {
		return mcp.ErrResult(fmt.Errorf("sketch_name is required"))
	}
	docName, _ := mcp.ArgStr(args, "doc_name")
	bodyName, _ := mcp.ArgStr(args, "body_name")
	name, _ := mcp.ArgStr(args, "name")
	if name == "" {
		name = "Hole"
	}
	threaded, _ := mcp.ArgBool(args, "threaded")

	return f.execPython(ctx, fmt.Sprintf(`
doc = FreeCAD.getDocument(%q) if %q else FreeCAD.ActiveDocument
if doc is None:
    raise ValueError("No document found")
sketch = doc.getObject(%q)
if sketch is None:
    raise ValueError("Sketch not found: " + %q)
body_name = %q
if body_name:
    body = doc.getObject(body_name)
else:
    body = sketch.getParentGeoFeatureGroup()
if body is None:
    raise ValueError("No body found")
hole = body.newObject("PartDesign::Hole", %q)
hole.Profile = sketch
hole.Diameter = %f
hole.Depth = %f
hole.Threaded = %s
doc.recompute()
s = hole.Shape
_result_ = {"name": hole.Name, "diameter": hole.Diameter.Value, "depth": hole.Depth.Value, "threaded": hole.Threaded, "volume": round(s.Volume, 4) if s and not s.isNull() else 0}
`, docName, docName, sketchName, sketchName, bodyName, name, diameter, depth, pyBool(threaded)))
}
