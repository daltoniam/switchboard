package growthbook

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	mcp "github.com/daltoniam/switchboard"
)

const (
	defaultLimit = 20
	maxLimit     = 100
	maxIDs       = 100
)

type handlerFunc func(ctx context.Context, g *growthbook, args map[string]any) (*mcp.ToolResult, error)

var dispatch = map[mcp.ToolName]handlerFunc{
	"growthbook_list_features": listHandler(listEndpoint{path: "/v2/features", paged: true,
		strings: map[string]string{"project_id": "projectId"}, bools: map[string]string{"archived": "archived"},
		scrub: dropEnvironmentDefinitions}),
	"growthbook_get_feature":           getHandler("/v2/features/%s", "feature_id", dropEnvironmentDefinitions),
	"growthbook_get_feature_staleness": getFeatureStaleness,
	"growthbook_list_feature_revisions": listHandler(listEndpoint{path: "/v2/features/%s/revisions", pathArg: "feature_id", paged: true,
		strings: map[string]string{"status": "status"}}),
	"growthbook_list_experiments": listHandler(listEndpoint{path: "/v1/experiments", paged: true,
		strings: map[string]string{"project_id": "projectId", "status": "status", "search": "q", "tag": "tag"},
		bools:   map[string]string{"archived": "archived"},
		enums:   map[string][]string{"status": {"draft", "running", "stopped"}}}),
	"growthbook_get_experiment":             getHandler("/v1/experiments/%s", "experiment_id", nil),
	"growthbook_get_experiment_results":     getExperimentResults,
	"growthbook_refresh_experiment_results": refreshExperimentResults,
	"growthbook_get_snapshot":               getHandler("/v1/snapshots/%s", "snapshot_id", nil),
	"growthbook_list_fact_metrics": listHandler(listEndpoint{path: "/v1/fact-metrics", paged: true,
		strings: map[string]string{"project_id": "projectId", "datasource_id": "datasourceId", "fact_table_id": "factTableId"}}),
	"growthbook_get_fact_metric": getHandler("/v1/fact-metrics/%s", "metric_id", nil),
	"growthbook_list_metrics": listHandler(listEndpoint{path: "/v1/metrics", paged: true,
		strings: map[string]string{"project_id": "projectId", "datasource_id": "datasourceId"}}),
	"growthbook_list_fact_tables": listHandler(listEndpoint{path: "/v1/fact-tables", paged: true,
		strings: map[string]string{"project_id": "projectId", "datasource_id": "datasourceId"}}),
	"growthbook_list_projects":     listHandler(listEndpoint{path: "/v1/projects", paged: true}),
	"growthbook_list_environments": listHandler(listEndpoint{path: "/v1/environments"}),
	"growthbook_list_sdk_connections": listHandler(listEndpoint{path: "/v1/sdk-connections", paged: true,
		strings: map[string]string{"project_id": "projectId"}, scrub: dropSDKConnectionSecrets}),
	"growthbook_list_saved_groups": listHandler(listEndpoint{path: "/v1/saved-groups", paged: true}),
	"growthbook_get_saved_group":   getHandler("/v1/saved-groups/%s", "saved_group_id", nil),
	"growthbook_list_attributes":   listHandler(listEndpoint{path: "/v1/attributes"}),
	"growthbook_list_data_sources": listHandler(listEndpoint{path: "/v1/data-sources", paged: true,
		strings: map[string]string{"project_id": "projectId"}, scrub: dropDataSourceConnections}),
	"growthbook_list_segments": listHandler(listEndpoint{path: "/v1/segments", paged: true,
		strings: map[string]string{"datasource_id": "datasourceId"}}),
	"growthbook_list_audit_logs":          listAuditLogs,
	"growthbook_list_code_refs":           listHandler(listEndpoint{path: "/v1/code-refs", paged: true}),
	"growthbook_get_feature_code_refs":    getHandler("/v1/code-refs/%s", "feature_id", nil),
	"growthbook_get_settings":             listHandler(listEndpoint{path: "/v1/settings", scrub: dropSettingsSecrets}),
	"growthbook_list_members":             listHandler(listEndpoint{path: "/v1/members", paged: true}),
	"growthbook_toggle_feature":           toggleFeature,
	"growthbook_create_feature":           createFeature,
	"growthbook_add_feature_rule":         addFeatureRule,
	"growthbook_update_feature_rule":      updateFeatureRule,
	"growthbook_delete_feature_rule":      deleteFeatureRule,
	"growthbook_publish_feature_revision": publishFeatureRevision,
	"growthbook_discard_feature_revision": discardFeatureRevision,
	"growthbook_create_experiment":        createExperiment,
	"growthbook_update_experiment":        updateExperiment,
	"growthbook_start_experiment":         startExperiment,
	"growthbook_stop_experiment":          stopExperiment,
}

