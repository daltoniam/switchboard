package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJenkinsDefaultsAndEnvironment(t *testing.T) {
	configuration := defaultConfig()
	integration := configuration.Integrations["jenkins"]
	require.NotNil(t, integration)
	require.False(t, integration.Enabled)
	require.Len(t, integration.Credentials, 4)
	for _, key := range []string{"base_url", "username", "api_token", "ca_cert"} {
		require.Contains(t, integration.Credentials, key)
		require.Empty(t, integration.Credentials[key])
	}
	require.Equal(t, map[string]string{"base_url": "JENKINS_BASE_URL", "username": "JENKINS_USERNAME", "api_token": "JENKINS_API_TOKEN", "ca_cert": "JENKINS_CA_CERT"}, envMapping["jenkins"])

	manager, _ := newTestManager(t)
	values := map[string]string{"JENKINS_BASE_URL": "https://jenkins.example.invalid/prefix", "JENKINS_USERNAME": "reader", "JENKINS_API_TOKEN": "test-api-token", "JENKINS_CA_CERT": "test-ca"}
	manager.envLookup = func(key string) string { return values[key] }
	require.NoError(t, manager.Load())
	loaded, ok := manager.GetIntegration("jenkins")
	require.True(t, ok)
	for key, variable := range envMapping["jenkins"] {
		require.Equal(t, values[variable], loaded.Credentials[key])
	}
}
