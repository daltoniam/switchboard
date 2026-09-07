package servicenow

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/compact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestFieldCompactionSpecs_AllParse(t *testing.T) {
	require.NotEmpty(t, fieldCompactionSpecs, "fieldCompactionSpecs should not be empty")
}

func TestFieldCompactionSpecs_NoDuplicateTools(t *testing.T) {
	var sf compact.SpecFile
	require.NoError(t, yaml.Unmarshal(compactYAML, &sf))
	assert.Equal(t, len(sf.Tools), len(fieldCompactionSpecs))
}

func TestFieldCompactionSpecs_NoOrphanSpecs(t *testing.T) {
	for toolName := range fieldCompactionSpecs {
		_, ok := dispatch[toolName]
		assert.True(t, ok, "field compaction spec for %q has no dispatch handler", toolName)
	}
}

func TestFieldCompactionSpec_ReturnsFieldsForListTool(t *testing.T) {
	s := &servicenow{}
	fields, ok := s.CompactSpec("servicenow_list_incidents")
	require.True(t, ok, "servicenow_list_incidents should have field compaction spec")
	assert.NotEmpty(t, fields)
}

func TestFieldCompactionSpec_ReturnsFalseForUnknownTool(t *testing.T) {
	s := &servicenow{}
	_, ok := s.CompactSpec("servicenow_nonexistent")
	assert.False(t, ok, "unknown tools should return false")
}

func TestFieldCompactionSpec_ReturnsFalseForMutationTool(t *testing.T) {
	s := &servicenow{}
	_, ok := s.CompactSpec("servicenow_create_incident")
	assert.False(t, ok, "mutation tools should not have compaction specs")
}

func TestFieldCompactionSpec_GenericGetHasMaxBytes(t *testing.T) {
	s := &servicenow{}
	n, ok := s.MaxBytes("servicenow_get_record")
	require.True(t, ok, "servicenow_get_record should cap response size")
	assert.Equal(t, 100000, n)
	n, ok = s.MaxBytes("servicenow_aggregate")
	require.True(t, ok, "servicenow_aggregate should cap response size")
	assert.Equal(t, 50000, n)
}

