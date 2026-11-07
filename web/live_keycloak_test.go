package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/daltoniam/switchboard/integrations/keycloak"
	"github.com/stretchr/testify/require"
)

const liveKeycloakImage = "quay.io/keycloak/keycloak:26.8.1@sha256:3f3c5f8e03c614774b4bd9173b88391fefa4153bfc010d8012d59ad825377332"

func init() {
	registerLiveSpec(liveSpec{
		Integration: "keycloak",
		Tier:        liveContainer,
		Adapter:     keycloak.New(),
		Provision:   liveKeycloakProvision,
		Cases: []liveCase{
			{Tool: "keycloak_list_users", Args: map[string]any{"search": "lovelace"}, Expect: []string{"ada.lovelace", "ada@example.test", "${ada_id}"}},
			{Tool: "keycloak_list_users", Expect: []string{"ada.lovelace", "grace.hopper"}},
			{Tool: "keycloak_get_user", Args: map[string]any{"user_id": "${grace_id}"}, Expect: []string{"grace.hopper", "Grace", "Hopper", `"enabled":false`}},
			{Tool: "keycloak_list_groups", Args: map[string]any{"search": "support"}, Expect: []string{"support-oncall", "/support-oncall"}},
			{Tool: "keycloak_list_events", Args: map[string]any{"date_from": "${today}", "date_to": "${today}"}, Expect: []string{"LOGIN", "LOGIN_ERROR", "${ada_id}", "live-app"}},
			{Tool: "keycloak_list_events", Args: map[string]any{"date_from": "${today}", "date_to": "${today}", "type": "LOGIN_ERROR"}, Expect: []string{"LOGIN_ERROR", "invalid_user_credentials"}},
		},
	})
}

// liveKeycloakProvision starts Keycloak, creates a realm with login events
// enabled, seeds users, a group and login activity, and creates a
// least-privilege service account with only the roles the setup guide lists.
func liveKeycloakProvision(t *testing.T) liveProvision {
	container := liveStartContainer(t, liveDockerOptions{
		Image: liveKeycloakImage,
		Env:   map[string]string{"KC_BOOTSTRAP_ADMIN_USERNAME": "admin", "KC_BOOTSTRAP_ADMIN_PASSWORD": "admin-password"},
		Ports: []string{"8080/tcp"},
		Cmd:   []string{"start-dev", "--http-port=8080"},
	})
	host, port := container.Address(t, "8080/tcp")
	base := fmt.Sprintf("http://%s:%s", host, port)
	kc := &liveKeycloak{t: t, base: base, client: &http.Client{Timeout: 20 * time.Second}}
	liveEventually(t, 180*time.Second, func() error {
		_, err := kc.token("master", url.Values{"grant_type": {"password"}, "client_id": {"admin-cli"}, "username": {"admin"}, "password": {"admin-password"}})
		return err
	})
	admin, err := kc.token("master", url.Values{"grant_type": {"password"}, "client_id": {"admin-cli"}, "username": {"admin"}, "password": {"admin-password"}})
	require.NoError(t, err)
	kc.admin = admin

	kc.post("/admin/realms", map[string]any{
		"realm": "live", "enabled": true,
		"eventsEnabled": true, "enabledEventTypes": []string{"LOGIN", "LOGIN_ERROR"}, "eventsExpiration": 86400,
	})
	kc.post("/admin/realms/live/groups", map[string]any{"name": "support-oncall"})
	adaID := kc.post("/admin/realms/live/users", map[string]any{
		"username": "ada.lovelace", "email": "ada@example.test", "firstName": "Ada", "lastName": "Lovelace",
		"enabled": true, "emailVerified": true,
		"credentials": []map[string]any{{"type": "password", "value": "ada-password", "temporary": false}},
	})
	graceID := kc.post("/admin/realms/live/users", map[string]any{
		"username": "grace.hopper", "email": "grace@example.test", "firstName": "Grace", "lastName": "Hopper", "enabled": false,
	})
	kc.post("/admin/realms/live/clients", map[string]any{"clientId": "live-app", "publicClient": true, "directAccessGrantsEnabled": true})
	serviceClient := kc.post("/admin/realms/live/clients", map[string]any{
		"clientId": "switchboard", "publicClient": false, "serviceAccountsEnabled": true,
		"standardFlowEnabled": false, "directAccessGrantsEnabled": false,
		"clientAuthenticatorType": "client-secret", "secret": "switchboard-live-secret",
	})

	var serviceUser struct {
		ID string `json:"id"`
	}
	kc.get("/admin/realms/live/clients/"+serviceClient+"/service-account-user", &serviceUser)
	var managementClients []struct {
		ID string `json:"id"`
	}
	kc.get("/admin/realms/live/clients?clientId=realm-management", &managementClients)
	require.Len(t, managementClients, 1)
	management := managementClients[0].ID
	var roles []map[string]any
	for _, name := range []string{"view-users", "query-users", "query-groups", "view-events"} {
		var role map[string]any
		kc.get("/admin/realms/live/clients/"+management+"/roles/"+name, &role)
		roles = append(roles, role)
	}
	kc.post("/admin/realms/live/users/"+serviceUser.ID+"/role-mappings/clients/"+management, roles)

	_, err = kc.token("live", url.Values{"grant_type": {"password"}, "client_id": {"live-app"}, "username": {"ada.lovelace"}, "password": {"ada-password"}})
	require.NoError(t, err, "seed LOGIN event")
	_, err = kc.token("live", url.Values{"grant_type": {"password"}, "client_id": {"live-app"}, "username": {"ada.lovelace"}, "password": {"wrong-password"}})
	require.Error(t, err, "seed LOGIN_ERROR event")

	return liveProvision{
		Credentials: map[string]string{
			"base_url":      base,
			"realm":         "live",
			"client_id":     "switchboard",
			"client_secret": "switchboard-live-secret",
		},
		Values: map[string]string{
			"ada_id":   adaID,
			"grace_id": graceID,
			"today":    time.Now().UTC().Format(time.DateOnly),
		},
	}
}

