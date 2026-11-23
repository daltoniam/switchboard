package web

import (
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	mcp "github.com/daltoniam/switchboard"
	"github.com/daltoniam/switchboard/integrations/jenkins"
	"github.com/stretchr/testify/require"
)

const liveJenkinsImage = "jenkins/jenkins:2.580.2-jdk21@sha256:74401de6a6f92e1afe1b02d1c8e3d0c33f263bfb5275dd8813f9150355f2367b"

const liveJenkinsPlugins = `apache-httpcomponents-client-4-api:4.5.14-269.vfa_2321039a_83
asm-api:9.11.1-226.vff833a_e4c58e
bouncycastle-api:2.30.1.86.2-310.v4655f6742752
branch-api:2.1303.v9f3b_95dc329d
caffeine-api:3.3.1-214.v18eb_56b_34920
cloudbees-folder:6.1107.v31f8955765c8
commons-lang3-api:3.21.0-122.v6a_ea_4fd34049
commons-text-api:1.15.0-218.va_61573470393
credentials:1514.vb_01eb_014e7d4
display-url-api:2.217.va_6b_de84cc74b_
git-client:6.7.1
git:5.11.0
gson-api:2.15.0-212.v085f60a_8093b_
instance-identity:203.v15e81a_1b_7a_38
ionicons-api:94.vcc3065403257
jakarta-activation-api:2.1.4-1
jakarta-mail-api:2.1.5-1
mailer:534.v1b_36f5864073
mina-sshd-api-common:2.20.1-199.v8ce76b_c93542
mina-sshd-api-core:2.20.1-199.v8ce76b_c93542
scm-api:732.v830c71a_cfed3
script-security:1436.v1082791135e4
ssh-credentials:372.va_250881b_08cd
structs:362.va_b_695ef4fdf9
variant:70.va_d9f17f859e0
workflow-api:1429.v7a_1b_efb_a_f7c5
workflow-cps:4403.v471713a_cea_19
workflow-job:1624.ve973eb_34e031
workflow-multibranch:842.v3a_b_59b_57b_e6e
workflow-scm-step:487.v41313a_96b_65d
workflow-step-api:724.v538c2362b_dfb_
workflow-support:1035.v6a_23b_e298a_5d
`

const liveJenkinsBootstrap = `import hudson.model.AbstractItem
import hudson.model.Item
import hudson.model.Job
import hudson.security.ACL
import hudson.security.AuthorizationStrategy
import hudson.security.HudsonPrivateSecurityRealm
import jenkins.model.Jenkins
import jenkins.security.ApiTokenProperty
import groovy.json.JsonOutput

class LiveAuthorizationStrategy extends AuthorizationStrategy {
    ACL scoped(String fullName) {
        return ACL.lambda2 { authentication, permission ->
            if (authentication == ACL.SYSTEM2 || authentication.name == 'bootstrap') {
                return true
            }
            if (authentication.name != 'viewer') {
                return false
            }
            if (permission == Jenkins.READ) {
                return true
            }
            if (permission != Item.READ && permission != Item.DISCOVER) {
                return false
            }
            return fullName in ['a-visible', 'b-team', 'b-team/nested', 'b-team/nested/c-build', 'c-branches', 'd-running', 'e-history'] || fullName in ['c-branches/main', 'c-branches/feature%2Ffoo']
        }
    }
    ACL getRootACL() { return scoped(null) }
    ACL getACL(AbstractItem item) { return scoped(item.fullName) }
    ACL getACL(Job item) { return scoped(item.fullName) }
    Collection<String> getGroups() { return [] }
}

def instance = Jenkins.get()
def realm = new HudsonPrivateSecurityRealm(false)
instance.setSecurityRealm(realm)
def tokens = [:]
['bootstrap', 'viewer'].each { name ->
    def user = realm.createAccount(name, java.util.UUID.randomUUID().toString())
    tokens[name] = user.getProperty(ApiTokenProperty.class).generateNewToken('disposable-live-test').plainValue
}
instance.setNumExecutors(2)
hudson.ExtensionList.lookup(jenkins.security.CustomClassFilter.class).get(jenkins.security.CustomClassFilter.Static.class).overrides.put(LiveAuthorizationStrategy.class.name, true)
instance.setAuthorizationStrategy(new LiveAuthorizationStrategy())
instance.save()
new File(instance.rootDir, 'live-tokens.json').text = JsonOutput.toJson(tokens)
`

