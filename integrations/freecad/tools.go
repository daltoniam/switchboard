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
	{
		Name: mcp.ToolName("freecad_save_document"), Description: "Save the active or named FreeCAD document. Optionally save to a new file path.",
		Parameters: map[string]string{
			"doc_name":  "Document name to save (optional, defaults to active document)",
			"file_path": "Save to this file path (optional, defaults to current path)",
		},
	},
	{
		Name: mcp.ToolName("freecad_close_document"), Description: "Close an open FreeCAD document by name, or the active document if no name given.",
		Parameters: map[string]string{
			"doc_name": "Document name to close (optional, defaults to active document)",
		},
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

	{
		Name: mcp.ToolName("freecad_get_document_errors"), Description: "Get all object errors, invalid states, and null shapes in the active FreeCAD document. Use to diagnose why a design or feature failed after recompute.",
		Parameters: map[string]string{},
	},
	{
		Name: mcp.ToolName("freecad_get_solver_status"), Description: "Get the constraint solver status for a sketch: degrees of freedom, fully-constrained state, and solver diagnostics. Use to diagnose under-constrained or over-constrained sketches.",
		Parameters: map[string]string{
			"sketch_name": "Sketch object name",
			"doc_name":    "Document name (optional)",
		},
		Required: []string{"sketch_name"},
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
		Name: mcp.ToolName("freecad_run_script"), Description: "Execute a custom FreeCAD Python script with full access to FreeCAD, Part, Mesh modules. For advanced CAD operations not covered by other tools. Set _result_ to return data.",
		Parameters: map[string]string{
			"script":    "Python script to execute (has access to FreeCAD, Part, Mesh modules; set _result_ to return data)",
			"file_path": "Optional .FCStd file to open before running script",
		},
		Required: []string{"script"},
	},

	// ── Sketcher ────────────────────────────────────────────────────
	{
		Name: mcp.ToolName("freecad_create_sketch"), Description: "Create a new 2D sketch in a FreeCAD document. Start here for parametric modeling — sketches are the foundation for pad, pocket, and revolution operations.",
		Parameters: map[string]string{
			"doc_name":  "Document name (optional, uses active document)",
			"name":      "Sketch object name (optional, defaults to 'Sketch')",
			"plane":     "Sketch plane: 'XY', 'XZ', or 'YZ' (default 'XY')",
			"body_name": "PartDesign Body to attach sketch to (optional)",
		},
	},
	{
		Name: mcp.ToolName("freecad_add_sketch_line"), Description: "Add a line segment to a sketch. Specify start and end coordinates in mm.",
		Parameters: map[string]string{
			"sketch_name": "Sketch object name",
			"x1":          "Start X coordinate in mm",
			"y1":          "Start Y coordinate in mm",
			"x2":          "End X coordinate in mm",
			"y2":          "End Y coordinate in mm",
			"doc_name":    "Document name (optional)",
		},
		Required: []string{"sketch_name", "x1", "y1", "x2", "y2"},
	},
	{
		Name: mcp.ToolName("freecad_add_sketch_circle"), Description: "Add a circle to a sketch. Specify center and radius in mm.",
		Parameters: map[string]string{
			"sketch_name": "Sketch object name",
			"cx":          "Center X coordinate in mm",
			"cy":          "Center Y coordinate in mm",
			"radius":      "Radius in mm",
			"doc_name":    "Document name (optional)",
		},
		Required: []string{"sketch_name", "cx", "cy", "radius"},
	},
	{
		Name: mcp.ToolName("freecad_add_sketch_arc"), Description: "Add a circular arc to a sketch. Specify center, radius, and angles in degrees.",
		Parameters: map[string]string{
			"sketch_name": "Sketch object name",
			"cx":          "Center X coordinate in mm",
			"cy":          "Center Y coordinate in mm",
			"radius":      "Radius in mm",
			"start_angle": "Start angle in degrees",
			"end_angle":   "End angle in degrees",
			"doc_name":    "Document name (optional)",
		},
		Required: []string{"sketch_name", "cx", "cy", "radius", "start_angle", "end_angle"},
	},
	{
		Name: mcp.ToolName("freecad_add_sketch_rectangle"), Description: "Add a rectangle to a sketch defined by two corner points. Automatically adds coincident, horizontal, and vertical constraints.",
		Parameters: map[string]string{
			"sketch_name": "Sketch object name",
			"x1":          "First corner X in mm",
			"y1":          "First corner Y in mm",
			"x2":          "Opposite corner X in mm",
			"y2":          "Opposite corner Y in mm",
			"doc_name":    "Document name (optional)",
		},
		Required: []string{"sketch_name", "x1", "y1", "x2", "y2"},
	},
	{
		Name: mcp.ToolName("freecad_add_sketch_polygon"), Description: "Add a closed polygon to a sketch from a list of vertex points. Automatically adds coincident constraints to close the shape.",
		Parameters: map[string]string{
			"sketch_name": "Sketch object name",
			"points":      "JSON array of [x, y] coordinate pairs, e.g. [[0,0],[10,0],[10,10],[0,10]]",
			"doc_name":    "Document name (optional)",
		},
		Required: []string{"sketch_name", "points"},
	},
	{
		Name: mcp.ToolName("freecad_add_constraint"), Description: "Add a geometric or dimensional constraint to a sketch. Supports: Coincident, Horizontal, Vertical, Parallel, Perpendicular, Tangent, Equal, Distance, DistanceX, DistanceY, Radius, Angle, Fixed, Block.",
		Parameters: map[string]string{
			"sketch_name": "Sketch object name",
			"type":        "Constraint type (e.g. Coincident, Distance, Horizontal, Vertical, Radius, Angle, Parallel, Perpendicular, Equal, Tangent, Fixed, Block)",
			"first":       "First geometry index (required for all constraints)",
			"first_pos":   "First geometry point: 1=start, 2=end, 3=center (for Coincident/Distance)",
			"second":      "Second geometry index (for two-geometry constraints, -1 if unused)",
			"second_pos":  "Second geometry point: 1=start, 2=end, 3=center",
			"value":       "Constraint value in mm or degrees (for Distance, Radius, Angle)",
			"doc_name":    "Document name (optional)",
		},
		Required: []string{"sketch_name", "type", "first"},
	},
	{
		Name: mcp.ToolName("freecad_get_sketch"), Description: "Get detailed information about a sketch: all geometry elements, constraints, and constraint status. Use to inspect sketch state.",
		Parameters: map[string]string{
			"sketch_name": "Sketch object name",
			"doc_name":    "Document name (optional)",
		},
		Required: []string{"sketch_name"},
	},
	{
		Name: mcp.ToolName("freecad_delete_sketch_geometry"), Description: "Delete a geometry element from a sketch by its index. Related constraints are automatically removed.",
		Parameters: map[string]string{
			"sketch_name": "Sketch object name",
			"index":       "Geometry index to delete (0-based)",
			"doc_name":    "Document name (optional)",
		},
		Required: []string{"sketch_name", "index"},
	},

	// ── PartDesign ──────────────────────────────────────────────────
	{
		Name: mcp.ToolName("freecad_create_body"), Description: "Create a PartDesign Body container in a FreeCAD document. Required before using pad, pocket, revolution, and other PartDesign features.",
		Parameters: map[string]string{
			"doc_name": "Document name (optional)",
			"name":     "Body name (optional, defaults to 'Body')",
		},
	},
	{
		Name: mcp.ToolName("freecad_pad"), Description: "Pad (extrude) a sketch to create a 3D solid. The primary way to turn a 2D sketch into a 3D part. Use after create_sketch.",
		Parameters: map[string]string{
			"sketch_name": "Sketch to pad",
			"length":      "Pad length/depth in mm",
			"name":        "Feature name (optional, defaults to 'Pad')",
			"body_name":   "Body name (optional, auto-detected from sketch parent)",
			"doc_name":    "Document name (optional)",
			"symmetric":   "Pad symmetrically in both directions (true/false, default false)",
			"reversed":    "Reverse pad direction (true/false, default false)",
		},
		Required: []string{"sketch_name", "length"},
	},
	{
		Name: mcp.ToolName("freecad_pocket"), Description: "Create a pocket (cut/hole) by extruding a sketch into existing solid. Subtractive operation — removes material.",
		Parameters: map[string]string{
			"sketch_name": "Sketch defining the pocket shape",
			"length":      "Pocket depth in mm (ignored if through_all is true)",
			"name":        "Feature name (optional, defaults to 'Pocket')",
			"body_name":   "Body name (optional)",
			"doc_name":    "Document name (optional)",
			"through_all": "Cut through entire part (true/false, default false)",
		},
		Required: []string{"sketch_name", "length"},
	},
	{
		Name: mcp.ToolName("freecad_revolution"), Description: "Revolve a sketch around an axis to create a solid of revolution (lathe operation). Creates rotationally symmetric parts.",
		Parameters: map[string]string{
			"sketch_name": "Sketch to revolve",
			"angle":       "Revolution angle in degrees (optional, default 360 for full revolution)",
			"axis":        "Revolution axis: 'V_Axis' (vertical), 'H_Axis' (horizontal), or 'N_Axis' (normal). Default 'V_Axis'",
			"name":        "Feature name (optional)",
			"body_name":   "Body name (optional)",
			"doc_name":    "Document name (optional)",
		},
		Required: []string{"sketch_name"},
	},
	{
		Name: mcp.ToolName("freecad_groove"), Description: "Create a groove by revolving a sketch profile as a subtractive operation. The rotational equivalent of pocket.",
		Parameters: map[string]string{
			"sketch_name": "Sketch to revolve (subtractive)",
			"angle":       "Groove angle in degrees (optional, default 360)",
			"axis":        "Revolution axis: 'V_Axis', 'H_Axis', or 'N_Axis' (default 'V_Axis')",
			"name":        "Feature name (optional)",
			"body_name":   "Body name (optional)",
			"doc_name":    "Document name (optional)",
		},
		Required: []string{"sketch_name"},
	},
	{
		Name: mcp.ToolName("freecad_pd_fillet"), Description: "Add fillets (rounded edges) to a PartDesign body. Rounds specified edges or all edges of the tip feature.",
		Parameters: map[string]string{
			"radius":    "Fillet radius in mm",
			"edges":     "JSON array of edge indices to fillet, e.g. [1,3,5] (optional, defaults to all edges)",
			"name":      "Feature name (optional)",
			"body_name": "Body name (optional)",
			"doc_name":  "Document name (optional)",
		},
		Required: []string{"radius"},
	},
	{
		Name: mcp.ToolName("freecad_pd_chamfer"), Description: "Add chamfers (beveled edges) to a PartDesign body. Bevels specified edges or all edges of the tip feature.",
		Parameters: map[string]string{
			"size":      "Chamfer size in mm",
			"edges":     "JSON array of edge indices to chamfer (optional, defaults to all edges)",
			"name":      "Feature name (optional)",
			"body_name": "Body name (optional)",
			"doc_name":  "Document name (optional)",
		},
		Required: []string{"size"},
	},
	{
		Name: mcp.ToolName("freecad_pd_mirror"), Description: "Mirror a PartDesign feature across a body origin plane. Creates a symmetric copy.",
		Parameters: map[string]string{
			"plane":     "Mirror plane: 'XY_Plane', 'XZ_Plane', or 'YZ_Plane' (default 'XY_Plane')",
			"name":      "Feature name (optional)",
			"body_name": "Body name (optional)",
			"doc_name":  "Document name (optional)",
		},
	},
	{
		Name: mcp.ToolName("freecad_linear_pattern"), Description: "Create a linear pattern (array) of the tip feature along a direction. Repeats the last feature at equal intervals.",
		Parameters: map[string]string{
			"occurrences": "Number of total occurrences (including original)",
			"length":      "Total pattern length in mm",
			"direction":   "Pattern direction: 'H_Axis' or 'V_Axis' (default 'H_Axis')",
			"name":        "Feature name (optional)",
			"body_name":   "Body name (optional)",
			"doc_name":    "Document name (optional)",
		},
		Required: []string{"occurrences", "length"},
	},
	{
		Name: mcp.ToolName("freecad_polar_pattern"), Description: "Create a polar (circular) pattern of the tip feature around an axis. Repeats the last feature in a circular arrangement.",
		Parameters: map[string]string{
			"occurrences": "Number of total occurrences (including original)",
			"angle":       "Total pattern angle in degrees (optional, default 360)",
			"name":        "Feature name (optional)",
			"body_name":   "Body name (optional)",
			"doc_name":    "Document name (optional)",
		},
		Required: []string{"occurrences"},
	},
	{
		Name: mcp.ToolName("freecad_pd_hole"), Description: "Create a hole feature in a PartDesign body from a sketch containing point(s). Supports simple holes and threaded holes.",
		Parameters: map[string]string{
			"sketch_name": "Sketch with center point(s) for holes",
			"diameter":    "Hole diameter in mm",
			"depth":       "Hole depth in mm",
			"threaded":    "Create threaded hole (true/false, default false)",
			"name":        "Feature name (optional)",
			"body_name":   "Body name (optional)",
			"doc_name":    "Document name (optional)",
		},
		Required: []string{"sketch_name", "diameter", "depth"},
	},
}
