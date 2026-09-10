package forgejo

import mcp "github.com/daltoniam/switchboard"

var (
	_ mcp.Integration                        = (*forgejo)(nil)
	_ mcp.PlainTextCredentials               = (*forgejo)(nil)
	_ mcp.PlaceholderHints                   = (*forgejo)(nil)
	_ mcp.PerToolMaxResponseBytesIntegration = (*forgejo)(nil)
)

func (f *forgejo) PlainTextKeys() []string { return []string{"base_url"} }

func (f *forgejo) Placeholders() map[string]string {
	return map[string]string{
		"base_url": "Forgejo instance URL including any deployment subpath",
		"token":    "Personal access token with the required repository, user, and organization permissions",
	}
}
