package mcp_test

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"mvdan.cc/sh/v3/syntax"
)

type goreleaserBuild struct {
	ID        string   `yaml:"id"`
	Env       []string `yaml:"env"`
	Goos      []string `yaml:"goos"`
	Goarch    []string `yaml:"goarch"`
	Overrides []struct {
		Goos   string   `yaml:"goos"`
		Goarch string   `yaml:"goarch"`
		Env    []string `yaml:"env"`
	} `yaml:"overrides"`
}

func loadGoreleaserBuilds(t *testing.T) []goreleaserBuild {
	t.Helper()
	data, err := os.ReadFile(".goreleaser.yml")
	require.NoError(t, err)
	var cfg struct {
		Builds []goreleaserBuild `yaml:"builds"`
	}
	require.NoError(t, yaml.Unmarshal(data, &cfg))
	require.NotEmpty(t, cfg.Builds)
	return cfg.Builds
}

// envValue returns the last value of key, as later entries override earlier
// ones.
func envValue(env []string, key string) (string, bool) {
	val, found := "", false
	for _, kv := range env {
		if k, v, ok := strings.Cut(kv, "="); ok && k == key {
			val, found = v, true
		}
	}
	return val, found
}

// TestGoreleaserCgoTargets guards the release build split. macOS binaries
// need cgo so the 1Password SDK can load the desktop app's library; Linux and
// Windows binaries must stay static for the FROM scratch Docker image and the
// distro packages.
func TestGoreleaserCgoTargets(t *testing.T) {
	builds := loadGoreleaserBuilds(t)

	tests := []struct {
		goos    string
		goarch  string
		cgo     string
		needsCC bool
	}{
		{"darwin", "amd64", "1", true},
		{"darwin", "arm64", "1", true},
		{"linux", "amd64", "0", false},
		{"linux", "arm64", "0", false},
		{"windows", "amd64", "0", false},
		{"windows", "arm64", "0", false},
	}
	for _, tc := range tests {
		t.Run(tc.goos+"_"+tc.goarch, func(t *testing.T) {
			var matched []goreleaserBuild
			for _, b := range builds {
				if slices.Contains(b.Goos, tc.goos) && slices.Contains(b.Goarch, tc.goarch) {
					matched = append(matched, b)
				}
			}
			require.Len(t, matched, 1, "exactly one build must produce %s/%s, or archives and packages get duplicate binaries", tc.goos, tc.goarch)
			b := matched[0]

			// goreleaser appends a matching override's env to the build env,
			// so the last value of a key wins.
			env := slices.Clone(b.Env)
			for _, o := range b.Overrides {
				if o.Goos == tc.goos && o.Goarch == tc.goarch {
					env = append(env, o.Env...)
				}
			}

			cgo, ok := envValue(env, "CGO_ENABLED")
			require.True(t, ok, "build %q must set CGO_ENABLED explicitly", b.ID)
			assert.Equal(t, tc.cgo, cgo, "build %q CGO_ENABLED for %s/%s", b.ID, tc.goos, tc.goarch)

			if tc.cgo == "1" {
				assert.Len(t, b.Goos, 1, "cgo build %q must only target %s, so other platforms stay static", b.ID, tc.goos)
			}

			cc, _ := envValue(env, "CC")
			if tc.needsCC {
				assert.NotEmpty(t, cc, "build %q needs a per-target CC override (osxcross clang) for %s/%s", b.ID, tc.goos, tc.goarch)
			} else {
				assert.Empty(t, cc, "build %q must not set a C compiler for %s/%s", b.ID, tc.goos, tc.goarch)
			}
		})
	}
}

// TestReleaseWorkflowsShareCrossImage keeps the tag-triggered release and
// the pull request dry run on the same goreleaser-cross toolchain, so a
// passing dry run proves the release build.
func TestReleaseWorkflowsShareCrossImage(t *testing.T) {
	script, err := os.ReadFile("scripts/goreleaser-cross.sh")
	require.NoError(t, err)
	image := regexp.MustCompile(`ghcr\.io/goreleaser/goreleaser-cross:v[0-9][0-9A-Za-z.\-]*`).Find(script)
	require.NotNil(t, image, "scripts/goreleaser-cross.sh must pin a goreleaser-cross image tag")

	tests := []struct {
		workflow string
		snapshot bool
	}{
		{".github/workflows/release.yml", false},
		{".github/workflows/release-dry-run.yml", true},
	}
	for _, tc := range tests {
		t.Run(tc.workflow, func(t *testing.T) {
			wf := parseWorkflowShell(t, tc.workflow)
			for _, uses := range wf.uses {
				assert.NotContains(t, uses, "goreleaser/goreleaser-action", "%s must not build with the host toolchain, which has no macOS SDK", tc.workflow)
			}
			assert.Empty(t, wf.hostGoreleaser, "%s must not run goreleaser with the host toolchain, which has no macOS SDK", tc.workflow)
			require.Len(t, wf.releases, 1, "%s must run `scripts/goreleaser-cross.sh release` exactly once", tc.workflow)
			args := wf.releases[0]
			if tc.snapshot {
				assert.Contains(t, args, "--snapshot", "the dry run must not publish")
				assert.Contains(t, args, "--skip=publish", "the dry run must not publish")
			} else {
				for _, a := range args {
					assert.False(t, a == "--snapshot" || strings.HasPrefix(a, "--skip"), "the release must publish every artifact, got %q", a)
				}
			}
		})
	}
}

