package dbt

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testManifestJSON is a trimmed dbt manifest.json:
//
//	source.raw.orders -> stg_orders -> int_orders -> mart_orders -> exposure.revenue
//	                                             \-> mart_orders_daily
//	stg_orders also feeds a not_null test; mart_orders uses the cents macro.
const testManifestJSON = `{
  "metadata": {"project_name": "data_dbt", "dbt_version": "1.9.0"},
  "nodes": {
    "model.data_dbt.stg_orders": {"unique_id": "model.data_dbt.stg_orders", "name": "stg_orders", "resource_type": "model", "package_name": "data_dbt", "original_file_path": "models/staging/stg_orders.sql", "patch_path": "data_dbt://models/staging/_staging.yml", "relation_name": "analytics.staging.stg_orders", "config": {"materialized": "view"}, "depends_on": {"macros": []}},
    "model.data_dbt.int_orders": {"unique_id": "model.data_dbt.int_orders", "name": "int_orders", "resource_type": "model", "package_name": "data_dbt", "original_file_path": "models/intermediate/int_orders.sql", "config": {"materialized": "incremental"}, "depends_on": {"macros": []}},
    "model.data_dbt.mart_orders": {"unique_id": "model.data_dbt.mart_orders", "name": "mart_orders", "resource_type": "model", "package_name": "data_dbt", "original_file_path": "models/marts/mart_orders.sql", "config": {"materialized": "table"}, "depends_on": {"macros": ["macro.data_dbt.cents"]}},
    "model.data_dbt.mart_orders_daily": {"unique_id": "model.data_dbt.mart_orders_daily", "name": "mart_orders_daily", "resource_type": "model", "package_name": "data_dbt", "original_file_path": "models/marts/mart_orders_daily.sql", "config": {"materialized": "table"}, "depends_on": {"macros": []}},
    "test.data_dbt.not_null_stg_orders_id.abc": {"unique_id": "test.data_dbt.not_null_stg_orders_id.abc", "name": "not_null_stg_orders_id", "resource_type": "test", "package_name": "data_dbt", "original_file_path": "models/staging/_staging.yml", "config": {"materialized": "test"}, "depends_on": {"macros": []}},
    "model.dbt_utils.util_model": {"unique_id": "model.dbt_utils.util_model", "name": "util_model", "resource_type": "model", "package_name": "dbt_utils", "original_file_path": "models/staging/stg_orders.sql", "config": {"materialized": "view"}, "depends_on": {"macros": []}}
  },
  "sources": {
    "source.data_dbt.raw.orders": {"unique_id": "source.data_dbt.raw.orders", "name": "orders", "resource_type": "source", "package_name": "data_dbt", "original_file_path": "models/staging/_sources.yml"}
  },
  "exposures": {
    "exposure.data_dbt.revenue": {"unique_id": "exposure.data_dbt.revenue", "name": "revenue", "type": "dashboard", "package_name": "data_dbt", "original_file_path": "models/exposures.yml", "url": "https://metabase.example.com/dashboard/1", "owner": {"name": "Finance", "email": "finance@example.com"}}
  },
  "macros": {
    "macro.data_dbt.cents": {"unique_id": "macro.data_dbt.cents", "name": "cents", "package_name": "data_dbt", "original_file_path": "macros/cents.sql"},
    "macro.dbt_utils.cents": {"unique_id": "macro.dbt_utils.cents", "name": "cents", "package_name": "dbt_utils", "original_file_path": "macros/cents.sql"}
  },
  "child_map": {
    "source.data_dbt.raw.orders": ["model.data_dbt.stg_orders"],
    "model.data_dbt.stg_orders": ["model.data_dbt.int_orders", "test.data_dbt.not_null_stg_orders_id.abc"],
    "model.data_dbt.int_orders": ["model.data_dbt.mart_orders", "model.data_dbt.mart_orders_daily"],
    "model.data_dbt.mart_orders": ["exposure.data_dbt.revenue"],
    "model.data_dbt.mart_orders_daily": [],
    "test.data_dbt.not_null_stg_orders_id.abc": [],
    "exposure.data_dbt.revenue": []
  },
  "parent_map": {
    "model.data_dbt.stg_orders": ["source.data_dbt.raw.orders"],
    "model.data_dbt.int_orders": ["model.data_dbt.stg_orders"],
    "model.data_dbt.mart_orders": ["model.data_dbt.int_orders"],
    "model.data_dbt.mart_orders_daily": ["model.data_dbt.int_orders"],
    "test.data_dbt.not_null_stg_orders_id.abc": ["model.data_dbt.stg_orders"],
    "exposure.data_dbt.revenue": ["model.data_dbt.mart_orders"],
    "source.data_dbt.raw.orders": []
  }
}`

func loadTestManifest(t *testing.T) *manifest {
	t.Helper()
	m, err := decodeManifest(strings.NewReader(testManifestJSON))
	require.NoError(t, err)
	return m
}

func TestDecodeManifest_IndexesRootProject(t *testing.T) {
	m := loadTestManifest(t)
	assert.Equal(t, "data_dbt", m.Metadata.ProjectName)
	assert.Len(t, m.Nodes, 6)
	assert.Contains(t, m.ChildMap, "model.data_dbt.stg_orders")
}

