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

	releaseCmd := regexp.MustCompile(`scripts/goreleaser-cross\.sh\s+release(\s|$)`)
	hostGoreleaser := regexp.MustCompile(`(^|[\s;&|(])goreleaser\s`)
	tests := []struct {
		workflow string
		snapshot bool
	}{
		{".github/workflows/release.yml", false},
		{".github/workflows/release-dry-run.yml", true},
	}
	for _, tc := range tests {
		t.Run(tc.workflow, func(t *testing.T) {
			var releases []string
			for _, cmd := range workflowCommands(t, tc.workflow) {
				if releaseCmd.MatchString(cmd) {
					releases = append(releases, cmd)
				}
				assert.NotRegexp(t, hostGoreleaser, cmd, "%s must not run goreleaser with the host toolchain, which has no macOS SDK", tc.workflow)
				assert.NotContains(t, cmd, "goreleaser/goreleaser-action", "%s must not build with the host toolchain, which has no macOS SDK", tc.workflow)
			}
			require.Len(t, releases, 1, "%s must run `scripts/goreleaser-cross.sh release` exactly once", tc.workflow)
			if tc.snapshot {
				assert.Contains(t, releases[0], "--snapshot", "the dry run must not publish")
				assert.Contains(t, releases[0], "--skip=publish", "the dry run must not publish")
			} else {
				assert.NotContains(t, releases[0], "--snapshot", "the release must publish")
				assert.NotContains(t, releases[0], "--skip", "the release must publish every artifact")
			}
		})
	}
}

// workflowCommands returns each step's `uses` value and its `run` script
// with shell comment lines removed, so commented-out commands don't count.
func workflowCommands(t *testing.T, path string) []string {
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
	var out []string
	for _, job := range wf.Jobs {
		for _, step := range job.Steps {
			if step.Uses != "" {
				out = append(out, step.Uses)
			}
			var lines []string
			for line := range strings.SplitSeq(step.Run, "\n") {
				if !strings.HasPrefix(strings.TrimSpace(line), "#") {
					lines = append(lines, line)
				}
			}
			if cmd := strings.TrimSpace(strings.Join(lines, "\n")); cmd != "" {
				out = append(out, cmd)
			}
		}
	}
	require.NotEmpty(t, out, "%s has no steps", path)
	return out
}
