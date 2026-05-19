package main

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLazyBrowserService_DoesNotStartUntilFirstSession(t *testing.T) {
	starts := 0
	svc := newLazyBrowserService(func(context.Context) (mcp.BrowserService, error) {
		starts++
		return &fakeBrowserService{}, nil
	})

	require.NoError(t, svc.Close())
	assert.Equal(t, 0, starts)
}

func TestLazyBrowserService_ReusesStartedService(t *testing.T) {
	starts := 0
	browser := &fakeBrowserService{}
	svc := newLazyBrowserService(func(context.Context) (mcp.BrowserService, error) {
		starts++
		return browser, nil
	})

	_, err := svc.NewSession(context.Background())
	require.NoError(t, err)
	_, err = svc.NewSession(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 1, starts)
	assert.Equal(t, 2, browser.sessions)
}

func TestLazyBrowserService_NewSessionAfterClose(t *testing.T) {
	svc := newLazyBrowserService(func(context.Context) (mcp.BrowserService, error) {
		return &fakeBrowserService{}, nil
	})

	require.NoError(t, svc.Close())
	_, err := svc.NewSession(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "browser service closed")
}

func TestLazyBrowserService_CloseForwardsToInner(t *testing.T) {
	inner := &fakeBrowserService{}
	svc := newLazyBrowserService(func(context.Context) (mcp.BrowserService, error) {
		return inner, nil
	})

	_, err := svc.NewSession(context.Background())
	require.NoError(t, err)

	require.NoError(t, svc.Close())
	assert.True(t, inner.closed)
}

func TestLazyBrowserService_DoubleCloseIsNoop(t *testing.T) {
	inner := &fakeBrowserService{}
	svc := newLazyBrowserService(func(context.Context) (mcp.BrowserService, error) {
		return inner, nil
	})

	_, err := svc.NewSession(context.Background())
	require.NoError(t, err)

	require.NoError(t, svc.Close())
	require.NoError(t, svc.Close())
	assert.Equal(t, 1, inner.closeCount)
}

func TestLazyBrowserService_FactoryRetryAfterTransientError(t *testing.T) {
	attempts := 0
	inner := &fakeBrowserService{}
	svc := newLazyBrowserService(func(context.Context) (mcp.BrowserService, error) {
		attempts++
		if attempts == 1 {
			return nil, fmt.Errorf("transient error")
		}
		return inner, nil
	})

	_, err := svc.NewSession(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "transient error")

	_, err = svc.NewSession(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, attempts)
}

func TestLazyBrowserService_ConcurrentInitOnce(t *testing.T) {
	var mu sync.Mutex
	starts := 0
	svc := newLazyBrowserService(func(context.Context) (mcp.BrowserService, error) {
		mu.Lock()
		starts++
		mu.Unlock()
		time.Sleep(10 * time.Millisecond)
		return &fakeBrowserService{}, nil
	})

	var wg sync.WaitGroup
	errCh := make(chan error, 10)
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.NewSession(context.Background())
			errCh <- err
		}()
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		require.NoError(t, err)
	}
	assert.Equal(t, 1, starts)
}

type fakeBrowserService struct {
	mu         sync.Mutex
	sessions   int
	closed     bool
	closeCount int
}

func (s *fakeBrowserService) NewSession(context.Context) (mcp.BrowserSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions++
	return &fakeBrowserSession{}, nil
}

func (s *fakeBrowserService) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	s.closeCount++
	return nil
}

type fakeBrowserSession struct{}

func (s *fakeBrowserSession) AddCookies(context.Context, []mcp.BrowserCookie) error { return nil }
func (s *fakeBrowserSession) NewPage(context.Context) (mcp.BrowserPage, error)      { return nil, nil }
func (s *fakeBrowserSession) Close() error                                          { return nil }
