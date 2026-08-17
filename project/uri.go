package project

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func fileURI(absPath string) string {
	cleaned := filepath.Clean(absPath)
	if !filepath.IsAbs(cleaned) {
		return ""
	}
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(cleaned)}).String()
}

func parseFileURI(raw string) (string, error) {
	if raw == "" {
		return "", &Error{Code: CodeInvalidRoot, Message: "rootUri is required"}
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "file" {
		return "", &Error{Code: CodeInvalidRoot, Message: "rootUri must be a file:// URI"}
	}
	path := u.Path
	if path == "" {
		path = u.Opaque
	}
	if path == "" {
		return "", &Error{Code: CodeInvalidRoot, Message: "rootUri path is empty"}
	}
	abs := filepath.Clean(filepath.FromSlash(path))
	if !filepath.IsAbs(abs) {
		return "", &Error{Code: CodeInvalidRoot, Message: "rootUri must be an absolute file:// directory"}
	}
	info, err := os.Lstat(abs)
	if err != nil {
		return "", &Error{Code: CodeInvalidRoot, Message: "rootUri does not exist"}
	}
	if info.Mode()&os.ModeSymlink != 0 {
		resolved, err := filepath.EvalSymlinks(abs)
		if err != nil {
			return "", &Error{Code: CodeInvalidRoot, Message: "rootUri symlink cannot be resolved"}
		}
		abs = resolved
		info, err = os.Stat(abs)
		if err != nil {
			return "", &Error{Code: CodeInvalidRoot, Message: "rootUri does not exist"}
		}
	}
	if !info.IsDir() {
		return "", &Error{Code: CodeInvalidRoot, Message: "rootUri must name an existing directory"}
	}
	return abs, nil
}

func definitionURI(id ProjectID) string {
	return fmt.Sprintf("project://registry/projects/%s/definition", url.PathEscape(string(id)))
}

func contextURI(id ProjectID) string {
	return fmt.Sprintf("project://registry/projects/%s/context", url.PathEscape(string(id)))
}

func diagnosticsURI(id ProjectID) string {
	return fmt.Sprintf("project://registry/projects/%s/diagnostics", url.PathEscape(string(id)))
}

func revisionURI(id ProjectID, rev Revision) string {
	return fmt.Sprintf("project://registry/projects/%s/revisions/%s", url.PathEscape(string(id)), url.PathEscape(string(rev)))
}

func projectFileName(id ProjectID) string {
	return string(id) + ".project.json"
}

func stemFromFileName(name string) (ProjectID, bool) {
	if !strings.HasSuffix(name, ".project.json") {
		return "", false
	}
	stem := strings.TrimSuffix(name, ".project.json")
	if stem == "" {
		return "", false
	}
	return ProjectID(stem), true
}
