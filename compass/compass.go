// Package compass provides adaptive instruction template rendering for AI coding agents.
// It is inspired by the Helmsman Rust crate (https://github.com/seuros/helmsman).
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

// DefaultModelTiers maps known model IDs to their capability tier.
// "large" = frontier/capable models, "small" = limited/fast models.
var DefaultModelTiers = map[string]string{
	// Anthropic
	"claude-opus-4-5-20251101":   "large",
	"claude-sonnet-4-20250514":   "large",
	"claude-3-5-sonnet-20241022": "large",
	"claude-3-5-haiku-20241022":  "small",
	"claude-3-haiku-20240307":    "small",

	// OpenAI
	"gpt-4o":        "large",
	"gpt-4o-mini":   "small",
	"gpt-4-turbo":   "large",
	"gpt-4":         "large",
	"o1":            "large",
	"o1-mini":       "small",
	"o1-preview":    "large",
	"o3":            "large",
	"o3-mini":       "small",
	"gpt-3.5-turbo": "small",

	// Google
	"gemini-2.0-flash":      "large",
	"gemini-2.0-flash-lite": "small",
	"gemini-1.5-pro":        "large",
	"gemini-1.5-flash":      "small",
	"gemini-2.5-pro":        "large",

	// DeepSeek
	"deepseek-chat":     "large",
	"deepseek-reasoner": "large",

	// Meta
	"llama-3.3-70b": "large",
	"llama-3.1-8b":  "small",

	// Mistral
	"mistral-large": "large",
	"mistral-small": "small",

	// Qwen
	"qwen-2.5-72b": "large",
}

// DefaultTier is used when a model ID is not recognized.
const DefaultTier = "large"

// GetModelTier returns the capability tier for a model ID.
// customTiers takes precedence over DefaultModelTiers.
func GetModelTier(modelID, defaultTier string, customTiers map[string]string) string {
	// Check custom tiers first (from config)
	if customTiers != nil {
		if tier, ok := customTiers[modelID]; ok {
			return tier
		}
	}
	// Check default tiers
	if tier, ok := DefaultModelTiers[modelID]; ok {
		return tier
	}
	// Try partial match (model ID prefix) in custom tiers first
	lower := strings.ToLower(modelID)
	for id, tier := range customTiers {
		if strings.HasPrefix(lower, strings.ToLower(id)) {
			return tier
		}
	}
	// Try partial match in default tiers
	for id, tier := range DefaultModelTiers {
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
// customTiers allows overriding or extending the default model tier mappings.
func RenderInstructions(instructions []*mcp.Instruction, modelID, defaultTier string, customTiers map[string]string) (string, error) {
	ctx := mcp.InstructionRenderContext{
		Model: mcp.ModelContext{
			ID:   modelID,
			Tier: GetModelTier(modelID, defaultTier, customTiers),
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
// customTiers allows overriding or extending the default model tier mappings.
func BuildRenderContext(modelID, defaultTier string, extraVars map[string]string, customTiers map[string]string) mcp.InstructionRenderContext {
	ctx := mcp.InstructionRenderContext{
		Model: mcp.ModelContext{
			ID:   modelID,
			Tier: GetModelTier(modelID, defaultTier, customTiers),
		},
		Env:  DetectEnv(),
		Vars: extraVars,
	}
	if ctx.Vars == nil {
		ctx.Vars = make(map[string]string)
	}
	return ctx
}