// listEndpoint describes a GET endpoint whose arguments map directly onto
// query parameters.
type listEndpoint struct {
	path    string
	pathArg string
	paged   bool
	strings map[string]string
	bools   map[string]string
	enums   map[string][]string
	scrub   func(map[string]any)
}

func listHandler(endpoint listEndpoint) handlerFunc {
	return func(ctx context.Context, g *growthbook, args map[string]any) (*mcp.ToolResult, error) {
		r := mcp.NewArgs(args)
		query := url.Values{}
		path := endpoint.path
		var pathValue string
		if endpoint.pathArg != "" {
			pathValue = r.Str(endpoint.pathArg)
		}
		for arg, param := range endpoint.strings {
			if value := strings.TrimSpace(r.Str(arg)); value != "" {
				if allowed, ok := endpoint.enums[arg]; ok && !containsString(allowed, value) {
					return mcp.ErrResult(fmt.Errorf("%s must be one of %s", arg, strings.Join(allowed, ", ")))
				}
				query.Set(param, value)
			}
		}
		for arg, param := range endpoint.bools {
			if value, present := args[arg]; present && value != nil {
				query.Set(param, strconv.FormatBool(r.Bool(arg)))
			}
		}
		if endpoint.paged {
			limit, offset, err := pageArgs(r, args)
			if err != nil {
				return mcp.ErrResult(err)
			}
			query.Set("limit", strconv.Itoa(limit))
			query.Set("offset", strconv.Itoa(offset))
		}
		if err := r.Err(); err != nil {
			return mcp.ErrResult(err)
		}
		if endpoint.pathArg != "" {
			escaped, err := pathSegment(endpoint.pathArg, pathValue)
			if err != nil {
				return mcp.ErrResult(err)
			}
			path = fmt.Sprintf(endpoint.path, escaped)
		}
		resp, err := g.get(ctx, path, query)
		if err != nil {
			return mcp.ErrResult(err)
		}
		return documentResult(resp.body, endpoint.scrub)
	}
}

func getHandler(pathFormat, idArg string, scrub func(map[string]any)) handlerFunc {
	return func(ctx context.Context, g *growthbook, args map[string]any) (*mcp.ToolResult, error) {
		r := mcp.NewArgs(args)
		id := r.Str(idArg)
		if err := r.Err(); err != nil {
			return mcp.ErrResult(err)
		}
		escaped, err := pathSegment(idArg, id)
		if err != nil {
			return mcp.ErrResult(err)
		}
		resp, err := g.get(ctx, fmt.Sprintf(pathFormat, escaped), nil)
		if err != nil {
			return mcp.ErrResult(err)
		}
		return documentResult(resp.body, scrub)
	}
}

// documentResult returns the response unchanged unless it needs secrets or
// noise removed, in which case it is parsed once and re-encoded once.
func documentResult(body []byte, scrub func(map[string]any)) (*mcp.ToolResult, error) {
	if scrub == nil {
		return mcp.RawResult(body)
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		return mcp.ErrResult(fmt.Errorf("growthbook: decode response: %w", err))
	}
	scrub(doc)
	return mcp.JSONResult(doc)
}

