package web

import (
	"fmt"
	"net/http"
	"strings"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/specimport"
	"github.com/daltoniam/switchboard/web/templates/pages"
)

// handleSpecImports renders the Spec Imports management page: the list of
// configured imports (with live registration status) plus the add form.
func (w *WebServer) handleSpecImports(rw http.ResponseWriter, r *http.Request) {
	cfg := w.services.Config.Get()

	rows := make([]pages.SpecImportRow, 0, len(cfg.SpecImports))
	for _, si := range cfg.SpecImports {
		regName := specimport.SanitizedName(si.Name)
		row := pages.SpecImportRow{
			Name:     si.Name,
			Kind:     si.Kind,
			Endpoint: si.Endpoint,
			Source:   specImportSource(si),
			Enabled:  si.Enabled,
		}
		if in, ok := w.services.Registry.Get(regName); ok {
			row.Registered = true
			row.ToolCount = len(in.Tools())
		}
		rows = append(rows, row)
	}

	page := w.pageData(r, "Spec Imports", "/spec-imports")
	data := pages.SpecImportsData{
		Imports: rows,
	}
	pages.SpecImports(page, data).Render(r.Context(), rw)
}

// handleSpecImportSave validates a submitted spec, persists it, and triggers a
// live reconcile so the tools appear without a restart. Validation happens
// before persistence: a spec that can't be loaded is rejected with a message
// rather than saved into a broken state.
func (w *WebServer) handleSpecImportSave(rw http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		specImportRedirect(rw, r, "", "invalid form submission")
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		specImportRedirect(rw, r, "", "name is required")
		return
	}
	regName := specimport.SanitizedName(name)
	if w.services.Registry != nil {
		if _, exists := w.services.Registry.Get(regName); exists {
			// Allow replace of an already-registered spec import (same name).
			// Reject only when the name is owned by a non-spec-import integration.
			if !isSpecImportName(w.services.Config.Get().SpecImports, regName) {
				specImportRedirect(rw, r, "", fmt.Sprintf("name %q is reserved by a built-in integration", regName))
				return
			}
		}
	}

	entry := mcp.SpecImportConfig{
		Name:     name,
		Kind:     strings.TrimSpace(r.FormValue("kind")),
		Spec:     r.FormValue("spec"),
		Endpoint: strings.TrimSpace(r.FormValue("endpoint")),
		Enabled:  true,
	}
	if creds := specImportCreds(r); len(creds) > 0 {
		entry.Credentials = creds
	}

	// Validate by loading before persisting.
	if _, err := specimport.Load(entry); err != nil {
		specImportRedirect(rw, r, "", err.Error())
		return
	}

	cfg := w.services.Config.Get()
	imports := upsertSpecImport(cfg.SpecImports, entry)
	if err := w.services.Config.SetSpecImports(imports); err != nil {
		specImportRedirect(rw, r, "", "save failed: "+err.Error())
		return
	}
	w.notifyConfigChanged()
	specImportRedirect(rw, r, "Imported "+name, "")
}

// handleSpecImportDelete removes a spec import by name and reconciles.
func (w *WebServer) handleSpecImportDelete(rw http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		specImportRedirect(rw, r, "", "invalid form submission")
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		specImportRedirect(rw, r, "", "name is required")
		return
	}

	cfg := w.services.Config.Get()
	imports := removeSpecImport(cfg.SpecImports, name)
	if err := w.services.Config.SetSpecImports(imports); err != nil {
		specImportRedirect(rw, r, "", "save failed: "+err.Error())
		return
	}
	w.notifyConfigChanged()
	specImportRedirect(rw, r, "Removed "+name, "")
}

// specImportCreds collects the optional credential fields from the form,
// returning nil when none are set so we don't persist empty maps.
func specImportCreds(r *http.Request) mcp.Credentials {
	creds := mcp.Credentials{}
	apiKey := strings.TrimSpace(r.FormValue("api_key"))
	if apiKey != "" {
		// Always record scheme when a key is present so empty means "raw key"
		// (not "default to Bearer").
		creds["api_key"] = apiKey
		creds["auth_scheme"] = strings.TrimSpace(r.FormValue("auth_scheme"))
	}
	if v := strings.TrimSpace(r.FormValue("auth_header")); v != "" {
		creds["auth_header"] = v
	}
	if len(creds) == 0 {
		return nil
	}
	return creds
}

// specImportSource describes where a spec came from for display.
func specImportSource(si mcp.SpecImportConfig) string {
	if strings.TrimSpace(si.Path) != "" {
		return si.Path
	}
	return "inline"
}

// upsertSpecImport replaces an entry with the same sanitized name or appends it.
func upsertSpecImport(list []mcp.SpecImportConfig, entry mcp.SpecImportConfig) []mcp.SpecImportConfig {
	target := specimport.SanitizedName(entry.Name)
	out := make([]mcp.SpecImportConfig, len(list))
	copy(out, list)
	for i := range out {
		if specimport.SanitizedName(out[i].Name) == target {
			out[i] = entry
			return out
		}
	}
	return append(out, entry)
}

// isSpecImportName reports whether sanitized name already belongs to a
// configured spec import (so re-save/replace is allowed).
func isSpecImportName(list []mcp.SpecImportConfig, sanitized string) bool {
	for _, si := range list {
		if specimport.SanitizedName(si.Name) == sanitized {
			return true
		}
	}
	return false
}

// removeSpecImport drops the entry whose sanitized name matches.
func removeSpecImport(list []mcp.SpecImportConfig, name string) []mcp.SpecImportConfig {
	target := specimport.SanitizedName(name)
	out := list[:0:0]
	for _, si := range list {
		if specimport.SanitizedName(si.Name) == target {
			continue
		}
		out = append(out, si)
	}
	return out
}

// specImportRedirect sends the browser back to the page with a flash message.
func specImportRedirect(rw http.ResponseWriter, r *http.Request, success, errMsg string) {
	q := "/spec-imports"
	switch {
	case errMsg != "":
		q += "?error=" + urlEncode(errMsg)
	case success != "":
		q += "?success=" + urlEncode(success)
	}
	http.Redirect(rw, r, q, http.StatusSeeOther)
}
