package freecad

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func createSketch(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	docName := r.Str("doc_name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	name, _ := mcp.ArgStr(args, "name")
	if name == "" {
		name = "Sketch"
	}
	plane, _ := mcp.ArgStr(args, "plane")
	if plane == "" {
		plane = "XY"
	}
	bodyName, _ := mcp.ArgStr(args, "body_name")

	return f.execPython(ctx, fmt.Sprintf(`
import Part, Sketcher
doc_name = %q
if doc_name:
    doc = FreeCAD.getDocument(doc_name)
else:
    doc = FreeCAD.ActiveDocument
if doc is None:
    raise ValueError("No document found")
body_name = %q
body = doc.getObject(body_name) if body_name else None
if body:
    sketch = body.newObject("Sketcher::SketchObject", %q)
else:
    sketch = doc.addObject("Sketcher::SketchObject", %q)
plane = %q
if plane == "XZ":
    sketch.Placement = FreeCAD.Placement(FreeCAD.Vector(0,0,0), FreeCAD.Rotation(FreeCAD.Vector(1,0,0), 90))
elif plane == "YZ":
    sketch.Placement = FreeCAD.Placement(FreeCAD.Vector(0,0,0), FreeCAD.Rotation(FreeCAD.Vector(0,1,0), -90))
doc.recompute()
_result_ = {"name": sketch.Name, "label": sketch.Label, "plane": plane, "geometry_count": sketch.GeometryCount, "constraint_count": sketch.ConstraintCount}
`, docName, bodyName, name, name, plane))
}

func addSketchLine(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	sketchName := r.Str("sketch_name")
	x1 := r.Float64("x1")
	y1 := r.Float64("y1")
	x2 := r.Float64("x2")
	y2 := r.Float64("y2")
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
idx = sketch.addGeometry(Part.LineSegment(FreeCAD.Vector(%f,%f,0), FreeCAD.Vector(%f,%f,0)), False)
doc.recompute()
_result_ = {"geometry_index": idx, "type": "LineSegment", "geometry_count": sketch.GeometryCount}
`, docName, docName, sketchName, sketchName, x1, y1, x2, y2))
}

func addSketchCircle(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	sketchName := r.Str("sketch_name")
	cx := r.Float64("cx")
	cy := r.Float64("cy")
	radius := r.Float64("radius")
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
idx = sketch.addGeometry(Part.Circle(FreeCAD.Vector(%f,%f,0), FreeCAD.Vector(0,0,1), %f), False)
doc.recompute()
_result_ = {"geometry_index": idx, "type": "Circle", "geometry_count": sketch.GeometryCount}
`, docName, docName, sketchName, sketchName, cx, cy, radius))
}

func addSketchArc(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	sketchName := r.Str("sketch_name")
	cx := r.Float64("cx")
	cy := r.Float64("cy")
	radius := r.Float64("radius")
	startAngle := r.Float64("start_angle")
	endAngle := r.Float64("end_angle")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if sketchName == "" {
		return mcp.ErrResult(fmt.Errorf("sketch_name is required"))
	}
	docName, _ := mcp.ArgStr(args, "doc_name")

	return f.execPython(ctx, fmt.Sprintf(`
import Part, Sketcher
import math
doc = FreeCAD.getDocument(%q) if %q else FreeCAD.ActiveDocument
if doc is None:
    raise ValueError("No document found")
sketch = doc.getObject(%q)
if sketch is None:
    raise ValueError("Sketch not found: " + %q)
start_rad = math.radians(%f)
end_rad = math.radians(%f)
arc = Part.ArcOfCircle(Part.Circle(FreeCAD.Vector(%f,%f,0), FreeCAD.Vector(0,0,1), %f), start_rad, end_rad)
idx = sketch.addGeometry(arc, False)
doc.recompute()
_result_ = {"geometry_index": idx, "type": "ArcOfCircle", "geometry_count": sketch.GeometryCount}
`, docName, docName, sketchName, sketchName, startAngle, endAngle, cx, cy, radius))
}

