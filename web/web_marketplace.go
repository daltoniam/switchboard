package web

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/daltoniam/switchboard/marketplace"
	"github.com/daltoniam/switchboard/web/templates/pages"
)

func (w *WebServer) handlePluginMarketplace(rw http.ResponseWriter, r *http.Request) {
	page := w.pageData(r, "Plugin Marketplace", "/plugins")
	data := pages.PluginMarketplaceData{}

	if w.marketplace == nil {
		pages.PluginMarketplace(page, data).Render(r.Context(), rw)
		return
	}

	cfg := w.marketplace.Config()
	data.AutoUpdate = cfg.AutoUpdate
	data.LastCheck = cfg.LastCheck

	for i, src := range cfg.ManifestSources {
		data.ManifestSources = append(data.ManifestSources, pages.ManifestSourceEntry{
			Index:   i,
			URL:     src.URL,
			Name:    src.Name,
			Enabled: src.Enabled,
		})
	}

	for _, ip := range w.marketplace.InstalledPlugins() {
		entry := pages.InstalledPluginEntry{
			Name:          ip.Name,
			Version:       ip.Version,
			Path:          ip.Path,
			InstalledAt:   ip.InstalledAt,
			AutoUpdate:    ip.AutoUpdate,
			LatestVersion: ip.LatestVersion,
			UpdateAvail:   ip.LatestVersion != "" && ip.LatestVersion != ip.Version,
		}
		data.Installed = append(data.Installed, entry)
	}

	if len(cfg.ManifestSources) > 0 {
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		results, err := w.marketplace.BrowsePlugins(ctx)
		if err != nil {
			data.FetchError = err.Error()
		} else {
			for _, br := range results {
				data.Available = append(data.Available, pages.PluginEntry{
					Name:             br.Name,
					Description:      br.Description,
					Author:           br.Author,
					Homepage:         br.Homepage,
					License:          br.License,
					LatestVersion:    br.LatestVersion,
					ManifestSource:   br.ManifestSource,
					Installed:        br.Installed,
					InstalledVersion: br.InstalledVersion,
					UpdateAvailable:  br.UpdateAvailable,
				})
			}
		}
	}

	pages.PluginMarketplace(page, data).Render(r.Context(), rw)
}

