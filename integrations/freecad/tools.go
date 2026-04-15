package freecad

import mcp "github.com/daltoniam/switchboard"

var tools = []mcp.ToolDefinition{
	// ── Version / Info ──────────────────────────────────────────────
	{
		Name: mcp.ToolName("freecad_get_version"), Description: "Get FreeCAD version and build information. Start here to verify the CAD environment is working.",
		Parameters: map[string]string{},
	},

	// ── Document Management ─────────────────────────────────────────
	{
		Name: mcp.ToolName("freecad_list_documents"), Description: "List all FreeCAD CAD document files (.FCStd) in the data directory. Start here to discover existing 3D models, designs, and parts.",
		Parameters: map[string]string{},
	},
	{
		Name: mcp.ToolName("freecad_create_document"), Description: "Create a new FreeCAD CAD document (.FCStd file). Use to start a new 3D model, mechanical part, or assembly design.",
		Parameters: map[string]string{
			"name":  "Document name (used as filename without extension)",
			"label": "Human-readable label for the document (optional, defaults to name)",
		},
		Required: []string{"name"},
	},
	{
		Name: mcp.ToolName("freecad_open_document"), Description: "Open an existing FreeCAD document file (.FCStd) and list its objects. Use to load a previously saved 3D model or design.",
		Parameters: map[string]string{
			"file_path": "Path to .FCStd file (relative to data_dir or absolute)",
		},
		Required: []string{"file_path"},
	},
	{
		Name: mcp.ToolName("freecad_get_document"), Description: "Get metadata and object summary for a FreeCAD document. Use after open_document or create_document.",
		Parameters: map[string]string{
			"file_path": "Path to .FCStd file (relative to data_dir or absolute)",
		},
		Required: []string{"file_path"},
	},

	// ── Objects ──────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("freecad_list_objects"), Description: "List all 3D objects (parts, shapes, features) in a FreeCAD document with type, label, and shape summary.",
		Parameters: map[string]string{
			"file_path": "Path to .FCStd file",
		},
		Required: []string{"file_path"},
	},
	{
		Name: mcp.ToolName("freecad_get_object"), Description: "Get detailed information about a specific 3D object in a FreeCAD document including properties, shape geometry, and measurements.",
		Parameters: map[string]string{
			"file_path":   "Path to .FCStd file",
			"object_name": "Object name within the document",
		},
		Required: []string{"file_path", "object_name"},
	},
	{
		Name: mcp.ToolName("freecad_delete_object"), Description: "Remove a 3D object from a FreeCAD document. Use to clean up or modify designs.",
		Parameters: map[string]string{
			"file_path":   "Path to .FCStd file",
			"object_name": "Object name to delete",
		},
		Required: []string{"file_path", "object_name"},
	},
	{
		Name: mcp.ToolName("freecad_set_placement"), Description: "Set the 3D position and rotation of an object in a FreeCAD document. Move or orient parts.",
		Parameters: map[string]string{
			"file_path":   "Path to .FCStd file",
			"object_name": "Object name",
			"x":           "X position in mm",
			"y":           "Y position in mm",
			"z":           "Z position in mm",
			"yaw":         "Yaw rotation in degrees (optional, default 0)",
			"pitch":       "Pitch rotation in degrees (optional, default 0)",
			"roll":        "Roll rotation in degrees (optional, default 0)",
		},
		Required: []string{"file_path", "object_name", "x", "y", "z"},
	},
	{
		Name: mcp.ToolName("freecad_get_properties"), Description: "Get all properties and their values for a 3D object in a FreeCAD document.",
		Parameters: map[string]string{
			"file_path":   "Path to .FCStd file",
			"object_name": "Object name",
		},
		Required: []string{"file_path", "object_name"},
	},
	{
		Name: mcp.ToolName("freecad_set_property"), Description: "Set a property value on a 3D object in a FreeCAD document (e.g. Length, Width, Height, Radius).",
		Parameters: map[string]string{
			"file_path":   "Path to .FCStd file",
			"object_name": "Object name",
			"property":    "Property name (e.g. Length, Width, Height, Radius)",
			"value":       "Property value (numeric for dimensions, string for labels)",
		},
		Required: []string{"file_path", "object_name", "property", "value"},
	},

	// ── Primitives ──────────────────────────────────────────────────
	{
		Name: mcp.ToolName("freecad_create_box"), Description: "Create a 3D box (rectangular prism) primitive in a FreeCAD document. Specify length, width, height in millimeters.",
		Parameters: map[string]string{
			"file_path": "Path to .FCStd file",
			"name":      "Object name (optional, defaults to 'Box')",
			"length":    "Length in mm (X dimension, default 10)",
			"width":     "Width in mm (Y dimension, default 10)",
			"height":    "Height in mm (Z dimension, default 10)",
		},
		Required: []string{"file_path"},
	},
	{
		Name: mcp.ToolName("freecad_create_cylinder"), Description: "Create a 3D cylinder primitive in a FreeCAD document. Specify radius and height in millimeters.",
		Parameters: map[string]string{
			"file_path": "Path to .FCStd file",
			"name":      "Object name (optional, defaults to 'Cylinder')",
			"radius":    "Radius in mm (default 5)",
			"height":    "Height in mm (default 10)",
		},
		Required: []string{"file_path"},
	},
	{
		Name: mcp.ToolName("freecad_create_sphere"), Description: "Create a 3D sphere primitive in a FreeCAD document. Specify radius in millimeters.",
		Parameters: map[string]string{
			"file_path": "Path to .FCStd file",
			"name":      "Object name (optional, defaults to 'Sphere')",
			"radius":    "Radius in mm (default 5)",
		},
		Required: []string{"file_path"},
	},
	{
		Name: mcp.ToolName("freecad_create_cone"), Description: "Create a 3D cone primitive in a FreeCAD document. Specify radii and height in millimeters.",
		Parameters: map[string]string{
			"file_path": "Path to .FCStd file",
			"name":      "Object name (optional, defaults to 'Cone')",
			"radius1":   "Base radius in mm (default 5)",
			"radius2":   "Top radius in mm (default 0 for pointed cone)",
			"height":    "Height in mm (default 10)",
		},
		Required: []string{"file_path"},
	},
	{
		Name: mcp.ToolName("freecad_create_torus"), Description: "Create a 3D torus (donut shape) primitive in a FreeCAD document. Specify major and minor radii in millimeters.",
		Parameters: map[string]string{
			"file_path": "Path to .FCStd file",
			"name":      "Object name (optional, defaults to 'Torus')",
			"radius1":   "Major radius in mm (default 10)",
			"radius2":   "Minor radius in mm (default 2)",
		},
		Required: []string{"file_path"},
	},

	// ── Boolean Operations ──────────────────────────────────────────
	{
		Name: mcp.ToolName("freecad_boolean_cut"), Description: "Perform a boolean cut (subtraction) between two 3D shapes in a FreeCAD document. Removes shape2 volume from shape1.",
		Parameters: map[string]string{
			"file_path": "Path to .FCStd file",
			"base":      "Name of the base object (shape to cut from)",
			"tool":      "Name of the tool object (shape to subtract)",
			"name":      "Name for the result object (optional, defaults to 'Cut')",
		},
		Required: []string{"file_path", "base", "tool"},
	},
	{
		Name: mcp.ToolName("freecad_boolean_fuse"), Description: "Perform a boolean fuse (union) of two 3D shapes in a FreeCAD document. Combines two shapes into one solid.",
		Parameters: map[string]string{
			"file_path": "Path to .FCStd file",
			"base":      "Name of the first object",
			"tool":      "Name of the second object",
			"name":      "Name for the result object (optional, defaults to 'Fuse')",
		},
		Required: []string{"file_path", "base", "tool"},
	},
	{
		Name: mcp.ToolName("freecad_boolean_common"), Description: "Perform a boolean intersection of two 3D shapes in a FreeCAD document. Keeps only the overlapping volume.",
		Parameters: map[string]string{
			"file_path": "Path to .FCStd file",
			"base":      "Name of the first object",
			"tool":      "Name of the second object",
			"name":      "Name for the result object (optional, defaults to 'Common')",
		},
		Required: []string{"file_path", "base", "tool"},
	},

	// ── Shape Operations ────────────────────────────────────────────
	{
		Name: mcp.ToolName("freecad_fillet"), Description: "Apply fillet (rounded edges) to a 3D object in a FreeCAD document. Rounds all edges by specified radius.",
		Parameters: map[string]string{
			"file_path":   "Path to .FCStd file",
			"object_name": "Object name to fillet",
			"radius":      "Fillet radius in mm",
			"name":        "Name for the result object (optional, defaults to 'Fillet')",
		},
		Required: []string{"file_path", "object_name", "radius"},
	},
	{
		Name: mcp.ToolName("freecad_chamfer"), Description: "Apply chamfer (beveled edges) to a 3D object in a FreeCAD document. Bevels all edges by specified size.",
		Parameters: map[string]string{
			"file_path":   "Path to .FCStd file",
			"object_name": "Object name to chamfer",
			"size":        "Chamfer size in mm",
			"name":        "Name for the result object (optional, defaults to 'Chamfer')",
		},
		Required: []string{"file_path", "object_name", "size"},
	},
	{
		Name: mcp.ToolName("freecad_extrude"), Description: "Extrude a 2D face along a direction vector to create a 3D solid. Creates depth from a flat shape.",
		Parameters: map[string]string{
			"file_path":   "Path to .FCStd file",
			"object_name": "Source object name (must have a valid face/wire)",
			"dx":          "Extrusion direction X component (default 0)",
			"dy":          "Extrusion direction Y component (default 0)",
			"dz":          "Extrusion direction Z component (default 10)",
			"name":        "Name for the result object (optional, defaults to 'Extrude')",
		},
		Required: []string{"file_path", "object_name"},
	},
	{
		Name: mcp.ToolName("freecad_mirror"), Description: "Mirror a 3D object across a plane in a FreeCAD document.",
		Parameters: map[string]string{
			"file_path":   "Path to .FCStd file",
			"object_name": "Object name to mirror",
			"plane":       "Mirror plane: 'xy', 'xz', or 'yz' (default 'yz')",
			"name":        "Name for the result object (optional, defaults to 'Mirror')",
		},
		Required: []string{"file_path", "object_name"},
	},

	// ── Measurement / Analysis ──────────────────────────────────────
	{
		Name: mcp.ToolName("freecad_measure_shape"), Description: "Measure a 3D shape: volume, surface area, center of mass, vertices, edges, faces count. Use for engineering analysis of CAD models.",
		Parameters: map[string]string{
			"file_path":   "Path to .FCStd file",
			"object_name": "Object name to measure",
		},
		Required: []string{"file_path", "object_name"},
	},
	{
		Name: mcp.ToolName("freecad_check_geometry"), Description: "Check a 3D shape for geometry errors (self-intersections, invalid topology). CAD model validation and repair diagnostics.",
		Parameters: map[string]string{
			"file_path":   "Path to .FCStd file",
			"object_name": "Object name to check",
		},
		Required: []string{"file_path", "object_name"},
	},
	{
		Name: mcp.ToolName("freecad_bounding_box"), Description: "Get the axis-aligned bounding box of a 3D object. Returns min/max coordinates and dimensions in mm.",
		Parameters: map[string]string{
			"file_path":   "Path to .FCStd file",
			"object_name": "Object name",
		},
		Required: []string{"file_path", "object_name"},
	},

	// ── Export / Import ─────────────────────────────────────────────
	{
		Name: mcp.ToolName("freecad_export_step"), Description: "Export a 3D object or entire document to STEP format (.step/.stp). Standard CAD exchange format for manufacturing and CNC.",
		Parameters: map[string]string{
			"file_path":   "Path to .FCStd file",
			"output_path": "Output STEP file path (relative to data_dir or absolute)",
			"object_name": "Object name to export (optional, exports all if omitted)",
		},
		Required: []string{"file_path", "output_path"},
	},
	{
		Name: mcp.ToolName("freecad_export_stl"), Description: "Export a 3D object to STL mesh format. Used for 3D printing, FDM/SLA slicing, and mesh-based workflows.",
		Parameters: map[string]string{
			"file_path":      "Path to .FCStd file",
			"output_path":    "Output STL file path",
			"object_name":    "Object name to export (optional, exports first shape if omitted)",
			"mesh_tolerance": "Mesh linear deflection in mm (optional, default 0.1, lower = finer mesh)",
		},
		Required: []string{"file_path", "output_path"},
	},
	{
		Name: mcp.ToolName("freecad_export_brep"), Description: "Export a 3D object to BRep (boundary representation) format. Lossless CAD geometry format.",
		Parameters: map[string]string{
			"file_path":   "Path to .FCStd file",
			"output_path": "Output BRep file path",
			"object_name": "Object name to export (optional, exports first shape if omitted)",
		},
		Required: []string{"file_path", "output_path"},
	},
	{
		Name: mcp.ToolName("freecad_import_file"), Description: "Import a CAD file (STEP, IGES, STL, OBJ, BRep) into a FreeCAD document.",
		Parameters: map[string]string{
			"file_path":   "Path to .FCStd document to import into",
			"import_path": "Path to file to import (STEP, IGES, STL, OBJ, BRep)",
		},
		Required: []string{"file_path", "import_path"},
	},

	// ── Python Scripting ────────────────────────────────────────────
	{
		Name: mcp.ToolName("freecad_run_script"), Description: "Execute a custom FreeCAD Python script with full access to FreeCAD, Part, Mesh modules. For advanced CAD operations not covered by other tools. Script must print JSON result.",
		Parameters: map[string]string{
			"script":    "Python script to execute (has access to FreeCAD, Part, Mesh, json modules)",
			"file_path": "Optional .FCStd file to open before running script",
		},
		Required: []string{"script"},
	},
}
