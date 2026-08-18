package project

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLock_Cancel(t *testing.T) {
	store := NewStore(t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := store.withLock(ctx, func() error { return nil })
	require.Error(t, err)
	assert.True(t, IsCode(err, CodeLockTimeout))
}

func TestCatalog_ConcurrentPatchOneWinner(t *testing.T) {
	store := NewStore(t.TempDir())
	created, err := store.Create(context.Background(), CreateRequest{Definition: Definition{Version: "1", Name: "acme", Description: "one"}})
	require.NoError(t, err)
	var okCount, conflictCount atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.Patch(context.Background(), PatchRequest{
				ProjectID:              "acme",
				ExpectedSourceRevision: created.SourceRevision,
				Patch:                  []byte(`{"description":"race"}`),
			})
			if err == nil {
				okCount.Add(1)
				return
			}
			if IsCode(err, CodeRevisionConflict) {
				conflictCount.Add(1)
			}
		}()
	}
	wg.Wait()
	assert.EqualValues(t, 1, okCount.Load())
	assert.EqualValues(t, 7, conflictCount.Load())
}

func TestCatalog_LockTimeout(t *testing.T) {
	dir := t.TempDir()
	holder := NewStore(dir)
	waiter := NewStore(dir)
	started := make(chan struct{})
	release := make(chan struct{})
	go func() {
		_ = holder.withLock(context.Background(), func() error {
			close(started)
			<-release
			return nil
		})
	}()
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	err := waiter.withLock(ctx, func() error { return nil })
	close(release)
	require.Error(t, err)
	assert.True(t, IsCode(err, CodeLockTimeout))
}
