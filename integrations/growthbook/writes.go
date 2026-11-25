package growthbook

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"strconv"
	"strings"

	mcp "github.com/daltoniam/switchboard"
)

// Rule and experiment writes. Rule changes always go through a draft
// revision, and nothing reaches an SDK until growthbook_publish_feature_revision
// (or growthbook_start_experiment) is called, so an agent can stage a change,
// show it, and publish it as a separate, deliberate step. Override flags that
// skip GrowthBook's own gates (warnings, schema validation, hooks, the launch
// checklist, approvals) are never sent: a blocked write comes back as an
// error for a person to resolve in GrowthBook.

var ruleTypes = []string{"force", "rollout", "experiment-ref"}

var stopResults = []string{"won", "lost", "inconclusive", "dnf"}

// gateOverrides are body fields that skip GrowthBook's publishing gates.
var gateOverrides = []string{
	"ignoreWarnings", "skipSchemaValidation", "skipHooks", "skipChecklist",
	"bypassApproval", "bypassDuplicateKeyCheck", "allowUnscopedRules",
}

// objectArg reads a JSON object passed either as an object or as a JSON
// string, which is how many MCP clients send nested values.
func objectArg(args map[string]any, key string) (map[string]any, error) {
	value, ok := args[key]
	if !ok || value == nil || value == "" {
		return nil, fmt.Errorf("%s is required (a JSON object)", key)
	}
	switch typed := value.(type) {
	case map[string]any:
		return maps.Clone(typed), nil
	case string:
		var object map[string]any
		if err := json.Unmarshal([]byte(typed), &object); err != nil || object == nil {
			return nil, fmt.Errorf("%s must be a JSON object", key)
		}
		return object, nil
	}
	return nil, fmt.Errorf("%s must be a JSON object, not %T", key, value)
}

func rejectOverrides(object map[string]any, name string, extra ...string) error {
	for _, key := range append(append([]string{}, gateOverrides...), extra...) {
		if _, ok := object[key]; ok {
			return fmt.Errorf("%s.%s is not accepted: a person must resolve GrowthBook's gates in the app", name, key)
		}
	}
	return nil
}

// draftVersion reads a draft revision version: a positive integer.
func draftVersion(args map[string]any) (int, error) {
	if value, ok := args["version"]; !ok || value == nil || value == "" {
		return 0, fmt.Errorf("version is required (the draft revision version)")
	}
	version, err := mcp.ArgInt(args, "version")
	if err != nil {
		return 0, err
	}
	if version < 1 {
		return 0, fmt.Errorf("version must be a positive draft revision version")
	}
	return version, nil
}

// ruleScopeSet reports whether a rule says which environments it targets.
func ruleScopeSet(rule map[string]any) bool {
	if all, ok := rule["allEnvironments"].(bool); ok && all {
		return true
	}
	environments, _ := rule["environments"].([]any)
	return len(environments) > 0
}

func revisionPath(featureID string, version string) (string, error) {
	escaped, err := pathSegment("feature_id", featureID)
	if err != nil {
		return "", err
	}
	return "/v2/features/" + escaped + "/revisions/" + version, nil
}

