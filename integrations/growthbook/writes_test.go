package growthbook

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const revisionBody = `{"revision":{"featureId":"f1","version":4,"baseVersion":3,"status":"draft","title":"Pittsburgh","definitions":{"production":"{}"},
"rules":[{"id":"fr_1","type":"force","value":"true","condition":"{\"cbsa_name\":\"38300\"}","environments":["production"]}]}}`

func decodeResult(t *testing.T, data string) map[string]any {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(data), &out))
	return out
}

func TestAddFeatureRule_NewDraft(t *testing.T) {
	adapter, requests := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, revisionBody)
	})
	rule := `{"type":"force","value":"true","condition":"{\"cbsa_name\":\"38300\"}","environments":["production"],"description":"Pittsburgh"}`
	result, err := adapter.Execute(context.Background(), "growthbook_add_feature_rule", map[string]any{
		"feature_id": "f1", "rule": rule, "revision_title": "Pittsburgh", "revision_comment": "First market",
	})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	got := requests.list()[0]
	assert.Equal(t, http.MethodPost, got.Method)
	assert.Equal(t, "/api/v2/features/f1/revisions/new/rules", got.Path)
	assert.Equal(t, map[string]any{
		"rule":          map[string]any{"type": "force", "value": "true", "condition": `{"cbsa_name":"38300"}`, "environments": []any{"production"}, "description": "Pittsburgh"},
		"revisionTitle": "Pittsburgh", "revisionComment": "First market",
	}, got.Body)
	out := decodeResult(t, result.Data)
	assert.EqualValues(t, 4, out["version"])
	assert.Equal(t, "draft", out["status"])
	assert.Contains(t, out["message"], "nothing is live")
	assert.NotContains(t, result.Data, "definitions", "compiled SDK payloads stay out of the result")
	assert.Contains(t, result.Data, "fr_1")
}

func TestAddFeatureRule_ExistingDraftDefaultsToForce(t *testing.T) {
	adapter, requests := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, revisionBody)
	})
	result, err := adapter.Execute(context.Background(), "growthbook_add_feature_rule", map[string]any{
		"feature_id": "f1", "version": 4, "revision_title": "ignored on an existing draft",
		"rule": map[string]any{"value": "true", "allEnvironments": true},
	})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	got := requests.list()[0]
	assert.Equal(t, "/api/v2/features/f1/revisions/4/rules", got.Path)
	assert.Equal(t, map[string]any{"rule": map[string]any{"type": "force", "value": "true", "allEnvironments": true}}, got.Body)
}

func TestAddFeatureRule_Validation(t *testing.T) {
	adapter, requests := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, revisionBody)
	})
	for name, tc := range map[string]struct {
		args map[string]any
		want string
	}{
		"no rule":            {map[string]any{"feature_id": "f1"}, "rule is required"},
		"rule not object":    {map[string]any{"feature_id": "f1", "rule": "[1]"}, "JSON object"},
		"no scope":           {map[string]any{"feature_id": "f1", "rule": `{"value":"true"}`}, "environments"},
		"empty environments": {map[string]any{"feature_id": "f1", "rule": `{"value":"true","environments":[]}`}, "environments"},
		"bad type":           {map[string]any{"feature_id": "f1", "rule": `{"type":"safe-rollout","allEnvironments":true}`}, "rule.type"},
		"override flag":      {map[string]any{"feature_id": "f1", "rule": `{"value":"true","allEnvironments":true,"skipSchemaValidation":true}`}, "skipSchemaValidation"},
		"ramp schedule":      {map[string]any{"feature_id": "f1", "rule": `{"value":"true","allEnvironments":true,"rampSchedule":{}}`}, "rampSchedule"},
		"bad version":        {map[string]any{"feature_id": "f1", "version": 0, "rule": `{"value":"true","allEnvironments":true}`}, "version"},
		"traversal":          {map[string]any{"feature_id": "../v1/members", "rule": `{"value":"true","allEnvironments":true}`}, "feature_id"},
	} {
		t.Run(name, func(t *testing.T) {
			result, err := adapter.Execute(context.Background(), "growthbook_add_feature_rule", tc.args)
			require.NoError(t, err)
			assert.True(t, result.IsError)
			assert.Contains(t, result.Data, tc.want)
		})
	}
	assert.Empty(t, requests.list())
}

