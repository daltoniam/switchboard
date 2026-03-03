package compass

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetModelTier(t *testing.T) {
	tests := []struct {
		name        string
		modelID     string
		defaultTier string
		want        string
	}{
		{
			name:    "claude opus exact match",
			modelID: "claude-opus-4-5-20251101",
			want:    "agi",
		},
		{
			name:    "gpt-4o exact match",
			modelID: "gpt-4o",
			want:    "engineer",
		},
		{
			name:    "unknown model uses default tier",
			modelID: "unknown-model-123",
			want:    DefaultTier,
		},
		{
			name:        "unknown model uses custom default",
			modelID:     "unknown-model-123",
			defaultTier: "monkey",
			want:        "monkey",
		},
		{
			name:    "partial match prefix",
			modelID: "claude-opus-4-5-20251101-extended",
			want:    "agi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetModelTier(tt.modelID, tt.defaultTier)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDetectEnv(t *testing.T) {
	env := DetectEnv()

	assert.NotEmpty(t, env.OS, "OS should be detected")
	assert.NotEmpty(t, env.Shell, "Shell should be detected")
	// Boolean fields have sensible defaults
	assert.IsType(t, false, env.InDocker)
	assert.IsType(t, false, env.InSSH)
}

func TestRenderInstruction(t *testing.T) {
	tests := []struct {
		name     string
		inst     *mcp.Instruction
		ctx      mcp.InstructionRenderContext
		want     string
		wantErr  bool
	}{
		{
			name: "simple template no variables",
			inst: &mcp.Instruction{
				ID:       "test1",
				Name:     "Test",
				Template: "Hello, world!",
				Enabled:  true,
			},
			ctx:  mcp.InstructionRenderContext{},
			want: "Hello, world!",
		},
		{
			name: "template with model context",
			inst: &mcp.Instruction{
				ID:       "test2",
				Name:     "Test",
				Template: "Model: {{.Model.ID}} (Tier: {{.Model.Tier}})",
				Enabled:  true,
			},
			ctx: mcp.InstructionRenderContext{
				Model: mcp.ModelContext{
					ID:   "claude-opus-4-5-20251101",
					Tier: "agi",
				},
			},
			want: "Model: claude-opus-4-5-20251101 (Tier: agi)",
		},
		{
			name: "template with env context",
			inst: &mcp.Instruction{
				ID:       "test3",
				Name:     "Test",
				Template: "{{if eq .Env.OS \"macos\"}}macOS detected{{else}}Other OS{{end}}",
				Enabled:  true,
			},
			ctx: mcp.InstructionRenderContext{
				Env: mcp.EnvContext{OS: "macos"},
			},
			want: "macOS detected",
		},
		{
			name: "template with custom variables",
			inst: &mcp.Instruction{
				ID:       "test4",
				Name:     "Test",
				Template: "Project: {{.Vars.project_name}}",
				Enabled:  true,
				Variables: map[string]string{
					"project_name": "Switchboard",
				},
			},
			ctx: mcp.InstructionRenderContext{
				Vars: map[string]string{},
			},
			want: "Project: Switchboard",
		},
		{
			name: "context vars override instruction vars",
			inst: &mcp.Instruction{
				ID:       "test5",
				Name:     "Test",
				Template: "Version: {{.Vars.version}}",
				Enabled:  true,
				Variables: map[string]string{
					"version": "1.0",
				},
			},
			ctx: mcp.InstructionRenderContext{
				Vars: map[string]string{
					"version": "2.0",
				},
			},
			want: "Version: 2.0",
		},
		{
			name: "conditional based on model tier",
			inst: &mcp.Instruction{
				ID:       "test6",
				Name:     "Test",
				Template: "{{if eq .Model.Tier \"agi\"}}Full capabilities{{else}}Limited mode{{end}}",
				Enabled:  true,
			},
			ctx: mcp.InstructionRenderContext{
				Model: mcp.ModelContext{Tier: "agi"},
			},
			want: "Full capabilities",
		},
		{
			name: "tool availability check",
			inst: &mcp.Instruction{
				ID:       "test7",
				Name:     "Test",
				Template: "{{if .Env.HasGit}}Git available{{else}}No git{{end}}",
				Enabled:  true,
			},
			ctx: mcp.InstructionRenderContext{
				Env: mcp.EnvContext{HasGit: true},
			},
			want: "Git available",
		},
		{
			name: "invalid template syntax",
			inst: &mcp.Instruction{
				ID:       "test8",
				Name:     "Test",
				Template: "{{.Invalid",
				Enabled:  true,
			},
			ctx:     mcp.InstructionRenderContext{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RenderInstruction(tt.inst, tt.ctx)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRenderInstructions(t *testing.T) {
	instructions := []*mcp.Instruction{
		{
			ID:       "inst1",
			Name:     "First",
			Template: "First instruction for {{.Model.Tier}}",
			Enabled:  true,
		},
		{
			ID:       "inst2",
			Name:     "Disabled",
			Template: "Should not appear",
			Enabled:  false,
		},
		{
			ID:       "inst3",
			Name:     "Third",
			Template: "Third instruction",
			Enabled:  true,
		},
	}

	result, err := RenderInstructions(instructions, "gpt-4o", "")
	require.NoError(t, err)

	assert.Contains(t, result, "First instruction for engineer")
	assert.Contains(t, result, "Third instruction")
	assert.NotContains(t, result, "Should not appear")
}

func TestBuildRenderContext(t *testing.T) {
	ctx := BuildRenderContext("claude-opus-4-5-20251101", "", map[string]string{
		"custom": "value",
	})

	assert.Equal(t, "claude-opus-4-5-20251101", ctx.Model.ID)
	assert.Equal(t, "agi", ctx.Model.Tier)
	assert.NotEmpty(t, ctx.Env.OS)
	assert.Equal(t, "value", ctx.Vars["custom"])
}

func TestBuildRenderContext_DefaultTier(t *testing.T) {
	ctx := BuildRenderContext("unknown-model", "monkey", nil)

	assert.Equal(t, "unknown-model", ctx.Model.ID)
	assert.Equal(t, "monkey", ctx.Model.Tier)
	assert.NotNil(t, ctx.Vars)
}
