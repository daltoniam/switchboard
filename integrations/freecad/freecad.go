package freecad

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

var (
	_ mcp.Integration                = (*freecad)(nil)
	_ mcp.FieldCompactionIntegration = (*freecad)(nil)
	_ mcp.PlainTextCredentials       = (*freecad)(nil)
)

const defaultXMLRPCPort = "9875"

type freecad struct {
	host    string
	port    string
	dataDir string
	client  *http.Client
}

func New() mcp.Integration {
	return &freecad{
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

func (f *freecad) Name() string { return "freecad" }

func (f *freecad) PlainTextKeys() []string {
	return []string{"host", "xmlrpc_port", "data_dir"}
}

func (f *freecad) Configure(_ context.Context, creds mcp.Credentials) error {
	f.host = creds["host"]
	if f.host == "" {
		f.host = "localhost"
	}
	f.port = creds["xmlrpc_port"]
	if f.port == "" {
		f.port = defaultXMLRPCPort
	}
	f.dataDir = creds["data_dir"]
	if f.dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("freecad: data_dir not set and cannot determine home directory: %w", err)
		}
		f.dataDir = filepath.Join(home, "FreeCAD")
	}
	if err := os.MkdirAll(f.dataDir, 0o750); err != nil {
		return fmt.Errorf("freecad: cannot create data_dir %q: %w", f.dataDir, err)
	}
	return nil
}

func (f *freecad) Healthy(ctx context.Context) bool {
	if f.host == "" {
		return false
	}
	_, err := f.rpcCall(ctx, "ping")
	return err == nil
}

func (f *freecad) Tools() []mcp.ToolDefinition { return tools }

func (f *freecad) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, f, args)
}

func (f *freecad) CompactSpec(toolName mcp.ToolName) ([]mcp.CompactField, bool) {
	fields, ok := fieldCompactionSpecs[toolName]
	return fields, ok
}

// --- XMLRPC client (RobustMCPBridge protocol) ---

func (f *freecad) rpcCall(ctx context.Context, method string, params ...string) (string, error) {
	var xmlBody string
	if len(params) > 0 {
		xmlBody = fmt.Sprintf(`<?xml version="1.0"?><methodCall><methodName>%s</methodName><params><param><value><string>%s</string></value></param></params></methodCall>`,
			method, xmlEscape(params[0]))
	} else {
		xmlBody = fmt.Sprintf(`<?xml version="1.0"?><methodCall><methodName>%s</methodName><params></params></methodCall>`, method)
	}

	url := fmt.Sprintf("http://%s:%s", f.host, f.port)
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(xmlBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "text/xml")

	resp, err := f.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("freecad: cannot reach FreeCAD bridge at %s:%s — is FreeCAD running with the RobustMCPBridge addon? %w", f.host, f.port, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return "", err
	}

	return parseXMLRPCResponse(body)
}

// execScript executes Python code in the running FreeCAD process.
func (f *freecad) execScript(ctx context.Context, script string) (string, error) {
	return f.rpcCall(ctx, "execute", script)
}

// --- XMLRPC response parsing ---

type xmlrpcResponse struct {
	Params struct {
		Param struct {
			Value xmlrpcValue `xml:"value"`
		} `xml:"param"`
	} `xml:"params"`
	Fault *struct {
		Value struct {
			Struct xmlrpcStruct `xml:"struct"`
		} `xml:"value"`
	} `xml:"fault"`
}

type xmlrpcValue struct {
	String  string        `xml:"string"`
	Boolean string        `xml:"boolean"`
	Int     string        `xml:"int"`
	I4      string        `xml:"i4"`
	Double  string        `xml:"double"`
	Struct  *xmlrpcStruct `xml:"struct"`
	Array   *xmlrpcArray  `xml:"array"`
	Nil     *struct{}     `xml:"nil"`
	Inner   string        `xml:",chardata"`
}

type xmlrpcStruct struct {
	Members []xmlrpcMember `xml:"member"`
}

type xmlrpcArray struct {
	Data struct {
		Values []xmlrpcValue `xml:"value"`
	} `xml:"data"`
}

