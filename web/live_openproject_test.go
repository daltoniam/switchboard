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
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/integrations/openproject"
	"github.com/stretchr/testify/require"
)

const liveOpenProjectImage = "openproject/openproject:17.9.2@sha256:3603f071443e16f773ca0e6bbe7f12f66eeeb95cca4540c4f93ec504305a0596"
const liveOpenProjectEncodedSubject = "Switchboard encoding & + % café"

func init() {
	registerLiveSpec(liveSpec{
		Integration: "openproject", Tier: liveContainer, Adapter: openproject.New(), Provision: liveOpenProjectProvision,
		Cases: []liveCase{
			{Tool: "openproject_list_work_packages", Args: map[string]any{"limit": 1}, Expect: []string{"Switchboard live open", "${work_package_id}", "${open_status}", "${type_name}"}},
			{Tool: "openproject_list_work_packages", Args: map[string]any{"page": 2, "limit": 1}, Expect: []string{"Switchboard live closed", "${closed_id}", "${closed_status}"}},
			{Tool: "openproject_list_work_packages", Args: map[string]any{"subject_or_id": liveOpenProjectEncodedSubject}, Expect: []string{"Switchboard encoding", "café", "${encoded_id}"}},
			{Tool: "openproject_list_work_packages", Args: map[string]any{"subject_or_id": "${closed_id}"}, Expect: []string{"Switchboard live closed", "${closed_id}"}},
			{Tool: "openproject_get_work_package", Args: map[string]any{"id": "${work_package_id}"}, Expect: []string{"Switchboard live open", "Live description", "2026-11-02", "2026-11-30", "${created_at}"}},
			{Tool: "openproject_get_work_package", Args: map[string]any{"id": "${long_id}"}, Expect: []string{"Switchboard live long description", "Long live description"}},
		},
	})
}

func TestLiveOpenProjectProvision(t *testing.T) {
	if os.Getenv("SWITCHBOARD_LIVE") != "1" {
		t.Skip("run with SWITCHBOARD_LIVE=1 to provision the real OpenProject image")
	}
	provision := liveOpenProjectProvision(t)
	require.NotEmpty(t, provision.Credentials["api_token"])
	require.True(t, strings.HasPrefix(provision.Credentials["base_url"], "https://"))
}

