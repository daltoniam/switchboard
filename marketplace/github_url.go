package marketplace

import (
	"net/url"
	"strings"
)

// normalizeGitHubURL rewrites GitHub HTML URLs (the kind a user copies from
// the browser) into the GitHub Contents API form so the file's raw bytes can
// be fetched with an Authorization header. This is the only way to retrieve
// files from a private repository over HTTPS without using `git` itself.
//
// Supported input forms:
//
//   - https://github.com/{owner}/{repo}/blob/{ref}/{path}   →  Contents API
//   - https://github.com/{owner}/{repo}/raw/{ref}/{path}    →  Contents API
//
// Any URL on raw.githubusercontent.com / api.github.com / non-GitHub hosts is
// passed through unchanged. Malformed URLs are also passed through so the
// caller can attempt the request and surface a normal HTTP error.
//
// useRawAccept is true when the rewritten URL is the Contents API form, in
// which case the request must carry `Accept: application/vnd.github.raw` to
// receive raw file bytes instead of the default JSON metadata wrapper.
func normalizeGitHubURL(rawURL string) (newURL string, useRawAccept bool) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return rawURL, false
	}
	if strings.ToLower(u.Host) != "github.com" {
		return rawURL, false
	}
	// /{owner}/{repo}/blob|raw/{ref}/{path...}
	parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
	if len(parts) < 5 || (parts[2] != "blob" && parts[2] != "raw") {
		return rawURL, false
	}
	owner, repo, ref := parts[0], parts[1], parts[3]
	path := strings.Join(parts[4:], "/")

	api := &url.URL{
		Scheme: "https",
		Host:   "api.github.com",
		Path:   "/repos/" + owner + "/" + repo + "/contents/" + path,
	}
	q := api.Query()
	q.Set("ref", ref)
	api.RawQuery = q.Encode()

	return api.String(), true
}