func TestFieldCompactionSpecs_ShapeParity(t *testing.T) {
	handlerOutputs := map[string]string{
		"servicenow_list_incidents":          `{"result":[{"sys_id":"1","number":"INC0010001","short_description":"VPN down","state":"2","priority":"1","urgency":"1","impact":"2","assigned_to":"Alice","assignment_group":"Network","caller_id":"Bob","category":"network","sys_updated_on":"2026-01-01 00:00:00","sys_created_on":"2026-01-01 00:00:00"}]}`,
		"servicenow_get_incident":            `{"result":{"sys_id":"1","number":"INC0010001","short_description":"VPN down","description":"Users cannot connect","state":"2","priority":"1","urgency":"1","impact":"2","assigned_to":"Alice","assignment_group":"Network","caller_id":"Bob","category":"network","subcategory":"vpn","cmdb_ci":"vpn-gw","close_code":"","close_notes":"","opened_at":"2026-01-01 00:00:00","resolved_at":"","closed_at":"","sys_updated_on":"2026-01-01 00:00:00","sys_created_on":"2026-01-01 00:00:00"}}`,
		"servicenow_list_problems":           `{"result":[{"sys_id":"p1","number":"PRB001","short_description":"Recurring outage","state":"2","priority":"2","assigned_to":"Alice","assignment_group":"SRE","sys_updated_on":"2026-01-01 00:00:00","sys_created_on":"2026-01-01 00:00:00"}]}`,
		"servicenow_get_problem":             `{"result":{"sys_id":"p1","number":"PRB001","short_description":"Recurring outage","description":"details","state":"2","priority":"2","assigned_to":"Alice","assignment_group":"SRE","workaround":"reboot","cause_notes":"memory leak","fix_notes":"patch","sys_updated_on":"2026-01-01 00:00:00","sys_created_on":"2026-01-01 00:00:00"}}`,
		"servicenow_list_change_requests":    `{"result":[{"sys_id":"c1","number":"CHG001","short_description":"Patch weekend","state":"-2","priority":"3","type":"normal","assigned_to":"Alice","assignment_group":"CAB","start_date":"2026-01-10","end_date":"2026-01-11","sys_updated_on":"2026-01-01 00:00:00"}]}`,
		"servicenow_get_change_request":      `{"result":{"sys_id":"c1","number":"CHG001","short_description":"Patch weekend","description":"apply patches","state":"-2","priority":"3","type":"normal","assigned_to":"Alice","assignment_group":"CAB","start_date":"2026-01-10","end_date":"2026-01-11","justification":"security","implementation_plan":"roll","backout_plan":"revert","test_plan":"smoke","risk":"moderate","impact":"2","sys_updated_on":"2026-01-01 00:00:00"}}`,
		"servicenow_list_catalog_requests":   `{"result":[{"sys_id":"r1","number":"REQ001","short_description":"Laptop","request_state":"in_process","stage":"fulfillment","priority":"3","requested_for":"Bob","assignment_group":"IT","sys_updated_on":"2026-01-01 00:00:00"}]}`,
		"servicenow_get_catalog_request":     `{"result":{"sys_id":"r1","number":"REQ001","short_description":"Laptop","description":"new hire laptop","request_state":"in_process","stage":"fulfillment","priority":"3","requested_for":"Bob","assignment_group":"IT","special_instructions":"dock included","sys_updated_on":"2026-01-01 00:00:00"}}`,
		"servicenow_list_knowledge_articles": `{"result":[{"sys_id":"k1","number":"KB001","short_description":"Reset VPN","workflow_state":"published","kb_knowledge_base":"IT","category":"network","sys_view_count":"12","published":"2026-01-01","sys_updated_on":"2026-01-01 00:00:00"}]}`,
		"servicenow_get_knowledge_article":   `{"result":{"sys_id":"k1","number":"KB001","short_description":"Reset VPN","workflow_state":"published","kb_knowledge_base":"IT","category":"network","text":"<p>Restart the client</p>","wiki":"","author":"Alice","published":"2026-01-01","valid_to":"2027-01-01","sys_view_count":"12","sys_updated_on":"2026-01-01 00:00:00"}}`,
		"servicenow_list_users":              `{"result":[{"sys_id":"u1","user_name":"alice","name":"Alice A","email":"a@b.com","title":"SRE","department":"IT","active":"true","sys_updated_on":"2026-01-01 00:00:00"}]}`,
		"servicenow_get_user":                `{"result":{"sys_id":"u1","user_name":"alice","name":"Alice A","email":"a@b.com","title":"SRE","department":"IT","active":"true","phone":"555","mobile_phone":"556","manager":"m1","location":"NY","company":"Acme","sys_updated_on":"2026-01-01 00:00:00"}}`,
		"servicenow_list_groups":             `{"result":[{"sys_id":"g1","name":"Network","description":"netops","email":"net@b.com","manager":"u1","active":"true","sys_updated_on":"2026-01-01 00:00:00"}]}`,
		"servicenow_list_cis":                `{"result":[{"sys_id":"ci1","name":"web-1","sys_class_name":"cmdb_ci_linux_server","operational_status":"1","install_status":"1","assigned_to":"Alice","support_group":"SRE","sys_updated_on":"2026-01-01 00:00:00"}]}`,
		"servicenow_get_ci":                  `{"result":{"sys_id":"ci1","name":"web-1","sys_class_name":"cmdb_ci_linux_server","operational_status":"1","install_status":"1","assigned_to":"Alice","support_group":"SRE","short_description":"web","ip_address":"10.0.0.1","fqdn":"web-1.example","serial_number":"s1","manufacturer":"Dell","model_id":"r640","location":"NY","owned_by":"IT","sys_updated_on":"2026-01-01 00:00:00"}}`,
		"servicenow_list_records":            `{"result":[{"sys_id":"1","number":"INC0010001","name":"","short_description":"VPN down","state":"2","sys_updated_on":"2026-01-01 00:00:00"}]}`,
		"servicenow_list_comments":           `{"result":[{"sys_id":"j1","element":"work_notes","element_id":"1","value":"looking","sys_created_on":"2026-01-01 00:00:00","sys_created_by":"alice"}]}`,
		"servicenow_list_attachments":        `{"result":[{"sys_id":"a1","file_name":"log.txt","content_type":"text/plain","size_bytes":"12","table_name":"incident","table_sys_id":"1","sys_created_on":"2026-01-01 00:00:00","sys_created_by":"alice"}]}`,
	}

	for toolName, payload := range handlerOutputs {
		t.Run(toolName, func(t *testing.T) {
			fields, ok := fieldCompactionSpecs[mcp.ToolName(toolName)]
			require.True(t, ok, "missing compaction spec for %s", toolName)
			compacted, err := mcp.CompactJSON([]byte(payload), fields)
			require.NoError(t, err)
			assert.NotEqual(t, "{}", string(compacted), "compaction returned empty object for %s: %s", toolName, compacted)
			assert.NotEqual(t, "[]", string(compacted), "compaction returned empty array for %s", toolName)
			assert.NotEqual(t, "[{}]", string(compacted), "compaction returned array of empty objects for %s: %s", toolName, compacted)
		})
	}
}
