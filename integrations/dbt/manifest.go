package dbt

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
)

// manifest is the subset of dbt's manifest.json that the lineage tools need.
// Production manifests run to tens of megabytes; decoding into this partial
// struct keeps memory bounded to the graph, never the compiled SQL.
type manifest struct {
	Metadata struct {
		ProjectName string `json:"project_name"`
		DbtVersion  string `json:"dbt_version"`
	} `json:"metadata"`
	Nodes     map[string]manifestNode     `json:"nodes"`
	Sources   map[string]manifestNode     `json:"sources"`
	Exposures map[string]manifestExposure `json:"exposures"`
	Macros    map[string]manifestMacro    `json:"macros"`
	ChildMap  map[string][]string         `json:"child_map"`
	ParentMap map[string][]string         `json:"parent_map"`
}

type manifestNode struct {
	UniqueID         string `json:"unique_id"`
	Name             string `json:"name"`
	ResourceType     string `json:"resource_type"`
	PackageName      string `json:"package_name"`
	OriginalFilePath string `json:"original_file_path"`
	PatchPath        string `json:"patch_path"`
	RelationName     string `json:"relation_name"`
	Config           struct {
		Materialized string `json:"materialized"`
	} `json:"config"`
	DependsOn struct {
		Macros []string `json:"macros"`
	} `json:"depends_on"`
}

