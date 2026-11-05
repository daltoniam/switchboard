package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultConfigMySQL(t *testing.T) {
	integration := defaultConfig().Integrations["mysql"]
	require.NotNil(t, integration)
	require.False(t, integration.Enabled)
	for _, key := range []string{"connection_string", "host", "port", "user", "password", "database", "tls", "connections"} {
		require.Contains(t, integration.Credentials, key)
	}
	for key, variable := range map[string]string{"host": "MYSQL_HOST", "port": "MYSQL_PORT", "user": "MYSQL_USER", "password": "MYSQL_PASSWORD", "database": "MYSQL_DATABASE", "tls": "MYSQL_TLS", "connection_string": "MYSQL_DSN"} {
		require.Equal(t, variable, envMapping["mysql"][key])
	}
}
