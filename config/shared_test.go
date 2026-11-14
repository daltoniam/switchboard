package config

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"

	mcp "github.com/daltoniam/switchboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// twoProcesses returns two managers on one config file, standing in for two
// Switchboard processes (for example two stdio sessions).
func twoProcesses(t *testing.T) (*manager, *manager) {
	t.Helper()
	first, path := newTestManager(t)
	require.NoError(t, first.Load())
	second := &manager{filePath: path, envLookup: noEnv}
	require.NoError(t, second.Load())
	return first, second
}

func setCredential(t *testing.T, m *manager, integration, key, value string) {
	t.Helper()
	require.NoError(t, m.UpdateIntegration(integration, func(ic *mcp.IntegrationConfig) error {
		ic.Credentials[key] = value
		return nil
	}))
}

func TestSave_KeepsCredentialAnotherProcessChanged(t *testing.T) {
	first, second := twoProcesses(t)
	setCredential(t, second, "higgsfield", "mcp_refresh_token", "rotated")

	setCredential(t, first, "github", "token", "ghp_first")

	reloaded := &manager{filePath: first.filePath, envLookup: noEnv}
	require.NoError(t, reloaded.Load())
	higgsfield, _ := reloaded.GetIntegration("higgsfield")
	github, _ := reloaded.GetIntegration("github")
	assert.Equal(t, "rotated", higgsfield.Credentials["mcp_refresh_token"], "a stale process must not write back an old refresh token")
	assert.Equal(t, "ghp_first", github.Credentials["token"])
	fromFirst, _ := first.GetIntegration("higgsfield")
	assert.Equal(t, "rotated", fromFirst.Credentials["mcp_refresh_token"], "the saving process picks up the newer value")
}

func TestSave_OwnChangeWinsWhenBothProcessesChangedACredential(t *testing.T) {
	first, second := twoProcesses(t)
	setCredential(t, second, "github", "token", "ghp_second")
	setCredential(t, first, "github", "token", "ghp_first")

	reloaded := &manager{filePath: first.filePath, envLookup: noEnv}
	require.NoError(t, reloaded.Load())
	github, _ := reloaded.GetIntegration("github")
	assert.Equal(t, "ghp_first", github.Credentials["token"])
}

func TestUpdateIntegrationShared_StartsFromSavedCopy(t *testing.T) {
	first, second := twoProcesses(t)
	setCredential(t, second, "higgsfield", "mcp_refresh_token", "rotated")

	var seen string
	require.NoError(t, first.UpdateIntegrationShared(context.Background(), "higgsfield", func(ic *mcp.IntegrationConfig) error {
		seen = ic.Credentials["mcp_refresh_token"]
		ic.Credentials["mcp_refresh_token"] = "rotated-again"
		return nil
	}))

	assert.Equal(t, "rotated", seen)
	reloaded := &manager{filePath: first.filePath, envLookup: noEnv}
	require.NoError(t, reloaded.Load())
	higgsfield, _ := reloaded.GetIntegration("higgsfield")
	assert.Equal(t, "rotated-again", higgsfield.Credentials["mcp_refresh_token"])
}

func TestUpdateIntegrationShared_UpdateErrorSavesNothing(t *testing.T) {
	first, _ := twoProcesses(t)
	failure := errors.New("token endpoint down")
	err := first.UpdateIntegrationShared(context.Background(), "higgsfield", func(ic *mcp.IntegrationConfig) error {
		ic.Credentials["mcp_refresh_token"] = "half-done"
		return failure
	})

	require.ErrorIs(t, err, failure)
	higgsfield, _ := first.GetIntegration("higgsfield")
	assert.Empty(t, higgsfield.Credentials["mcp_refresh_token"])
}

// Each process increments a counter under the shared lock. Without the file
// lock and the re-read, increments from the other process would be lost.
func TestUpdateIntegrationShared_SerializesProcesses(t *testing.T) {
	first, second := twoProcesses(t)
	const perProcess = 20
	var wg sync.WaitGroup
	for _, m := range []*manager{first, second} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range perProcess {
				err := m.UpdateIntegrationShared(context.Background(), "higgsfield", func(ic *mcp.IntegrationConfig) error {
					n, _ := strconv.Atoi(ic.Credentials["counter"])
					ic.Credentials["counter"] = strconv.Itoa(n + 1)
					return nil
				})
				assert.NoError(t, err)
			}
		}()
	}
	wg.Wait()

	reloaded := &manager{filePath: first.filePath, envLookup: noEnv}
	require.NoError(t, reloaded.Load())
	higgsfield, _ := reloaded.GetIntegration("higgsfield")
	assert.Equal(t, strconv.Itoa(2*perProcess), higgsfield.Credentials["counter"])
}