func liveOpenProjectProvision(t *testing.T) liveProvision {
	container := liveStartContainer(t, liveDockerOptions{
		Image: liveOpenProjectImage, Ports: []string{"80/tcp"},
		Env: map[string]string{
			"SECRET_KEY_BASE":                    "30d2e86ecaa672bc70e37cfde5fbb2c2d3bc48c9885d1c3676a3fd0b5ff3a4c9b",
			"OPENPROJECT_HOST__NAME":             "localhost",
			"OPENPROJECT_HTTPS":                  "false",
			"OPENPROJECT_DEFAULT__LANGUAGE":      "en",
			"OPENPROJECT_WEB_WORKERS":            "1",
			"OPENPROJECT_APIV3__MAX__PAGE__SIZE": "2",
		},
	})
	host, port := container.Address(t, "80/tcp")
	base := fmt.Sprintf("http://%s:%s", host, port)
	client := &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	liveEventually(t, 6*time.Minute, func() error {
		request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, base+"/health_checks/default", nil)
		if err != nil {
			return err
		}
		request.Host = "localhost"
		response, err := client.Do(request)
		if err != nil {
			return err
		}
		_ = response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return fmt.Errorf("OpenProject not ready: HTTP %d", response.StatusCode)
		}
		return nil
	})
	output := container.Exec(t, `
admin = User.find_by!(login: "admin")
admin.update!(force_password_change: false)
User.current = admin
Project.find_each { |project| project.update!(public: false) }
[GlobalRole.standard, ProjectRole.non_member, ProjectRole.anonymous].each do |role|
  role.permissions = []
  role.save!
end
role = ProjectRole.create!(name: "Switchboard Work Package Reader", permissions: [:view_work_packages])
reader = User.create!(login: "switchboard-reader", firstname: "Live", lastname: "Reader", mail: "reader@localhost.test", password: "Live-Reader-6f9b73!", password_confirmation: "Live-Reader-6f9b73!", status: User.statuses[:active], admin: false, force_password_change: false)
make_project = lambda do |name, identifier, parent|
  Project.create!(name: name, identifier: identifier, public: false, workspace_type: "project", parent: parent, enabled_module_names: ["work_package_tracking"], project_types: Type.all.map { |type| ProjectType.new(type: type) })
end
project = make_project.call("Switchboard Live", "switchboard-live", nil)
other = make_project.call("Other Private", "other-private", nil)
child = make_project.call("Private Descendant", "private-descendant", project)
Member.create!(principal: reader, project: project, roles: [role])
reader.reload
raise "Reader must not be admin" if reader.admin?
raise "Unexpected groups" unless reader.groups.empty?
raise "Unexpected memberships" unless reader.members.count == 1 && reader.members.first.project_id == project.id && reader.members.first.entity_id.nil?
raise "Unexpected role permissions" unless reader.all_permissions_for(project).sort == [:view_work_packages]
raise "Unexpected global grants" unless GlobalRole.standard.reload.permissions.empty? && reader.members.where(project_id: nil).empty?
raise "Unexpected non-member grants" unless ProjectRole.non_member.reload.permissions.empty? && ProjectRole.anonymous.reload.permissions.empty?
raise "Unexpected public projects" if Project.where(public: true).exists?
raise "Unauthorized project permission" if reader.allowed_in_project?(:view_work_packages, other) || reader.allowed_in_project?(:view_work_packages, child)
puts "SWITCHBOARD_BOOTSTRAP=" + {admin_token: Token::API.create!(user: admin, token_name: "Live bootstrap").plain_value, reader_token: Token::API.create!(user: reader, token_name: "Switchboard live").plain_value, project_id: project.id, other_id: other.id, child_id: child.id, type_id: Type.first.id, open_status_id: Status.where(is_closed: false).first.id, closed_status_id: Status.where(is_closed: true).first.id}.to_json
`, "bundle", "exec", "rails", "runner", "-")
	var bootstrap struct {
		AdminToken     string `json:"admin_token"`
		ReaderToken    string `json:"reader_token"`
		ProjectID      int    `json:"project_id"`
		OtherID        int    `json:"other_id"`
		ChildID        int    `json:"child_id"`
		TypeID         int    `json:"type_id"`
		OpenStatusID   int    `json:"open_status_id"`
		ClosedStatusID int    `json:"closed_status_id"`
	}
	_, encoded, ok := strings.Cut(output, "SWITCHBOARD_BOOTSTRAP=")
	require.True(t, ok, "Rails bootstrap did not return fixture metadata")
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(encoded)), &bootstrap))
	require.NotEmpty(t, bootstrap.ReaderToken)
	require.Positive(t, bootstrap.ProjectID)
	target, err := url.Parse(base)
	require.NoError(t, err)
	proxyHandler := httputil.NewSingleHostReverseProxy(target)
	director := proxyHandler.Director
	proxyHandler.Director = func(request *http.Request) {
		director(request)
		request.Host = "localhost"
	}
	proxy := httptest.NewTLSServer(proxyHandler)
	t.Cleanup(proxy.Close)
	client = proxy.Client()
	client.Timeout = 20 * time.Second
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	call := func(token, method, path string, body any, want int) []byte {
		t.Helper()
		var data []byte
		if body != nil {
			var err error
			data, err = json.Marshal(body)
			require.NoError(t, err)
		}
		request, err := http.NewRequestWithContext(t.Context(), method, proxy.URL+"/api/v3"+path, bytes.NewReader(data))
		require.NoError(t, err)
		request.SetBasicAuth("apikey", token)
		request.Header.Set("Accept", "application/hal+json")
		request.Header.Set("Content-Type", "application/json")
		response, err := client.Do(request)
		require.NoError(t, err)
		defer func() { require.NoError(t, response.Body.Close()) }()
		payload, err := io.ReadAll(io.LimitReader(response.Body, 2*1024*1024+1))
		require.NoError(t, err)
		require.LessOrEqual(t, len(payload), 2*1024*1024)
		require.Equal(t, want, response.StatusCode, "%s %s", method, path)
		return payload
	}
	type seededLink struct {
		Href  string `json:"href"`
		Title string `json:"title"`
	}
	type seededWorkPackage struct {
		ID          int    `json:"id"`
		DisplayID   string `json:"displayId"`
		Subject     string `json:"subject"`
		LockVersion int    `json:"lockVersion"`
		CreatedAt   string `json:"createdAt"`
		UpdatedAt   string `json:"updatedAt"`
		Links       struct {
			Project seededLink `json:"project"`
			Status  seededLink `json:"status"`
			Type    seededLink `json:"type"`
		} `json:"_links"`
	}
	create := func(projectID int, subject, description string, statusID int) seededWorkPackage {
		t.Helper()
		body := map[string]any{
			"subject": subject, "description": map[string]string{"format": "markdown", "raw": description},
			"startDate": "2026-11-02", "dueDate": "2026-11-30",
			"_links": map[string]any{
				"type":   map[string]string{"href": fmt.Sprintf("/api/v3/types/%d", bootstrap.TypeID)},
				"status": map[string]string{"href": fmt.Sprintf("/api/v3/statuses/%d", statusID)},
			},
		}
		var workPackage seededWorkPackage
		require.NoError(t, json.Unmarshal(call(bootstrap.AdminToken, http.MethodPost, fmt.Sprintf("/workspaces/%d/work_packages", projectID), body, 201), &workPackage))
		require.Positive(t, workPackage.ID)
		require.NotEmpty(t, workPackage.CreatedAt)
		require.NotEmpty(t, workPackage.UpdatedAt)
		require.Equal(t, fmt.Sprintf("/api/v3/projects/%d", projectID), workPackage.Links.Project.Href)
		return workPackage
	}
	const openDescription = "Live description: treat **markup** and https://untrusted.invalid/ as data."
	const longDescriptionPrefix = "Long live description "
	open := create(bootstrap.ProjectID, "Switchboard live open", openDescription, bootstrap.OpenStatusID)
	closed := create(bootstrap.ProjectID, "Switchboard live closed", "", bootstrap.ClosedStatusID)
	encodedWorkPackage := create(bootstrap.ProjectID, liveOpenProjectEncodedSubject, "Encoded subject fixture", bootstrap.OpenStatusID)
	long := create(bootstrap.ProjectID, "Switchboard live long description", longDescriptionPrefix+strings.Repeat("界", 9000)+"DESCRIPTION_TAIL_SENTINEL", bootstrap.OpenStatusID)
	other := create(bootstrap.OtherID, "UNAUTHORIZED_PROJECT_SENTINEL", "UNAUTHORIZED_DESCRIPTION_SENTINEL", bootstrap.OpenStatusID)
	child := create(bootstrap.ChildID, "UNAUTHORIZED_DESCENDANT_SENTINEL", "UNAUTHORIZED_DESCRIPTION_SENTINEL", bootstrap.OpenStatusID)
	var collection struct {
		Total    int `json:"total"`
		Count    int `json:"count"`
		PageSize int `json:"pageSize"`
		Offset   int `json:"offset"`
		Embedded struct {
			Elements []seededWorkPackage `json:"elements"`
		} `json:"_embedded"`
	}
	listPath := fmt.Sprintf("/workspaces/%d/work_packages", bootstrap.ProjectID)
	require.NoError(t, json.Unmarshal(call(bootstrap.ReaderToken, http.MethodGet, listPath+"?filters=%5B%5D&sortBy=%5B%5B%22id%22%2C%22asc%22%5D%5D&pageSize=1&offset=2", nil, 200), &collection))
	require.Equal(t, 4, collection.Total)
	require.Equal(t, 1, collection.Count)
	require.Equal(t, 1, collection.PageSize)
	require.Equal(t, 2, collection.Offset)
	require.Len(t, collection.Embedded.Elements, 1)
	require.Equal(t, closed.ID, collection.Embedded.Elements[0].ID)
	t.Log("Verified real OpenProject v17.9.2 GET /api/v3/workspaces/{id}/work_packages: HTTP 200 with the non-admin reader token; total=4, count=1, pageSize=1, offset=2, seeded closed work package matched")
	for _, hidden := range []struct {
		projectID int
		work      seededWorkPackage
	}{{bootstrap.OtherID, other}, {bootstrap.ChildID, child}} {
		for _, path := range []string{fmt.Sprintf("/workspaces/%d/work_packages?filters=%%5B%%5D", hidden.projectID), fmt.Sprintf("/work_packages/%d", hidden.work.ID)} {
			payload := call(bootstrap.ReaderToken, http.MethodGet, path, nil, 404)
			require.NotContains(t, string(payload), "SENTINEL")
		}
	}
	for _, denied := range []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodPost, listPath, map[string]any{"subject": "DENIED", "_links": map[string]any{"type": map[string]string{"href": fmt.Sprintf("/api/v3/types/%d", bootstrap.TypeID)}}}},
		{http.MethodPatch, fmt.Sprintf("/work_packages/%d", open.ID), map[string]any{"lockVersion": open.LockVersion, "subject": "DENIED"}},
		{http.MethodDelete, fmt.Sprintf("/work_packages/%d", open.ID), nil},
	} {
		payload := call(bootstrap.ReaderToken, denied.method, denied.path, denied.body, 403)
		require.Contains(t, string(payload), "MissingPermission")
	}
	var unchanged seededWorkPackage
	require.NoError(t, json.Unmarshal(call(bootstrap.ReaderToken, http.MethodGet, fmt.Sprintf("/work_packages/%d", open.ID), nil, 200), &unchanged))
	require.Equal(t, open.Subject, unchanged.Subject)
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: proxy.Certificate().Raw})
	credentials := map[string]string{
		"base_url": proxy.URL, "api_token": bootstrap.ReaderToken, "workspace_id": strconv.Itoa(bootstrap.ProjectID), "ca_cert": strings.ReplaceAll(string(cert), "\n", `\n`),
	}
	adapter := openproject.New()
	t.Cleanup(func() { require.NoError(t, adapter.(io.Closer).Close()) })
	require.NoError(t, adapter.Configure(t.Context(), credentials))
	require.True(t, adapter.Healthy(t.Context()))
	execute := func(tool mcp.ToolName, args map[string]any) map[string]any {
		t.Helper()
		result, err := adapter.Execute(t.Context(), tool, args)
		require.NoError(t, err)
		require.False(t, result.IsError, result.Data)
		require.Less(t, len(result.Data), 40*1024)
		for _, forbidden := range []string{"SENTINEL", "_links", "_embedded", "attachments", "author", "assignee", "watchers", "email", "lockVersion", "customField", "\"html\"", bootstrap.ReaderToken, bootstrap.AdminToken} {
			require.NotContains(t, result.Data, forbidden)
		}
		var value map[string]any
		require.NoError(t, json.Unmarshal([]byte(result.Data), &value))
		return value
	}
	summaryKeys := []string{"id", "displayId", "subject", "status", "type", "project_id", "updatedAt"}
	assertKeys := func(value map[string]any, want []string) {
		t.Helper()
		got := make([]string, 0, len(value))
		for key := range value {
			got = append(got, key)
		}
		require.ElementsMatch(t, want, got)
	}
	assertSummary := func(value map[string]any, work seededWorkPackage) {
		t.Helper()
		require.EqualValues(t, work.ID, value["id"])
		require.Equal(t, work.DisplayID, value["displayId"])
		require.Equal(t, work.Subject, value["subject"])
		require.Equal(t, work.Links.Status.Title, value["status"])
		require.Equal(t, work.Links.Type.Title, value["type"])
		require.EqualValues(t, bootstrap.ProjectID, value["project_id"])
		require.Equal(t, work.UpdatedAt, value["updatedAt"])
	}
	for _, page := range []struct {
		page int
		work seededWorkPackage
		next any
	}{{1, open, float64(2)}, {2, closed, float64(3)}, {4, long, nil}} {
		value := execute("openproject_list_work_packages", map[string]any{"page": page.page, "limit": 1})
		assertKeys(value, []string{"work_packages", "total", "count", "page", "page_size", "returned", "omitted", "truncated", "next_page"})
		require.EqualValues(t, 4, value["total"])
		require.EqualValues(t, 1, value["count"])
		require.EqualValues(t, page.page, value["page"])
		require.EqualValues(t, 1, value["page_size"])
		require.EqualValues(t, 1, value["returned"])
		require.EqualValues(t, 0, value["omitted"])
		require.Equal(t, false, value["truncated"])
		require.Equal(t, page.next, value["next_page"])
		rows, ok := value["work_packages"].([]any)
		require.True(t, ok)
		require.Len(t, rows, 1)
		row, ok := rows[0].(map[string]any)
		require.True(t, ok)
		assertKeys(row, summaryKeys)
		assertSummary(row, page.work)
	}
	smallPage := execute("openproject_list_work_packages", map[string]any{"limit": 50})
	require.EqualValues(t, 2, smallPage["page_size"])
	require.EqualValues(t, 2, smallPage["count"])
	require.EqualValues(t, 2, smallPage["returned"])
	require.EqualValues(t, 2, smallPage["next_page"])
	empty := execute("openproject_list_work_packages", map[string]any{"subject_or_id": "NO_MATCH_SWITCHBOARD_FIXTURE"})
	require.EqualValues(t, 0, empty["total"])
	require.EqualValues(t, 0, empty["count"])
	require.EqualValues(t, 0, empty["returned"])
	require.Nil(t, empty["next_page"])
	require.Empty(t, empty["work_packages"])
	for _, search := range []struct {
		query string
		work  seededWorkPackage
	}{{liveOpenProjectEncodedSubject, encodedWorkPackage}, {strconv.Itoa(closed.ID), closed}} {
		value := execute("openproject_list_work_packages", map[string]any{"subject_or_id": search.query})
		require.EqualValues(t, 1, value["total"])
		rows, ok := value["work_packages"].([]any)
		require.True(t, ok)
		require.Len(t, rows, 1)
		row, ok := rows[0].(map[string]any)
		require.True(t, ok)
		assertSummary(row, search.work)
	}
	for _, detail := range []struct {
		work        seededWorkPackage
		description string
		truncated   bool
	}{
		{open, openDescription, false},
		{closed, "", false},
		{long, longDescriptionPrefix + strings.Repeat("界", 8000-utf8.RuneCountInString(longDescriptionPrefix)), true},
	} {
		work := detail.work
		value := execute("openproject_get_work_package", map[string]any{"id": work.ID})
		assertKeys(value, append(slices.Clone(summaryKeys), "createdAt", "startDate", "dueDate", "description", "description_truncated", "truncated"))
		assertSummary(value, work)
		require.Equal(t, work.CreatedAt, value["createdAt"])
		require.Equal(t, "2026-11-02", value["startDate"])
		require.Equal(t, "2026-11-30", value["dueDate"])
		description, ok := value["description"].(string)
		require.True(t, ok)
		require.Equal(t, detail.description, description)
		require.Equal(t, detail.truncated, value["description_truncated"])
		require.Equal(t, detail.truncated, value["truncated"])
	}
	for _, work := range []seededWorkPackage{other, child} {
		result, err := adapter.Execute(t.Context(), "openproject_get_work_package", map[string]any{"id": work.ID})
		require.NoError(t, err)
		require.True(t, result.IsError)
		require.NotContains(t, result.Data, "SENTINEL")
		require.NotContains(t, result.Data, bootstrap.ReaderToken)
	}
	return liveProvision{Credentials: credentials, Values: map[string]string{
		"work_package_id": strconv.Itoa(open.ID), "closed_id": strconv.Itoa(closed.ID), "encoded_id": strconv.Itoa(encodedWorkPackage.ID), "long_id": strconv.Itoa(long.ID),
		"open_status": open.Links.Status.Title, "closed_status": closed.Links.Status.Title, "type_name": open.Links.Type.Title, "created_at": open.CreatedAt,
	}}
}
