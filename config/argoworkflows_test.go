package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestArgoWorkflowsDefaults(t *testing.T) {
	integration, ok := defaultConfig().Integrations["argoworkflows"]
	require.True(t, ok)
	require.False(t, integration.Enabled)
	require.Equal(t, map[string]string{"base_url": "", "namespace": "", "token": "", "ca_cert": ""}, map[string]string(integration.Credentials))
	require.Equal(t, map[string]string{"base_url": "ARGO_WORKFLOWS_BASE_URL", "namespace": "ARGO_WORKFLOWS_NAMESPACE", "token": "ARGO_WORKFLOWS_TOKEN", "ca_cert": "ARGO_WORKFLOWS_CA_CERT"}, envMapping["argoworkflows"])
}
