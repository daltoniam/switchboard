package web

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/daltoniam/switchboard/awm"
	"github.com/daltoniam/switchboard/web/templates/pages"
)

// WithAWMStore injects the work-model store for Work / profiles / sessions UI.
func WithAWMStore(store *awm.Store) Option {
	return func(w *WebServer) { w.awmStore = store }
}

func (w *WebServer) handleWorkHub(rw http.ResponseWriter, r *http.Request) {
	page := w.pageData(r, "Work", "/work")
	data := pages.WorkHubData{
		StateFilter:   strings.TrimSpace(r.URL.Query().Get("state")),
		ProjectFilter: strings.TrimSpace(r.URL.Query().Get("project_id")),
	}
	if w.awmStore == nil {
		rw.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = pages.WorkHub(page, data).Render(r.Context(), rw)
		return
	}
	ctx := r.Context()
	data.WorkProfiles, _ = w.awmStore.ListWorkProfiles(ctx)
	data.AgentProfiles, _ = w.awmStore.ListAgentProfiles(ctx)
	data.WorkSessions, _ = w.awmStore.ListWorkSessions(ctx, data.StateFilter, data.ProjectFilter)
	if data.WorkProfiles == nil {
		data.WorkProfiles = []awm.WorkProfile{}
	}
	if data.AgentProfiles == nil {
		data.AgentProfiles = []awm.AgentProfile{}
	}
	if data.WorkSessions == nil {
		data.WorkSessions = []awm.WorkSession{}
	}
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = pages.WorkHub(page, data).Render(ctx, rw)
}

func (w *WebServer) handleWorkProfilesList(rw http.ResponseWriter, r *http.Request) {
	page := w.pageData(r, "Work profiles", "/work")
	data := pages.WorkProfilesListData{Profiles: []awm.WorkProfile{}}
	if w.awmStore != nil {
		list, err := w.awmStore.ListWorkProfiles(r.Context())
		if err != nil {
			page.FlashError = err.Error()
		} else if list != nil {
			data.Profiles = list
		}
	}
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = pages.WorkProfilesList(page, data).Render(r.Context(), rw)
}

func (w *WebServer) handleWorkProfileDetail(rw http.ResponseWriter, r *http.Request) {
	id, err := url.PathUnescape(r.PathValue("id"))
	if err != nil || id == "" {
		http.NotFound(rw, r)
		return
	}
	page := w.pageData(r, id, "/work")
	data := pages.WorkProfileDetailData{ID: id, NotFound: true}
	if w.awmStore != nil {
		p, err := w.awmStore.GetWorkProfile(r.Context(), id)
		if err == nil {
			data.NotFound = false
			data.Profile = p
			page.Title = firstNonEmpty(p.DisplayName, p.WorkProfileID)
			data.JSON = mustJSON(p)
		}
	}
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = pages.WorkProfileDetail(page, data).Render(r.Context(), rw)
}

func (w *WebServer) handleAgentProfilesList(rw http.ResponseWriter, r *http.Request) {
	page := w.pageData(r, "Agent profiles", "/work")
	data := pages.AgentProfilesListData{Profiles: []awm.AgentProfile{}}
	if w.awmStore != nil {
		list, err := w.awmStore.ListAgentProfiles(r.Context())
		if err != nil {
			page.FlashError = err.Error()
		} else if list != nil {
			data.Profiles = list
		}
	}
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = pages.AgentProfilesList(page, data).Render(r.Context(), rw)
}

func (w *WebServer) handleAgentProfileDetail(rw http.ResponseWriter, r *http.Request) {
	id, err := url.PathUnescape(r.PathValue("id"))
	if err != nil || id == "" {
		http.NotFound(rw, r)
		return
	}
	page := w.pageData(r, id, "/work")
	data := pages.AgentProfileDetailData{ID: id, NotFound: true}
	if w.awmStore != nil {
		p, err := w.awmStore.GetAgentProfile(r.Context(), id)
		if err == nil {
			data.NotFound = false
			data.Profile = p
			page.Title = firstNonEmpty(p.DisplayName, p.AgentProfileID)
			data.JSON = mustJSON(p)
		}
	}
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = pages.AgentProfileDetail(page, data).Render(r.Context(), rw)
}

func (w *WebServer) handleWorkSessionsList(rw http.ResponseWriter, r *http.Request) {
	page := w.pageData(r, "Work sessions", "/work")
	data := pages.WorkSessionsListData{
		Sessions:      []awm.WorkSession{},
		StateFilter:   strings.TrimSpace(r.URL.Query().Get("state")),
		ProjectFilter: strings.TrimSpace(r.URL.Query().Get("project_id")),
	}
	if w.awmStore != nil {
		list, err := w.awmStore.ListWorkSessions(r.Context(), data.StateFilter, data.ProjectFilter)
		if err != nil {
			page.FlashError = err.Error()
		} else if list != nil {
			data.Sessions = list
		}
	}
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = pages.WorkSessionsList(page, data).Render(r.Context(), rw)
}

func (w *WebServer) handleWorkSessionDetail(rw http.ResponseWriter, r *http.Request) {
	id, err := url.PathUnescape(r.PathValue("id"))
	if err != nil || id == "" {
		http.NotFound(rw, r)
		return
	}
	page := w.pageData(r, id, "/work")
	data := pages.WorkSessionDetailData{ID: id, NotFound: true}
	if w.awmStore != nil {
		s, err := w.awmStore.GetWorkSession(r.Context(), id)
		if err == nil {
			data.NotFound = false
			data.Session = s
			page.Title = firstNonEmpty(s.DisplayName, s.WorkSessionID)
			data.JSON = mustJSON(s)
		}
	}
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = pages.WorkSessionDetail(page, data).Render(r.Context(), rw)
}

func mustJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(b)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
