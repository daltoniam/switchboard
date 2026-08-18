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

// projectResourceURI is the project envelope (definition + summary).
func projectResourceURI(id project.ProjectID) string {
	return fmt.Sprintf("project://registry/projects/%s", url.PathEscape(string(id)))
}

// definitionResourceURI is a compatibility alias of the project envelope URI.
func definitionResourceURI(id project.ProjectID) string {
	return projectResourceURI(id) + "/definition"
}

func diagnosticsResourceURI(id project.ProjectID) string {
	return fmt.Sprintf("project://registry/projects/%s/diagnostics", url.PathEscape(string(id)))
}

func revisionResourceURI(id project.ProjectID, rev project.Revision) string {
	return fmt.Sprintf("project://registry/projects/%s/revisions/%s", url.PathEscape(string(id)), url.PathEscape(string(rev)))
}

func projectResourcesURI(id project.ProjectID) string {
	return fmt.Sprintf("project://registry/projects/%s/resources", url.PathEscape(string(id)))
}

func projectResourceMetaURI(id project.ProjectID, resourceID string) string {
	return fmt.Sprintf("project://registry/projects/%s/resources/%s",
		url.PathEscape(string(id)), url.PathEscape(resourceID))
}

func projectResourceContentURI(id project.ProjectID, resourceID string) string {
	return projectResourceMetaURI(id, resourceID) + "/content"
}

// projectResourceFileURI builds a progressive file URI under a files resource.
// The path segment is a single url-encoded path (slashes become %2F).
func projectResourceFileURI(id project.ProjectID, resourceID, filePath string) string {
	return fmt.Sprintf("project://registry/projects/%s/resources/%s/files/%s",
		url.PathEscape(string(id)),
		url.PathEscape(resourceID),
		url.PathEscape(filepathToSlash(filePath)),
	)
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
	parts := strings.Split(filepathToSlash(path), "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func filepathToSlash(path string) string {
	return strings.ReplaceAll(path, "\\", "/")
}

type parsedCatalogURI struct {
	kind       string // catalog|project|definition|diagnostics|revision|context|resources|resource|resource-content|resource-file
	projectID  project.ProjectID
	revision   project.Revision
	resourceID string
	path       string
	rootURI    string
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
	if rest == "" || rest == u.Path {
		// host-style: project://registry/projects/... uses Path "/projects/..."
		// Opaque is empty for standard URLs.
		rest = strings.TrimPrefix(strings.TrimPrefix(u.Path, "/"), "projects/")
	}
	if rest == "" {
		return parsedCatalogURI{}, fmt.Errorf("unknown catalog uri")
	}
	parts := strings.Split(rest, "/")
	if len(parts) < 1 || parts[0] == "" {
		return parsedCatalogURI{}, fmt.Errorf("unknown catalog uri")
	}
	id, err := url.PathUnescape(parts[0])
	if err != nil || id == "" {
		return parsedCatalogURI{}, fmt.Errorf("invalid project id")
	}
	out := parsedCatalogURI{projectID: project.ProjectID(id), rootURI: u.Query().Get("rootUri")}

	if len(parts) == 1 {
		out.kind = "project"
		return out, nil
	}

	switch parts[1] {
	case "definition":
		if len(parts) != 2 {
			return parsedCatalogURI{}, fmt.Errorf("unknown catalog uri")
		}
		// Compatibility alias of project envelope.
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
	case "resources":
		if len(parts) == 2 {
			out.kind = "resources"
			return out, nil
		}
		resID, err := url.PathUnescape(parts[2])
		if err != nil || resID == "" {
			return parsedCatalogURI{}, fmt.Errorf("invalid resource id")
		}
		out.resourceID = resID
		if len(parts) == 3 {
			out.kind = "resource"
			return out, nil
		}
		switch parts[3] {
		case "content":
			if len(parts) != 4 {
				return parsedCatalogURI{}, fmt.Errorf("unknown catalog uri")
			}
			out.kind = "resource-content"
		case "files":
			if len(parts) < 5 {
				return parsedCatalogURI{}, fmt.Errorf("unknown catalog uri")
			}
			// Single urlencoded path segment preferred; also accept multi-segment.
			var segs []string
			for _, p := range parts[4:] {
				seg, err := url.PathUnescape(p)
				if err != nil {
					return parsedCatalogURI{}, fmt.Errorf("invalid file path")
				}
				segs = append(segs, seg)
			}
			out.path = strings.Join(segs, "/")
			if out.path == "" {
				return parsedCatalogURI{}, fmt.Errorf("invalid file path")
			}
			out.kind = "resource-file"
		default:
			return parsedCatalogURI{}, fmt.Errorf("unknown catalog uri")
		}
	default:
		return parsedCatalogURI{}, fmt.Errorf("unknown catalog uri")
	}
	return out, nil
}
