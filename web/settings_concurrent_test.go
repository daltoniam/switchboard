package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type settingsBarrierConfig struct {
	mcp.ConfigService
	once    sync.Once
	entered chan struct{}
	release chan struct{}
}

func (s *settingsBarrierConfig) wait() {
	s.once.Do(func() { close(s.entered); <-s.release })
}

func (s *settingsBarrierConfig) Get() *mcp.Config {
	cfg := s.ConfigService.Get()
	s.wait()
	return cfg
}

func (s *settingsBarrierConfig) UpdateConfig(update func(*mcp.Config) error) error {
	s.wait()
	return s.ConfigService.(interface {
		UpdateConfig(func(*mcp.Config) error) error
	}).UpdateConfig(update)
}

func TestSettingsSavePreservesConcurrentRotation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	store, err := config.NewManager()
	require.NoError(t, err)
	require.NoError(t, store.SetIntegration("primer", &mcp.IntegrationConfig{Credentials: mcp.Credentials{"oauth_refresh_token": "before"}}))
	barrier := &settingsBarrierConfig{ConfigService: store, entered: make(chan struct{}), release: make(chan struct{})}
	w, _, _ := setupTestWeb()
	w.services.Config = barrier
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		r := httptest.NewRequest(http.MethodPost, "/settings", strings.NewReader("session_store=file&show_dollar_estimate=true&dollars_per_mtok_input=2.5"))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()
		w.Handler().ServeHTTP(rr, r)
		done <- rr
	}()
	<-barrier.entered
	err = store.SetIntegration("primer", &mcp.IntegrationConfig{Enabled: true, Credentials: mcp.Credentials{"oauth_refresh_token": "after"}, ToolGlobs: []string{"primer_*"}})
	close(barrier.release)
	require.NoError(t, err)
	rr := <-done
	require.Equal(t, http.StatusSeeOther, rr.Code)
	require.Contains(t, rr.Header().Get("Location"), "success=")
	require.NoError(t, store.Load())
	cfg := store.Get()
	assert.Equal(t, "after", cfg.Integrations["primer"].Credentials["oauth_refresh_token"])
	assert.True(t, cfg.Integrations["primer"].Enabled)
	assert.Equal(t, []string{"primer_*"}, cfg.Integrations["primer"].ToolGlobs)
	assert.Equal(t, "file", cfg.SessionStore)
	assert.True(t, cfg.ShowDollarEstimate)
	assert.Equal(t, 2.5, cfg.DollarsPerMTokInput)
}
