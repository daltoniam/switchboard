package freecad

import (
	"context"
	"fmt"

	mcp "github.com/daltoniam/switchboard"
)

func createBox(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	filePath := r.Str("file_path")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if filePath == "" {
		return mcp.ErrResult(fmt.Errorf("file_path is required"))
	}
	name, _ := mcp.ArgStr(args, "name")
	if name == "" {
		name = "Box"
	}
	length := optFloat(args, "length", 10)
	width := optFloat(args, "width", 10)
	height := optFloat(args, "height", 10)

	fp := f.filePath(filePath)

	return jsonScriptResult(ctx, f, fmt.Sprintf(`
import FreeCAD
import json
doc = FreeCAD.open(%q)
obj = doc.addObject("Part::Box", %q)
obj.Length = %f
obj.Width = %f
obj.Height = %f
doc.recompute()
doc.save()
s = obj.Shape
print(json.dumps({
    "name": obj.Name, "label": obj.Label, "type": obj.TypeId,
    "length": obj.Length.Value, "width": obj.Width.Value, "height": obj.Height.Value,
    "volume": round(s.Volume, 4), "area": round(s.Area, 4)
}))
FreeCAD.closeDocument(doc.Name)
`, fp, name, length, width, height))
}

func createCylinder(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	filePath := r.Str("file_path")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if filePath == "" {
		return mcp.ErrResult(fmt.Errorf("file_path is required"))
	}
	name, _ := mcp.ArgStr(args, "name")
	if name == "" {
		name = "Cylinder"
	}
	radius := optFloat(args, "radius", 5)
	height := optFloat(args, "height", 10)

	fp := f.filePath(filePath)

	return jsonScriptResult(ctx, f, fmt.Sprintf(`
import FreeCAD
import json
doc = FreeCAD.open(%q)
obj = doc.addObject("Part::Cylinder", %q)
obj.Radius = %f
obj.Height = %f
doc.recompute()
doc.save()
s = obj.Shape
print(json.dumps({
    "name": obj.Name, "label": obj.Label, "type": obj.TypeId,
    "radius": obj.Radius.Value, "height": obj.Height.Value,
    "volume": round(s.Volume, 4), "area": round(s.Area, 4)
}))
FreeCAD.closeDocument(doc.Name)
`, fp, name, radius, height))
}

func createSphere(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	filePath := r.Str("file_path")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if filePath == "" {
		return mcp.ErrResult(fmt.Errorf("file_path is required"))
	}
	name, _ := mcp.ArgStr(args, "name")
	if name == "" {
		name = "Sphere"
	}
	radius := optFloat(args, "radius", 5)

	fp := f.filePath(filePath)

	return jsonScriptResult(ctx, f, fmt.Sprintf(`
import FreeCAD
import json
doc = FreeCAD.open(%q)
obj = doc.addObject("Part::Sphere", %q)
obj.Radius = %f
doc.recompute()
doc.save()
s = obj.Shape
print(json.dumps({
    "name": obj.Name, "label": obj.Label, "type": obj.TypeId,
    "radius": obj.Radius.Value,
    "volume": round(s.Volume, 4), "area": round(s.Area, 4)
}))
FreeCAD.closeDocument(doc.Name)
`, fp, name, radius))
}

func createCone(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	filePath := r.Str("file_path")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if filePath == "" {
		return mcp.ErrResult(fmt.Errorf("file_path is required"))
	}
	name, _ := mcp.ArgStr(args, "name")
	if name == "" {
		name = "Cone"
	}
	radius1 := optFloat(args, "radius1", 5)
	radius2 := optFloat(args, "radius2", 0)
	height := optFloat(args, "height", 10)

	fp := f.filePath(filePath)

	return jsonScriptResult(ctx, f, fmt.Sprintf(`
import FreeCAD
import json
doc = FreeCAD.open(%q)
obj = doc.addObject("Part::Cone", %q)
obj.Radius1 = %f
obj.Radius2 = %f
obj.Height = %f
doc.recompute()
doc.save()
s = obj.Shape
print(json.dumps({
    "name": obj.Name, "label": obj.Label, "type": obj.TypeId,
    "radius1": obj.Radius1.Value, "radius2": obj.Radius2.Value, "height": obj.Height.Value,
    "volume": round(s.Volume, 4), "area": round(s.Area, 4)
}))
FreeCAD.closeDocument(doc.Name)
`, fp, name, radius1, radius2, height))
}

func createTorus(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	filePath := r.Str("file_path")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if filePath == "" {
		return mcp.ErrResult(fmt.Errorf("file_path is required"))
	}
	name, _ := mcp.ArgStr(args, "name")
	if name == "" {
		name = "Torus"
	}
	radius1 := optFloat(args, "radius1", 10)
	radius2 := optFloat(args, "radius2", 2)

	fp := f.filePath(filePath)

	return jsonScriptResult(ctx, f, fmt.Sprintf(`
import FreeCAD
import json
doc = FreeCAD.open(%q)
obj = doc.addObject("Part::Torus", %q)
obj.Radius1 = %f
obj.Radius2 = %f
doc.recompute()
doc.save()
s = obj.Shape
print(json.dumps({
    "name": obj.Name, "label": obj.Label, "type": obj.TypeId,
    "radius1": obj.Radius1.Value, "radius2": obj.Radius2.Value,
    "volume": round(s.Volume, 4), "area": round(s.Area, 4)
}))
FreeCAD.closeDocument(doc.Name)
`, fp, name, radius1, radius2))
}

// optFloat extracts a float64 from args with a default value.
func optFloat(args map[string]any, key string, def float64) float64 {
	v, err := mcp.ArgFloat64(args, key)
	if err != nil || v == 0 {
		return def
	}
	return v
}