type workflowShell struct {
	uses           []string
	releases       [][]string // arguments after `scripts/goreleaser-cross.sh release`
	hostGoreleaser []string   // goreleaser run directly on the runner
}

// parseWorkflowShell parses every step's `run` script with a real shell
// parser and collects the commands that actually execute, so comments,
// echoed text and chained commands are told apart from real invocations.
func parseWorkflowShell(t *testing.T, path string) workflowShell {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var wf struct {
		Jobs map[string]struct {
			Steps []struct {
				Uses string `yaml:"uses"`
				Run  string `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	require.NoError(t, yaml.Unmarshal(data, &wf))

	var out workflowShell
	for _, job := range wf.Jobs {
		for _, step := range job.Steps {
			if step.Uses != "" {
				out.uses = append(out.uses, step.Uses)
			}
			if step.Run == "" {
				continue
			}
			file, err := syntax.NewParser().Parse(strings.NewReader(step.Run), path)
			require.NoError(t, err, "%s: parse run script", path)
			syntax.Walk(file, func(node syntax.Node) bool {
				call, ok := node.(*syntax.CallExpr)
				if !ok || len(call.Args) == 0 {
					return true
				}
				words := make([]string, len(call.Args))
				for i, w := range call.Args {
					words[i] = w.Lit() // "" when the word is not a plain literal
				}
				if words[0] == "goreleaser" {
					out.hostGoreleaser = append(out.hostGoreleaser, strings.Join(words, " "))
				}
				if args, ok := crossScriptArgs(words); ok && len(args) > 0 && args[0] == "release" {
					out.releases = append(out.releases, args[1:])
				}
				return true
			})
		}
	}
	return out
}

// crossScriptArgs returns the arguments of a command that runs
// scripts/goreleaser-cross.sh, directly or through bash or sh.
func crossScriptArgs(words []string) ([]string, bool) {
	isScript := func(w string) bool {
		return w == "scripts/goreleaser-cross.sh" || w == "./scripts/goreleaser-cross.sh"
	}
	switch {
	case isScript(words[0]):
		return words[1:], true
	case (words[0] == "bash" || words[0] == "sh") && len(words) > 1 && isScript(words[1]):
		return words[2:], true
	}
	return nil, false
}

func TestParseWorkflowShell(t *testing.T) {
	tests := []struct {
		name     string
		run      string
		releases [][]string
		host     int
	}{
		{"wrapper with env prefix and continuation", "A=\"$HOME\" \\\n  bash scripts/goreleaser-cross.sh release --snapshot --skip=publish", [][]string{{"--snapshot", "--skip=publish"}}, 0},
		{"trailing comment is not an argument", "bash scripts/goreleaser-cross.sh release --clean # --snapshot --skip=publish", [][]string{{"--clean"}}, 0},
		{"commented-out command", "# bash scripts/goreleaser-cross.sh release --snapshot\ngoreleaser release", nil, 1},
		{"echo is not an invocation", "echo scripts/goreleaser-cross.sh release --snapshot --skip=publish", nil, 0},
		{"chained invocations both count", "bash scripts/goreleaser-cross.sh release --snapshot --skip=publish; scripts/goreleaser-cross.sh release", [][]string{{"--snapshot", "--skip=publish"}, {}}, 0},
		{"other subcommand", "bash scripts/goreleaser-cross.sh check", nil, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := dir + "/wf.yml"
			wf, err := yaml.Marshal(map[string]any{"jobs": map[string]any{"j": map[string]any{"steps": []map[string]string{{"run": tc.run}}}}})
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(path, wf, 0o600))
			got := parseWorkflowShell(t, path)
			assert.Equal(t, tc.releases, got.releases)
			assert.Len(t, got.hostGoreleaser, tc.host)
		})
	}
}
