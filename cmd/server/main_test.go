package main

import (
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/project"
	"github.com/stretchr/testify/assert"
)

func TestProjectConfigRoot(t *testing.T) {
	tests := []struct {
		name string
		cfg  *mcp.IntegrationConfig
		want string
	}{
		{name: "missing integration", want: project.DefaultConfigDir()},
		{name: "empty override", cfg: &mcp.IntegrationConfig{Credentials: mcp.Credentials{"config_root": ""}}, want: project.DefaultConfigDir()},
		{name: "configured override", cfg: &mcp.IntegrationConfig{Credentials: mcp.Credentials{"config_root": "/tmp/custom-projects"}}, want: "/tmp/custom-projects"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, projectConfigRoot(test.cfg))
		})
	}
}
