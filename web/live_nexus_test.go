package web

import (
	"bytes"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/daltoniam/switchboard/integrations/nexus"
	"github.com/stretchr/testify/require"
)

const liveNexusImage = "sonatype/nexus3:3.96.0@sha256:56f8e1d241507338be1f91c9de9a9b97d72a7376f0404f6c1e914b542c983f75"

func init() {
	registerLiveSpec(liveSpec{
		Integration: "nexus", Tier: liveContainer, Adapter: nexus.New(), Provision: liveNexusProvision,
		Cases: []liveCase{
			{Tool: "nexus_list_components", Args: map[string]any{"repository": "maven-releases"}, Expect: []string{"org.switchboard", "live-demo", "1.0.0", "maven2"}},
			{Tool: "nexus_search_components", Args: map[string]any{"repository": "maven-releases", "name": "live-demo", "group": "org.switchboard", "version": "1.0.0"}, Expect: []string{"org.switchboard", "live-demo", "1.0.0", "maven2"}},
			{Tool: "nexus_get_component", Args: map[string]any{"id": "${component_id}"}, Expect: []string{"org.switchboard", "live-demo", "1.0.0", "asset_count", "truncated", "application/xml", "fileSize", "checksum", "lastModified", "live-demo-1.0.0.pom"}},
		},
	})
}

func liveNexusProvision(t *testing.T) liveProvision {
	container := liveStartContainer(t, liveDockerOptions{
		Image: liveNexusImage,
		Env: map[string]string{
			"NEXUS_SECURITY_RANDOMPASSWORD": "false",
			"INSTALL4J_ADD_VM_PARAMS":       "-Xms512m -Xmx1024m -XX:MaxDirectMemorySize=512m -Djava.util.prefs.userRoot=/nexus-data/javaprefs",
		},
		Ports: []string{"8081/tcp"},
	})
	host, port := container.Address(t, "8081/tcp")
	base := fmt.Sprintf("http://%s:%s", host, port)
	client := &http.Client{Timeout: 15 * time.Second}
	liveEventually(t, 3*time.Minute, func() error {
		request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, base+"/service/rest/v1/status", nil)
		if err != nil {
			return err
		}
		response, err := client.Do(request)
		if err != nil {
			return err
		}
		_ = response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return fmt.Errorf("Nexus not ready: HTTP %d", response.StatusCode)
		}
		return nil
	})
	call := func(user, password, method, path, contentType string, data []byte, want int) []byte {
		t.Helper()
		request, err := http.NewRequestWithContext(t.Context(), method, base+path, bytes.NewReader(data))
		require.NoError(t, err)
		request.SetBasicAuth(user, password)
		request.Header.Set("Content-Type", contentType)
		request.Header.Set("Accept", "application/json")
		response, err := client.Do(request)
		require.NoError(t, err)
		defer func() { require.NoError(t, response.Body.Close()) }()
		payload, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		require.Equal(t, want, response.StatusCode, "%s %s: %s", method, path, payload)
		return payload
	}
	adminJSON := func(method, path string, body any, want int) []byte {
		t.Helper()
		var data []byte
		if body != nil {
			var err error
			data, err = json.Marshal(body)
			require.NoError(t, err)
		}
		return call("admin", "admin123", method, "/service/rest/v1"+path, "application/json", data, want)
	}
	var eula map[string]any
	require.NoError(t, json.Unmarshal(adminJSON(http.MethodGet, "/system/eula", nil, 200), &eula))
	eula["accepted"] = true
	adminJSON(http.MethodPost, "/system/eula", eula, 204)
	adminJSON(http.MethodPut, "/security/anonymous", map[string]any{"enabled": false, "userId": "anonymous", "realmName": "NexusAuthorizingRealm"}, 200)
	adminJSON(http.MethodPost, "/security/roles", map[string]any{
		"id": "switchboard-reader", "name": "Switchboard Reader", "description": "Metadata reads for one approved repository",
		"privileges": []string{"nx-search-read", "nx-repository-view-maven2-maven-releases-browse", "nx-repository-view-maven2-maven-releases-read"}, "roles": []string{},
	}, 200)
	const readerPassword = "disposable-live-reader-password"
	adminJSON(http.MethodPost, "/security/users", map[string]any{
		"userId": "reader", "firstName": "Live", "lastName": "Reader", "emailAddress": "reader@localhost", "password": readerPassword,
		"status": "active", "roles": []string{"switchboard-reader"},
	}, 200)
	pom := []byte(`<project><modelVersion>4.0.0</modelVersion><groupId>org.switchboard</groupId><artifactId>live-demo</artifactId><version>1.0.0</version><description>SECRET_SENTINEL</description></project>`)
	artifactPath := "/repository/maven-releases/org/switchboard/live-demo/1.0.0/live-demo-1.0.0.pom"
	call("admin", "admin123", http.MethodPut, artifactPath, "application/xml", pom, 201)
	var page struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	require.NoError(t, json.Unmarshal(call("reader", readerPassword, http.MethodGet, "/service/rest/v1/components?repository=maven-releases", "application/json", nil, 200), &page))
	require.Len(t, page.Items, 1)
	componentID := page.Items[0].ID
	liveEventually(t, 60*time.Second, func() error {
		payload := call("reader", readerPassword, http.MethodGet, "/service/rest/v1/search?repository=maven-releases&name=live-demo&group=org.switchboard&version=1.0.0", "application/json", nil, 200)
		if !bytes.Contains(payload, []byte(componentID)) {
			return fmt.Errorf("waiting for search indexing")
		}
		return nil
	})
	for _, denied := range []struct {
		method, path, contentType string
		data                      []byte
	}{
		{http.MethodPut, artifactPath, "application/xml", pom},
		{http.MethodDelete, "/service/rest/v1/components/" + url.PathEscape(componentID), "application/json", nil},
		{http.MethodGet, "/service/rest/v1/security/users", "application/json", nil},
		{http.MethodGet, "/service/rest/v1/components?repository=maven-snapshots", "application/json", nil},
	} {
		call("reader", readerPassword, denied.method, denied.path, denied.contentType, denied.data, 403)
	}
	target, err := url.Parse(base)
	require.NoError(t, err)
	proxy := httptest.NewTLSServer(httputil.NewSingleHostReverseProxy(target))
	t.Cleanup(proxy.Close)
	cert := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: proxy.Certificate().Raw}))
	credentials := map[string]string{"base_url": proxy.URL, "username": "reader", "password": readerPassword, "repositories": "maven-releases", "ca_cert": strings.ReplaceAll(cert, "\n", `\n`)}
	adapter := nexus.New()
	t.Cleanup(func() { require.NoError(t, adapter.(io.Closer).Close()) })
	require.NoError(t, adapter.Configure(t.Context(), credentials))
	require.True(t, adapter.Healthy(t.Context()))
	for _, tool := range adapter.Tools() {
		args := map[string]any{"repository": "maven-releases"}
		if tool.Name == "nexus_get_component" {
			args = map[string]any{"id": componentID}
		}
		result, err := adapter.Execute(t.Context(), tool.Name, args)
		require.NoError(t, err)
		require.False(t, result.IsError, result.Data)
		for _, forbidden := range []string{"SECRET_SENTINEL", "downloadUrl", "uploader", "blobRef", readerPassword} {
			require.NotContains(t, result.Data, forbidden)
		}
	}
	return liveProvision{Credentials: credentials, Values: map[string]string{"component_id": componentID}}
}