type xmlrpcMember struct {
	Name  string      `xml:"name"`
	Value xmlrpcValue `xml:"value"`
}

func parseXMLRPCResponse(data []byte) (string, error) {
	var resp xmlrpcResponse
	if err := xml.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("freecad: parse xmlrpc: %w", err)
	}
	if resp.Fault != nil {
		for _, m := range resp.Fault.Value.Struct.Members {
			if m.Name == "faultString" {
				return "", fmt.Errorf("freecad: xmlrpc fault: %s", m.Value.String)
			}
		}
		return "", fmt.Errorf("freecad: xmlrpc fault (unknown)")
	}

	v := resp.Params.Param.Value
	result := valueToAny(&v)
	if result == nil {
		return "", nil
	}
	out, err := json.Marshal(result)
	if err != nil {
		return fmt.Sprintf("%v", result), nil
	}
	return string(out), nil
}

func valueToAny(v *xmlrpcValue) any {
	if v.Nil != nil {
		return nil
	}
	if v.Struct != nil {
		m := make(map[string]any, len(v.Struct.Members))
		for _, mem := range v.Struct.Members {
			m[mem.Name] = valueToAny(&mem.Value)
		}
		return m
	}
	if v.Array != nil {
		arr := make([]any, len(v.Array.Data.Values))
		for i := range v.Array.Data.Values {
			arr[i] = valueToAny(&v.Array.Data.Values[i])
		}
		return arr
	}
	if v.String != "" {
		return v.String
	}
	if v.Boolean != "" {
		return v.Boolean == "1"
	}
	if v.Int != "" || v.I4 != "" {
		s := v.Int
		if s == "" {
			s = v.I4
		}
		var n int
		_, _ = fmt.Sscanf(s, "%d", &n)
		return n
	}
	if v.Double != "" {
		var fl float64
		_, _ = fmt.Sscanf(v.Double, "%f", &fl)
		return fl
	}
	return strings.TrimSpace(v.Inner)
}

func xmlEscape(s string) string {
	var buf bytes.Buffer
	if err := xml.EscapeText(&buf, []byte(s)); err != nil {
		return s
	}
	return buf.String()
}

// filePath resolves a file path relative to the data directory.
func (f *freecad) filePath(name string) string {
	if filepath.IsAbs(name) {
		return name
	}
	return filepath.Join(f.dataDir, name)
}

// --- Result helpers ---

type handlerFunc func(ctx context.Context, f *freecad, args map[string]any) (*mcp.ToolResult, error)