func init() {
	registerLiveSpec(liveSpec{
		Integration: "jenkins", Tier: liveContainer, Adapter: jenkins.New(), Provision: liveJenkinsProvision,
		Cases: []liveCase{
			{Tool: "jenkins_list_jobs", Args: map[string]any{"folder_path": []string{}, "limit": 2}, Expect: []string{"a-visible", "b-team", "SUCCESS", `"returned_count":2`, `"offset":0`, `"limit":2`, `"possibly_more":true`}},
			{Tool: "jenkins_list_jobs", Args: map[string]any{"offset": 1, "limit": 3}, Expect: []string{"b-team", "c-branches", "d-running", `"returned_count":3`, `"offset":1`, `"possibly_more":true`}},
			{Tool: "jenkins_list_jobs", Args: map[string]any{"folder_path": []string{"b-team", "nested"}}, Expect: []string{"c-build", "FAILURE", `"returned_count":1`, `"possibly_more":false`}},
			{Tool: "jenkins_list_jobs", Args: map[string]any{"folder_path": []string{"c-branches"}}, Expect: []string{"feature%2Ffoo", "main", "SUCCESS", `"returned_count":2`}},
			{Tool: "jenkins_get_job", Args: map[string]any{"job_path": []string{"a-visible"}, "limit": 1}, Expect: []string{"a-visible", "hudson.model.FreeStyleProject", `"buildable":true`, `"inQueue":false`, `"number":2`, "SUCCESS", "${root_timestamp}", `"returned_count":1`, `"possibly_more":true`}},
			{Tool: "jenkins_get_job", Args: map[string]any{"job_path": []string{"a-visible"}, "offset": 1, "limit": 2}, Expect: []string{"a-visible", `"number":1`, `"number":2`, `"offset":1`, `"returned_count":1`, `"possibly_more":false`}},
			{Tool: "jenkins_get_job", Args: map[string]any{"job_path": []string{"e-history"}, "offset": 100, "limit": 2}, Expect: []string{"e-history", `"builds":[`, `"number":103,`, `"number":3,`, `"number":2,`, "SUCCESS", `"building":false`, `"offset":100`, `"limit":2`, `"returned_count":2`, `"possibly_more":true`}},
			{Tool: "jenkins_get_job", Args: map[string]any{"job_path": []string{"e-history"}, "offset": 102, "limit": 2}, Expect: []string{"e-history", `"builds":[`, `"number":103,`, `"number":1,`, "SUCCESS", `"building":false`, `"offset":102`, `"limit":2`, `"returned_count":1`, `"possibly_more":false`}},
			{Tool: "jenkins_get_job", Args: map[string]any{"job_path": []string{"b-team", "nested", "c-build"}}, Expect: []string{"c-build", "FAILURE", "${nested_timestamp}", `"building":false`}},
			{Tool: "jenkins_get_job", Args: map[string]any{"job_path": []string{"c-branches", "feature%2Ffoo"}}, Expect: []string{"feature%2Ffoo", "org.jenkinsci.plugins.workflow.job.WorkflowJob", "SUCCESS", "${branch_timestamp}"}},
			{Tool: "jenkins_get_build", Args: map[string]any{"job_path": []string{"a-visible"}, "build_number": 2}, Expect: []string{`"number":2`, "SUCCESS", `"phase":"completed"`, `"building":false`, "${root_timestamp}", `"duration":${root_duration}`}},
			{Tool: "jenkins_get_build", Args: map[string]any{"job_path": []string{"b-team", "nested", "c-build"}, "build_number": 1}, Expect: []string{`"number":1`, "FAILURE", `"phase":"completed"`, "${nested_timestamp}"}},
			{Tool: "jenkins_get_build", Args: map[string]any{"job_path": []string{"c-branches", "feature/foo"}, "build_number": 1}, Expect: []string{`"number":1`, "SUCCESS", `"phase":"completed"`, "${branch_timestamp}"}},
			{Tool: "jenkins_get_build", Args: map[string]any{"job_path": []string{"d-running"}, "build_number": 1}, Expect: []string{`"number":1`, `"phase":"building"`, `"building":true`, "${running_timestamp}"}},
		},
	})
}

func TestLiveJenkinsSpec(t *testing.T) {
	spec, ok := liveSpecs["jenkins"]
	require.True(t, ok, "Jenkins must register real-container acceptance coverage")
	require.Equal(t, liveContainer, spec.Tier)
	require.Empty(t, liveSpecProblems(spec))
	var names []mcp.ToolName
	for _, tool := range spec.Adapter.Tools() {
		require.Equal(t, mcp.AccessRead, tool.Access)
		names = append(names, tool.Name)
	}
	require.ElementsMatch(t, []mcp.ToolName{"jenkins_list_jobs", "jenkins_get_job", "jenkins_get_build"}, names)
	require.Empty(t, spec.Skips)
}

