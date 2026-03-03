// Package compass provides adaptive instruction template rendering for AI coding agents.
// It is inspired by the Helmsman Ruby gem (https://github.com/seuros/helmsman).
package compass

import (
	"bytes"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"text/template"

	mcp "github.com/daltoniam/switchboard"
)

// ModelTiers maps known model IDs to their capability tier.
// "agi" = frontier models, "engineer" = capable models, "monkey" = limited models.
var ModelTiers = map[string]string{
	// Anthropic
	"claude-opus-4-5-20251101":   "agi",
	"claude-sonnet-4-20250514":   "engineer",
	"claude-3-5-sonnet-20241022": "engineer",
	"claude-3-5-haiku-20241022":  "monkey",
	"claude-3-haiku-20240307":    "monkey",

	// OpenAI
	"gpt-4o":         "engineer",
	"gpt-4o-mini":    "monkey",
	"gpt-4-turbo":    "engineer",
	"gpt-4":          "engineer",
	"o1":             "agi",
	"o1-mini":        "monkey",
	"o1-preview":     "agi",
	"o3":             "agi",
	"o3-mini":        "engineer",
	"gpt-3.5-turbo":  "monkey",

	// Google
	"gemini-2.0-flash":       "engineer",
	"gemini-2.0-flash-lite":  "monkey",
	"gemini-1.5-pro":         "engineer",
	"gemini-1.5-flash":       "monkey",
	"gemini-2.5-pro":         "agi",

	// Other
	"deepseek-chat":       "engineer",
	"deepseek-reasoner":   "agi",
	"llama-3.3-70b":       "engineer",
	"llama-3.1-8b":        "monkey",
	"mistral-large":       "engineer",
	"mistral-small":       "monkey",
	"qwen-2.5-72b":        "engineer",
}

// DefaultTier is used when a model ID is not recognized.
const DefaultTier = "engineer"

// GetModelTier returns the capability tier for a model ID.
func GetModelTier(modelID, defaultTier string) string {
	if tier, ok := ModelTiers[modelID]; ok {
		return tier
	}
	// Try partial match (model ID prefix)
	lower := strings.ToLower(modelID)
	for id, tier := range ModelTiers {
		if strings.HasPrefix(lower, strings.ToLower(id)) {
			return tier
		}
	}
	if defaultTier != "" {
		return defaultTier
	}
	return DefaultTier
}

// DetectEnv builds an EnvContext by probing the runtime environment.
func DetectEnv() mcp.EnvContext {
	env := mcp.EnvContext{}

	// OS detection
	switch runtime.GOOS {
	case "darwin":
		env.OS = "macos"
	case "linux":
		env.OS = detectLinuxDistro()
	case "windows":
		env.OS = "windows"
	default:
		env.OS = runtime.GOOS
	}

	// Shell detection
	env.Shell = detectShell()

	// Docker detection
	env.InDocker = fileExists("/.dockerenv")

	// SSH detection
	env.InSSH = os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_CLIENT") != ""

	// Tool availability
	env.HasGit = commandExists("git")
	env.HasGh = commandExists("gh")
	env.HasMise = commandExists("mise")
	env.HasBrew = commandExists("brew")
	env.HasApt = commandExists("apt")

	return env
}

func detectLinuxDistro() string {
	// Try to read /etc/os-release
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "linux"
	}
	content := string(data)
	lower := strings.ToLower(content)

	if strings.Contains(lower, "arch") {
		return "arch"
	}
	if strings.Contains(lower, "debian") {
		return "debian"
	}
	if strings.Contains(lower, "ubuntu") {
		return "ubuntu"
	}
	if strings.Contains(lower, "alpine") {
		return "alpine"
	}
	if strings.Contains(lower, "fedora") {
		return "fedora"
	}
	if strings.Contains(lower, "centos") || strings.Contains(lower, "rhel") {
		return "rhel"
	}
	return "linux"
}

func detectShell() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		if runtime.GOOS == "windows" {
			return "powershell"
		}
		return "sh"
	}
	// Extract shell name from path
	parts := strings.Split(shell, "/")
	name := parts[len(parts)-1]
	switch name {
	case "zsh", "bash", "fish", "sh", "dash", "ksh", "csh", "tcsh":
		return name
	default:
		return "sh"
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// RenderInstruction renders an instruction template with the given context.
func RenderInstruction(inst *mcp.Instruction, ctx mcp.InstructionRenderContext) (string, error) {
	// Merge instruction-level variables with context variables
	vars := make(map[string]string)
	for k, v := range inst.Variables {
		vars[k] = v
	}
	for k, v := range ctx.Vars {
		vars[k] = v
	}
	ctx.Vars = vars

	tmpl, err := template.New(inst.ID).Parse(inst.Template)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// RenderInstructions renders all enabled instructions and concatenates them.
func RenderInstructions(instructions []*mcp.Instruction, modelID, defaultTier string) (string, error) {
	ctx := mcp.InstructionRenderContext{
		Model: mcp.ModelContext{
			ID:   modelID,
			Tier: GetModelTier(modelID, defaultTier),
		},
		Env:  DetectEnv(),
		Vars: make(map[string]string),
	}

	var results []string
	for _, inst := range instructions {
		if !inst.Enabled {
			continue
		}
		rendered, err := RenderInstruction(inst, ctx)
		if err != nil {
			return "", err
		}
		results = append(results, rendered)
	}

	return strings.Join(results, "\n\n"), nil
}

// BuildRenderContext creates a render context for the given model ID.
func BuildRenderContext(modelID, defaultTier string, extraVars map[string]string) mcp.InstructionRenderContext {
	ctx := mcp.InstructionRenderContext{
		Model: mcp.ModelContext{
			ID:   modelID,
			Tier: GetModelTier(modelID, defaultTier),
		},
		Env:  DetectEnv(),
		Vars: extraVars,
	}
	if ctx.Vars == nil {
		ctx.Vars = make(map[string]string)
	}
	return ctx
}
