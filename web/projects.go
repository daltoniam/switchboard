package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/daltoniam/switchboard/project"
	"github.com/daltoniam/switchboard/web/templates/pages"
)

// WithProjectCatalog injects the filesystem catalog for the Projects UI.
func WithProjectCatalog(cat project.Catalog) Option {
	return func(w *WebServer) { w.catalog = cat }
}

func (w *WebServer) handleProjectsList(rw http.ResponseWriter, r *http.Request) {
	page := w.pageData(r, "Projects", "/projects")
	data := pages.ProjectsListData{
		Query:           strings.TrimSpace(r.URL.Query().Get("q")),
		Projects:        []project.ProjectSummary{},
		InvalidProjects: []project.InvalidProjectSummary{},
	}

	if w.catalog == nil {
		_ = pages.ProjectsList(page, data).Render(r.Context(), rw)
		return
	}

	result, err := w.catalog.Search(r.Context(), project.SearchRequest{
		Query:  data.Query,
		Cursor: r.URL.Query().Get("cursor"),
	})
	if err != nil {
		page.FlashError = "Failed to load projects: " + err.Error()
		_ = pages.ProjectsList(page, data).Render(r.Context(), rw)
		return
	}
	data.Projects = result.Projects
	data.InvalidProjects = result.InvalidProjects
	data.NextCursor = result.NextCursor
	data.TotalValid = len(result.Projects)
	data.TotalInvalid = len(result.InvalidProjects)

	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = pages.ProjectsList(page, data).Render(r.Context(), rw)
}

func (w *WebServer) handleProjectDetail(rw http.ResponseWriter, r *http.Request) {
	rawID := r.PathValue("id")
	id, err := url.PathUnescape(rawID)
	if err != nil || id == "" {
		http.NotFound(rw, r)
		return
	}
	page := w.pageData(r, id, "/projects")
	data := pages.ProjectDetailData{ProjectID: id}

	if w.catalog == nil {
		data.NotFound = true
		rw.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = pages.ProjectDetail(page, data).Render(r.Context(), rw)
		return
	}

	snap, err := w.catalog.Get(r.Context(), project.ProjectID(id))
	if err != nil {
		if project.IsCode(err, project.CodeInvalidDefinition) {
			// Prefer invalid summary from search/list.
			if inv, ok := w.lookupInvalid(r.Context(), project.ProjectID(id)); ok {
				data.Invalid = &inv
				page.Title = inv.Title
				rw.Header().Set("Content-Type", "text/html; charset=utf-8")
				_ = pages.ProjectDetail(page, data).Render(r.Context(), rw)
				return
			}
		}
		if project.IsCode(err, project.CodeProjectNotFound) {
			// Maybe only present as invalid.
			if inv, ok := w.lookupInvalid(r.Context(), project.ProjectID(id)); ok {
				data.Invalid = &inv
				page.Title = inv.Title
				rw.Header().Set("Content-Type", "text/html; charset=utf-8")
				_ = pages.ProjectDetail(page, data).Render(r.Context(), rw)
				return
			}
			data.NotFound = true
			rw.Header().Set("Content-Type", "text/html; charset=utf-8")
			_ = pages.ProjectDetail(page, data).Render(r.Context(), rw)
			return
		}
		page.FlashError = err.Error()
		data.NotFound = true
		rw.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = pages.ProjectDetail(page, data).Render(r.Context(), rw)
		return
	}

	data.Snapshot = snap
	page.Title = snap.Definition.Name
	if cfgDir := catalogConfigDir(w.catalog); cfgDir != "" {
		data.Context = project.AssembleManifest(&snap.Definition, cfgDir)
		if data.Context == nil {
			data.Context = []project.ContextEntry{}
		}
	}
	raw, err := json.MarshalIndent(snap.Definition, "", "  ")
	if err != nil {
		data.JSON = "{}"
	} else {
		data.JSON = string(raw)
	}

	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = pages.ProjectDetail(page, data).Render(r.Context(), rw)
}

func (w *WebServer) lookupInvalid(ctx context.Context, id project.ProjectID) (project.InvalidProjectSummary, bool) {
	page, err := w.catalog.List(ctx, "")
	if err != nil {
		return project.InvalidProjectSummary{}, false
	}
	for _, inv := range page.InvalidProjects {
		if inv.ProjectID == id {
			return inv, true
		}
	}
	// walk cursors lightly
	cursor := page.NextCursor
	for cursor != "" {
		next, err := w.catalog.List(ctx, cursor)
		if err != nil {
			break
		}
		for _, inv := range next.InvalidProjects {
			if inv.ProjectID == id {
				return inv, true
			}
		}
		if next.NextCursor == "" || next.NextCursor == cursor {
			break
		}
		cursor = next.NextCursor
	}
	return project.InvalidProjectSummary{}, false
}

func catalogConfigDir(cat project.Catalog) string {
	if s, ok := cat.(interface{ ConfigDir() string }); ok {
		return s.ConfigDir()
	}
	return ""
}
