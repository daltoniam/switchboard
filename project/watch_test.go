package project

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWatch_StopsOnCancel(t *testing.T) {
	store := NewStore(t.TempDir())
	require.NoError(t, store.Load())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- store.Watch(ctx) }()
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		require.Error(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("watch did not stop")
	}
}
