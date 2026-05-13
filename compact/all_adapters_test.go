package compact_test

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/daltoniam/switchboard/compact"
)

// TestAllAdapterYAMLsLoadStrict is the CI gate that catches malformed
// compact.yaml across every adapter. Runtime mode is lenient — bad entries
// are skipped with a warning, server stays up. This test inverts that:
// strict mode treats any malformed entry as a hard failure, so a typo in
// a spec string or a non-positive max_bytes value is caught before merge.
//
// The Never Block hybrid: lenient at runtime, strict in test.
func TestAllAdapterYAMLsLoadStrict(t *testing.T) {
	matches, err := filepath.Glob("../integrations/*/compact.yaml")
	if err != nil {
		t.Fatalf("glob failed: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("no compact.yaml files found under ../integrations/*/ (this test must run from the compact package directory)")
	}
	sort.Strings(matches)
	for _, path := range matches {
		t.Run(filepath.Base(filepath.Dir(path)), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			res, err := compact.Load(data, compact.Options{Strict: true})
			if err != nil {
				t.Fatalf("strict load failed: %v", err)
			}
			if len(res.Specs) == 0 {
				t.Fatalf("%s loaded but produced 0 specs", path)
			}
		})
	}
}
