package gcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/firestore"
	functions "cloud.google.com/go/functions/apiv2"
	logging "cloud.google.com/go/logging/apiv2"
	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"cloud.google.com/go/pubsub"
	resourcemanager "cloud.google.com/go/resourcemanager/apiv3"
	run "cloud.google.com/go/run/apiv2"
	"cloud.google.com/go/storage"
	"google.golang.org/api/option"

	iamv1 "google.golang.org/api/iam/v1"

	mcp "github.com/daltoniam/switchboard"
)

type integration struct {
	projectID string

	storageClient     *storage.Client
	instancesClient   *compute.InstancesClient
	disksClient       *compute.DisksClient
	networksClient    *compute.NetworksClient
	subnetworksClient *compute.SubnetworksClient
	firewallsClient   *compute.FirewallsClient
	functionsClient   *functions.FunctionClient
	iamService        *iamv1.Service
	monitoringClient  *monitoring.MetricClient
	alertClient       *monitoring.AlertPolicyClient
	runServicesClient  *run.ServicesClient
	runRevisionsClient *run.RevisionsClient
	pubsubClient      *pubsub.Client
	firestoreClient   *firestore.Client
	loggingClient     *logging.Client
	loggingConfigClient *logging.ConfigClient
	projectsClient    *resourcemanager.ProjectsClient
	foldersClient     *resourcemanager.FoldersClient
}

func New() mcp.Integration {
	return &integration{}
}

func (g *integration) Name() string { return "gcp" }

func (g *integration) Configure(creds mcp.Credentials) error {
	g.projectID = creds["project_id"]
	if g.projectID == "" {
		return fmt.Errorf("gcp: project_id is required")
	}

	ctx := context.Background()

	var opts []option.ClientOption
	if v := creds["credentials_json"]; v != "" {
		opts = append(opts, option.WithCredentialsJSON([]byte(v)))
	}

	var err error

	g.storageClient, err = storage.NewClient(ctx, opts...)
	if err != nil {
		return fmt.Errorf("gcp: storage client: %w", err)
	}

	g.instancesClient, err = compute.NewInstancesRESTClient(ctx, opts...)
	if err != nil {
		return fmt.Errorf("gcp: compute instances client: %w", err)
	}

	g.disksClient, err = compute.NewDisksRESTClient(ctx, opts...)
	if err != nil {
		return fmt.Errorf("gcp: compute disks client: %w", err)
	}

	g.networksClient, err = compute.NewNetworksRESTClient(ctx, opts...)
	if err != nil {
		return fmt.Errorf("gcp: compute networks client: %w", err)
	}

	g.subnetworksClient, err = compute.NewSubnetworksRESTClient(ctx, opts...)
	if err != nil {
		return fmt.Errorf("gcp: compute subnetworks client: %w", err)
	}

	g.firewallsClient, err = compute.NewFirewallsRESTClient(ctx, opts...)
	if err != nil {
		return fmt.Errorf("gcp: compute firewalls client: %w", err)
	}

	g.functionsClient, err = functions.NewFunctionClient(ctx, opts...)
	if err != nil {
		return fmt.Errorf("gcp: functions client: %w", err)
	}

	g.iamService, err = iamv1.NewService(ctx, opts...)
	if err != nil {
		return fmt.Errorf("gcp: iam service: %w", err)
	}

	g.monitoringClient, err = monitoring.NewMetricClient(ctx, opts...)
	if err != nil {
		return fmt.Errorf("gcp: monitoring client: %w", err)
	}

	g.alertClient, err = monitoring.NewAlertPolicyClient(ctx, opts...)
	if err != nil {
		return fmt.Errorf("gcp: alert policy client: %w", err)
	}

	g.runServicesClient, err = run.NewServicesClient(ctx, opts...)
	if err != nil {
		return fmt.Errorf("gcp: cloud run services client: %w", err)
	}

	g.runRevisionsClient, err = run.NewRevisionsClient(ctx, opts...)
	if err != nil {
		return fmt.Errorf("gcp: cloud run revisions client: %w", err)
	}

	g.pubsubClient, err = pubsub.NewClient(ctx, g.projectID, opts...)
	if err != nil {
		return fmt.Errorf("gcp: pubsub client: %w", err)
	}

	g.firestoreClient, err = firestore.NewClient(ctx, g.projectID, opts...)
	if err != nil {
		return fmt.Errorf("gcp: firestore client: %w", err)
	}

	g.loggingClient, err = logging.NewClient(ctx, opts...)
	if err != nil {
		return fmt.Errorf("gcp: logging client: %w", err)
	}

	g.loggingConfigClient, err = logging.NewConfigClient(ctx, opts...)
	if err != nil {
		return fmt.Errorf("gcp: logging config client: %w", err)
	}

	g.projectsClient, err = resourcemanager.NewProjectsClient(ctx, opts...)
	if err != nil {
		return fmt.Errorf("gcp: projects client: %w", err)
	}

	g.foldersClient, err = resourcemanager.NewFoldersClient(ctx, opts...)
	if err != nil {
		return fmt.Errorf("gcp: folders client: %w", err)
	}

	return nil
}

func (g *integration) Healthy(ctx context.Context) bool {
	if g.storageClient == nil {
		return false
	}
	it := g.storageClient.Buckets(ctx, g.projectID)
	_, _ = it.Next()
	return true
}

func (g *integration) Tools() []mcp.ToolDefinition {
	return tools
}

