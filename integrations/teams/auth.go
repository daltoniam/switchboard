package teams

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
)

// teamsLogin starts (or returns the in-progress) device-code OAuth flow.
func teamsLogin(ctx context.Context, t *teamsIntegration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	tenantHint := r.Str("tenant")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}

	// If a flow is already in progress and not yet done, return the same prompt
	// so the human can keep using the existing user_code.
	activeFlow.mu.Lock()
	existing := activeFlow.flow
	activeFlow.mu.Unlock()
	if existing != nil {
		existing.mu.Lock()
		stillRunning := !existing.done && time.Since(existing.startedAt) < time.Duration(existing.deviceResp.ExpiresIn)*time.Second
		var dcr *deviceCodeResponse
		if stillRunning {
			dcr = existing.deviceResp
		}
		existing.mu.Unlock()
		if dcr != nil {
			return loginResponse(dcr, "resumed"), nil
		}
	}

	dcr, err := t.startOAuth(ctx, tenantHint)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return loginResponse(dcr, "started"), nil
}

func loginResponse(dcr *deviceCodeResponse, status string) *mcp.ToolResult {
	payload := map[string]any{
		"status":           status,
		"user_code":        dcr.UserCode,
		"verification_uri": dcr.VerificationURI,
		"expires_in":       dcr.ExpiresIn,
		"interval_seconds": dcr.Interval,
		"message":          dcr.Message,
		"next_step":        "Visit verification_uri in a browser, enter user_code, then call teams_login_poll to detect completion.",
	}
	data, _ := json.Marshal(payload)
	return &mcp.ToolResult{Data: string(data)}
}

// teamsLoginPoll reports the state of the active device-code flow.
func teamsLoginPoll(_ context.Context, _ *teamsIntegration, _ map[string]any) (*mcp.ToolResult, error) {
	res := pollOAuth()
	return mcp.JSONResult(res)
}

// teamsTokenStatus reports per-tenant token health.
func teamsTokenStatus(_ context.Context, t *teamsIntegration, _ map[string]any) (*mcp.ToolResult, error) {
	type entry struct {
		TenantID    string  `json:"tenant_id"`
		TenantName  string  `json:"tenant_name,omitempty"`
		UserUPN     string  `json:"user_upn,omitempty"`
		UserDisplay string  `json:"user_display,omitempty"`
		Status      string  `json:"status"`
		HasRefresh  bool    `json:"has_refresh"`
		AgeMinutes  float64 `json:"age_minutes"`
		ExpiresIn   string  `json:"expires_in,omitempty"`
		Source      string  `json:"source,omitempty"`
		IsDefault   bool    `json:"is_default"`
	}
	defaultID := t.store.defaultID()
	var entries []entry
	for _, tn := range t.store.all() {
		age := 0.0
		if !tn.UpdatedAt.IsZero() {
			age = math.Round(time.Since(tn.UpdatedAt).Minutes()*10) / 10
		}
		status := "healthy"
		expiresIn := ""
		if !tn.ExpiresAt.IsZero() {
			remaining := time.Until(tn.ExpiresAt)
			if remaining <= 0 {
				status = "expired"
				expiresIn = "0s"
			} else {
				if remaining < 5*time.Minute {
					status = "expiring_soon"
				}
				expiresIn = remaining.Round(time.Second).String()
			}
		}
		if tn.AccessToken == "" {
			status = "missing"
		}
		entries = append(entries, entry{
			TenantID:    tn.TenantID,
			TenantName:  tn.TenantName,
			UserUPN:     tn.UserUPN,
			UserDisplay: tn.UserDisplay,
			Status:      status,
			HasRefresh:  tn.RefreshToken != "",
			AgeMinutes:  age,
			ExpiresIn:   expiresIn,
			Source:      tn.Source,
			IsDefault:   tn.TenantID == defaultID,
		})
	}
	return mcp.JSONResult(map[string]any{
		"tenant_count":      len(entries),
		"default_tenant_id": defaultID,
		"tenants":           entries,
		"auto_refresh": map[string]any{
			"enabled":          true,
			"interval_minutes": 15,
			"skew_seconds":     int(refreshSkew.Seconds()),
		},
	})
}