func pageArgs(r *mcp.Args, args map[string]any) (int, int, error) {
	limit := defaultLimit
	if value, ok := args["limit"]; ok && value != nil {
		limit = r.Int("limit")
		if r.Err() != nil {
			return 0, 0, r.Err()
		}
		if limit < 1 {
			return 0, 0, fmt.Errorf("limit must be between 1 and %d", maxLimit)
		}
		limit = min(limit, maxLimit)
	}
	offset := r.Int("offset")
	if r.Err() != nil {
		return 0, 0, r.Err()
	}
	if offset < 0 {
		return 0, 0, fmt.Errorf("offset must be zero or greater")
	}
	return limit, offset, nil
}

var featureKeyPattern = regexp.MustCompile(`^[a-zA-Z0-9_.:|\-]+$`)

// pathSegment validates an identifier before it becomes one URL path
// segment, so it cannot address a different API route.
func pathSegment(name, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	if value == "." || value == ".." || len(value) > 256 || strings.ContainsAny(value, `/\?#%`) ||
		strings.ContainsFunc(value, unicode.IsControl) {
		return "", fmt.Errorf("%s is not a valid GrowthBook id", name)
	}
	return url.PathEscape(value), nil
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func objectList(doc map[string]any, key string) []map[string]any {
	items, _ := doc[key].([]any)
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if object, ok := item.(map[string]any); ok {
			out = append(out, object)
		}
	}
	return out
}

// dropEnvironmentDefinitions removes the compiled SDK payload string that
// GrowthBook repeats under every environment of a feature flag. It restates
// the rules, and compaction cannot reach keys under the environment map.
func dropEnvironmentDefinitions(doc map[string]any) {
	features := objectList(doc, "features")
	if feature, ok := doc["feature"].(map[string]any); ok {
		features = append(features, feature)
	}
	for _, feature := range features {
		environments, _ := feature["environments"].(map[string]any)
		for _, environment := range environments {
			if settings, ok := environment.(map[string]any); ok {
				delete(settings, "definition")
			}
		}
	}
}

// dropSDKConnectionSecrets removes the payload encryption key and proxy
// signing key. Scripts bypass compaction, so this must happen here.
func dropSDKConnectionSecrets(doc map[string]any) {
	for _, connection := range objectList(doc, "connections") {
		delete(connection, "encryptionKey")
		delete(connection, "proxySigningKey")
		if proxy, ok := connection["proxy"].(map[string]any); ok {
			delete(proxy, "signingKey")
		}
	}
}

// dropDataSourceConnections removes warehouse connection settings in case a
// server version returns them.
func dropDataSourceConnections(doc map[string]any) {
	for _, source := range objectList(doc, "dataSources") {
		delete(source, "params")
		delete(source, "connection")
	}
}

// dropSettingsSecrets removes the salt GrowthBook uses to hash secure
// attributes, which would let a reader reverse hashed targeting values.
func dropSettingsSecrets(doc map[string]any) {
	if settings, ok := doc["settings"].(map[string]any); ok {
		delete(settings, "secureAttributeSalt")
	}
}

func getFeatureStaleness(ctx context.Context, g *growthbook, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	raw := r.StrSlice("feature_ids")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	ids := make([]string, 0, len(raw))
	for _, id := range raw {
		if id = strings.TrimSpace(id); id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return mcp.ErrResult(fmt.Errorf("feature_ids is required"))
	}
	if len(ids) > maxIDs {
		return mcp.ErrResult(fmt.Errorf("feature_ids accepts at most %d keys", maxIDs))
	}
	resp, err := g.get(ctx, "/v2/stale-features", url.Values{"ids": {strings.Join(ids, ",")}})
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(resp.body)
}

