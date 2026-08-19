package awmgrpc

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProtoContract_HasTypedMCPParityWithoutDynamicPayloads(t *testing.T) {
	raw, err := os.ReadFile("../api/switchboard/awm/v1/awm.proto")
	require.NoError(t, err)
	proto := string(raw)

	for _, banned := range []string{
		"google.protobuf.Struct",
		"google.protobuf.Value",
		"google.protobuf.Any",
		"bytes ",
		"map<",
	} {
		assert.NotContains(t, proto, banned)
	}

	for _, rpc := range []string{
		"ListProjects", "SearchProjects", "GetProject", "ResolveProject",
		"ValidateProject", "CreateProject", "UpdateProject", "PatchProject", "DeleteProject",
		"ListWorkProfiles", "GetWorkProfile", "PutWorkProfile", "DeleteWorkProfile",
		"ListAgentProfiles", "GetAgentProfile", "PutAgentProfile", "DeleteAgentProfile",
		"ListWorkSessions", "GetWorkSession", "CreateWorkSession",
		"TransitionWorkSession", "PatchWorkSession", "DeleteWorkSession",
		"ListResources", "GetResource", "PutResource", "DeleteResource",
		"ListResourceBindings", "GetResourceBinding", "CreateResourceBinding",
		"TransitionResourceBinding", "PatchResourceBinding", "DeleteResourceBinding",
	} {
		assert.True(t, strings.Contains(proto, "rpc "+rpc+"("), "missing RPC %s", rpc)
	}
}
