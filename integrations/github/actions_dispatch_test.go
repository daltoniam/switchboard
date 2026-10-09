package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTriggerWorkflow_Inputs(t *testing.T) {
	tests := []struct {
		name   string
		inputs any
		want   map[string]any
	}{
		{name: "no inputs", inputs: nil, want: nil},
		{name: "object", inputs: map[string]any{"mode": "implement", "target": "263"}, want: map[string]any{"mode": "implement", "target": "263"}},
		{name: "json string", inputs: `{"mode":"shepherd","target":"274"}`, want: map[string]any{"mode": "shepherd", "target": "274"}},
		{name: "blank string", inputs: "  ", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var path string
			var body map[string]any
			g, ts := newTestGitHub(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				path = r.URL.Path
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				w.WriteHeader(http.StatusNoContent)
			}))
			defer ts.Close()
			args := map[string]any{"owner": "o", "repo": "r", "workflow_id": "implement.yml", "ref": "main"}
			if tt.inputs != nil {
				args["inputs"] = tt.inputs
			}
			result, err := triggerWorkflow(context.Background(), g, args)
			require.NoError(t, err)
			require.False(t, result.IsError, result.Data)
			assert.Equal(t, "/repos/o/r/actions/workflows/implement.yml/dispatches", path)
			assert.Equal(t, "main", body["ref"])
			got, _ := body["inputs"].(map[string]any)
			if tt.want == nil {
				assert.Empty(t, got)
			} else {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestTriggerWorkflow_InvalidInputs(t *testing.T) {
	tests := []struct {
		name   string
		inputs any
	}{
		{name: "malformed json", inputs: `{"mode":`},
		{name: "json array", inputs: `["implement"]`},
		{name: "wrong type", inputs: 42},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			g, ts := newTestGitHub(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
			defer ts.Close()
			result, err := triggerWorkflow(context.Background(), g, map[string]any{"owner": "o", "repo": "r", "workflow_id": "implement.yml", "ref": "main", "inputs": tt.inputs})
			require.NoError(t, err)
			assert.True(t, result.IsError)
			assert.False(t, called, "no request should be sent")
		})
	}
}
