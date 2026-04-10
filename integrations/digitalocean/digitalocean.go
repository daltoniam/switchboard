package digitalocean

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/digitalocean/godo"
	"golang.org/x/oauth2"

	mcp "github.com/daltoniam/switchboard"
)

var _ mcp.Integration = (*integration)(nil)

const maxResponseSize = 10 * 1024 * 1024 // 10 MB

type integration struct {
	client     *godo.Client
	token      string
	httpClient *http.Client
	baseURL    string
}

func New() mcp.Integration {
	return &integration{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    "https://api.digitalocean.com",
	}
}

func (d *integration) Name() string { return "digitalocean" }

func (d *integration) Configure(ctx context.Context, creds mcp.Credentials) error {
	token := creds["api_token"]
	if token == "" {
		return fmt.Errorf("digitalocean: api_token is required")
	}

	d.token = token

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	oauthClient := oauth2.NewClient(ctx, ts)
	d.client = godo.NewClient(oauthClient)

	if v := creds["base_url"]; v != "" {
		d.baseURL = strings.TrimRight(v, "/")
		d.client.BaseURL, _ = d.client.BaseURL.Parse(d.baseURL + "/")
	}

	return nil
}

func (d *integration) Healthy(ctx context.Context) bool {
	if d.client == nil {
		return false
	}
	_, _, err := d.client.Account.Get(ctx)
	return err == nil
}

func (d *integration) Tools() []mcp.ToolDefinition {
	return tools
}

func (d *integration) Execute(ctx context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, d, args)
}

// --- Result helpers ---

type handlerFunc func(ctx context.Context, d *integration, args map[string]any) (*mcp.ToolResult, error)

func errResult(err error) (*mcp.ToolResult, error) {
	return mcp.ErrResult(wrapRetryable(err))
}

func wrapRetryable(err error) error {
	if err == nil {
		return nil
	}
	if resp, ok := err.(*godo.ErrorResponse); ok {
		code := resp.Response.StatusCode
		if code == http.StatusTooManyRequests || code >= 500 {
			return &mcp.RetryableError{StatusCode: code, Err: err}
		}
	}
	return err
}

func listOpts(args map[string]any) *godo.ListOptions {
	return &godo.ListOptions{
		Page:    mcp.OptInt(args, "page", 0),
		PerPage: mcp.OptInt(args, "per_page", 200),
	}
}

// --- HTTP helpers (used by App Platform handlers) ---

func (d *integration) doRequest(ctx context.Context, method, path string, body any) (json.RawMessage, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, d.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+d.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		re := &mcp.RetryableError{StatusCode: resp.StatusCode, Err: fmt.Errorf("digitalocean API error (%d): %s", resp.StatusCode, string(data))}
		re.RetryAfter = mcp.ParseRetryAfter(resp.Header.Get("Retry-After"))
		return nil, re
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("digitalocean API error (%d): %s", resp.StatusCode, string(data))
	}
	if resp.StatusCode == 204 || len(data) == 0 {
		return json.RawMessage(`{"status":"success"}`), nil
	}
	return json.RawMessage(data), nil
}

func (d *integration) doGet(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return d.doRequest(ctx, "GET", fmt.Sprintf(pathFmt, args...), nil)
}

func (d *integration) doPost(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return d.doRequest(ctx, "POST", path, body)
}

func (d *integration) doPut(ctx context.Context, path string, body any) (json.RawMessage, error) {
	return d.doRequest(ctx, "PUT", path, body)
}

func (d *integration) doDel(ctx context.Context, pathFmt string, args ...any) (json.RawMessage, error) {
	return d.doRequest(ctx, "DELETE", fmt.Sprintf(pathFmt, args...), nil)
}

// --- Dispatch map ---

var dispatch = map[string]handlerFunc{
	// Account
	"digitalocean_get_account": getAccount,

	// Droplets
	"digitalocean_list_droplets":    listDroplets,
	"digitalocean_get_droplet":      getDroplet,
	"digitalocean_create_droplet":   createDroplet,
	"digitalocean_delete_droplet":   deleteDroplet,
	"digitalocean_reboot_droplet":   rebootDroplet,
	"digitalocean_poweroff_droplet": powerOffDroplet,
	"digitalocean_poweron_droplet":  powerOnDroplet,

	// Kubernetes
	"digitalocean_list_kubernetes_clusters":   listKubernetesClusters,
	"digitalocean_get_kubernetes_cluster":     getKubernetesCluster,
	"digitalocean_list_kubernetes_node_pools": listKubernetesNodePools,

	// Databases
	"digitalocean_list_databases":      listDatabases,
	"digitalocean_get_database":        getDatabase,
	"digitalocean_list_database_dbs":   listDatabaseDBs,
	"digitalocean_list_database_users": listDatabaseUsers,
	"digitalocean_list_database_pools": listDatabasePools,

	// Networking
	"digitalocean_list_domains":        listDomains,
	"digitalocean_get_domain":          getDomain,
	"digitalocean_list_domain_records": listDomainRecords,
	"digitalocean_list_load_balancers": listLoadBalancers,
	"digitalocean_get_load_balancer":   getLoadBalancer,
	"digitalocean_list_firewalls":      listFirewalls,
	"digitalocean_get_firewall":        getFirewall,
	"digitalocean_list_vpcs":           listVPCs,
	"digitalocean_get_vpc":             getVPC,

	// Volumes
	"digitalocean_list_volumes": listVolumes,
	"digitalocean_get_volume":   getVolume,

	// Apps
	"digitalocean_list_apps":             listApps,
	"digitalocean_get_app":               getApp,
	"digitalocean_create_app":            createApp,
	"digitalocean_update_app":            updateApp,
	"digitalocean_delete_app":            deleteApp,
	"digitalocean_restart_app":           restartApp,
	"digitalocean_list_app_deployments":  listAppDeployments,
	"digitalocean_get_app_deployment":    getAppDeployment,
	"digitalocean_create_app_deployment": createAppDeployment,
	"digitalocean_cancel_app_deployment": cancelAppDeployment,
	"digitalocean_get_app_logs":          getAppLogs,
	"digitalocean_get_app_health":        getAppHealth,
	"digitalocean_list_app_alerts":       listAppAlerts,
	"digitalocean_rollback_app":          rollbackApp,

	// Extras
	"digitalocean_list_regions":       listRegions,
	"digitalocean_list_sizes":         listSizes,
	"digitalocean_list_images":        listImages,
	"digitalocean_list_ssh_keys":      listSSHKeys,
	"digitalocean_list_snapshots":     listSnapshots,
	"digitalocean_list_projects":      listProjects,
	"digitalocean_get_project":        getProject,
	"digitalocean_get_balance":        getBalance,
	"digitalocean_list_invoices":      listInvoices,
	"digitalocean_list_cdn_endpoints": listCDNEndpoints,
	"digitalocean_list_certificates":  listCertificates,
	"digitalocean_list_registries":    listRegistryRepositories,
	"digitalocean_list_tags":          listTags,
}
