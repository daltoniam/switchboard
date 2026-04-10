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
	"digitalocean_delete_app":            deleteApp,
	"digitalocean_restart_app":           restartApp,
	"digitalocean_list_app_deployments":  listAppDeployments,
	"digitalocean_get_app_deployment":    getAppDeployment,
	"digitalocean_create_app_deployment": createAppDeployment,
	"digitalocean_get_app_logs":          getAppLogs,
	"digitalocean_get_app_health":        getAppHealth,
	"digitalocean_list_app_alerts":       listAppAlerts,

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
