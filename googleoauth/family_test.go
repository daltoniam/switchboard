package googleoauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestServices_CoverAllElevenIntegrations(t *testing.T) {
	want := []string{
		"gmail", "gcal", "gdrive", "gdocs", "gsheets", "gslides",
		"gforms", "gtasks", "gchat", "gpeople", "gmeet",
	}
	got := make([]string, 0, len(Services()))
	for _, s := range Services() {
		got = append(got, s.Name)
		if s.DisplayName == "" {
			t.Errorf("service %q has empty DisplayName", s.Name)
		}
		if len(s.Scopes) == 0 {
			t.Errorf("service %q has no scopes", s.Name)
		}
	}
	sortedWant := append([]string(nil), want...)
	sortedGot := append([]string(nil), got...)
	sort.Strings(sortedWant)
	sort.Strings(sortedGot)
	if !reflect.DeepEqual(sortedWant, sortedGot) {
		t.Errorf("service names = %v, want %v", sortedGot, sortedWant)
	}
}

func TestScopesFor(t *testing.T) {
	if got := ScopesFor("gmail"); len(got) != 1 || got[0] != gmailScope {
		t.Errorf("ScopesFor(gmail) = %v", got)
	}
	if got := ScopesFor("gpeople"); len(got) != 3 {
		t.Errorf("ScopesFor(gpeople) = %v, want 3 scopes", got)
	}
	if got := ScopesFor("unknown"); got != nil {
		t.Errorf("ScopesFor(unknown) = %v, want nil", got)
	}
}

func TestUnionScopes_DedupesAndSorts(t *testing.T) {
	got := UnionScopes([]string{"gmail", "gcal", "gmail", "unknown"})
	want := []string{
		"https://mail.google.com/",
		"https://www.googleapis.com/auth/calendar",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("UnionScopes = %v, want %v", got, want)
	}
	// Determinism: same input -> same output order.
	if !reflect.DeepEqual(UnionScopes([]string{"gcal", "gmail"}), got) {
		t.Error("UnionScopes not order-independent")
	}
}

func TestUnionScopes_Empty(t *testing.T) {
	if got := UnionScopes(nil); len(got) != 0 {
		t.Errorf("UnionScopes(nil) = %v, want empty", got)
	}
	if got := UnionScopes([]string{"nope"}); len(got) != 0 {
		t.Errorf("UnionScopes(unknown) = %v, want empty", got)
	}
}

func TestGrantedServices(t *testing.T) {
	requested := []string{"gmail", "gcal", "gpeople"}
	// User granted gmail + calendar scopes but only one of gpeople's three.
	granted := strings.Join([]string{
		"https://mail.google.com/",
		"https://www.googleapis.com/auth/calendar",
		"https://www.googleapis.com/auth/contacts",
	}, " ")
	got := GrantedServices(requested, granted)
	want := []string{"gmail", "gcal"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GrantedServices = %v, want %v (partial grant must drop gpeople)", got, want)
	}
}

func TestGrantedServices_AllGranted(t *testing.T) {
	requested := []string{"gmail", "gcal"}
	granted := strings.Join(UnionScopes(requested), " ")
	got := GrantedServices(requested, granted)
	if !reflect.DeepEqual(got, requested) {
		t.Errorf("GrantedServices = %v, want %v", got, requested)
	}
}

func TestStartGroup_UnionScopeAuthURL(t *testing.T) {
	Reset()
	res, err := StartGroup("cid", "secret", "http://localhost/cb", []string{"gmail", "gcal"})
	if err != nil {
		t.Fatalf("StartGroup: %v", err)
	}
	u, err := url.Parse(res.AuthorizeURL)
	if err != nil {
		t.Fatalf("parse authorize url: %v", err)
	}
	q := u.Query()
	if q.Get("include_granted_scopes") != "true" {
		t.Error("authorize url missing include_granted_scopes=true")
	}
	scope := q.Get("scope")
	if !strings.Contains(scope, "https://mail.google.com/") ||
		!strings.Contains(scope, "https://www.googleapis.com/auth/calendar") {
		t.Errorf("scope = %q, want gmail + calendar union", scope)
	}
	// Flow is keyed under the group name, not a real integration.
	if Poll(GroupName).Status == "no_flow" {
		t.Error("group flow not registered under GroupName")
	}
}

func TestStartGroup_NoServices(t *testing.T) {
	Reset()
	if _, err := StartGroup("cid", "secret", "http://localhost/cb", nil); err == nil {
		t.Error("StartGroup with no services should error")
	}
}

func TestTokenValid(t *testing.T) {
	tests := []struct {
		name       string
		token      string
		statusCode int
		want       bool
	}{
		{name: "empty token", token: "", statusCode: 200, want: false},
		{name: "valid token", token: "good", statusCode: 200, want: true},
		{name: "invalid token", token: "bad", statusCode: 400, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer srv.Close()
			orig := tokenInfoURL
			tokenInfoURL = srv.URL
			defer func() { tokenInfoURL = orig }()

			if got := TokenValid(context.Background(), tt.token); got != tt.want {
				t.Errorf("TokenValid() = %v, want %v", got, tt.want)
			}
		})
	}
}