// execPython runs Python code that sets _result_ and returns the JSON result.
func (f *freecad) execPython(ctx context.Context, script string) (*mcp.ToolResult, error) {
	raw, err := f.execScript(ctx, script)
	if err != nil {
		return mcp.ErrResult(err)
	}
	var resp struct {
		Success      bool   `json:"success"`
		Result       any    `json:"result"`
		ErrorType    string `json:"error_type"`
		ErrorMessage string `json:"error_message"`
		Stdout       string `json:"stdout"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		if raw == "" {
			return &mcp.ToolResult{Data: `{"status":"ok"}`}, nil
		}
		return &mcp.ToolResult{Data: raw}, nil
	}
	if !resp.Success {
		msg := resp.ErrorMessage
		if msg == "" {
			msg = resp.ErrorType
		}
		return &mcp.ToolResult{Data: msg, IsError: true}, nil
	}
	if resp.Result != nil {
		data, err := json.Marshal(resp.Result)
		if err != nil {
			return &mcp.ToolResult{Data: fmt.Sprintf("%v", resp.Result)}, nil
		}
		return &mcp.ToolResult{Data: string(data)}, nil
	}
	if resp.Stdout != "" {
		return &mcp.ToolResult{Data: strings.TrimSpace(resp.Stdout)}, nil
	}
	return &mcp.ToolResult{Data: `{"status":"ok"}`}, nil
}

// optFloat extracts a float64 from args with a default value.
func optFloat(args map[string]any, key string, def float64) float64 {
	v, err := mcp.ArgFloat64(args, key)
	if err != nil || v == 0 {
		return def
	}
	return v
}

// pyBool converts a Go bool to a Python bool string ("True"/"False").
func pyBool(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

// --- Dispatch map ---

var dispatch = map[mcp.ToolName]handlerFunc{
	// Version / info
	mcp.ToolName("freecad_get_version"): getVersion,

	// Document management
	mcp.ToolName("freecad_list_documents"):  listDocuments,
	mcp.ToolName("freecad_create_document"): createDocument,
	mcp.ToolName("freecad_open_document"):   openDocument,
	mcp.ToolName("freecad_save_document"):   saveDocument,
	mcp.ToolName("freecad_close_document"):  closeDocument,
	mcp.ToolName("freecad_get_document"):    getDocument,

	// Objects
	mcp.ToolName("freecad_list_objects"):   listObjects,
	mcp.ToolName("freecad_get_object"):     getObject,
	mcp.ToolName("freecad_delete_object"):  deleteObject,
	mcp.ToolName("freecad_set_placement"):  setPlacement,
	mcp.ToolName("freecad_get_properties"): getProperties,
	mcp.ToolName("freecad_set_property"):   setProperty,

	// Primitives
	mcp.ToolName("freecad_create_box"):      createBox,
	mcp.ToolName("freecad_create_cylinder"): createCylinder,
	mcp.ToolName("freecad_create_sphere"):   createSphere,
	mcp.ToolName("freecad_create_cone"):     createCone,
	mcp.ToolName("freecad_create_torus"):    createTorus,

	// Boolean operations
	mcp.ToolName("freecad_boolean_cut"):    booleanCut,
	mcp.ToolName("freecad_boolean_fuse"):   booleanFuse,
	mcp.ToolName("freecad_boolean_common"): booleanCommon,

	// Shape operations
	mcp.ToolName("freecad_fillet"):  fillet,
	mcp.ToolName("freecad_chamfer"): chamfer,
	mcp.ToolName("freecad_extrude"): extrude,
	mcp.ToolName("freecad_mirror"):  mirror,

	// Measurement / analysis
	mcp.ToolName("freecad_measure_shape"):  measureShape,
	mcp.ToolName("freecad_check_geometry"): checkGeometry,
	mcp.ToolName("freecad_bounding_box"):   boundingBox,

	// Export / import
	mcp.ToolName("freecad_export_step"): exportSTEP,
	mcp.ToolName("freecad_export_stl"):  exportSTL,
	mcp.ToolName("freecad_export_brep"): exportBRep,
	mcp.ToolName("freecad_import_file"): importFile,

	// Sketcher
	mcp.ToolName("freecad_create_sketch"):          createSketch,
	mcp.ToolName("freecad_add_sketch_line"):        addSketchLine,
	mcp.ToolName("freecad_add_sketch_circle"):      addSketchCircle,
	mcp.ToolName("freecad_add_sketch_arc"):         addSketchArc,
	mcp.ToolName("freecad_add_sketch_rectangle"):   addSketchRectangle,
	mcp.ToolName("freecad_add_sketch_polygon"):     addSketchPolygon,
	mcp.ToolName("freecad_add_constraint"):         addConstraint,
	mcp.ToolName("freecad_get_sketch"):             getSketch,
	mcp.ToolName("freecad_delete_sketch_geometry"): deleteSketchGeometry,

	// PartDesign
	mcp.ToolName("freecad_create_body"):    createBody,
	mcp.ToolName("freecad_pad"):            pad,
	mcp.ToolName("freecad_pocket"):         pocket,
	mcp.ToolName("freecad_revolution"):     revolution,
	mcp.ToolName("freecad_groove"):         groove,
	mcp.ToolName("freecad_pd_fillet"):      pdFillet,
	mcp.ToolName("freecad_pd_chamfer"):     pdChamfer,
	mcp.ToolName("freecad_pd_mirror"):      pdMirror,
	mcp.ToolName("freecad_linear_pattern"): linearPattern,
	mcp.ToolName("freecad_polar_pattern"):  polarPattern,
	mcp.ToolName("freecad_pd_hole"):        pdHole,

	// Python scripting
	mcp.ToolName("freecad_run_script"): runUserScript,
}