type manifestExposure struct {
	UniqueID         string `json:"unique_id"`
	Name             string `json:"name"`
	Type             string `json:"type"`
	PackageName      string `json:"package_name"`
	OriginalFilePath string `json:"original_file_path"`
	URL              string `json:"url"`
	Owner            struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"owner"`
}

type manifestMacro struct {
	UniqueID         string `json:"unique_id"`
	Name             string `json:"name"`
	PackageName      string `json:"package_name"`
	OriginalFilePath string `json:"original_file_path"`
}

func decodeManifest(r io.Reader) (*manifest, error) {
	var m manifest
	if err := json.NewDecoder(r).Decode(&m); err != nil {
		return nil, fmt.Errorf("dbt: decode manifest.json: %w", err)
	}
	return &m, nil
}

// inRootProject excludes installed packages (dbt_utils etc.), whose
// original_file_path is relative to the package and collides with project paths.
func (m *manifest) inRootProject(pkg string) bool {
	return m.Metadata.ProjectName == "" || pkg == m.Metadata.ProjectName
}

// nodeSummary is the per-node shape returned by the manifest tools.
type nodeSummary struct {
	UniqueID         string   `json:"unique_id"`
	Name             string   `json:"name"`
	ResourceType     string   `json:"resource_type"`
	Materialized     string   `json:"materialized,omitempty"`
	OriginalFilePath string   `json:"original_file_path,omitempty"`
	RelationName     string   `json:"relation_name,omitempty"`
	Depth            int      `json:"depth,omitempty"`
	MacroDependents  []string `json:"macro_dependents,omitempty"`
}

func summarize(n manifestNode) nodeSummary {
	return nodeSummary{
		UniqueID:         n.UniqueID,
		Name:             n.Name,
		ResourceType:     n.ResourceType,
		Materialized:     n.Config.Materialized,
		OriginalFilePath: n.OriginalFilePath,
		RelationName:     n.RelationName,
	}
}

func normalizePath(p string) string {
	p = strings.ReplaceAll(strings.TrimSpace(p), "\\", "/")
	return strings.TrimPrefix(p, "./")
}

// patchFilePath strips the "<package>://" prefix dbt puts on patch_path.
func patchFilePath(p string) string {
	if i := strings.Index(p, "://"); i >= 0 {
		return p[i+3:]
	}
	return p
}

type matchResult struct {
	Nodes     []nodeSummary `json:"nodes"`
	Unmatched []string      `json:"unmatched"`
}

// matchNodes maps changed file paths and node names to manifest nodes.
// A path matches a node's SQL file or the schema YAML that patches it; macro
// paths return the macro plus every node that calls it.
func (m *manifest) matchNodes(paths, names []string) matchResult {
	res := matchResult{Nodes: []nodeSummary{}, Unmatched: []string{}}
	seen := map[string]bool{}
	add := func(s nodeSummary) {
		if !seen[s.UniqueID] {
			seen[s.UniqueID] = true
			res.Nodes = append(res.Nodes, s)
		}
	}

	for _, raw := range paths {
		p := normalizePath(raw)
		if p == "" {
			continue
		}
		found := false
		for _, group := range []map[string]manifestNode{m.Nodes, m.Sources} {
			for _, n := range group {
				if !m.inRootProject(n.PackageName) {
					continue
				}
				if n.OriginalFilePath == p || (n.PatchPath != "" && patchFilePath(n.PatchPath) == p) {
					add(summarize(n))
					found = true
				}
			}
		}
		for _, mac := range m.Macros {
			if !m.inRootProject(mac.PackageName) || mac.OriginalFilePath != p {
				continue
			}
			found = true
			add(nodeSummary{
				UniqueID:         mac.UniqueID,
				Name:             mac.Name,
				ResourceType:     "macro",
				OriginalFilePath: mac.OriginalFilePath,
				MacroDependents:  m.macroDependents(mac.UniqueID),
			})
		}
		if !found {
			res.Unmatched = append(res.Unmatched, raw)
		}
	}

	for _, name := range names {
		ids, missing := m.resolveIDs(nil, []string{name})
		if len(missing) > 0 {
			res.Unmatched = append(res.Unmatched, name)
			continue
		}
		for _, id := range ids {
			if n, ok := m.lookup(id); ok {
				add(summarize(n))
			}
		}
	}
	sort.Slice(res.Nodes, func(i, j int) bool { return res.Nodes[i].UniqueID < res.Nodes[j].UniqueID })
	return res
}

func (m *manifest) macroDependents(macroID string) []string {
	var out []string
	for id, n := range m.Nodes {
		if n.ResourceType == "test" {
			continue
		}
		for _, dep := range n.DependsOn.Macros {
			if dep == macroID {
				out = append(out, id)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

func (m *manifest) lookup(id string) (manifestNode, bool) {
	if n, ok := m.Nodes[id]; ok {
		return n, true
	}
	n, ok := m.Sources[id]
	return n, ok
}

// resolveIDs accepts unique_ids as-is when present and resolves bare names
// against root-project nodes. Names that resolve to nothing are returned
// separately so callers can report them.
func (m *manifest) resolveIDs(uniqueIDs, names []string) (ids, unmatched []string) {
	for _, id := range uniqueIDs {
		if _, ok := m.lookup(id); ok {
			ids = append(ids, id)
		} else {
			unmatched = append(unmatched, id)
		}
	}
	for _, name := range names {
		found := false
		for id, n := range m.Nodes {
			if n.Name == name && n.ResourceType != "test" && m.inRootProject(n.PackageName) {
				ids = append(ids, id)
				found = true
			}
		}
		if !found {
			unmatched = append(unmatched, name)
		}
	}
	sort.Strings(ids)
	return ids, unmatched
}

type exposureSummary struct {
	UniqueID   string `json:"unique_id"`
	Name       string `json:"name"`
	Type       string `json:"type,omitempty"`
	URL        string `json:"url,omitempty"`
	OwnerName  string `json:"owner_name,omitempty"`
	OwnerEmail string `json:"owner_email,omitempty"`
}

type lineageResult struct {
	Direction string            `json:"direction"`
	Depth     int               `json:"depth"`
	Start     []string          `json:"start"`
	Nodes     []nodeSummary     `json:"nodes"`
	Tests     []string          `json:"tests"`
	Exposures []exposureSummary `json:"exposures"`
	Truncated bool              `json:"truncated,omitempty"`
}

// lineage walks child_map (down) or parent_map (up) breadth-first from start.
// depth <= 0 means unlimited. Tests and exposures are reported separately so
// the node list reads as the models a change actually rebuilds.
func (m *manifest) lineage(start []string, direction string, depth, limit int) lineageResult {
	graph := m.ChildMap
	if direction == "up" {
		graph = m.ParentMap
	} else {
		direction = "down"
	}
	res := lineageResult{
		Direction: direction,
		Depth:     depth,
		Start:     start,
		Nodes:     []nodeSummary{},
		Tests:     []string{},
		Exposures: []exposureSummary{},
	}

	visited := map[string]bool{}
	for _, id := range start {
		visited[id] = true
	}
	frontier := append([]string(nil), start...)
	for level := 1; len(frontier) > 0 && (depth <= 0 || level <= depth); level++ {
		var next []string
		for _, id := range frontier {
			children := append([]string(nil), graph[id]...)
			sort.Strings(children)
			for _, child := range children {
				if visited[child] {
					continue
				}
				visited[child] = true
				switch {
				case strings.HasPrefix(child, "exposure."):
					e := m.Exposures[child]
					res.Exposures = append(res.Exposures, exposureSummary{
						UniqueID: child, Name: e.Name, Type: e.Type, URL: e.URL,
						OwnerName: e.Owner.Name, OwnerEmail: e.Owner.Email,
					})
				case strings.HasPrefix(child, "test."):
					res.Tests = append(res.Tests, child)
				default:
					if len(res.Nodes) >= limit {
						res.Truncated = true
						continue
					}
					s := nodeSummary{UniqueID: child, Depth: level}
					if n, ok := m.lookup(child); ok {
						s = summarize(n)
						s.Depth = level
					}
					res.Nodes = append(res.Nodes, s)
					next = append(next, child)
				}
			}
		}
		frontier = next
	}
	return res
}

// manifestCache keeps the most recently used decoded manifests. Run
// artifacts are immutable, so entries never go stale.
type manifestCache struct {
	mu    sync.Mutex
	size  int
	order []string
	items map[string]*manifest
}

func newManifestCache(size int) *manifestCache {
	return &manifestCache{size: size, items: map[string]*manifest{}}
}

func (c *manifestCache) get(key string) (*manifest, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	m, ok := c.items[key]
	if ok {
		c.touch(key)
	}
	return m, ok
}

func (c *manifestCache) put(key string, m *manifest) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.items[key]; !ok && len(c.order) >= c.size {
		oldest := c.order[0]
		c.order = c.order[1:]
		delete(c.items, oldest)
	}
	c.items[key] = m
	c.touch(key)
}

func (c *manifestCache) touch(key string) {
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}
	c.order = append(c.order, key)
}
