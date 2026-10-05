package slack

import (
	"net/http"
	"strings"
)

// credKind says what a Slack token is. It is decided once, from the token,
// and every behavior that depends on it switches on the kind.
type credKind int

const (
	kindUnknown credKind = iota
	kindUserToken
	kindBrowserSession
	kindBot
)

func parseCredKind(token string) credKind {
	switch {
	case strings.HasPrefix(token, "xoxp-"):
		return kindUserToken
	case strings.HasPrefix(token, "xoxc-"):
		return kindBrowserSession
	case strings.HasPrefix(token, "xoxb-"):
		return kindBot
	}
	return kindUnknown
}

// String is the token_type reported by token status tools.
func (k credKind) String() string {
	switch k {
	case kindUserToken:
		return "oauth_user"
	case kindBrowserSession:
		return "browser_session"
	case kindBot:
		return "bot"
	}
	return "unknown"
}

// newCookieTransport builds the guarded transport for a workspace. Only a
// browser session sends the d cookie; user and bot tokens never do.
func newCookieTransport(ws *workspace, revoked *revokedCredentials) *cookieTransport {
	t := &cookieTransport{key: credentialKey(ws), inner: http.DefaultTransport, revoked: revoked}
	if parseCredKind(ws.Token) == kindBrowserSession {
		t.cookie = ws.Cookie
	}
	return t
}