func (w *WebServer) handlePluginInstall(rw http.ResponseWriter, r *http.Request) {
	if w.marketplace == nil {
		http.Redirect(rw, r, "/plugins?error=Marketplace+not+configured", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(rw, r, "/plugins?error=Invalid+form+data", http.StatusSeeOther)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Redirect(rw, r, "/plugins?error=Plugin+name+is+required", http.StatusSeeOther)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	ip, err := w.marketplace.InstallPlugin(ctx, name)
	if err != nil {
		http.Redirect(rw, r, "/plugins?error=Install+failed:+"+urlEncode(err.Error()), http.StatusSeeOther)
		return
	}

	http.Redirect(rw, r, fmt.Sprintf("/plugins?success=Installed+%s@%s.+Restart+Switchboard+to+load+it.", ip.Name, ip.Version), http.StatusSeeOther)
}

func (w *WebServer) handlePluginInstallURL(rw http.ResponseWriter, r *http.Request) {
	if w.marketplace == nil {
		http.Redirect(rw, r, "/plugins?error=Marketplace+not+configured", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(rw, r, "/plugins?error=Invalid+form+data", http.StatusSeeOther)
		return
	}

	url := strings.TrimSpace(r.FormValue("url"))
	if url == "" {
		http.Redirect(rw, r, "/plugins?error=URL+is+required", http.StatusSeeOther)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	ip, err := w.marketplace.InstallFromURL(ctx, url)
	if err != nil {
		http.Redirect(rw, r, "/plugins?error=Install+failed:+"+urlEncode(err.Error()), http.StatusSeeOther)
		return
	}

	http.Redirect(rw, r, fmt.Sprintf("/plugins?success=Installed+%s.+Restart+Switchboard+to+load+it.", ip.Name), http.StatusSeeOther)
}

func (w *WebServer) handlePluginUpload(rw http.ResponseWriter, r *http.Request) {
	if w.marketplace == nil {
		http.Redirect(rw, r, "/plugins?error=Marketplace+not+configured", http.StatusSeeOther)
		return
	}

	if err := r.ParseMultipartForm(100 << 20); err != nil { // 100MB limit
		http.Redirect(rw, r, "/plugins?error=File+too+large+or+invalid+form", http.StatusSeeOther)
		return
	}

	file, _, err := r.FormFile("wasm")
	if err != nil {
		http.Redirect(rw, r, "/plugins?error=No+file+uploaded", http.StatusSeeOther)
		return
	}
	defer file.Close() //nolint:errcheck

	data, err := io.ReadAll(io.LimitReader(file, 100<<20))
	if err != nil {
		http.Redirect(rw, r, "/plugins?error=Failed+to+read+file", http.StatusSeeOther)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	ip, err := w.marketplace.InstallFromBytes(name, data)
	if err != nil {
		http.Redirect(rw, r, "/plugins?error=Upload+failed:+"+urlEncode(err.Error()), http.StatusSeeOther)
		return
	}

	http.Redirect(rw, r, fmt.Sprintf("/plugins?success=Uploaded+%s.+Restart+Switchboard+to+load+it.", ip.Name), http.StatusSeeOther)
}

func (w *WebServer) handlePluginUninstall(rw http.ResponseWriter, r *http.Request) {
	if w.marketplace == nil {
		http.Redirect(rw, r, "/plugins?error=Marketplace+not+configured", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(rw, r, "/plugins?error=Invalid+form+data", http.StatusSeeOther)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if err := w.marketplace.UninstallPlugin(name); err != nil {
		http.Redirect(rw, r, "/plugins?error=Uninstall+failed:+"+urlEncode(err.Error()), http.StatusSeeOther)
		return
	}

	http.Redirect(rw, r, fmt.Sprintf("/plugins?success=Uninstalled+%s.+Restart+Switchboard+to+apply.", name), http.StatusSeeOther)
}

func (w *WebServer) handlePluginUpdate(rw http.ResponseWriter, r *http.Request) {
	if w.marketplace == nil {
		http.Redirect(rw, r, "/plugins?error=Marketplace+not+configured", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(rw, r, "/plugins?error=Invalid+form+data", http.StatusSeeOther)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Redirect(rw, r, "/plugins?error=Plugin+name+is+required", http.StatusSeeOther)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	ip, err := w.marketplace.UpdatePlugin(ctx, name)
	if err != nil {
		http.Redirect(rw, r, "/plugins?error=Update+failed:+"+urlEncode(err.Error()), http.StatusSeeOther)
		return
	}

	http.Redirect(rw, r, fmt.Sprintf("/plugins?success=Updated+%s+to+%s.+Restart+Switchboard+to+apply.", ip.Name, ip.Version), http.StatusSeeOther)
}

func (w *WebServer) handlePluginCheckUpdates(rw http.ResponseWriter, r *http.Request) {
	if w.marketplace == nil {
		http.Redirect(rw, r, "/plugins?error=Marketplace+not+configured", http.StatusSeeOther)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	updates, err := w.marketplace.CheckForUpdates(ctx)
	if err != nil {
		http.Redirect(rw, r, "/plugins?error=Check+failed:+"+urlEncode(err.Error()), http.StatusSeeOther)
		return
	}

	if len(updates) > 0 {
		names := make([]string, len(updates))
		for i, u := range updates {
			names[i] = u.Name
		}
		http.Redirect(rw, r, fmt.Sprintf("/plugins?success=Updates+available+for:+%s", strings.Join(names, ",+")), http.StatusSeeOther)
	} else {
		http.Redirect(rw, r, "/plugins?success=All+plugins+are+up+to+date.", http.StatusSeeOther)
	}
}

func (w *WebServer) handlePluginAutoUpdate(rw http.ResponseWriter, r *http.Request) {
	if w.marketplace == nil {
		http.Redirect(rw, r, "/plugins?error=Marketplace+not+configured", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(rw, r, "/plugins?error=Invalid+form+data", http.StatusSeeOther)
		return
	}

	enabled := r.FormValue("enabled") == "true"
	if err := w.marketplace.SetAutoUpdate(enabled); err != nil {
		http.Redirect(rw, r, "/plugins?error=Failed+to+save:+"+urlEncode(err.Error()), http.StatusSeeOther)
		return
	}

	if enabled {
		http.Redirect(rw, r, "/plugins?success=Automatic+updates+enabled.", http.StatusSeeOther)
	} else {
		http.Redirect(rw, r, "/plugins?success=Automatic+updates+disabled.", http.StatusSeeOther)
	}
}

func (w *WebServer) handlePluginAddManifest(rw http.ResponseWriter, r *http.Request) {
	if w.marketplace == nil {
		http.Redirect(rw, r, "/plugins?error=Marketplace+not+configured", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(rw, r, "/plugins?error=Invalid+form+data", http.StatusSeeOther)
		return
	}

	url := strings.TrimSpace(r.FormValue("url"))
	if url == "" {
		http.Redirect(rw, r, "/plugins?error=URL+is+required", http.StatusSeeOther)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	src := marketplace.ManifestSource{
		URL:     url,
		Name:    name,
		Enabled: true,
	}
	if err := w.marketplace.AddManifestSource(src); err != nil {
		http.Redirect(rw, r, "/plugins?error="+urlEncode(err.Error()), http.StatusSeeOther)
		return
	}

	http.Redirect(rw, r, "/plugins?success=Manifest+source+added.", http.StatusSeeOther)
}

func (w *WebServer) handlePluginRemoveManifest(rw http.ResponseWriter, r *http.Request) {
	if w.marketplace == nil {
		http.Redirect(rw, r, "/plugins?error=Marketplace+not+configured", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(rw, r, "/plugins?error=Invalid+form+data", http.StatusSeeOther)
		return
	}

	url := strings.TrimSpace(r.FormValue("url"))
	if err := w.marketplace.RemoveManifestSource(url); err != nil {
		http.Redirect(rw, r, "/plugins?error="+urlEncode(err.Error()), http.StatusSeeOther)
		return
	}

	http.Redirect(rw, r, "/plugins?success=Manifest+source+removed.", http.StatusSeeOther)
}

func urlEncode(s string) string {
	return strings.ReplaceAll(s, " ", "+")
}
