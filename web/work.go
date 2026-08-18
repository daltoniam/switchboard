package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/daltoniam/switchboard/awm"
	"github.com/daltoniam/switchboard/web/templates/pages"
)

// WithAWMStore injects the work-model store for project-scoped Work UI.
func WithAWMStore(store *awm.Store) Option {
	return func(w *WebServer) { w.awmStore = store }
}

func (w *WebServer) projectPathID(r *http.Request) (string, bool) {
	id, err := url.PathUnescape(r.PathValue("id"))
	if err != nil || strings.TrimSpace(id) == "" {
		return "", false
	}
	return id, true
}

func (w *WebServer) loadProjectWork(ctx context.Context, projectID, stateFilter string) (pages.ProjectWorkData, error) {
	data := pages.ProjectWorkData{
		ProjectID:     projectID,
		StateFilter:   stateFilter,
		WorkProfiles:  []awm.WorkProfile{},
		AgentProfiles: []awm.AgentProfile{},
		WorkSessions:  []awm.WorkSession{},
	}
	if w.awmStore == nil {
		return data, nil
	}
	profiles, err := w.awmStore.ListWorkProfiles(ctx)
	if err != nil {
		return data, err
	}
	for _, p := range profiles {
		if workProfileTouchesProject(p, projectID) {
			data.WorkProfiles = append(data.WorkProfiles, p)
		}
	}
	sessions, err := w.awmStore.ListWorkSessions(ctx, stateFilter, projectID)
	if err != nil {
		return data, err
	}
	if sessions != nil {
		data.WorkSessions = sessions
	}
	agentIDs := map[string]struct{}{}
	for _, s := range data.WorkSessions {
		for _, id := range s.AgentProfileIDs {
			agentIDs[id] = struct{}{}
		}
	}
	agents, err := w.awmStore.ListAgentProfiles(ctx)
	if err != nil {
		return data, err
	}
	for _, a := range agents {
		if _, ok := agentIDs[a.AgentProfileID]; ok {
			data.AgentProfiles = append(data.AgentProfiles, a)
			continue
		}
		if agentProfileTouchesProject(a, projectID) {
			data.AgentProfiles = append(data.AgentProfiles, a)
		}
	}
	return data, nil
}

func workProfileTouchesProject(p awm.WorkProfile, projectID string) bool {
	// Empty project_ids means globally applicable (matches validateSessionProfile / docs).
	if len(p.ProjectIDs) == 0 {
		return true
	}
	if slices.Contains(p.ProjectIDs, projectID) {
		return true
	}
	// Exact default-profile convention only (avoid "switch" matching "switchboard.*").
	if p.WorkProfileID == projectID+".default" {
		return true
	}
	return false
}

func agentProfileTouchesProject(a awm.AgentProfile, projectID string) bool {
	// Exact default-agent convention only (avoid prefix false positives).
	if a.AgentProfileID == projectID+".default" {
		return true
	}
	if a.Constraints != nil {
		if raw, ok := a.Constraints["project_ids"]; ok {
			switch v := raw.(type) {
			case []string:
				return slices.Contains(v, projectID)
			case []any:
				for _, item := range v {
					if s, ok := item.(string); ok && s == projectID {
						return true
					}
				}
			}
		}
		if pid, ok := a.Constraints["project_id"].(string); ok && pid == projectID {
			return true
		}
	}
	return false
}

func (w *WebServer) handleProjectWorkHub(rw http.ResponseWriter, r *http.Request) {
	projectID, ok := w.projectPathID(r)
	if !ok {
		http.NotFound(rw, r)
		return
	}
	page := w.pageData(r, projectID+" work", "/projects")
	data, err := w.loadProjectWork(r.Context(), projectID, strings.TrimSpace(r.URL.Query().Get("state")))
	if err != nil {
		page.FlashError = err.Error()
	}
	page.Title = projectID + " · Work"
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = pages.ProjectWorkHub(page, data).Render(r.Context(), rw)
}

func (w *WebServer) handleProjectWorkProfilesList(rw http.ResponseWriter, r *http.Request) {
	projectID, ok := w.projectPathID(r)
	if !ok {
		http.NotFound(rw, r)
		return
	}
	page := w.pageData(r, "Work profiles", "/projects")
	data, err := w.loadProjectWork(r.Context(), projectID, "")
	if err != nil {
		page.FlashError = err.Error()
	}
	list := pages.WorkProfilesListData{ProjectID: projectID, Profiles: data.WorkProfiles}
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = pages.WorkProfilesList(page, list).Render(r.Context(), rw)
}