// teamsRefreshTokens forces a refresh for the (default or specified) tenant.
func teamsRefreshTokens(ctx context.Context, t *teamsIntegration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	tid := r.Str("tenant_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	var tn *tenant
	if tid != "" {
		tn = t.store.get(tid)
		if tn == nil {
			return mcp.ErrResult(fmt.Errorf("unknown tenant: %s", tid))
		}
	} else {
		tn = t.activeTenant()
		if tn == nil {
			return mcp.ErrResult(fmt.Errorf("no tenant configured — run teams_login first"))
		}
	}
	if tn.RefreshToken == "" {
		return &mcp.ToolResult{
			Data:    fmt.Sprintf("tenant %s has no refresh_token; re-run teams_login to acquire one", tn.TenantID),
			IsError: true,
		}, nil
	}
	if err := t.refreshTenant(ctx, tn); err != nil {
		return mcp.ErrResult(err)
	}
	refreshed := t.store.get(tn.TenantID)
	return mcp.JSONResult(map[string]any{
		"status":     "refreshed",
		"tenant_id":  refreshed.TenantID,
		"expires_at": refreshed.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

// teamsListTenants returns all configured tenants for the user.
func teamsListTenants(_ context.Context, t *teamsIntegration, _ map[string]any) (*mcp.ToolResult, error) {
	type info struct {
		TenantID    string `json:"tenant_id"`
		TenantName  string `json:"tenant_name,omitempty"`
		UserUPN     string `json:"user_upn,omitempty"`
		UserDisplay string `json:"user_display,omitempty"`
		IsDefault   bool   `json:"is_default"`
	}
	defaultID := t.store.defaultID()
	all := t.store.all()
	out := make([]info, 0, len(all))
	for _, tn := range all {
		out = append(out, info{
			TenantID:    tn.TenantID,
			TenantName:  tn.TenantName,
			UserUPN:     tn.UserUPN,
			UserDisplay: tn.UserDisplay,
			IsDefault:   tn.TenantID == defaultID,
		})
	}
	return mcp.JSONResult(map[string]any{
		"count":             len(out),
		"default_tenant_id": defaultID,
		"tenants":           out,
	})
}

// teamsRemoveTenant forgets cached tokens for a tenant.
func teamsRemoveTenant(_ context.Context, t *teamsIntegration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	tid := r.Str("tenant_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if tid == "" {
		return mcp.ErrResult(fmt.Errorf("tenant_id is required"))
	}
	if t.store.get(tid) == nil {
		return mcp.ErrResult(fmt.Errorf("unknown tenant: %s", tid))
	}
	t.store.remove(tid)
	_ = t.store.saveToFile()
	return mcp.JSONResult(map[string]any{
		"status":            "removed",
		"tenant_id":         tid,
		"default_tenant_id": t.store.defaultID(),
	})
}

// teamsSetDefault pins the default tenant for tools called without tenant_id.
func teamsSetDefault(_ context.Context, t *teamsIntegration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	tid := r.Str("tenant_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if tid == "" {
		return mcp.ErrResult(fmt.Errorf("tenant_id is required"))
	}
	if t.store.get(tid) == nil {
		return mcp.ErrResult(fmt.Errorf("unknown tenant: %s", tid))
	}
	t.store.setDefault(tid)
	_ = t.store.saveToFile()
	return mcp.JSONResult(map[string]any{
		"status":            "ok",
		"default_tenant_id": tid,
	})
}

// teamsGetMe returns the /me Graph profile for the chosen tenant.
func teamsGetMe(ctx context.Context, t *teamsIntegration, args map[string]any) (*mcp.ToolResult, error) {
	tn, err := t.tenantFromArgs(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	data, err := t.graphGet(ctx, tn.TenantID, "/me")
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(data)
}

// formatContentType is shared by chat/channel senders.
func formatContentType(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" || s == "html" {
		return "html"
	}
	return "text"
}