func (g *integration) Execute(ctx context.Context, toolName string, args map[string]any) (*mcp.ToolResult, error) {
	fn, ok := dispatch[toolName]
	if !ok {
		return &mcp.ToolResult{Data: fmt.Sprintf("unknown tool: %s", toolName), IsError: true}, nil
	}
	return fn(ctx, g, args)
}

type handlerFunc func(ctx context.Context, g *integration, args map[string]any) (*mcp.ToolResult, error)

func jsonResult(v any) (*mcp.ToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return &mcp.ToolResult{Data: err.Error(), IsError: true}, nil
	}
	return &mcp.ToolResult{Data: string(data)}, nil
}

func errResult(err error) (*mcp.ToolResult, error) {
	return &mcp.ToolResult{Data: err.Error(), IsError: true}, nil
}

func argStr(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return v
}

func argInt(args map[string]any, key string) int {
	switch v := args[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		n, _ := strconv.Atoi(v)
		return n
	}
	return 0
}

func argInt32(args map[string]any, key string) int32 {
	switch v := args[key].(type) {
	case float64:
		return int32(v)
	case int:
		return int32(v)
	case string:
		n, _ := strconv.ParseInt(v, 10, 32)
		return int32(n)
	}
	return 0
}

func argInt64(args map[string]any, key string) int64 {
	switch v := args[key].(type) {
	case float64:
		return int64(v)
	case int:
		return int64(v)
	case int64:
		return v
	case string:
		n, _ := strconv.ParseInt(v, 10, 64)
		return n
	}
	return 0
}

func argBool(args map[string]any, key string) bool {
	switch v := args[key].(type) {
	case bool:
		return v
	case string:
		return v == "true"
	}
	return false
}

func argStrSlice(args map[string]any, key string) []string {
	switch v := args[key].(type) {
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return v
	case string:
		if v == "" {
			return nil
		}
		return strings.Split(v, ",")
	}
	return nil
}

func (g *integration) projectName() string {
	return "projects/" + g.projectID
}

var dispatch = map[string]handlerFunc{
	// Resource Manager
	"gcp_get_project":    getProject,
	"gcp_list_projects":  listProjects,
	"gcp_list_folders":   listFolders,
	"gcp_get_folder":     getFolder,
	"gcp_get_iam_policy": getIAMPolicy,

	// Storage
	"gcp_storage_list_buckets":  storageListBuckets,
	"gcp_storage_get_bucket":    storageGetBucket,
	"gcp_storage_list_objects":  storageListObjects,
	"gcp_storage_get_object":    storageGetObject,
	"gcp_storage_put_object":    storagePutObject,
	"gcp_storage_delete_object": storageDeleteObject,
	"gcp_storage_copy_object":   storageCopyObject,

	// Compute Engine
	"gcp_compute_list_instances":   computeListInstances,
	"gcp_compute_get_instance":     computeGetInstance,
	"gcp_compute_start_instance":   computeStartInstance,
	"gcp_compute_stop_instance":    computeStopInstance,
	"gcp_compute_list_disks":       computeListDisks,
	"gcp_compute_list_networks":    computeListNetworks,
	"gcp_compute_list_subnetworks": computeListSubnetworks,
	"gcp_compute_list_firewalls":   computeListFirewalls,
	"gcp_compute_get_firewall":     computeGetFirewall,

	// Cloud Functions
	"gcp_functions_list":           functionsList,
	"gcp_functions_get":            functionsGet,
	"gcp_functions_get_iam_policy": functionsGetIAMPolicy,

	// IAM
	"gcp_iam_list_service_accounts":     iamListServiceAccounts,
	"gcp_iam_get_service_account":       iamGetServiceAccount,
	"gcp_iam_list_service_account_keys": iamListServiceAccountKeys,
	"gcp_iam_list_roles":                iamListRoles,
	"gcp_iam_get_role":                  iamGetRole,

	// Cloud Monitoring
	"gcp_monitoring_list_metric_descriptors":  monitoringListMetricDescriptors,
	"gcp_monitoring_list_time_series":         monitoringListTimeSeries,
	"gcp_monitoring_list_alert_policies":      monitoringListAlertPolicies,
	"gcp_monitoring_get_alert_policy":         monitoringGetAlertPolicy,
	"gcp_monitoring_list_monitored_resources": monitoringListMonitoredResources,

	// Cloud Run
	"gcp_run_list_services":  runListServices,
	"gcp_run_get_service":    runGetService,
	"gcp_run_list_revisions": runListRevisions,
	"gcp_run_get_revision":   runGetRevision,

	// Pub/Sub
	"gcp_pubsub_list_topics":        pubsubListTopics,
	"gcp_pubsub_get_topic":          pubsubGetTopic,
	"gcp_pubsub_publish":            pubsubPublish,
	"gcp_pubsub_list_subscriptions": pubsubListSubscriptions,
	"gcp_pubsub_get_subscription":   pubsubGetSubscription,
	"gcp_pubsub_pull":               pubsubPull,

	// Firestore
	"gcp_firestore_list_collections": firestoreListCollections,
	"gcp_firestore_list_documents":   firestoreListDocuments,
	"gcp_firestore_get_document":     firestoreGetDocument,
	"gcp_firestore_set_document":     firestoreSetDocument,
	"gcp_firestore_delete_document":  firestoreDeleteDocument,
	"gcp_firestore_query":            firestoreQuery,

	// Cloud Logging
	"gcp_logging_list_entries":   loggingListEntries,
	"gcp_logging_list_log_names": loggingListLogNames,
	"gcp_logging_list_sinks":    loggingListSinks,
	"gcp_logging_get_sink":      loggingGetSink,
}