func TestUpdateAndDeleteFeatureRule(t *testing.T) {
	adapter, requests := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, revisionBody)
	})
	result, err := adapter.Execute(context.Background(), "growthbook_update_feature_rule", map[string]any{
		"feature_id": "f1", "version": "4", "rule_id": "fr_1", "rule": map[string]any{"condition": `{"cbsa_name":{"$in":["38300","19100"]}}`},
	})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	result, err = adapter.Execute(context.Background(), "growthbook_delete_feature_rule", map[string]any{"feature_id": "f1", "version": 4, "rule_id": "fr_1"})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	got := requests.list()
	require.Len(t, got, 2)
	assert.Equal(t, http.MethodPut, got[0].Method)
	assert.Equal(t, "/api/v2/features/f1/revisions/4/rules/fr_1", got[0].Path)
	assert.Equal(t, map[string]any{"rule": map[string]any{"condition": `{"cbsa_name":{"$in":["38300","19100"]}}`}}, got[0].Body)
	assert.Equal(t, http.MethodDelete, got[1].Method)
	assert.Equal(t, "/api/v2/features/f1/revisions/4/rules/fr_1", got[1].Path)

	for _, args := range []map[string]any{
		{"feature_id": "f1", "version": 4, "rule_id": "fr_1", "rule": map[string]any{"type": "rollout"}},
		{"feature_id": "f1", "version": 4, "rule_id": "fr_1", "rule": map[string]any{}},
		{"feature_id": "f1", "version": 4, "rule_id": "fr_1", "rule": map[string]any{"ignoreWarnings": true}},
		{"feature_id": "f1", "rule_id": "fr_1", "rule": map[string]any{"enabled": false}},
		{"feature_id": "f1", "version": 4, "rule_id": "a/b", "rule": map[string]any{"enabled": false}},
	} {
		result, err := adapter.Execute(context.Background(), "growthbook_update_feature_rule", args)
		require.NoError(t, err)
		assert.True(t, result.IsError, "%v", args)
	}
	assert.Len(t, requests.list(), 2)
}

func TestPublishFeatureRevision(t *testing.T) {
	adapter, requests := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"revision":{"version":4,"status":"published"},"bypassedGates":[{"type":"approval","via":"restApiBypassesReviews"}]}`)
	})
	result, err := adapter.Execute(context.Background(), "growthbook_publish_feature_revision", map[string]any{"feature_id": "f1", "version": 4, "comment": "Pittsburgh on the engine"})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	got := requests.list()[0]
	assert.Equal(t, "/api/v2/features/f1/revisions/4/publish", got.Path)
	assert.Equal(t, map[string]any{"comment": "Pittsburgh on the engine"}, got.Body, "no gate overrides are ever sent")
	out := decodeResult(t, result.Data)
	assert.Equal(t, "published", out["status"])
	assert.EqualValues(t, 4, out["version"])
	assert.NotNil(t, out["bypassed_gates"])

	result, err = adapter.Execute(context.Background(), "growthbook_publish_feature_revision", map[string]any{"feature_id": "f1", "version": 4})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "comment is required")
	assert.Len(t, requests.list(), 1)
}

func TestPublishFeatureRevision_GateIsAnError(t *testing.T) {
	adapter, _ := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = io.WriteString(w, `{"message":"Approval required","gates":[{"type":"approval","override":"bypassApproval"}]}`)
	})
	result, err := adapter.Execute(context.Background(), "growthbook_publish_feature_revision", map[string]any{"feature_id": "f1", "version": 4, "comment": "go"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Data, "Approval required")
	assert.Contains(t, result.Data, "gates")
}

func TestDiscardFeatureRevision(t *testing.T) {
	adapter, requests := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"revision":{"version":4,"status":"discarded"}}`)
	})
	result, err := adapter.Execute(context.Background(), "growthbook_discard_feature_revision", map[string]any{"feature_id": "f1", "version": 4})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	assert.Equal(t, "/api/v2/features/f1/revisions/4/discard", requests.list()[0].Path)
	assert.Equal(t, "discarded", decodeResult(t, result.Data)["status"])
}

const experimentBody = `{"experiment":{"id":"exp_1","trackingKey":"fe-go","name":"Engine","status":"draft","project":"prj_1",
"variations":[{"variationId":"var_0","key":"0","name":"TS"},{"variationId":"var_1","key":"1","name":"Go"}],
"phases":[{"name":"Old"},{"name":"Main","coverage":0.1}],"settings":{"big":"noise"}},"publishedFeatureDrafts":[{"featureId":"f1","version":5}]}`

