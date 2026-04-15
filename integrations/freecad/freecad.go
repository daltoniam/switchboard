package freecad

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

var (
	_ mcp.Integration                = (*freecad)(nil)
	_ mcp.FieldCompactionIntegration = (*freecad)(nil)
	_ mcp.PlainTextCredentials       = (*freecad)(nil)
)

type freecad struct {
	binary     string
	dataDir    string
	bridgePath string

	mu      sync.Mutex
	port    int
	cmd     *exec.Cmd
	client  *http.Client
	started bool
}

func New() mcp.Integration {
	return &freecad{
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

func (f *freecad) Name() string { return "freecad" }

func (f *freecad) PlainTextKeys() []string {
	return []string{"binary_path", "data_dir"}
}

func (f *freecad) Configure(_ context.Context, creds mcp.Credentials) error {
	f.binary = creds["binary_path"]
	if f.binary == "" {
		path, err := exec.LookPath("freecad")
		if err != nil {
			return fmt.Errorf("freecad: binary_path not set and freecad not found in PATH")
		}
		f.binary = path
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

	// Locate bridge script
	f.bridgePath = locateBridge(f.binary)
	if f.bridgePath == "" {
		return fmt.Errorf("freecad: SwitchboardBridge not found — install it in FreeCAD's Mod directory")
	}
	return nil
}

func (f *freecad) Healthy(ctx context.Context) bool {
	if f.binary == "" {
		return false
	}
	if err := f.ensureServer(ctx); err != nil {
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

// --- Bridge lifecycle ---

// locateBridge finds the SwitchboardBridge bridge.py relative to the FreeCAD binary
// or in the standard FreeCAD Mod directories.
func locateBridge(binary string) string {
	// Check standard FreeCAD user data locations
	home, _ := os.UserHomeDir()
	candidates := []string{}
	if home != "" {
		// Linux
		candidates = append(candidates,
			filepath.Join(home, ".local/share/FreeCAD/v1-1/Mod/SwitchboardBridge/bridge.py"),
			filepath.Join(home, ".local/share/FreeCAD/Mod/SwitchboardBridge/bridge.py"),
			filepath.Join(home, ".FreeCAD/Mod/SwitchboardBridge/bridge.py"),
		)
		// macOS
		candidates = append(candidates,
			filepath.Join(home, "Library/Application Support/FreeCAD/Mod/SwitchboardBridge/bridge.py"),
			filepath.Join(home, "Library/Preferences/FreeCAD/Mod/SwitchboardBridge/bridge.py"),
		)
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func (f *freecad) ensureServer(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.started && f.cmd != nil && f.cmd.ProcessState == nil {
		return nil
	}
	f.started = false

	pr, pw, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("freecad: pipe: %w", err)
	}

	// Launch: echo 'exec(open("bridge.py").read())' | freecad --console
	launcher := fmt.Sprintf(`exec(open(%q).read())`, f.bridgePath)
	cmd := exec.CommandContext(ctx, f.binary, "--console") // #nosec G204 -- binary path from trusted config
	cmd.Stdin = strings.NewReader(launcher)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.ExtraFiles = []*os.File{pw} // fd 3

	if err := cmd.Start(); err != nil {
		_ = pr.Close()
		_ = pw.Close()
		return fmt.Errorf("freecad: start bridge: %w", err)
	}
	_ = pw.Close()

	buf := make([]byte, 64)
	_ = pr.SetDeadline(time.Now().Add(30 * time.Second))
	n, err := pr.Read(buf)
	_ = pr.Close()
	if err != nil {
		_ = cmd.Process.Kill()
		return fmt.Errorf("freecad: read port from bridge: %w", err)
	}

	portStr := strings.TrimSpace(string(buf[:n]))
	var port int
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		_ = cmd.Process.Kill()
		return fmt.Errorf("freecad: invalid port %q: %w", portStr, err)
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 500*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	f.cmd = cmd
	f.port = port
	f.started = true
	go func() { _ = cmd.Wait() }()

	return nil
}

// --- XMLRPC client (RobustMCPBridge protocol) ---

func (f *freecad) rpcCall(ctx context.Context, method string, params ...string) (string, error) {
	if err := f.ensureServer(ctx); err != nil {
		return "", err
	}

	var xmlBody string
	if len(params) > 0 {
		xmlBody = fmt.Sprintf(`<?xml version="1.0"?><methodCall><methodName>%s</methodName><params><param><value><string>%s</string></value></param></params></methodCall>`,
			method, xmlEscape(params[0]))
	} else {
		xmlBody = fmt.Sprintf(`<?xml version="1.0"?><methodCall><methodName>%s</methodName><params></params></methodCall>`, method)
	}

	url := fmt.Sprintf("http://127.0.0.1:%d", f.port)
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(xmlBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "text/xml")

	resp, err := f.client.Do(req)
	if err != nil {
		f.mu.Lock()
		f.started = false
		f.mu.Unlock()
		return "", fmt.Errorf("freecad: rpc call failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return "", err
	}

	return parseXMLRPCResponse(body)
}

// execScript executes Python code in the persistent FreeCAD process.
// The code should set _result_ to return structured data.
func (f *freecad) execScript(ctx context.Context, script string) (string, error) {
	raw, err := f.rpcCall(ctx, "execute", script)
	if err != nil {
		return "", err
	}
	// The XMLRPC execute method returns a dict serialized as XMLRPC struct.
	// Our simple parser returns it as a string — parse the result.
	return raw, nil
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
	Inner   string        `xml:",chardata"`
}

type xmlrpcStruct struct {
	Members []xmlrpcMember `xml:"member"`
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
	if v.Struct != nil {
		return structToJSON(v.Struct), nil
	}
	if v.String != "" {
		return v.String, nil
	}
	if v.Boolean != "" {
		return v.Boolean, nil
	}
	if v.Inner != "" {
		return strings.TrimSpace(v.Inner), nil
	}
	return "", nil
}

func structToJSON(s *xmlrpcStruct) string {
	m := make(map[string]any, len(s.Members))
	for _, mem := range s.Members {
		m[mem.Name] = valueToAny(&mem.Value)
	}
	data, _ := json.Marshal(m)
	return string(data)
}

func valueToAny(v *xmlrpcValue) any {
	if v.Struct != nil {
		m := make(map[string]any, len(v.Struct.Members))
		for _, mem := range v.Struct.Members {
			m[mem.Name] = valueToAny(&mem.Value)
		}
		return m
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
		var f float64
		_, _ = fmt.Sscanf(v.Double, "%f", &f)
		return f
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
	// raw is the XMLRPC struct with {success, result, stdout, stderr, ...}
	// Parse it to extract the _result_ value
	var resp struct {
		Success      bool   `json:"success"`
		Result       any    `json:"result"`
		ErrorType    string `json:"error_type"`
		ErrorMessage string `json:"error_message"`
		Stdout       string `json:"stdout"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		// Not JSON struct — return as-is
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

	// Python scripting
	mcp.ToolName("freecad_run_script"): runUserScript,
}