// revisionResult summarizes a draft after an edit: its version, status and
// full rule list (rule ids included), and says it is not live yet.
func revisionResult(featureID string, body []byte, message string) (*mcp.ToolResult, error) {
	var doc struct {
		Revision map[string]any `json:"revision"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return mcp.ErrResult(fmt.Errorf("growthbook: decode revision: %w", err))
	}
	out := map[string]any{"feature_id": featureID, "message": message}
	for _, key := range []string{"version", "status", "baseVersion", "title", "comment", "rules", "environmentsEnabled", "defaultValue"} {
		if value, ok := doc.Revision[key]; ok {
			out[key] = value
		}
	}
	return mcp.JSONResult(out)
}

const draftMessage = "Saved to the draft revision only; nothing is live. Review it, then call growthbook_publish_feature_revision with this version."

func addFeatureRule(ctx context.Context, g *growthbook, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	featureID := r.Str("feature_id")
	title := strings.TrimSpace(r.Str("revision_title"))
	comment := strings.TrimSpace(r.Str("revision_comment"))
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if _, err := pathSegment("feature_id", featureID); err != nil {
		return mcp.ErrResult(err)
	}
	rule, err := objectArg(args, "rule")
	if err != nil {
		return mcp.ErrResult(err)
	}
	ruleType, _ := rule["type"].(string)
	if ruleType == "" {
		ruleType = "force"
		rule["type"] = ruleType
	}
	if !containsString(ruleTypes, ruleType) {
		return mcp.ErrResult(fmt.Errorf("rule.type must be one of %s", strings.Join(ruleTypes, ", ")))
	}
	if !ruleScopeSet(rule) {
		return mcp.ErrResult(fmt.Errorf("rule must name its environments (environments: [\"dev\"]) or set allEnvironments: true, so production is never targeted by accident"))
	}
	if err := rejectOverrides(rule, "rule", "rampSchedule", "schedule"); err != nil {
		return mcp.ErrResult(err)
	}
	version := "new"
	if value, ok := args["version"]; ok && value != nil && value != "" {
		v, err := draftVersion(args)
		if err != nil {
			return mcp.ErrResult(err)
		}
		version = strconv.Itoa(v)
	}
	path, err := revisionPath(featureID, version)
	if err != nil {
		return mcp.ErrResult(err)
	}
	body := map[string]any{"rule": rule}
	if version == "new" {
		if title != "" {
			body["revisionTitle"] = title
		}
		if comment != "" {
			body["revisionComment"] = comment
		}
	}
	resp, err := g.do(ctx, http.MethodPost, path+"/rules", nil, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if resp.status == http.StatusAccepted {
		return confirmationResult(resp.body)
	}
	return revisionResult(strings.TrimSpace(featureID), resp.body, draftMessage)
}

func updateFeatureRule(ctx context.Context, g *growthbook, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	featureID := r.Str("feature_id")
	ruleID := r.Str("rule_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if _, err := pathSegment("feature_id", featureID); err != nil {
		return mcp.ErrResult(err)
	}
	version, err := draftVersion(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	escapedRule, err := pathSegment("rule_id", ruleID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	patch, err := objectArg(args, "rule")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if len(patch) == 0 {
		return mcp.ErrResult(fmt.Errorf("rule must change at least one field"))
	}
	if _, ok := patch["type"]; ok {
		return mcp.ErrResult(fmt.Errorf("rule.type cannot be changed; delete the rule and add a new one"))
	}
	if err := rejectOverrides(patch, "rule", "rampSchedule", "schedule"); err != nil {
		return mcp.ErrResult(err)
	}
	path, err := revisionPath(featureID, strconv.Itoa(version))
	if err != nil {
		return mcp.ErrResult(err)
	}
	resp, err := g.do(ctx, http.MethodPut, path+"/rules/"+escapedRule, nil, map[string]any{"rule": patch})
	if err != nil {
		return mcp.ErrResult(err)
	}
	if resp.status == http.StatusAccepted {
		return confirmationResult(resp.body)
	}
	return revisionResult(strings.TrimSpace(featureID), resp.body, draftMessage)
}

func deleteFeatureRule(ctx context.Context, g *growthbook, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	featureID := r.Str("feature_id")
	ruleID := r.Str("rule_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if _, err := pathSegment("feature_id", featureID); err != nil {
		return mcp.ErrResult(err)
	}
	version, err := draftVersion(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	escapedRule, err := pathSegment("rule_id", ruleID)
	if err != nil {
		return mcp.ErrResult(err)
	}
	path, err := revisionPath(featureID, strconv.Itoa(version))
	if err != nil {
		return mcp.ErrResult(err)
	}
	resp, err := g.do(ctx, http.MethodDelete, path+"/rules/"+escapedRule, nil, nil)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if resp.status == http.StatusAccepted {
		return confirmationResult(resp.body)
	}
	return revisionResult(strings.TrimSpace(featureID), resp.body, draftMessage)
}

func publishFeatureRevision(ctx context.Context, g *growthbook, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	featureID := r.Str("feature_id")
	comment := strings.TrimSpace(r.Str("comment"))
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if _, err := pathSegment("feature_id", featureID); err != nil {
		return mcp.ErrResult(err)
	}
	version, err := draftVersion(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if comment == "" {
		return mcp.ErrResult(fmt.Errorf("comment is required: what this publish changes and why, for the audit log"))
	}
	path, err := revisionPath(featureID, strconv.Itoa(version))
	if err != nil {
		return mcp.ErrResult(err)
	}
	resp, err := g.do(ctx, http.MethodPost, path+"/publish", nil, map[string]any{"comment": comment})
	if err != nil {
		return mcp.ErrResult(err)
	}
	if resp.status == http.StatusAccepted {
		return confirmationResult(resp.body)
	}
	var doc struct {
		Revision      map[string]any `json:"revision"`
		BypassedGates any            `json:"bypassedGates"`
	}
	if err := json.Unmarshal(resp.body, &doc); err != nil {
		return mcp.ErrResult(fmt.Errorf("growthbook: decode publish response: %w", err))
	}
	out := map[string]any{
		"status":     "published",
		"feature_id": strings.TrimSpace(featureID),
		"version":    doc.Revision["version"],
		"message":    "The revision is live: SDKs serve it from their next refresh.",
	}
	if doc.BypassedGates != nil {
		out["bypassed_gates"] = doc.BypassedGates
	}
	return mcp.JSONResult(out)
}

func discardFeatureRevision(ctx context.Context, g *growthbook, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	featureID := r.Str("feature_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	if _, err := pathSegment("feature_id", featureID); err != nil {
		return mcp.ErrResult(err)
	}
	version, err := draftVersion(args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	path, err := revisionPath(featureID, strconv.Itoa(version))
	if err != nil {
		return mcp.ErrResult(err)
	}
	resp, err := g.do(ctx, http.MethodPost, path+"/discard", nil, map[string]any{})
	if err != nil {
		return mcp.ErrResult(err)
	}
	if resp.status == http.StatusAccepted {
		return confirmationResult(resp.body)
	}
	return revisionResult(strings.TrimSpace(featureID), resp.body, "The draft is discarded; the live revision is unchanged.")
}

func createExperiment(ctx context.Context, g *growthbook, args map[string]any) (*mcp.ToolResult, error) {
	experiment, err := objectArg(args, "experiment")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if err := rejectOverrides(experiment, "experiment"); err != nil {
		return mcp.ErrResult(err)
	}
	if status, ok := experiment["status"]; ok && status != "draft" {
		return mcp.ErrResult(fmt.Errorf("experiment.status must be draft (or omitted); start it with growthbook_start_experiment"))
	}
	experiment["status"] = "draft"
	for _, key := range []string{"trackingKey", "name"} {
		if value, _ := experiment[key].(string); strings.TrimSpace(value) == "" {
			return mcp.ErrResult(fmt.Errorf("experiment.%s is required", key))
		}
	}
	if variations, _ := experiment["variations"].([]any); len(variations) < 2 {
		return mcp.ErrResult(fmt.Errorf("experiment.variations needs at least two variations, each with key and name"))
	}
	resp, err := g.do(ctx, http.MethodPost, "/v1/experiments", nil, experiment)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if resp.status == http.StatusAccepted {
		return confirmationResult(resp.body)
	}
	return experimentResult(resp.body, "Created as a draft; nothing is served. Link it to a flag with an experiment-ref rule, then growthbook_start_experiment.")
}

func updateExperiment(ctx context.Context, g *growthbook, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("experiment_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	escaped, err := pathSegment("experiment_id", id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	changes, err := objectArg(args, "changes")
	if err != nil {
		return mcp.ErrResult(err)
	}
	if len(changes) == 0 {
		return mcp.ErrResult(fmt.Errorf("changes must set at least one field"))
	}
	if err := rejectOverrides(changes, "changes"); err != nil {
		return mcp.ErrResult(err)
	}
	for _, key := range []string{"status", "linkedChanges"} {
		if _, ok := changes[key]; ok {
			return mcp.ErrResult(fmt.Errorf("changes.%s is not accepted; use growthbook_start_experiment or growthbook_stop_experiment", key))
		}
	}
	resp, err := g.do(ctx, http.MethodPost, "/v1/experiments/"+escaped, nil, changes)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if resp.status == http.StatusAccepted {
		return confirmationResult(resp.body)
	}
	return experimentResult(resp.body, "Updated. A running experiment serves the change from the SDKs' next refresh.")
}

func startExperiment(ctx context.Context, g *growthbook, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("experiment_id")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	escaped, err := pathSegment("experiment_id", id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	resp, err := g.do(ctx, http.MethodPost, "/v1/experiments/"+escaped+"/start", nil, map[string]any{})
	if err != nil {
		return mcp.ErrResult(err)
	}
	if resp.status == http.StatusAccepted {
		return confirmationResult(resp.body)
	}
	return experimentResult(resp.body, "Started: users matching its targeting are assigned from the SDKs' next refresh.")
}

func stopExperiment(ctx context.Context, g *growthbook, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	id := r.Str("experiment_id")
	results := strings.TrimSpace(r.Str("results"))
	analysis := strings.TrimSpace(r.Str("analysis"))
	reason := strings.TrimSpace(r.Str("reason"))
	winner := strings.TrimSpace(r.Str("winner_variation_id"))
	released := strings.TrimSpace(r.Str("released_variation_id"))
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	escaped, err := pathSegment("experiment_id", id)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if !containsString(stopResults, results) {
		return mcp.ErrResult(fmt.Errorf("results is required: one of %s", strings.Join(stopResults, ", ")))
	}
	body := map[string]any{"results": results}
	if value, ok := args["enable_temporary_rollout"]; ok && value != nil && value != "" {
		rollout, err := mcp.ArgBool(args, "enable_temporary_rollout")
		if err != nil {
			return mcp.ErrResult(err)
		}
		if rollout && released == "" {
			return mcp.ErrResult(fmt.Errorf("released_variation_id is required to roll a variation out to everyone"))
		}
		body["enableTemporaryRollout"] = rollout
	}
	for key, value := range map[string]string{"analysis": analysis, "reason": reason, "winnerVariationId": winner, "releasedVariationId": released} {
		if value != "" {
			body[key] = value
		}
	}
	resp, err := g.do(ctx, http.MethodPost, "/v1/experiments/"+escaped+"/stop", nil, body)
	if err != nil {
		return mcp.ErrResult(err)
	}
	if resp.status == http.StatusAccepted {
		return confirmationResult(resp.body)
	}
	return experimentResult(resp.body, "Stopped: no new assignments from the SDKs' next refresh, unless a variation was rolled out.")
}

// experimentResult returns what an experiment write changed: its identity,
// status, variations and latest phase, plus any feature drafts GrowthBook
// published with it.
func experimentResult(body []byte, message string) (*mcp.ToolResult, error) {
	var doc struct {
		Experiment             map[string]any `json:"experiment"`
		PublishedFeatureDrafts any            `json:"publishedFeatureDrafts"`
		BypassedGates          any            `json:"bypassedGates"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return mcp.ErrResult(fmt.Errorf("growthbook: decode experiment: %w", err))
	}
	out := map[string]any{"message": message}
	for _, key := range []string{"id", "trackingKey", "name", "status", "project", "hashAttribute", "variations", "resultSummary", "linkedFeatures"} {
		if value, ok := doc.Experiment[key]; ok {
			out[key] = value
		}
	}
	if phases, _ := doc.Experiment["phases"].([]any); len(phases) > 0 {
		out["latestPhase"] = phases[len(phases)-1]
	}
	if doc.PublishedFeatureDrafts != nil {
		out["publishedFeatureDrafts"] = doc.PublishedFeatureDrafts
	}
	if doc.BypassedGates != nil {
		out["bypassedGates"] = doc.BypassedGates
	}
	return mcp.JSONResult(out)
}