func addSketchRectangle(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	sketchName := r.Str("sketch_name")
	x1 := r.Float64("x1")
	y1 := r.Float64("y1")
	x2 := r.Float64("x2")
	y2 := r.Float64("y2")
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
x1, y1, x2, y2 = %f, %f, %f, %f
i0 = sketch.addGeometry(Part.LineSegment(FreeCAD.Vector(x1,y1,0), FreeCAD.Vector(x2,y1,0)), False)
i1 = sketch.addGeometry(Part.LineSegment(FreeCAD.Vector(x2,y1,0), FreeCAD.Vector(x2,y2,0)), False)
i2 = sketch.addGeometry(Part.LineSegment(FreeCAD.Vector(x2,y2,0), FreeCAD.Vector(x1,y2,0)), False)
i3 = sketch.addGeometry(Part.LineSegment(FreeCAD.Vector(x1,y2,0), FreeCAD.Vector(x1,y1,0)), False)
sketch.addConstraint(Sketcher.Constraint("Coincident", i0, 2, i1, 1))
sketch.addConstraint(Sketcher.Constraint("Coincident", i1, 2, i2, 1))
sketch.addConstraint(Sketcher.Constraint("Coincident", i2, 2, i3, 1))
sketch.addConstraint(Sketcher.Constraint("Coincident", i3, 2, i0, 1))
sketch.addConstraint(Sketcher.Constraint("Horizontal", i0))
sketch.addConstraint(Sketcher.Constraint("Vertical", i1))
sketch.addConstraint(Sketcher.Constraint("Horizontal", i2))
sketch.addConstraint(Sketcher.Constraint("Vertical", i3))
doc.recompute()
_result_ = {"geometry_indices": [i0, i1, i2, i3], "type": "Rectangle", "geometry_count": sketch.GeometryCount, "constraint_count": sketch.ConstraintCount}
`, docName, docName, sketchName, sketchName, x1, y1, x2, y2))
}

func addSketchPolygon(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	sketchName := r.Str("sketch_name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if sketchName == "" {
		return mcp.ErrResult(fmt.Errorf("sketch_name is required"))
	}
	points, _ := mcp.ArgStr(args, "points")
	if points == "" {
		return mcp.ErrResult(fmt.Errorf("points is required (JSON array of [x,y] pairs)"))
	}
	docName, _ := mcp.ArgStr(args, "doc_name")

	return f.execPython(ctx, fmt.Sprintf(`
import Part, Sketcher
import json as _json
doc = FreeCAD.getDocument(%q) if %q else FreeCAD.ActiveDocument
if doc is None:
    raise ValueError("No document found")
sketch = doc.getObject(%q)
if sketch is None:
    raise ValueError("Sketch not found: " + %q)
points = _json.loads(%q)
if len(points) < 3:
    raise ValueError("Need at least 3 points for a polygon")
indices = []
for i in range(len(points)):
    p1 = points[i]
    p2 = points[(i + 1) %% len(points)]
    idx = sketch.addGeometry(Part.LineSegment(FreeCAD.Vector(p1[0], p1[1], 0), FreeCAD.Vector(p2[0], p2[1], 0)), False)
    indices.append(idx)
for i in range(len(indices)):
    sketch.addConstraint(Sketcher.Constraint("Coincident", indices[i], 2, indices[(i + 1) %% len(indices)], 1))
doc.recompute()
_result_ = {"geometry_indices": indices, "type": "Polygon", "sides": len(points), "geometry_count": sketch.GeometryCount}
`, docName, docName, sketchName, sketchName, points))
}

func addConstraint(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	sketchName := r.Str("sketch_name")
	constraintType := r.Str("type")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if sketchName == "" {
		return mcp.ErrResult(fmt.Errorf("sketch_name is required"))
	}
	if constraintType == "" {
		return mcp.ErrResult(fmt.Errorf("type is required"))
	}
	first, _ := mcp.ArgInt(args, "first")
	firstPos, _ := mcp.ArgInt(args, "first_pos")
	second, _ := mcp.ArgInt(args, "second")
	secondPos, _ := mcp.ArgInt(args, "second_pos")
	value, _ := mcp.ArgFloat64(args, "value")
	docName, _ := mcp.ArgStr(args, "doc_name")

	return f.execPython(ctx, fmt.Sprintf(`
import Part, Sketcher
doc = FreeCAD.getDocument(%q) if %q else FreeCAD.ActiveDocument
if doc is None:
    raise ValueError("No document found")
sketch = doc.getObject(%q)
if sketch is None:
    raise ValueError("Sketch not found: " + %q)
ctype = %q
first = %d
first_pos = %d
second = %d
second_pos = %d
value = %f
valid_types = ["Coincident", "Horizontal", "Vertical", "Parallel", "Perpendicular", "Tangent", "Equal", "Symmetric", "Distance", "DistanceX", "DistanceY", "Radius", "Angle", "Fixed", "Block"]
if ctype not in valid_types:
    raise ValueError("Invalid constraint type: " + ctype + ". Valid: " + ", ".join(valid_types))
if ctype in ("Coincident",):
    idx = sketch.addConstraint(Sketcher.Constraint(ctype, first, first_pos, second, second_pos))
elif ctype in ("Horizontal", "Vertical", "Fixed", "Block"):
    idx = sketch.addConstraint(Sketcher.Constraint(ctype, first))
elif ctype in ("Parallel", "Perpendicular", "Tangent", "Equal"):
    idx = sketch.addConstraint(Sketcher.Constraint(ctype, first, second))
elif ctype in ("Distance", "DistanceX", "DistanceY", "Radius", "Angle"):
    if second >= 0:
        idx = sketch.addConstraint(Sketcher.Constraint(ctype, first, first_pos, second, second_pos, value))
    else:
        idx = sketch.addConstraint(Sketcher.Constraint(ctype, first, value))
else:
    idx = sketch.addConstraint(Sketcher.Constraint(ctype, first, second))
doc.recompute()
fc = sketch.FullyConstrained if hasattr(sketch, "FullyConstrained") else None
_result_ = {"constraint_index": idx, "type": ctype, "constraint_count": sketch.ConstraintCount, "fully_constrained": fc}
`, docName, docName, sketchName, sketchName, constraintType, first, firstPos, second, secondPos, value))
}

func getSketch(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
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
geometries = []
for i in range(sketch.GeometryCount):
    geo = sketch.Geometry[i]
    ginfo = {"index": i, "type": type(geo).__name__}
    if hasattr(geo, "StartPoint"):
        ginfo["start"] = {"x": round(geo.StartPoint.x, 4), "y": round(geo.StartPoint.y, 4)}
    if hasattr(geo, "EndPoint"):
        ginfo["end"] = {"x": round(geo.EndPoint.x, 4), "y": round(geo.EndPoint.y, 4)}
    if hasattr(geo, "Center"):
        ginfo["center"] = {"x": round(geo.Center.x, 4), "y": round(geo.Center.y, 4)}
    if hasattr(geo, "Radius"):
        ginfo["radius"] = round(geo.Radius, 4)
    geometries.append(ginfo)
constraints = []
for i in range(sketch.ConstraintCount):
    c = sketch.Constraints[i]
    constraints.append({"index": i, "type": c.Type, "first": c.First, "second": c.Second, "value": round(c.Value, 4) if c.Value else 0})
fc = sketch.FullyConstrained if hasattr(sketch, "FullyConstrained") else None
_result_ = {"name": sketch.Name, "geometry_count": sketch.GeometryCount, "constraint_count": sketch.ConstraintCount, "fully_constrained": fc, "geometries": geometries, "constraints": constraints}
`, docName, docName, sketchName, sketchName))
}

func deleteSketchGeometry(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	sketchName := r.Str("sketch_name")
	index := r.Int("index")
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
sketch.delGeometry(%d)
doc.recompute()
_result_ = {"status": "deleted", "geometry_count": sketch.GeometryCount, "constraint_count": sketch.ConstraintCount}
`, docName, docName, sketchName, sketchName, index))
}
