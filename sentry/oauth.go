package sentry

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	deviceCodeEndpoint = "https://sentry.io/oauth/device/code/"
	tokenEndpoint      = "https://sentry.io/oauth/token/"
	oauthScope         = "org:read project:read event:read event:write member:read team:read"
)

type DeviceCodeResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

type AccessTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

type OAuthFlow struct {
	mu         sync.Mutex
	clientID   string
	deviceResp *DeviceCodeResponse
	startedAt  time.Time
	token      string
	err        string
	done       bool
}

var activeFlow struct {
	mu   sync.Mutex
	flow *OAuthFlow
}

func StartOAuthFlow(clientID string) (*DeviceCodeResponse, error) {
	if clientID == "" {
		return nil, fmt.Errorf("sentry OAuth client_id is not configured")
	}

	data := url.Values{
		"client_id": {clientID},
		"scope":     {oauthScope},
	}

	req, err := http.NewRequest("POST", deviceCodeEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request device code: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("sentry returned %d: %s", resp.StatusCode, string(body))
	}

	var dcr DeviceCodeResponse
	if err := json.Unmarshal(body, &dcr); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	flow := &OAuthFlow{
		clientID:   clientID,
		deviceResp: &dcr,
		startedAt:  time.Now(),
	}

	activeFlow.mu.Lock()
	activeFlow.flow = flow
	activeFlow.mu.Unlock()

	go flow.poll()

	return &dcr, nil
}

func (f *OAuthFlow) poll() {
	interval := time.Duration(f.deviceResp.Interval) * time.Second
	if interval < 5*time.Second {
		interval = 5 * time.Second
	}
	deadline := f.startedAt.Add(time.Duration(f.deviceResp.ExpiresIn) * time.Second)

	for {
		time.Sleep(interval)

		if time.Now().After(deadline) {
			f.mu.Lock()
			f.err = "Authorization timed out. Please try again."
			f.done = true
			f.mu.Unlock()
			return
		}

		data := url.Values{
			"client_id":   {f.clientID},
			"device_code": {f.deviceResp.DeviceCode},
			"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
		}

		req, err := http.NewRequest("POST", tokenEndpoint, strings.NewReader(data.Encode()))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		var atr AccessTokenResponse
		if err := json.Unmarshal(body, &atr); err != nil {
			continue
		}

		switch atr.Error {
		case "":
			if atr.AccessToken != "" {
				f.mu.Lock()
				f.token = atr.AccessToken
				f.done = true
				f.mu.Unlock()
				return
			}
		case "authorization_pending":
			continue
		case "slow_down":
			interval += 5 * time.Second
			continue
		case "expired_token":
			f.mu.Lock()
			f.err = "Device code expired. Please try again."
			f.done = true
			f.mu.Unlock()
			return
		case "access_denied":
			f.mu.Lock()
			f.err = "Authorization was denied by the user."
			f.done = true
			f.mu.Unlock()
			return
		default:
			f.mu.Lock()
			f.err = fmt.Sprintf("OAuth error: %s — %s", atr.Error, atr.ErrorDesc)
			f.done = true
			f.mu.Unlock()
			return
		}
	}
}

type PollResult struct {
	Status string `json:"status"`
	Token  string `json:"token,omitempty"`
	Error  string `json:"error,omitempty"`
}

func PollOAuthFlow() PollResult {
	activeFlow.mu.Lock()
	flow := activeFlow.flow
	activeFlow.mu.Unlock()

	if flow == nil {
		return PollResult{Status: "no_flow", Error: "No OAuth flow in progress"}
	}

	flow.mu.Lock()
	defer flow.mu.Unlock()

	if !flow.done {
		return PollResult{Status: "pending"}
	}

	if flow.err != "" {
		return PollResult{Status: "error", Error: flow.err}
	}

	return PollResult{Status: "complete", Token: flow.token}
}
