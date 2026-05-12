package compactyaml

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
)

// EnvOverrideDir is the environment variable that points to a directory
// containing per-adapter YAML override files.
const EnvOverrideDir = "SWITCHBOARD_COMPACT_DIR"

// LoadWithOverlay loads embedded defaults, then merges per-tool overrides from
// $SWITCHBOARD_COMPACT_DIR/<adapter>.yaml if the file exists.
// Merge is per-tool key: overlay entries replace embedded ones; missing entries fall through.
func LoadWithOverlay(adapter string, embedded []byte, opts Options) (Result, error) {
	base, err := Load(embedded, opts)
	if err != nil {
		return Result{}, err
	}

	dir := os.Getenv(EnvOverrideDir)
	if dir == "" {
		return base, nil
	}

	path := filepath.Join(dir, adapter+".yaml")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return base, nil
	}
	if err != nil {
		return absorbOverlayErr(base, opts, fmt.Errorf("compactyaml: read overlay %s: %w", path, err))
	}

	overlay, err := Load(data, opts)
	if err != nil {
		return absorbOverlayErr(base, opts, fmt.Errorf("compactyaml: overlay %s: %w", path, err))
	}

	mergeOverlay(&base, overlay)
	return base, nil
}

// MustLoadWithOverlay panics on error.
func MustLoadWithOverlay(adapter string, embedded []byte, opts Options) Result {
	res, err := LoadWithOverlay(adapter, embedded, opts)
	if err != nil {
		panic(err)
	}
	return res
}

// absorbOverlayErr applies strict/lenient posture to an overlay-layer error.
// Strict: bubble up. Lenient: record as a warning and return the base unchanged.
func absorbOverlayErr(base Result, opts Options, err error) (Result, error) {
	if opts.Strict {
		return Result{}, err
	}
	base.Warnings = append(base.Warnings, err)
	return base, nil
}

// mergeOverlay applies overlay's per-tool entries on top of base.
// Tools present only in overlay (no embedded counterpart) are accepted with a warning.
func mergeOverlay(base *Result, overlay Result) {
	for name, fields := range overlay.Specs {
		if _, present := base.Specs[name]; !present {
			base.Warnings = append(base.Warnings,
				fmt.Errorf("compactyaml: overlay tool %q has no embedded counterpart (possible typo)", name))
		}
		base.Specs[name] = fields
		delete(base.MaxBytes, name)
	}
	maps.Copy(base.MaxBytes, overlay.MaxBytes)
	base.Warnings = append(base.Warnings, overlay.Warnings...)
}
