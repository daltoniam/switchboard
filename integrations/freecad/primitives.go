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

	return f.execPython(ctx, fmt.Sprintf(`
import os
fp = %q
doc = None
for d in FreeCAD.listDocuments().values():
    if d.FileName == fp:
        doc = d
        break
if doc is None:
    if os.path.exists(fp):
        doc = FreeCAD.open(fp)
    else:
        doc = FreeCAD.newDocument(%q)
        doc.saveAs(fp)
obj = doc.addObject("Part::Box", %q)
obj.Length = %f
obj.Width = %f
obj.Height = %f
doc.recompute()
s = obj.Shape
_result_ = {
    "name": obj.Name, "label": obj.Label, "type": obj.TypeId,
    "length": obj.Length.Value, "width": obj.Width.Value, "height": obj.Height.Value,
    "volume": round(s.Volume, 4), "area": round(s.Area, 4)
}
`, fp, name, name, length, width, height))
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

	return f.execPython(ctx, fmt.Sprintf(`
import os
fp = %q
doc = None
for d in FreeCAD.listDocuments().values():
    if d.FileName == fp:
        doc = d
        break
if doc is None:
    if os.path.exists(fp):
        doc = FreeCAD.open(fp)
    else:
        doc = FreeCAD.newDocument(%q)
        doc.saveAs(fp)
obj = doc.addObject("Part::Cylinder", %q)
obj.Radius = %f
obj.Height = %f
doc.recompute()
s = obj.Shape
_result_ = {
    "name": obj.Name, "label": obj.Label, "type": obj.TypeId,
    "radius": obj.Radius.Value, "height": obj.Height.Value,
    "volume": round(s.Volume, 4), "area": round(s.Area, 4)
}
`, fp, name, name, radius, height))
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

	return f.execPython(ctx, fmt.Sprintf(`
import os
fp = %q
doc = None
for d in FreeCAD.listDocuments().values():
    if d.FileName == fp:
        doc = d
        break
if doc is None:
    if os.path.exists(fp):
        doc = FreeCAD.open(fp)
    else:
        doc = FreeCAD.newDocument(%q)
        doc.saveAs(fp)
obj = doc.addObject("Part::Sphere", %q)
obj.Radius = %f
doc.recompute()
s = obj.Shape
_result_ = {
    "name": obj.Name, "label": obj.Label, "type": obj.TypeId,
    "radius": obj.Radius.Value,
    "volume": round(s.Volume, 4), "area": round(s.Area, 4)
}
`, fp, name, name, radius))
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

	return f.execPython(ctx, fmt.Sprintf(`
import os
fp = %q
doc = None
for d in FreeCAD.listDocuments().values():
    if d.FileName == fp:
        doc = d
        break
if doc is None:
    if os.path.exists(fp):
        doc = FreeCAD.open(fp)
    else:
        doc = FreeCAD.newDocument(%q)
        doc.saveAs(fp)
obj = doc.addObject("Part::Cone", %q)
obj.Radius1 = %f
obj.Radius2 = %f
obj.Height = %f
doc.recompute()
s = obj.Shape
_result_ = {
    "name": obj.Name, "label": obj.Label, "type": obj.TypeId,
    "radius1": obj.Radius1.Value, "radius2": obj.Radius2.Value, "height": obj.Height.Value,
    "volume": round(s.Volume, 4), "area": round(s.Area, 4)
}
`, fp, name, name, radius1, radius2, height))
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

	return f.execPython(ctx, fmt.Sprintf(`
import os
fp = %q
doc = None
for d in FreeCAD.listDocuments().values():
    if d.FileName == fp:
        doc = d
        break
if doc is None:
    if os.path.exists(fp):
        doc = FreeCAD.open(fp)
    else:
        doc = FreeCAD.newDocument(%q)
        doc.saveAs(fp)
obj = doc.addObject("Part::Torus", %q)
obj.Radius1 = %f
obj.Radius2 = %f
doc.recompute()
s = obj.Shape
_result_ = {
    "name": obj.Name, "label": obj.Label, "type": obj.TypeId,
    "radius1": obj.Radius1.Value, "radius2": obj.Radius2.Value,
    "volume": round(s.Volume, 4), "area": round(s.Area, 4)
}
`, fp, name, name, radius1, radius2))
}