func optionalPhase(r *mcp.Args, args map[string]any) (int, bool, error) {
	if value, ok := args["phase"]; !ok || value == nil || value == "" {
		return 0, false, nil
	}
	phase := r.Int("phase")
	if err := r.Err(); err != nil {
		return 0, false, err
	}
	if phase < 0 {
		return 0, false, fmt.Errorf("phase must be zero or greater")
	}
	return phase, true, nil
}

func getExperimentResults(ctx context.Context, g *growthbook, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("experiment_id")
	dimension := strings.TrimSpace(r.Str("dimension"))
	phase, hasPhase, err := optionalPhase(r, args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	escaped, err := pathSegment("experiment_id", id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	query := url.Values{}
	if hasPhase {
		query.Set("phase", strconv.Itoa(phase))
	}
	if dimension != "" {
		query.Set("dimension", dimension)
	}
	resp, err := g.get(ctx, "/v1/experiments/"+escaped+"/results", query)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(resp.body)
}

func refreshExperimentResults(ctx context.Context, g *growthbook, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("experiment_id")
	dimension := strings.TrimSpace(r.Str("dimension"))
	phase, hasPhase, err := optionalPhase(r, args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	escaped, err := pathSegment("experiment_id", id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{}
	if hasPhase {
		body["phase"] = phase
	}
	if dimension != "" {
		body["dimension"] = dimension
	}
	resp, err := g.do(ctx, http.MethodPost, "/v1/experiments/"+escaped+"/snapshot", nil, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if resp.status == http.StatusAccepted {
		return confirmationResult(resp.body)
	}
	return mcp.RawResult(resp.body)
}

func listAuditLogs(ctx context.Context, g *growthbook, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	entityType := strings.TrimSpace(r.Str("entity_type"))
	entityID := strings.TrimSpace(r.Str("entity_id"))
	before := strings.TrimSpace(r.Str("before"))
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if entityType == "" {
		return mcp.ErrResult(fmt.Errorf("entity_type is required"))
	}
	limit := defaultLimit
	if value, ok := args["limit"]; ok && value != nil {
		limit = r.Int("limit")
		if err := r.Err(); err != nil {
			return mcp.ErrResult(err)
		}
		if limit < 1 {
			return mcp.ErrResult(fmt.Errorf("limit must be between 1 and %d", maxLimit))
		}
		limit = min(limit, maxLimit)
	}
	query := url.Values{"entityType": {entityType}, "limit": {strconv.Itoa(limit)}}
	if entityID != "" {
		query.Set("entityId", entityID)
	}
	if before != "" {
		query.Set("before", before)
	}
	resp, err := g.get(ctx, "/v1/audits", query)
	if err != nil {
		return mcp.ErrResult(err)
	}
	return mcp.RawResult(resp.body)
}

// requiredBool reads a boolean that has no safe default.
func requiredBool(args map[string]any, key string) (bool, error) {
	if value, ok := args[key]; !ok || value == nil || value == "" {
		return false, fmt.Errorf("%s is required (true or false)", key)
	}
	return mcp.ArgBool(args, key)
}

func toggleFeature(ctx context.Context, g *growthbook, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("feature_id")
	environment := strings.TrimSpace(r.Str("environment"))
	reason := strings.TrimSpace(r.Str("reason"))
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	escaped, err := pathSegment("feature_id", id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if environment == "" {
		return mcp.ErrResult(fmt.Errorf("environment is required"))
	}
	enabled, err := requiredBool(args, "enabled")
	if err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{"environments": map[string]any{environment: enabled}}
	if reason != "" {
		body["reason"] = reason
	}
	resp, err := g.do(ctx, http.MethodPost, "/v2/features/"+escaped+"/toggle", nil, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if resp.status == http.StatusAccepted {
		return confirmationResult(resp.body)
	}
	var doc struct {
		Feature struct {
			ID           string                    `json:"id"`
			DateUpdated  string                    `json:"dateUpdated"`
			Environments map[string]map[string]any `json:"environments"`
			Revision     struct {
				Version json.Number `json:"version"`
			} `json:"revision"`
		} `json:"feature"`
	}
	if err := json.Unmarshal(resp.body, &doc); err != nil {
		return mcp.ErrResult(fmt.Errorf("growthbook: decode toggle response: %w", err))
	}
	out := map[string]any{
		"status":      "updated",
		"feature_id":  doc.Feature.ID,
		"environment": environment,
	}
	if state, ok := doc.Feature.Environments[environment]["enabled"]; ok {
		out["enabled"] = state
	}
	if doc.Feature.Revision.Version != "" {
		out["version"] = doc.Feature.Revision.Version
	}
	if doc.Feature.DateUpdated != "" {
		out["date_updated"] = doc.Feature.DateUpdated
	}
	return mcp.JSONResult(out)
}

var valueTypes = []string{"boolean", "string", "number", "json"}

func createFeature(ctx context.Context, g *growthbook, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := strings.TrimSpace(r.Str("feature_id"))
	valueType := strings.TrimSpace(r.Str("value_type"))
	defaultValue := r.Str("default_value")
	description := r.Str("description")
	project := strings.TrimSpace(r.Str("project_id"))
	owner := strings.TrimSpace(r.Str("owner"))
	tags := r.StrSlice("tags")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if id == "" {
		return mcp.ErrResult(fmt.Errorf("feature_id is required"))
	}
	if !featureKeyPattern.MatchString(id) || len(id) > 256 {
		return mcp.ErrResult(fmt.Errorf("feature_id may contain only letters, digits and _ . : | -"))
	}
	if !containsString(valueTypes, valueType) {
		return mcp.ErrResult(fmt.Errorf("value_type is required: one of %s", strings.Join(valueTypes, ", ")))
	}
	if _, ok := args["default_value"]; !ok || args["default_value"] == nil {
		return mcp.ErrResult(fmt.Errorf("default_value is required"))
	}

	envResp, err := g.get(ctx, "/v1/environments", nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	var envDoc struct {
		Environments []struct {
			ID string `json:"id"`
		} `json:"environments"`
	}
	if err := json.Unmarshal(envResp.body, &envDoc); err != nil {
		return mcp.ErrResult(fmt.Errorf("growthbook: decode environments: %w", err))
	}
	if len(envDoc.Environments) == 0 {
		return mcp.ErrResult(fmt.Errorf("growthbook: no environments found; cannot create a flag that is disabled everywhere"))
	}
	environments := map[string]any{}
	for _, environment := range envDoc.Environments {
		environments[environment.ID] = map[string]any{"enabled": false}
	}

	body := map[string]any{
		"id":           id,
		"valueType":    valueType,
		"defaultValue": defaultValue,
		"environments": environments,
	}
	if description != "" {
		body["description"] = description
	}
	if project != "" {
		body["project"] = project
	}
	if owner != "" {
		body["owner"] = owner
	}
	cleanTags := make([]string, 0, len(tags))
	for _, tag := range tags {
		if tag = strings.TrimSpace(tag); tag != "" {
			cleanTags = append(cleanTags, tag)
		}
	}
	if len(cleanTags) > 0 {
		body["tags"] = cleanTags
	}
	resp, err := g.do(ctx, http.MethodPost, "/v2/features", nil, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if resp.status == http.StatusAccepted {
		return confirmationResult(resp.body)
	}
	return documentResult(resp.body, dropEnvironmentDefinitions)
}

// confirmationResult reports a write that GrowthBook is holding for a
// person to confirm. Nothing has changed yet, so it must not read as success.
func confirmationResult(body []byte) (*mcp.ToolResult, error) {
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		return mcp.ErrResult(fmt.Errorf("growthbook: decode confirmation: %w", err))
	}
	return mcp.JSONResult(map[string]any{
		"status":       "pending_confirmation",
		"message":      "GrowthBook is holding this change until a person confirms it. It has not been applied. Share the confirmation url with a person and do not retry.",
		"confirmation": doc["confirmation"],
	})
}
