package project

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCatalog_IndependentProcessCreate(t *testing.T) {
	if os.Getenv("SWITCHBOARD_CATALOG_WORKER") == "1" {
		runCatalogWorker()
		return
	}
	dir := t.TempDir()
	exe, err := os.Executable()
	require.NoError(t, err)

	start := func() *exec.Cmd {
		cmd := exec.Command(exe, "-test.run", "TestCatalog_IndependentProcessCreate", "-test.v=false")
		cmd.Env = append(os.Environ(),
			"SWITCHBOARD_CATALOG_WORKER=1",
			"SWITCHBOARD_CATALOG_ROOT="+dir,
			"SWITCHBOARD_CATALOG_OP=create",
		)
		cmd.Dir = dir
		return cmd
	}
	c1 := start()
	c2 := start()
	err1 := c1.Start()
	err2 := c2.Start()
	require.NoError(t, err1)
	require.NoError(t, err2)
	w1 := c1.Wait()
	w2 := c2.Wait()

	ok := 0
	if w1 == nil {
		ok++
	}
	if w2 == nil {
		ok++
	}
	require.Equal(t, 1, ok, "exactly one same-name create must succeed")
	entries, err := os.ReadDir(filepath.Join(dir, "projects"))
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

func runCatalogWorker() {
	root := os.Getenv("SWITCHBOARD_CATALOG_ROOT")
	store := NewStore(root)
	_, err := store.Create(context.Background(), CreateRequest{Definition: Definition{Version: "1", Name: "acme"}})
	if err != nil {
		os.Stderr.WriteString(err.Error())
		os.Exit(1)
	}
	_ = json.RawMessage(nil)
}