func TestLiveJenkinsProvision(t *testing.T) {
	if os.Getenv("SWITCHBOARD_LIVE") != "1" {
		t.Skip("run with SWITCHBOARD_LIVE=1 to verify disposable Jenkins provisioning")
	}
	provision := liveJenkinsProvision(t)
	require.NotEmpty(t, provision.Credentials["api_token"])
	require.NotEmpty(t, provision.Values["branch_timestamp"])
}

func liveJenkinsProvision(t *testing.T) liveProvision {
	container := liveStartContainer(t, liveDockerOptions{
		Image: liveJenkinsImage,
		Env: map[string]string{
			"JAVA_OPTS":      "-Djenkins.install.runSetupWizard=false -Dhudson.plugins.git.GitSCM.ALLOW_LOCAL_CHECKOUT=true -Djenkins.update_center.check=false",
			"JENKINS_OPTS":   "--prefix=/jenkins",
			"LIVE_BOOTSTRAP": liveJenkinsBootstrap,
			"LIVE_PLUGINS":   liveJenkinsPlugins,
		},
		Ports: []string{"8080/tcp"}, Entrypoint: "/bin/sh",
		Cmd: []string{"-ec", `mkdir -p "$JENKINS_HOME/init.groovy.d"
printf '%s' "$LIVE_BOOTSTRAP" > "$JENKINS_HOME/init.groovy.d/live.groovy"
printf '%s' "$LIVE_PLUGINS" > /tmp/live-plugins.txt
jenkins-plugin-cli --plugin-file /tmp/live-plugins.txt --latest=false
git init -b main "$JENKINS_HOME/seed-repo"
git -C "$JENKINS_HOME/seed-repo" config user.name Fixture
git -C "$JENKINS_HOME/seed-repo" config user.email fixture@example.invalid
printf 'return null\n' > "$JENKINS_HOME/seed-repo/Jenkinsfile"
git -C "$JENKINS_HOME/seed-repo" add Jenkinsfile
git -C "$JENKINS_HOME/seed-repo" commit -m seed
git -C "$JENKINS_HOME/seed-repo" checkout -b feature/foo
exec /usr/local/bin/jenkins.sh`},
	})
	host, port := container.Address(t, "8080/tcp")
	upstream, err := url.Parse(fmt.Sprintf("http://%s:%s", host, port))
	require.NoError(t, err)
	handler := httputil.NewSingleHostReverseProxy(upstream)
	handler.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) {
		w.WriteHeader(http.StatusBadGateway)
	}
	proxy := httptest.NewTLSServer(handler)
	t.Cleanup(proxy.Close)
	base := proxy.URL + "/jenkins"
	client := proxy.Client()
	client.Timeout = 10 * time.Second
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	call := func(method, path, username, token, body string) (int, []byte, error) {
		request, err := http.NewRequestWithContext(t.Context(), method, base+path, strings.NewReader(body))
		if err != nil {
			return 0, nil, err
		}
		if username != "" {
			request.SetBasicAuth(username, token)
		}
		request.Header.Set("Content-Type", "application/xml")
		response, err := client.Do(request)
		if err != nil {
			return 0, nil, err
		}
		defer func() { _ = response.Body.Close() }()
		data, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
		return response.StatusCode, data, err
	}
	liveEventually(t, 5*time.Minute, func() error {
		status, _, err := call(http.MethodGet, "/login", "", "", "")
		if err != nil {
			return err
		}
		if status != http.StatusOK {
			return fmt.Errorf("Jenkins starting: HTTP %d", status)
		}
		return nil
	})
	var tokens map[string]string
	require.NoError(t, json.Unmarshal([]byte(container.Exec(t, "", "cat", "/var/jenkins_home/live-tokens.json")), &tokens))
	require.NotEmpty(t, tokens["bootstrap"])
	require.NotEmpty(t, tokens["viewer"])
	admin := func(method, path, body string, want int) []byte {
		status, data, err := call(method, path, "bootstrap", tokens["bootstrap"], body)
		require.NoError(t, err)
		require.Equal(t, want, status, "%s %s", method, path)
		return data
	}
	folderXML := `<com.cloudbees.hudson.plugins.folder.Folder><properties/></com.cloudbees.hudson.plugins.folder.Folder>`
	jobXML := func(command string) string {
		return `<project><description>SECRET_SENTINEL</description><keepDependencies>false</keepDependencies><properties/><scm class="hudson.scm.NullSCM"/><canRoam>true</canRoam><disabled>false</disabled><blockBuildWhenDownstreamBuilding>false</blockBuildWhenDownstreamBuilding><blockBuildWhenUpstreamBuilding>false</blockBuildWhenUpstreamBuilding><triggers/><concurrentBuild>false</concurrentBuild><builders><hudson.tasks.Shell><command>` + command + `</command></hudson.tasks.Shell></builders><publishers/><buildWrappers/></project>`
	}
	for _, seed := range []struct{ path, xml string }{
		{"/createItem?name=a-visible", jobXML("echo LIVE_COMPLETED")},
		{"/createItem?name=b-team", folderXML},
		{"/job/b-team/createItem?name=nested", folderXML},
		{"/job/b-team/job/nested/createItem?name=c-build", jobXML("exit 1")},
		{"/createItem?name=d-running", jobXML("sleep 900")},
		{"/createItem?name=e-history", jobXML("echo LIVE_HISTORY")},
		{"/createItem?name=z-hidden", jobXML("echo PRIVATE_HIDDEN")},
		{"/job/b-team/job/nested/createItem?name=z-hidden", jobXML("echo PRIVATE_HIDDEN")},
	} {
		admin(http.MethodPost, seed.path, seed.xml, http.StatusOK)
	}
	multiXML := `<org.jenkinsci.plugins.workflow.multibranch.WorkflowMultiBranchProject><sources class="jenkins.branch.MultiBranchProject$BranchSourceList"><data><jenkins.branch.BranchSource><source class="jenkins.plugins.git.GitSCMSource"><id>live-git</id><remote>/var/jenkins_home/seed-repo</remote><traits><jenkins.plugins.git.traits.BranchDiscoveryTrait/></traits></source><strategy class="jenkins.branch.DefaultBranchPropertyStrategy"><properties class="empty-list"/></strategy></jenkins.branch.BranchSource></data><owner class="org.jenkinsci.plugins.workflow.multibranch.WorkflowMultiBranchProject" reference="../.."/></sources><factory class="org.jenkinsci.plugins.workflow.multibranch.WorkflowBranchProjectFactory"><owner class="org.jenkinsci.plugins.workflow.multibranch.WorkflowMultiBranchProject" reference="../.."/><scriptPath>Jenkinsfile</scriptPath></factory></org.jenkinsci.plugins.workflow.multibranch.WorkflowMultiBranchProject>`
	admin(http.MethodPost, "/createItem?name=c-branches", multiXML, http.StatusOK)
	admin(http.MethodPost, "/job/c-branches/build?delay=0sec", "", http.StatusFound)
	values := map[string]string{}
	waitBuild := func(path, prefix, result string, building bool) {
		liveEventually(t, 90*time.Second, func() error {
			status, data, err := call(http.MethodGet, path+"/api/json?tree=number,result,building,timestamp,duration,estimatedDuration", "bootstrap", tokens["bootstrap"], "")
			if err != nil {
				return err
			}
			if status != http.StatusOK {
				return fmt.Errorf("waiting for seeded build: HTTP %d", status)
			}
			var build struct {
				Result    *string `json:"result"`
				Building  bool    `json:"building"`
				Timestamp int64   `json:"timestamp"`
				Duration  int64   `json:"duration"`
			}
			if err := json.Unmarshal(data, &build); err != nil {
				return err
			}
			if build.Building != building || build.Timestamp == 0 || (!building && (build.Result == nil || *build.Result != result)) || (building && build.Result != nil) {
				return fmt.Errorf("waiting for seeded build state %s building=%v", result, building)
			}
			values[prefix+"_timestamp"] = fmt.Sprint(build.Timestamp)
			values[prefix+"_duration"] = fmt.Sprint(build.Duration)
			return nil
		})
	}
	for number := 1; number <= 2; number++ {
		admin(http.MethodPost, "/job/a-visible/build?delay=0sec", "", http.StatusCreated)
		waitBuild(fmt.Sprintf("/job/a-visible/%d", number), "root", "SUCCESS", false)
	}
	for number := 1; number <= 103; number++ {
		admin(http.MethodPost, "/job/e-history/build?delay=0sec", "", http.StatusCreated)
		waitBuild(fmt.Sprintf("/job/e-history/%d", number), "history", "SUCCESS", false)
	}
	admin(http.MethodPost, "/job/b-team/job/nested/job/c-build/build?delay=0sec", "", http.StatusCreated)
	waitBuild("/job/b-team/job/nested/job/c-build/1", "nested", "FAILURE", false)
	waitBuild("/job/c-branches/job/feature%252Ffoo/1", "branch", "SUCCESS", false)
	waitBuild("/job/c-branches/job/main/1", "main", "SUCCESS", false)
	admin(http.MethodPost, "/job/d-running/build?delay=0sec", "", http.StatusCreated)
	waitBuild("/job/d-running/1", "running", "", true)
	for _, path := range []string{"/api/json?tree=jobs[name]", "/job/b-team/job/nested/api/json?tree=jobs[name]"} {
		adminData := admin(http.MethodGet, path, "", http.StatusOK)
		require.Contains(t, string(adminData), "z-hidden")
		status, data, err := call(http.MethodGet, path, "viewer", tokens["viewer"], "")
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, status)
		require.NotContains(t, string(data), "z-hidden")
	}
	checks := []struct {
		method, path, body string
		status             int
	}{
		{http.MethodGet, "/job/z-hidden/api/json", "", http.StatusNotFound},
		{http.MethodGet, "/job/b-team/job/nested/job/z-hidden/api/json", "", http.StatusNotFound},
		{http.MethodGet, "/job/a-visible/config.xml", "", http.StatusForbidden},
		{http.MethodPost, "/job/a-visible/config.xml", jobXML("exit 1"), http.StatusForbidden},
		{http.MethodPost, "/job/a-visible/build", "", http.StatusForbidden},
		{http.MethodPost, "/job/a-visible/doDelete", "", http.StatusForbidden},
		{http.MethodPost, "/job/a-visible/1/doDelete", "", http.StatusForbidden},
		{http.MethodPost, "/job/d-running/1/stop", "", http.StatusForbidden},
		{http.MethodPost, "/createItem?name=must-not-create", jobXML("exit 1"), http.StatusForbidden},
		{http.MethodPost, "/scriptText", "script=println('must-not-run')", http.StatusForbidden},
		{http.MethodPost, "/computer/createItem?name=must-not-create", "", http.StatusForbidden},
		{http.MethodGet, "/job/a-visible/ws/", "", http.StatusForbidden},
		{http.MethodGet, "/manage/", "", http.StatusForbidden},
		{http.MethodGet, "/credentials/store/system/domain/_/api/json", "", http.StatusNotFound},
		{http.MethodPost, "/credentials/store/system/domain/_/createCredentials", "", http.StatusNotFound},
	}
	for _, check := range checks {
		status, data, err := call(check.method, check.path, "viewer", tokens["viewer"], check.body)
		require.NoError(t, err)
		require.Equal(t, check.status, status, "%s %s", check.method, check.path)
		require.NotContains(t, string(data), "No valid crumb", "API tokens must reach authorization rather than fail CSRF")
	}
	admin(http.MethodGet, "/credentials/store/system/domain/_/api/json", "", http.StatusOK)
	certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: proxy.Certificate().Raw})
	credentials := map[string]string{"base_url": base, "username": "viewer", "api_token": tokens["viewer"], "ca_cert": strings.ReplaceAll(string(certificate), "\n", `\n`)}
	adapter := jenkins.New()
	t.Cleanup(func() { require.NoError(t, adapter.(io.Closer).Close()) })
	require.NoError(t, adapter.Configure(t.Context(), mcp.Credentials(credentials)))
	require.True(t, adapter.Healthy(t.Context()))
	for _, c := range liveExpandCases(t, liveSpecs["jenkins"].Cases, values) {
		result, err := adapter.Execute(t.Context(), c.Tool, c.Args)
		require.NoError(t, err)
		require.False(t, result.IsError, "%s: %s", c.Tool, result.Data)
		require.True(t, liveContainsAll(result.Data, c.Expect), "%s expected %q in %s", c.Tool, c.Expect, result.Data)
		for _, forbidden := range []string{"SECRET_SENTINEL", "PRIVATE_HIDDEN", "z-hidden", "actions", "parameters", "changeSets", "console", "description", "url"} {
			require.NotContains(t, result.Data, forbidden)
		}
	}
	for _, c := range []struct {
		tool mcp.ToolName
		args map[string]any
	}{
		{"jenkins_get_job", map[string]any{"job_path": []string{"z-hidden"}}},
		{"jenkins_get_build", map[string]any{"job_path": []string{"z-hidden"}, "build_number": 1}},
	} {
		result, err := adapter.Execute(t.Context(), c.tool, c.args)
		require.NoError(t, err)
		require.True(t, result.IsError)
		require.Contains(t, result.Data, "404")
		require.NotContains(t, result.Data, tokens["viewer"])
	}
	return liveProvision{Credentials: credentials, Values: values}
}