func (w *WebServer) handleProjectWorkProfileDetail(rw http.ResponseWriter, r *http.Request) {
	projectID, ok := w.projectPathID(r)
	if !ok {
		http.NotFound(rw, r)
		return
	}
	profileID, err := url.PathUnescape(r.PathValue("profileID"))
	if err != nil || profileID == "" {
		http.NotFound(rw, r)
		return
	}
	page := w.pageData(r, profileID, "/projects")
	data := pages.WorkProfileDetailData{ProjectID: projectID, ID: profileID, NotFound: true}
	if w.awmStore != nil {
		p, err := w.awmStore.GetWorkProfile(r.Context(), profileID)
		if err == nil && workProfileTouchesProject(p, projectID) {
			data.NotFound = false
			data.Profile = p
			page.Title = firstNonEmpty(p.DisplayName, p.WorkProfileID)
			data.JSON = mustJSON(p)
		}
	}
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = pages.WorkProfileDetail(page, data).Render(r.Context(), rw)
}

func (w *WebServer) handleProjectAgentProfilesList(rw http.ResponseWriter, r *http.Request) {
	projectID, ok := w.projectPathID(r)
	if !ok {
		http.NotFound(rw, r)
		return
	}
	page := w.pageData(r, "Agent profiles", "/projects")
	data, err := w.loadProjectWork(r.Context(), projectID, "")
	if err != nil {
		page.FlashError = err.Error()
	}
	list := pages.AgentProfilesListData{
		ProjectID: projectID,
		Profiles:  data.AgentProfiles,
	}
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = pages.AgentProfilesList(page, list).Render(r.Context(), rw)
}

func (w *WebServer) handleProjectAgentProfileDetail(rw http.ResponseWriter, r *http.Request) {
	projectID, ok := w.projectPathID(r)
	if !ok {
		http.NotFound(rw, r)
		return
	}
	agentID, err := url.PathUnescape(r.PathValue("agentID"))
	if err != nil || agentID == "" {
		http.NotFound(rw, r)
		return
	}
	page := w.pageData(r, agentID, "/projects")
	data := pages.AgentProfileDetailData{ProjectID: projectID, ID: agentID, NotFound: true}
	if p, ok := w.lookupProjectAgent(r.Context(), projectID, agentID); ok {
		data.NotFound = false
		data.Profile = p
		page.Title = firstNonEmpty(p.DisplayName, p.AgentProfileID)
		data.JSON = mustJSON(p)
	}
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = pages.AgentProfileDetail(page, data).Render(r.Context(), rw)
}

func (w *WebServer) lookupProjectAgent(ctx context.Context, projectID, agentID string) (awm.AgentProfile, bool) {
	if w.awmStore == nil {
		return awm.AgentProfile{}, false
	}
	p, err := w.awmStore.GetAgentProfile(ctx, agentID)
	if err != nil {
		return awm.AgentProfile{}, false
	}
	if agentProfileTouchesProject(p, projectID) {
		return p, true
	}
	sessions, err := w.awmStore.ListWorkSessions(ctx, "", projectID)
	if err != nil {
		return awm.AgentProfile{}, false
	}
	for _, s := range sessions {
		if slices.Contains(s.AgentProfileIDs, agentID) {
			return p, true
		}
	}
	return awm.AgentProfile{}, false
}

func (w *WebServer) handleProjectWorkSessionsList(rw http.ResponseWriter, r *http.Request) {
	projectID, ok := w.projectPathID(r)
	if !ok {
		http.NotFound(rw, r)
		return
	}
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	page := w.pageData(r, "Work sessions", "/projects")
	data, err := w.loadProjectWork(r.Context(), projectID, state)
	if err != nil {
		page.FlashError = err.Error()
	}
	list := pages.WorkSessionsListData{
		ProjectID:   projectID,
		Sessions:    data.WorkSessions,
		StateFilter: state,
	}
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = pages.WorkSessionsList(page, list).Render(r.Context(), rw)
}

func (w *WebServer) handleProjectWorkSessionDetail(rw http.ResponseWriter, r *http.Request) {
	projectID, ok := w.projectPathID(r)
	if !ok {
		http.NotFound(rw, r)
		return
	}
	sessionID, err := url.PathUnescape(r.PathValue("sessionID"))
	if err != nil || sessionID == "" {
		http.NotFound(rw, r)
		return
	}
	page := w.pageData(r, sessionID, "/projects")
	data := pages.WorkSessionDetailData{ProjectID: projectID, ID: sessionID, NotFound: true}
	if w.awmStore != nil {
		s, err := w.awmStore.GetWorkSession(r.Context(), sessionID)
		if err == nil && s.ProjectID == projectID {
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
