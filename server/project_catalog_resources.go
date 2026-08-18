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

func projectResourceURI(id project.ProjectID) string {
	return fmt.Sprintf("project://registry/projects/%s", url.PathEscape(string(id)))
}

func definitionResourceURI(id project.ProjectID) string {
	return projectResourceURI(id) + "/definition"
}

func diagnosticsResourceURI(id project.ProjectID) string {
	return fmt.Sprintf("project://registry/projects/%s/diagnostics", url.PathEscape(string(id)))
}

type parsedCatalogURI struct {
	kind      string
	projectID project.ProjectID
	revision  project.Revision
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
	default:
		return parsedCatalogURI{}, fmt.Errorf("unknown catalog uri")
	}
	return out, nil
}
