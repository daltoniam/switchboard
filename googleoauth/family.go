package googleoauth

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// tokenInfoURL is Google's tokeninfo endpoint. It reports whether an access
// token is currently valid and which scopes it carries. It is a var so tests
// can point it at an httptest server.
var tokenInfoURL = "https://www.googleapis.com/oauth2/v3/tokeninfo"

// TokenValid reports whether the given Google access token is currently valid
// (not expired or revoked). It does not check any particular API — a token can
// be valid yet still fail an API call because that API is not enabled in the
// Cloud project. Callers use this to distinguish a genuinely bad token from a
// disabled/unpermitted API when a health check fails.
func TokenValid(ctx context.Context, accessToken string) bool {
	if accessToken == "" {
		return false
	}
	reqURL := tokenInfoURL + "?access_token=" + url.QueryEscape(accessToken)
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	// tokeninfo returns 200 for a live token and 400 for an
	// invalid/expired/revoked one.
	return resp.StatusCode == http.StatusOK
}

// errNoServices is returned when a unified flow is started with no selected
// services (or only unknown names).
var errNoServices = errors.New("googleoauth: no valid Google Workspace services selected")

// Service describes one Google Workspace integration that can be authorized
// through the unified "Google Workspace" setup flow. The Name matches the
// integration's registry / config key.
type Service struct {
	// Name is the integration registry key (e.g. "gmail", "gcal").
	Name string
	// DisplayName is the human-facing label shown in the setup UI.
	DisplayName string
	// Scopes are the OAuth scopes this service requires. They are unioned
	// with the scopes of the other selected services into a single consent
	// request.
	Scopes []string
}

// gmailScope grants read/write/send access to the user's mail.
const gmailScope = "https://mail.google.com/"

// GroupName is the flow key used for the unified "Google Workspace" setup
// flow. It is not a real integration — no adapter registers it — so it can
// never collide with a service's own single-integration flow.
const GroupName = "google-workspace"

// StartGroup begins a unified OAuth + PKCE flow requesting the union of the
// selected services' scopes in a single consent screen. The resulting token
// covers every granted service and is fanned out to each service's config by
// the caller.
func StartGroup(clientID, clientSecret, redirectURI string, serviceNames []string) (*StartResult, error) {
	scopes := UnionScopes(serviceNames)
	if len(scopes) == 0 {
		return nil, errNoServices
	}
	return Start(Config{
		IntegrationName: GroupName,
		ClientID:        clientID,
		ClientSecret:    clientSecret,
		RedirectURI:     redirectURI,
		Scopes:          scopes,
	})
}

// HandleGroupCallback completes the token exchange for the in-progress
// unified flow.
func HandleGroupCallback(code, state string) error {
	return HandleCallback(GroupName, code, state)
}

// PollGroup reports the status of the in-progress unified flow.
func PollGroup() PollResult {
	return Poll(GroupName)
}

// services is the single source of truth for the Google Workspace suite: the
// set of integrations, their display names, and the OAuth scopes each one
// needs. Per-integration oauth.go wrappers reference these scopes so there is
// exactly one place to update when a service's scope set changes.
//
// Order is intentional — it controls the order services appear in the unified
// setup UI (mail/calendar/drive first, productivity apps next, comms last).
var services = []Service{
	{Name: "gmail", DisplayName: "Gmail", Scopes: []string{gmailScope}},
	{Name: "gcal", DisplayName: "Google Calendar", Scopes: []string{
		"https://www.googleapis.com/auth/calendar",
	}},
	{Name: "gdrive", DisplayName: "Google Drive", Scopes: []string{
		"https://www.googleapis.com/auth/drive",
	}},
	{Name: "gdocs", DisplayName: "Google Docs", Scopes: []string{
		"https://www.googleapis.com/auth/documents",
	}},
	{Name: "gsheets", DisplayName: "Google Sheets", Scopes: []string{
		"https://www.googleapis.com/auth/spreadsheets",
	}},
	{Name: "gslides", DisplayName: "Google Slides", Scopes: []string{
		"https://www.googleapis.com/auth/presentations",
	}},
	{Name: "gforms", DisplayName: "Google Forms", Scopes: []string{
		"https://www.googleapis.com/auth/forms.body",
		"https://www.googleapis.com/auth/forms.responses.readonly",
	}},
	{Name: "gtasks", DisplayName: "Google Tasks", Scopes: []string{
		"https://www.googleapis.com/auth/tasks",
	}},
	{Name: "gchat", DisplayName: "Google Chat", Scopes: []string{
		"https://www.googleapis.com/auth/chat.spaces.readonly",
		"https://www.googleapis.com/auth/chat.messages",
	}},
	{Name: "gpeople", DisplayName: "Google Contacts", Scopes: []string{
		"https://www.googleapis.com/auth/contacts",
		"https://www.googleapis.com/auth/contacts.other.readonly",
		"https://www.googleapis.com/auth/directory.readonly",
	}},
	{Name: "gmeet", DisplayName: "Google Meet", Scopes: []string{
		"https://www.googleapis.com/auth/meetings.space.created",
		"https://www.googleapis.com/auth/meetings.space.readonly",
		"https://www.googleapis.com/auth/meetings.space.settings",
	}},
}

var serviceByName = func() map[string]Service {
	m := make(map[string]Service, len(services))
	for _, s := range services {
		m[s.Name] = s
	}
	return m
}()

// Services returns the ordered list of Google Workspace services available in
// the unified setup flow. The returned slice is a copy; callers may not mutate
// the shared definitions.
func Services() []Service {
	out := make([]Service, len(services))
	copy(out, services)
	return out
}

// ScopesFor returns the OAuth scopes for a single integration name, or nil if
// the name is not a known Google Workspace service.
func ScopesFor(name string) []string {
	s, ok := serviceByName[name]
	if !ok {
		return nil
	}
	out := make([]string, len(s.Scopes))
	copy(out, s.Scopes)
	return out
}

// UnionScopes returns the deduplicated, sorted union of scopes across the
// named services. Unknown names are ignored so a stale UI selection can't
// break the flow. The result is deterministic (sorted) so authorization URLs
// are stable and testable.
func UnionScopes(names []string) []string {
	seen := make(map[string]struct{})
	for _, name := range names {
		for _, scope := range ScopesFor(name) {
			seen[scope] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for scope := range seen {
		out = append(out, scope)
	}
	sort.Strings(out)
	return out
}

// GrantedServices returns the subset of requested service names whose scopes
// are all present in the granted scope string returned by Google. Google lets
// a user deny individual scopes on the consent screen, so a service is only
// considered connected when every scope it needs was actually granted.
func GrantedServices(requested []string, grantedScope string) []string {
	granted := make(map[string]struct{})
	for _, s := range strings.Fields(grantedScope) {
		granted[s] = struct{}{}
	}
	var out []string
	for _, name := range requested {
		scopes := ScopesFor(name)
		if len(scopes) == 0 {
			continue
		}
		all := true
		for _, scope := range scopes {
			if _, ok := granted[scope]; !ok {
				all = false
				break
			}
		}
		if all {
			out = append(out, name)
		}
	}
	return out
}
