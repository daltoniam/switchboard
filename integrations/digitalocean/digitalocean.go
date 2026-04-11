package digitalocean

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/digitalocean/godo"
	"golang.org/x/oauth2"

	mcp "github.com/daltoniam/switchboard"
)

var _ mcp.Integration = (*integration)(nil)

type integration struct {
	client *godo.Client
}

func New() mcp.Integration {
	return &integration{}
}

func (d *integration) Name() string { return "digitalocean" }

func (d *integration) Configure(ctx context.Context, creds mcp.Credentials) error {
	token := creds["api_token"]
	if token == "" {
		return fmt.Errorf("digitalocean: api_token is required")
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	oauthClient := oauth2.NewClient(ctx, ts)
	d.client = godo.NewClient(oauthClient)

	if v := creds["base_url"]; v != "" {
		d.client.BaseURL, _ = d.client.BaseURL.Parse(strings.TrimRight(v, "/") + "/")
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

func (d *integration) Execute(ctx context.Context, toolName mcp.ToolName, args map[string]any) (*mcp.ToolResult, error) {
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

// --- Dispatch map ---

var dispatch = map[mcp.ToolName]handlerFunc{
	// Account
	mcp.ToolName("digitalocean_get_account"): getAccount,

	// Droplets
	mcp.ToolName("digitalocean_list_droplets"):    listDroplets,
	mcp.ToolName("digitalocean_get_droplet"):      getDroplet,
	mcp.ToolName("digitalocean_create_droplet"):   createDroplet,
	mcp.ToolName("digitalocean_delete_droplet"):   deleteDroplet,
	mcp.ToolName("digitalocean_reboot_droplet"):   rebootDroplet,
	mcp.ToolName("digitalocean_poweroff_droplet"): powerOffDroplet,
	mcp.ToolName("digitalocean_poweron_droplet"):  powerOnDroplet,

	// Kubernetes
	mcp.ToolName("digitalocean_list_kubernetes_clusters"):   listKubernetesClusters,
	mcp.ToolName("digitalocean_get_kubernetes_cluster"):     getKubernetesCluster,
	mcp.ToolName("digitalocean_list_kubernetes_node_pools"): listKubernetesNodePools,

	// Databases
	mcp.ToolName("digitalocean_list_databases"):      listDatabases,
	mcp.ToolName("digitalocean_get_database"):        getDatabase,
	mcp.ToolName("digitalocean_list_database_dbs"):   listDatabaseDBs,
	mcp.ToolName("digitalocean_list_database_users"): listDatabaseUsers,
	mcp.ToolName("digitalocean_list_database_pools"): listDatabasePools,

	// Networking
	mcp.ToolName("digitalocean_list_domains"):        listDomains,
	mcp.ToolName("digitalocean_get_domain"):          getDomain,
	mcp.ToolName("digitalocean_list_domain_records"): listDomainRecords,
	mcp.ToolName("digitalocean_list_load_balancers"): listLoadBalancers,
	mcp.ToolName("digitalocean_get_load_balancer"):   getLoadBalancer,
	mcp.ToolName("digitalocean_list_firewalls"):      listFirewalls,
	mcp.ToolName("digitalocean_get_firewall"):        getFirewall,
	mcp.ToolName("digitalocean_list_vpcs"):           listVPCs,
	mcp.ToolName("digitalocean_get_vpc"):             getVPC,

	// Volumes
	mcp.ToolName("digitalocean_list_volumes"): listVolumes,
	mcp.ToolName("digitalocean_get_volume"):   getVolume,

	// Apps
	mcp.ToolName("digitalocean_list_apps"):             listApps,
	mcp.ToolName("digitalocean_get_app"):               getApp,
	mcp.ToolName("digitalocean_delete_app"):            deleteApp,
	mcp.ToolName("digitalocean_restart_app"):           restartApp,
	mcp.ToolName("digitalocean_list_app_deployments"):  listAppDeployments,
	mcp.ToolName("digitalocean_get_app_deployment"):    getAppDeployment,
	mcp.ToolName("digitalocean_create_app_deployment"): createAppDeployment,
	mcp.ToolName("digitalocean_get_app_logs"):          getAppLogs,
	mcp.ToolName("digitalocean_get_app_health"):        getAppHealth,
	mcp.ToolName("digitalocean_list_app_alerts"):       listAppAlerts,

	// Extras
	mcp.ToolName("digitalocean_list_regions"):       listRegions,
	mcp.ToolName("digitalocean_list_sizes"):         listSizes,
	mcp.ToolName("digitalocean_list_images"):        listImages,
	mcp.ToolName("digitalocean_list_ssh_keys"):      listSSHKeys,
	mcp.ToolName("digitalocean_list_snapshots"):     listSnapshots,
	mcp.ToolName("digitalocean_list_projects"):      listProjects,
	mcp.ToolName("digitalocean_get_project"):        getProject,
	mcp.ToolName("digitalocean_get_balance"):        getBalance,
	mcp.ToolName("digitalocean_list_invoices"):      listInvoices,
	mcp.ToolName("digitalocean_list_cdn_endpoints"): listCDNEndpoints,
	mcp.ToolName("digitalocean_list_certificates"):  listCertificates,
	mcp.ToolName("digitalocean_list_registries"):    listRegistryRepositories,
	mcp.ToolName("digitalocean_list_tags"):          listTags,
}