func TestCreateExperiment(t *testing.T) {
	adapter, requests := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, experimentBody)
	})
	experiment := `{"trackingKey":"fe-go","name":"Engine","project":"prj_1","hashAttribute":"fulfillment_request_id",
"variations":[{"key":"0","name":"TS"},{"key":"1","name":"Go"}]}`
	result, err := adapter.Execute(context.Background(), "growthbook_create_experiment", map[string]any{"experiment": experiment})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	got := requests.list()[0]
	assert.Equal(t, http.MethodPost, got.Method)
	assert.Equal(t, "/api/v1/experiments", got.Path)
	assert.Equal(t, "draft", got.Body["status"], "created experiments are always drafts")
	out := decodeResult(t, result.Data)
	assert.Equal(t, "exp_1", out["id"])
	assert.Equal(t, map[string]any{"name": "Main", "coverage": 0.1}, out["latestPhase"])
	assert.NotContains(t, result.Data, "noise")

	for name, experiment := range map[string]string{
		"running":        `{"trackingKey":"k","name":"n","status":"running","variations":[{"key":"0","name":"a"},{"key":"1","name":"b"}]}`,
		"no key":         `{"name":"n","variations":[{"key":"0","name":"a"},{"key":"1","name":"b"}]}`,
		"one variation":  `{"trackingKey":"k","name":"n","variations":[{"key":"0","name":"a"}]}`,
		"skip checklist": `{"trackingKey":"k","name":"n","skipChecklist":true,"variations":[{"key":"0","name":"a"},{"key":"1","name":"b"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			result, err := adapter.Execute(context.Background(), "growthbook_create_experiment", map[string]any{"experiment": experiment})
			require.NoError(t, err)
			assert.True(t, result.IsError)
		})
	}
	assert.Len(t, requests.list(), 1)
}

func TestUpdateStartStopExperiment(t *testing.T) {
	adapter, requests := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, experimentBody)
	})
	result, err := adapter.Execute(context.Background(), "growthbook_update_experiment", map[string]any{"experiment_id": "exp_1", "changes": map[string]any{"hypothesis": "Go assigns faster"}})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	result, err = adapter.Execute(context.Background(), "growthbook_start_experiment", map[string]any{"experiment_id": "exp_1"})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	assert.Contains(t, result.Data, "publishedFeatureDrafts")
	result, err = adapter.Execute(context.Background(), "growthbook_stop_experiment", map[string]any{
		"experiment_id": "exp_1", "results": "won", "winner_variation_id": "var_1", "released_variation_id": "var_1",
		"enable_temporary_rollout": true, "analysis": "Go won", "reason": "Significant",
	})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Data)
	got := requests.list()
	require.Len(t, got, 3)
	assert.Equal(t, "/api/v1/experiments/exp_1", got[0].Path)
	assert.Equal(t, map[string]any{"hypothesis": "Go assigns faster"}, got[0].Body)
	assert.Equal(t, "/api/v1/experiments/exp_1/start", got[1].Path)
	assert.Empty(t, got[1].Body, "the launch checklist and approvals are never skipped")
	assert.Equal(t, "/api/v1/experiments/exp_1/stop", got[2].Path)
	assert.Equal(t, map[string]any{"results": "won", "winnerVariationId": "var_1", "releasedVariationId": "var_1", "enableTemporaryRollout": true, "analysis": "Go won", "reason": "Significant"}, got[2].Body)

	for tool, args := range map[string]map[string]any{
		"status change":           {"tool": "growthbook_update_experiment", "experiment_id": "exp_1", "changes": map[string]any{"status": "running"}},
		"override":                {"tool": "growthbook_update_experiment", "experiment_id": "exp_1", "changes": map[string]any{"ignoreWarnings": true}},
		"no results":              {"tool": "growthbook_stop_experiment", "experiment_id": "exp_1"},
		"bad results":             {"tool": "growthbook_stop_experiment", "experiment_id": "exp_1", "results": "great"},
		"rollout without release": {"tool": "growthbook_stop_experiment", "experiment_id": "exp_1", "results": "won", "enable_temporary_rollout": true},
		"start traversal":         {"tool": "growthbook_start_experiment", "experiment_id": "../members"},
	} {
		t.Run(tool, func(t *testing.T) {
			name, _ := args["tool"].(string)
			delete(args, "tool")
			result, err := adapter.Execute(context.Background(), toolName(name), args)
			require.NoError(t, err)
			assert.True(t, result.IsError)
		})
	}
	assert.Len(t, requests.list(), 3)
}

func TestRuleAndExperimentWrites_Confirmation(t *testing.T) {
	adapter, _ := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(w, `{"confirmation":{"id":"cnf_1","status":"pending","url":"https://app.growthbook.io/confirm/cnf_1"}}`)
	})
	for tool, args := range map[string]map[string]any{
		"growthbook_publish_feature_revision": {"feature_id": "f1", "version": 4, "comment": "go"},
		"growthbook_start_experiment":         {"experiment_id": "exp_1"},
		"growthbook_add_feature_rule":         {"feature_id": "f1", "rule": map[string]any{"value": "true", "allEnvironments": true}},
	} {
		result, err := adapter.Execute(context.Background(), toolName(tool), args)
		require.NoError(t, err)
		require.False(t, result.IsError, result.Data)
		assert.Equal(t, "pending_confirmation", decodeResult(t, result.Data)["status"], tool)
	}
}

func toolName(name string) mcp.ToolName { return mcp.ToolName(name) }
