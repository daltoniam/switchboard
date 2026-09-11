package wasm

import (
	"context"
	"errors"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/pluginoauth"
)

var _ mcp.OAuthIntegration = (*Module)(nil)

func (m *Module) SetConfigService(cfg mcp.ConfigService) {
	m.callMu.Lock()
	defer m.callMu.Unlock()
	m.cfgMgr = cfg
}

func (m *Module) StartOAuth(ctx context.Context, creds mcp.Credentials, redirectURI, browser string) (string, error) {
	name := m.Name()
	m.callMu.Lock()
	defer m.callMu.Unlock()
	if m.closed.Load() {
		return "", ErrModuleClosed
	}
	if m.cfgMgr == nil {
		return "", pluginoauth.ErrPersistence
	}
	if m.oauth == nil {
		m.oauth = pluginoauth.New(name, m.cfgMgr)
	}
	return m.oauth.Start(ctx, creds, redirectURI, browser)
}

func (m *Module) CompleteOAuth(ctx context.Context, code, state, browser, issuer string) error {
	m.callMu.Lock()
	defer m.callMu.Unlock()
	if m.closed.Load() {
		return ErrModuleClosed
	}
	if m.oauth == nil {
		return pluginoauth.ErrState
	}
	err := m.oauth.Callback(ctx, code, state, browser, issuer)
	if err == nil {
		m.oauthErr = nil
	}
	return err
}

func (m *Module) prepareOAuth(ctx context.Context) error {
	if m.oauthErr != nil {
		return m.oauthErr
	}
	if m.oauth == nil || !m.oauth.Active() {
		return nil
	}
	creds, err := m.oauth.Credentials(ctx)
	if err != nil {
		return err
	}
	if err := m.configureGuest(ctx, creds); err != nil {
		return errors.New("oauth: guest configuration failed")
	}
	return nil
}

func (m *Module) EditCredentials(ctx context.Context, updates mcp.Credentials, enabled bool) error {
	name := m.Name()
	m.callMu.Lock()
	defer m.callMu.Unlock()
	if m.closed.Load() {
		return ErrModuleClosed
	}
	if m.oauth == nil {
		m.oauth = pluginoauth.New(name, m.cfgMgr)
	}
	if err := m.oauth.EditCredentials(ctx, updates, enabled); err != nil {
		return err
	}
	ic, ok := m.cfgMgr.GetIntegration(name)
	if !ok {
		return pluginoauth.ErrPersistence
	}
	if pluginoauth.Configured(ic.Credentials) {
		m.oauthErr = m.oauth.Load(ic.Credentials)
		return nil
	}
	m.oauth, m.oauthErr = nil, nil
	return m.configureGuest(ctx, ic.Credentials)
}