func TestDecodeManifest_RejectsInvalidJSON(t *testing.T) {
	_, err := decodeManifest(strings.NewReader("{not json"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode manifest")
}

func TestMatchNodes(t *testing.T) {
	m := loadTestManifest(t)
	tests := []struct {
		name          string
		paths         []string
		names         []string
		wantIDs       []string
		wantUnmatched []string
		wantMacroDeps []string
	}{
		{
			name:    "sql path matches root-project model only",
			paths:   []string{"models/staging/stg_orders.sql"},
			wantIDs: []string{"model.data_dbt.stg_orders"},
		},
		{
			name:    "schema yml matches patched model, its tests, and sources",
			paths:   []string{"models/staging/_staging.yml", "models/staging/_sources.yml"},
			wantIDs: []string{"model.data_dbt.stg_orders", "source.data_dbt.raw.orders", "test.data_dbt.not_null_stg_orders_id.abc"},
		},
		{
			name:          "new file is reported unmatched",
			paths:         []string{"models/marts/mart_new.sql"},
			wantUnmatched: []string{"models/marts/mart_new.sql"},
		},
		{
			name:          "macro path returns dependent models",
			paths:         []string{"macros/cents.sql"},
			wantIDs:       []string{"macro.data_dbt.cents"},
			wantMacroDeps: []string{"model.data_dbt.mart_orders"},
		},
		{
			name:    "name lookup",
			names:   []string{"int_orders"},
			wantIDs: []string{"model.data_dbt.int_orders"},
		},
		{
			name:          "unknown name is unmatched",
			names:         []string{"nope"},
			wantUnmatched: []string{"nope"},
		},
		{
			name:    "leading ./ and windows separators are normalized",
			paths:   []string{"./models\\marts\\mart_orders.sql"},
			wantIDs: []string{"model.data_dbt.mart_orders"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := m.matchNodes(tt.paths, tt.names)
			var ids []string
			for _, n := range res.Nodes {
				ids = append(ids, n.UniqueID)
			}
			assert.ElementsMatch(t, tt.wantIDs, ids)
			assert.ElementsMatch(t, tt.wantUnmatched, res.Unmatched)
			var deps []string
			for _, n := range res.Nodes {
				deps = append(deps, n.MacroDependents...)
			}
			assert.ElementsMatch(t, tt.wantMacroDeps, deps)
		})
	}
}

func TestLineage(t *testing.T) {
	m := loadTestManifest(t)
	tests := []struct {
		name          string
		start         []string
		direction     string
		depth         int
		wantNodes     map[string]int
		wantTests     []string
		wantExposures []string
	}{
		{
			name:      "downstream depth 1",
			start:     []string{"model.data_dbt.stg_orders"},
			direction: "down",
			depth:     1,
			wantNodes: map[string]int{"model.data_dbt.int_orders": 1},
			wantTests: []string{"test.data_dbt.not_null_stg_orders_id.abc"},
		},
		{
			name:      "downstream depth 2",
			start:     []string{"model.data_dbt.stg_orders"},
			direction: "down",
			depth:     2,
			wantNodes: map[string]int{"model.data_dbt.int_orders": 1, "model.data_dbt.mart_orders": 2, "model.data_dbt.mart_orders_daily": 2},
			wantTests: []string{"test.data_dbt.not_null_stg_orders_id.abc"},
		},
		{
			name:          "downstream unlimited reaches exposures",
			start:         []string{"model.data_dbt.stg_orders"},
			direction:     "down",
			depth:         0,
			wantNodes:     map[string]int{"model.data_dbt.int_orders": 1, "model.data_dbt.mart_orders": 2, "model.data_dbt.mart_orders_daily": 2},
			wantTests:     []string{"test.data_dbt.not_null_stg_orders_id.abc"},
			wantExposures: []string{"exposure.data_dbt.revenue"},
		},
		{
			name:      "upstream unlimited",
			start:     []string{"model.data_dbt.mart_orders"},
			direction: "up",
			depth:     0,
			wantNodes: map[string]int{"model.data_dbt.int_orders": 1, "model.data_dbt.stg_orders": 2, "source.data_dbt.raw.orders": 3},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := m.lineage(tt.start, tt.direction, tt.depth, 500)
			got := map[string]int{}
			for _, n := range res.Nodes {
				got[n.UniqueID] = n.Depth
			}
			assert.Equal(t, tt.wantNodes, got)
			assert.ElementsMatch(t, tt.wantTests, res.Tests)
			var exp []string
			for _, e := range res.Exposures {
				exp = append(exp, e.UniqueID)
			}
			assert.ElementsMatch(t, tt.wantExposures, exp)
			assert.False(t, res.Truncated)
		})
	}
}

func TestLineage_TruncatesAtLimit(t *testing.T) {
	m := loadTestManifest(t)
	res := m.lineage([]string{"model.data_dbt.stg_orders"}, "down", 0, 1)
	assert.Len(t, res.Nodes, 1)
	assert.True(t, res.Truncated)
}

func TestResolveIDs(t *testing.T) {
	m := loadTestManifest(t)
	ids, unmatched := m.resolveIDs([]string{"model.data_dbt.int_orders"}, []string{"mart_orders", "ghost"})
	assert.ElementsMatch(t, []string{"model.data_dbt.int_orders", "model.data_dbt.mart_orders"}, ids)
	assert.Equal(t, []string{"ghost"}, unmatched)
}

func TestManifestCache_EvictsOldest(t *testing.T) {
	c := newManifestCache(2)
	a, b, d := &manifest{}, &manifest{}, &manifest{}
	c.put("a", a)
	c.put("b", b)
	got, ok := c.get("a")
	require.True(t, ok)
	assert.Same(t, a, got)
	c.put("d", d) // evicts b: a was touched more recently
	_, ok = c.get("b")
	assert.False(t, ok)
	_, ok = c.get("a")
	assert.True(t, ok)
	_, ok = c.get("d")
	assert.True(t, ok)
}