type liveKeycloak struct {
	t      *testing.T
	base   string
	admin  string
	client *http.Client
}

func (kc *liveKeycloak) token(realm string, form url.Values) (string, error) {
	response, err := kc.client.PostForm(kc.base+"/realms/"+realm+"/protocol/openid-connect/token", form)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token %s: %d %s", realm, response.StatusCode, body)
	}
	var token struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &token); err != nil {
		return "", err
	}
	return token.AccessToken, nil
}

// post creates an admin resource and returns the ID from its Location header.
func (kc *liveKeycloak) post(resource string, payload any) string {
	kc.t.Helper()
	body, err := json.Marshal(payload)
	require.NoError(kc.t, err)
	request, err := http.NewRequestWithContext(kc.t.Context(), http.MethodPost, kc.base+resource, bytes.NewReader(body))
	require.NoError(kc.t, err)
	request.Header.Set("Authorization", "Bearer "+kc.admin)
	request.Header.Set("Content-Type", "application/json")
	response, err := kc.client.Do(request)
	require.NoError(kc.t, err)
	defer response.Body.Close()
	detail, _ := io.ReadAll(response.Body)
	require.Contains(kc.t, []int{http.StatusCreated, http.StatusNoContent}, response.StatusCode, "POST %s: %s", resource, detail)
	return path.Base(strings.TrimRight(response.Header.Get("Location"), "/"))
}

func (kc *liveKeycloak) get(resource string, target any) {
	kc.t.Helper()
	request, err := http.NewRequestWithContext(kc.t.Context(), http.MethodGet, kc.base+resource, nil)
	require.NoError(kc.t, err)
	request.Header.Set("Authorization", "Bearer "+kc.admin)
	response, err := kc.client.Do(request)
	require.NoError(kc.t, err)
	defer response.Body.Close()
	detail, _ := io.ReadAll(response.Body)
	require.Equal(kc.t, http.StatusOK, response.StatusCode, "GET %s: %s", resource, detail)
	require.NoError(kc.t, json.Unmarshal(detail, target))
}
