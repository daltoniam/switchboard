package project

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWatch_ExternalReplace(t *testing.T) {
	store := newTestCatalog(t)
	bus := NewEventBus()
	store.SetEventBus(bus)
	ch := bus.Subscribe(8)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = store.Watch(ctx) }()

	_, err := store.Create(context.Background(), CreateRequest{Definition: testDefRepo("acme", "/tmp/acme", "one")})
	require.NoError(t, err)
	drainEvents(ch)

	tmp := filepath.Join(store.ConfigDir(), "projects", "acme.project.json.tmp")
	dst := filepath.Join(store.ConfigDir(), "projects", "acme.project.json")
	require.NoError(t, os.WriteFile(tmp, []byte(`{"version":"1","name":"acme","resources":{"main":{"type":"repo","path":"/tmp/acme","branch":"external"}}}`), 0600))
	require.NoError(t, os.Rename(tmp, dst))

	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("expected filesystem change event")
	}
}

func drainEvents(ch <-chan Event) {
	timeout := time.After(200 * time.Millisecond)
	for {
		select {
		case <-ch:
		case <-timeout:
			return
		}
	}
}
