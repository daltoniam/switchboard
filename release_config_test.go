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

func envValue(env []string, key string) (string, bool) {
	for _, kv := range env {
		if k, v, ok := strings.Cut(kv, "="); ok && k == key {
			return v, true
		}
	}
	return "", false
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

			cgo, ok := envValue(b.Env, "CGO_ENABLED")
			require.True(t, ok, "build %q must set CGO_ENABLED explicitly", b.ID)
			assert.Equal(t, tc.cgo, cgo, "build %q CGO_ENABLED", b.ID)

			if tc.cgo == "1" {
				assert.Len(t, b.Goos, 1, "cgo build %q must only target %s, so other platforms stay static", b.ID, tc.goos)
			}

			var cc string
			for _, o := range b.Overrides {
				if o.Goos == tc.goos && o.Goarch == tc.goarch {
					cc, _ = envValue(o.Env, "CC")
				}
			}
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

	for _, wf := range []string{".github/workflows/release.yml", ".github/workflows/release-dry-run.yml"} {
		data, err := os.ReadFile(wf)
		require.NoError(t, err)
		assert.Contains(t, string(data), "scripts/goreleaser-cross.sh", "%s must run goreleaser through scripts/goreleaser-cross.sh", wf)
		assert.NotContains(t, string(data), "goreleaser/goreleaser-action", "%s must not build with the host toolchain, which has no macOS SDK", wf)
	}
}
