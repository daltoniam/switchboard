package server

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/daltoniam/switchboard/project"
)

const (
	catalogResourceURI = "project://registry/catalog"
	registryPrefix     = "project://registry/projects/"
)

func definitionResourceURI(id project.ProjectID) string {
	return fmt.Sprintf("project://registry/projects/%s/definition", url.PathEscape(string(id)))
}

func diagnosticsResourceURI(id project.ProjectID) string {
	return fmt.Sprintf("project://registry/projects/%s/diagnostics", url.PathEscape(string(id)))
}

func revisionResourceURI(id project.ProjectID, rev project.Revision) string {
	return fmt.Sprintf("project://registry/projects/%s/revisions/%s", url.PathEscape(string(id)), url.PathEscape(string(rev)))
}

func contextManifestURI(id project.ProjectID, root string) string {
	base := fmt.Sprintf("project://registry/projects/%s/context", url.PathEscape(string(id)))
	if root == "" {
		return base
	}
	return base + "?rootUri=" + url.QueryEscape(root)
}

func contextFileURI(id project.ProjectID, path, root string) string {
	base := fmt.Sprintf("project://registry/projects/%s/context/%s", url.PathEscape(string(id)), escapeContextPath(path))
	if root == "" {
		return base
	}
	return base + "?rootUri=" + url.QueryEscape(root)
}

func escapeContextPath(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

type parsedCatalogURI struct {
	kind      string
	projectID project.ProjectID
	revision  project.Revision
	path      string
	rootURI   string
}

func parseCatalogURI(raw string) (parsedCatalogURI, error) {
	if raw == catalogResourceURI {
		return parsedCatalogURI{kind: "catalog"}, nil
	}
	if !strings.HasPrefix(raw, registryPrefix) {
		return parsedCatalogURI{}, fmt.Errorf("unknown catalog uri")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return parsedCatalogURI{}, err
	}
	rest := strings.TrimPrefix(u.Path, "/projects/")
	parts := strings.Split(rest, "/")
	if len(parts) < 2 {
		return parsedCatalogURI{}, fmt.Errorf("unknown catalog uri")
	}
	id, err := url.PathUnescape(parts[0])
	if err != nil || id == "" {
		return parsedCatalogURI{}, fmt.Errorf("invalid project id")
	}
	out := parsedCatalogURI{projectID: project.ProjectID(id), rootURI: u.Query().Get("rootUri")}
	switch parts[1] {
	case "definition":
		if len(parts) != 2 {
			return parsedCatalogURI{}, fmt.Errorf("unknown catalog uri")
		}
		out.kind = "definition"
	case "diagnostics":
		if len(parts) != 2 {
			return parsedCatalogURI{}, fmt.Errorf("unknown catalog uri")
		}
		out.kind = "diagnostics"
	case "revisions":
		if len(parts) != 3 {
			return parsedCatalogURI{}, fmt.Errorf("unknown catalog uri")
		}
		rev, err := url.PathUnescape(parts[2])
		if err != nil {
			return parsedCatalogURI{}, fmt.Errorf("invalid revision")
		}
		out.kind = "revision"
		out.revision = project.Revision(rev)
	case "context":
		out.kind = "context"
		if len(parts) > 2 {
			var segs []string
			for _, p := range parts[2:] {
				seg, err := url.PathUnescape(p)
				if err != nil {
					return parsedCatalogURI{}, fmt.Errorf("invalid context path")
				}
				segs = append(segs, seg)
			}
			out.path = strings.Join(segs, "/")
		}
	default:
		return parsedCatalogURI{}, fmt.Errorf("unknown catalog uri")
	}
	return out, nil
}
