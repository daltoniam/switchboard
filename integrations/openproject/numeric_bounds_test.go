package openproject

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
	"testing"
)

func TestGetRejectsRoundedJSONIDBeforeNetwork(t *testing.T) {
	var calls atomic.Int32
	integration, _ := fixture(t, func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		http.Error(w, "unexpected request", http.StatusBadRequest)
	})
	var args map[string]any
	if err := json.Unmarshal([]byte(`{"id":9007199254740993}`), &args); err != nil {
		t.Fatal(err)
	}
	result := execute(t, integration, "openproject_get_work_package", args)
	if !result.IsError || calls.Load() != 0 {
		t.Fatalf("rounded numeric ID reached upstream: result=%+v calls=%d", result, calls.Load())
	}
}

func TestPositiveIntegerPreservesExactLargeIDs(t *testing.T) {
	for _, value := range []any{int64(9007199254740993), json.Number("9007199254740993"), "9007199254740993"} {
		t.Run(stringType(value), func(t *testing.T) {
			got, err := positiveInteger(map[string]any{"id": value}, "id", 0)
			if err != nil || got != 9007199254740993 {
				t.Fatalf("exact large ID changed: got=%d err=%v", got, err)
			}
		})
	}
}

func stringType(value any) string {
	switch value.(type) {
	case int64:
		return "int64"
	case json.Number:
		return "json.Number"
	default:
		return "string"
	}
}
