package kubernetes

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	mcp "github.com/daltoniam/switchboard"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
)

func listContexts(_ context.Context, k *kubernetes, _ map[string]any) (*mcp.ToolResult, error) {
	data, err := k.contextConfig()
	if err != nil {
		return mcp.ErrResult(err)
	}
	cfg, err := clientcmd.Load(data)
	if err != nil {
		return mcp.ErrResult(fmt.Errorf("kubernetes: load kubeconfig contexts: %w", err))
	}
	out := make([]contextSummary, 0, len(cfg.Contexts))
	for name, ctx := range cfg.Contexts {
		out = append(out, contextSummary{
			Name:      name,
			Current:   name == cfg.CurrentContext,
			Cluster:   ctx.Cluster,
			AuthInfo:  ctx.AuthInfo,
			Namespace: ctx.Namespace,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Current != out[j].Current {
			return out[i].Current
		}
		return out[i].Name < out[j].Name
	})
	return mcp.JSONResult(out)
}

func listNamespaces(ctx context.Context, k *kubernetes, _ map[string]any) (*mcp.ToolResult, error) {
	items, err := k.client.CoreV1().Namespaces().List(ctx, listOpts(0, "", ""))
	if err != nil {
		return mcp.ErrResult(fmt.Errorf("kubernetes: list namespaces: %w", err))
	}
	out := make([]namespaceSummary, 0, len(items.Items))
	for _, ns := range items.Items {
		out = append(out, namespaceSummary{
			Name:              ns.Name,
			Status:            string(ns.Status.Phase),
			Labels:            ns.Labels,
			CreationTimestamp: ns.CreationTimestamp.Time,
		})
	}
	return mcp.JSONResult(out)
}

func listPods(ctx context.Context, k *kubernetes, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	labelSelector := r.Str("label_selector")
	fieldSelector := r.Str("field_selector")
	limit := boundedLimit(r.Int("limit"), 100, 500)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	ns, err := namespaceFromArgs(k, args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	items, err := k.client.CoreV1().Pods(ns).List(ctx, listOpts(limit, labelSelector, fieldSelector))
	if err != nil {
		return mcp.ErrResult(fmt.Errorf("kubernetes: list pods: %w", err))
	}
	out := make([]podSummary, 0, len(items.Items))
	for _, pod := range items.Items {
		out = append(out, summarizePod(pod))
	}
	return mcp.JSONResult(out)
}

func getPod(ctx context.Context, k *kubernetes, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	podName := r.Str("pod")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	ns, err := namespaceFromArgs(k, args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	pod, err := k.client.CoreV1().Pods(ns).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return mcp.ErrResult(fmt.Errorf("kubernetes: get pod: %w", err))
	}
	return mcp.JSONResult(summarizePod(*pod))
}

func readPodLogs(ctx context.Context, k *kubernetes, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	podName := r.Str("pod")
	container := r.Str("container")
	tail := boundedLimit(r.Int("tail"), 200, 2000)
	previous := r.Bool("previous")
	timestamps := r.Bool("timestamps")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	ns, err := namespaceFromArgs(k, args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	opts := &corev1.PodLogOptions{Container: container, TailLines: &tail, Previous: previous, Timestamps: timestamps}
	stream, err := k.client.CoreV1().Pods(ns).GetLogs(podName, opts).Stream(ctx)
	if err != nil {
		return mcp.ErrResult(fmt.Errorf("kubernetes: read pod logs: %w", err))
	}
	defer func() { _ = stream.Close() }()
	data, err := io.ReadAll(io.LimitReader(stream, 1024*1024))
	if err != nil {
		return mcp.ErrResult(fmt.Errorf("kubernetes: read pod logs: %w", err))
	}
	return mcp.JSONResult(podLogs{Namespace: ns, Pod: podName, Container: container, TailLines: tail, Previous: previous, Timestamps: timestamps, Logs: string(data)})
}

func listEvents(ctx context.Context, k *kubernetes, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	fieldSelector := r.Str("field_selector")
	limit := boundedLimit(r.Int("limit"), 100, 500)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	ns, err := namespaceFromArgs(k, args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	items, err := k.client.CoreV1().Events(ns).List(ctx, listOpts(limit, "", fieldSelector))
	if err != nil {
		return mcp.ErrResult(fmt.Errorf("kubernetes: list events: %w", err))
	}
	out := make([]eventSummary, 0, len(items.Items))
	for _, event := range items.Items {
		out = append(out, summarizeEvent(event))
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Time.After(out[j].Time) })
	return mcp.JSONResult(out)
}

func listDeployments(ctx context.Context, k *kubernetes, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	labelSelector := r.Str("label_selector")
	limit := boundedLimit(r.Int("limit"), 100, 500)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	ns, err := namespaceFromArgs(k, args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	items, err := k.client.AppsV1().Deployments(ns).List(ctx, listOpts(limit, labelSelector, ""))
	if err != nil {
		return mcp.ErrResult(fmt.Errorf("kubernetes: list deployments: %w", err))
	}
	out := make([]deploymentSummary, 0, len(items.Items))
	for _, deployment := range items.Items {
		out = append(out, summarizeDeployment(deployment))
	}
	return mcp.JSONResult(out)
}

func getDeployment(ctx context.Context, k *kubernetes, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("deployment")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	ns, err := namespaceFromArgs(k, args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	deployment, err := k.client.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return mcp.ErrResult(fmt.Errorf("kubernetes: get deployment: %w", err))
	}
	return mcp.JSONResult(summarizeDeployment(*deployment))
}

func rolloutStatus(ctx context.Context, k *kubernetes, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	name := r.Str("deployment")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	ns, err := namespaceFromArgs(k, args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	deployment, err := k.client.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return mcp.ErrResult(fmt.Errorf("kubernetes: rollout status: %w", err))
	}
	return mcp.JSONResult(summarizeRollout(*deployment))
}

func listServices(ctx context.Context, k *kubernetes, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	labelSelector := r.Str("label_selector")
	limit := boundedLimit(r.Int("limit"), 100, 500)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	ns, err := namespaceFromArgs(k, args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	items, err := k.client.CoreV1().Services(ns).List(ctx, listOpts(limit, labelSelector, ""))
	if err != nil {
		return mcp.ErrResult(fmt.Errorf("kubernetes: list services: %w", err))
	}
	out := make([]serviceSummary, 0, len(items.Items))
	for _, svc := range items.Items {
		out = append(out, summarizeService(svc))
	}
	return mcp.JSONResult(out)
}

func listIngresses(ctx context.Context, k *kubernetes, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	labelSelector := r.Str("label_selector")
	limit := boundedLimit(r.Int("limit"), 100, 500)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	ns, err := namespaceFromArgs(k, args)
	if err != nil {
		return mcp.ErrResult(err)
	}
	items, err := k.client.NetworkingV1().Ingresses(ns).List(ctx, listOpts(limit, labelSelector, ""))
	if err != nil {
		return mcp.ErrResult(fmt.Errorf("kubernetes: list ingresses: %w", err))
	}
	out := make([]ingressSummary, 0, len(items.Items))
	for _, ingress := range items.Items {
		out = append(out, summarizeIngress(ingress))
	}
	return mcp.JSONResult(out)
}

func listNodes(ctx context.Context, k *kubernetes, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	labelSelector := r.Str("label_selector")
	limit := boundedLimit(r.Int("limit"), 100, 500)
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	items, err := k.client.CoreV1().Nodes().List(ctx, listOpts(limit, labelSelector, ""))
	if err != nil {
		return mcp.ErrResult(fmt.Errorf("kubernetes: list nodes: %w", err))
	}
	out := make([]nodeSummary, 0, len(items.Items))
	for _, node := range items.Items {
		out = append(out, summarizeNode(node))
	}
	return mcp.JSONResult(out)
}

func listOpts(limit int64, labelSelector, fieldSelector string) metav1.ListOptions {
	return metav1.ListOptions{Limit: limit, LabelSelector: labelSelector, FieldSelector: fieldSelector}
}

func summarizePod(pod corev1.Pod) podSummary {
	containers := make([]containerSummary, 0, len(pod.Status.ContainerStatuses))
	restarts := int32(0)
	ready := 0
	for _, status := range pod.Status.ContainerStatuses {
		restarts += status.RestartCount
		if status.Ready {
			ready++
		}
		containers = append(containers, containerSummary{Name: status.Name, Ready: status.Ready, RestartCount: status.RestartCount, Image: status.Image})
	}
	return podSummary{
		Namespace:         pod.Namespace,
		Name:              pod.Name,
		Phase:             string(pod.Status.Phase),
		Node:              pod.Spec.NodeName,
		PodIP:             pod.Status.PodIP,
		HostIP:            pod.Status.HostIP,
		Labels:            pod.Labels,
		ReadyContainers:   ready,
		TotalContainers:   len(pod.Spec.Containers),
		RestartCount:      restarts,
		Containers:        containers,
		Conditions:        summarizePodConditions(pod.Status.Conditions),
		Owners:            summarizeOwners(pod.OwnerReferences),
		CreationTimestamp: pod.CreationTimestamp.Time,
	}
}

func summarizePodConditions(conditions []corev1.PodCondition) []conditionSummary {
	out := make([]conditionSummary, 0, len(conditions))
	for _, condition := range conditions {
		out = append(out, conditionSummary{Type: string(condition.Type), Status: string(condition.Status), Reason: condition.Reason, Message: condition.Message})
	}
	return out
}

func summarizeDeployment(deployment appsv1.Deployment) deploymentSummary {
	images := []string{}
	seen := map[string]bool{}
	for _, container := range deployment.Spec.Template.Spec.Containers {
		if !seen[container.Image] {
			images = append(images, container.Image)
			seen[container.Image] = true
		}
	}
	return deploymentSummary{
		Namespace:           deployment.Namespace,
		Name:                deployment.Name,
		Labels:              deployment.Labels,
		Replicas:            int32Value(deployment.Spec.Replicas),
		UpdatedReplicas:     deployment.Status.UpdatedReplicas,
		ReadyReplicas:       deployment.Status.ReadyReplicas,
		AvailableReplicas:   deployment.Status.AvailableReplicas,
		UnavailableReplicas: deployment.Status.UnavailableReplicas,
		Selector:            metav1.FormatLabelSelector(deployment.Spec.Selector),
		Images:              images,
		Strategy:            string(deployment.Spec.Strategy.Type),
		Conditions:          summarizeDeploymentConditions(deployment.Status.Conditions),
		CreationTimestamp:   deployment.CreationTimestamp.Time,
	}
}

func summarizeDeploymentConditions(conditions []appsv1.DeploymentCondition) []conditionSummary {
	out := make([]conditionSummary, 0, len(conditions))
	for _, condition := range conditions {
		out = append(out, conditionSummary{Type: string(condition.Type), Status: string(condition.Status), Reason: condition.Reason, Message: condition.Message})
	}
	return out
}

func summarizeRollout(deployment appsv1.Deployment) rolloutSummary {
	summary := summarizeDeployment(deployment)
	status := "complete"
	message := "deployment rollout is complete"
	if deployment.Generation > deployment.Status.ObservedGeneration {
		status = "progressing"
		message = "deployment controller has not observed the latest generation"
	} else if summary.UpdatedReplicas < summary.Replicas {
		status = "progressing"
		message = "not all replicas are updated"
	} else if summary.AvailableReplicas < summary.Replicas {
		status = "progressing"
		message = "not all replicas are available"
	}
	for _, condition := range deployment.Status.Conditions {
		if condition.Type == appsv1.DeploymentProgressing && condition.Status == corev1.ConditionFalse {
			status = "stuck"
			message = condition.Message
		}
	}
	return rolloutSummary{Deployment: summary, Status: status, Message: message}
}

func summarizeEvent(event corev1.Event) eventSummary {
	t := event.LastTimestamp.Time
	if t.IsZero() {
		t = event.EventTime.Time
	}
	if t.IsZero() {
		t = event.CreationTimestamp.Time
	}
	return eventSummary{
		Namespace:    event.Namespace,
		Name:         event.Name,
		Type:         event.Type,
		Reason:       event.Reason,
		Message:      event.Message,
		Count:        event.Count,
		InvolvedKind: event.InvolvedObject.Kind,
		InvolvedName: event.InvolvedObject.Name,
		InvolvedUID:  string(event.InvolvedObject.UID),
		Source:       event.Source.Component,
		Reporting:    event.ReportingController,
		Time:         t,
	}
}

func summarizeService(svc corev1.Service) serviceSummary {
	ports := make([]servicePortSummary, 0, len(svc.Spec.Ports))
	for _, port := range svc.Spec.Ports {
		ports = append(ports, servicePortSummary{Name: port.Name, Protocol: string(port.Protocol), Port: port.Port, TargetPort: port.TargetPort.String(), NodePort: port.NodePort})
	}
	return serviceSummary{
		Namespace:         svc.Namespace,
		Name:              svc.Name,
		Type:              string(svc.Spec.Type),
		ClusterIP:         svc.Spec.ClusterIP,
		ClusterIPs:        svc.Spec.ClusterIPs,
		ExternalIPs:       svc.Spec.ExternalIPs,
		LoadBalancerIPs:   loadBalancerIPs(svc.Status.LoadBalancer.Ingress),
		Ports:             ports,
		Selector:          svc.Spec.Selector,
		CreationTimestamp: svc.CreationTimestamp.Time,
	}
}

func summarizeIngress(ingress networkingv1.Ingress) ingressSummary {
	rules := make([]ingressRuleSummary, 0, len(ingress.Spec.Rules))
	for _, rule := range ingress.Spec.Rules {
		paths := []string{}
		if rule.HTTP != nil {
			for _, path := range rule.HTTP.Paths {
				paths = append(paths, path.Path)
			}
		}
		rules = append(rules, ingressRuleSummary{Host: rule.Host, Paths: paths})
	}
	tls := make([]ingressTLSSummary, 0, len(ingress.Spec.TLS))
	for _, item := range ingress.Spec.TLS {
		tls = append(tls, ingressTLSSummary{Hosts: item.Hosts, SecretName: item.SecretName})
	}
	className := ""
	if ingress.Spec.IngressClassName != nil {
		className = *ingress.Spec.IngressClassName
	}
	return ingressSummary{
		Namespace:         ingress.Namespace,
		Name:              ingress.Name,
		ClassName:         className,
		Rules:             rules,
		TLS:               tls,
		LoadBalancerIPs:   ingressLoadBalancerIPs(ingress.Status.LoadBalancer.Ingress),
		CreationTimestamp: ingress.CreationTimestamp.Time,
	}
}

func summarizeNode(node corev1.Node) nodeSummary {
	roles := []string{}
	for key := range node.Labels {
		if strings.HasPrefix(key, "node-role.kubernetes.io/") {
			role := strings.TrimPrefix(key, "node-role.kubernetes.io/")
			if role == "" {
				role = "master"
			}
			roles = append(roles, role)
		}
	}
	sort.Strings(roles)
	return nodeSummary{
		Name:              node.Name,
		Ready:             nodeReady(node.Status.Conditions),
		Roles:             roles,
		KubeletVersion:    node.Status.NodeInfo.KubeletVersion,
		OSImage:           node.Status.NodeInfo.OSImage,
		ContainerRuntime:  node.Status.NodeInfo.ContainerRuntimeVersion,
		Capacity:          resourceList(node.Status.Capacity),
		Allocatable:       resourceList(node.Status.Allocatable),
		Taints:            summarizeTaints(node.Spec.Taints),
		Labels:            node.Labels,
		CreationTimestamp: node.CreationTimestamp.Time,
	}
}

func summarizeOwners(owners []metav1.OwnerReference) []ownerSummary {
	out := make([]ownerSummary, 0, len(owners))
	for _, owner := range owners {
		out = append(out, ownerSummary{Kind: owner.Kind, Name: owner.Name})
	}
	return out
}

func summarizeTaints(taints []corev1.Taint) []taintSummary {
	out := make([]taintSummary, 0, len(taints))
	for _, taint := range taints {
		out = append(out, taintSummary{Key: taint.Key, Value: taint.Value, Effect: string(taint.Effect)})
	}
	return out
}

func nodeReady(conditions []corev1.NodeCondition) bool {
	for _, condition := range conditions {
		if condition.Type == corev1.NodeReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

func resourceList(values corev1.ResourceList) map[string]string {
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[string(key)] = resourceValue(value)
	}
	return out
}

func resourceValue(value resource.Quantity) string {
	return value.String()
}

func loadBalancerIPs(values []corev1.LoadBalancerIngress) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value.IP != "" {
			out = append(out, value.IP)
		}
		if value.Hostname != "" {
			out = append(out, value.Hostname)
		}
	}
	return out
}

func ingressLoadBalancerIPs(values []networkingv1.IngressLoadBalancerIngress) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value.IP != "" {
			out = append(out, value.IP)
		}
		if value.Hostname != "" {
			out = append(out, value.Hostname)
		}
	}
	return out
}

func int32Value(value *int32) int32 {
	if value == nil {
		return 0
	}
	return *value
}

type contextSummary struct {
	Name      string `json:"name"`
	Current   bool   `json:"current"`
	Cluster   string `json:"cluster"`
	AuthInfo  string `json:"auth_info"`
	Namespace string `json:"namespace,omitempty"`
}

type namespaceSummary struct {
	Name              string            `json:"name"`
	Status            string            `json:"status"`
	Labels            map[string]string `json:"labels,omitempty"`
	CreationTimestamp time.Time         `json:"creation_timestamp"`
}

type podSummary struct {
	Namespace         string             `json:"namespace"`
	Name              string             `json:"name"`
	Phase             string             `json:"phase"`
	Node              string             `json:"node,omitempty"`
	PodIP             string             `json:"pod_ip,omitempty"`
	HostIP            string             `json:"host_ip,omitempty"`
	Labels            map[string]string  `json:"labels,omitempty"`
	ReadyContainers   int                `json:"ready_containers"`
	TotalContainers   int                `json:"total_containers"`
	RestartCount      int32              `json:"restart_count"`
	Containers        []containerSummary `json:"containers,omitempty"`
	Conditions        []conditionSummary `json:"conditions,omitempty"`
	Owners            []ownerSummary     `json:"owners,omitempty"`
	CreationTimestamp time.Time          `json:"creation_timestamp"`
}

type containerSummary struct {
	Name         string `json:"name"`
	Ready        bool   `json:"ready"`
	RestartCount int32  `json:"restart_count"`
	Image        string `json:"image,omitempty"`
}

type conditionSummary struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

type ownerSummary struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

type podLogs struct {
	Namespace  string `json:"namespace"`
	Pod        string `json:"pod"`
	Container  string `json:"container,omitempty"`
	TailLines  int64  `json:"tail_lines"`
	Previous   bool   `json:"previous"`
	Timestamps bool   `json:"timestamps"`
	Logs       string `json:"logs"`
}

type eventSummary struct {
	Namespace    string    `json:"namespace"`
	Name         string    `json:"name"`
	Type         string    `json:"type,omitempty"`
	Reason       string    `json:"reason,omitempty"`
	Message      string    `json:"message,omitempty"`
	Count        int32     `json:"count"`
	InvolvedKind string    `json:"involved_kind,omitempty"`
	InvolvedName string    `json:"involved_name,omitempty"`
	InvolvedUID  string    `json:"involved_uid,omitempty"`
	Source       string    `json:"source,omitempty"`
	Reporting    string    `json:"reporting,omitempty"`
	Time         time.Time `json:"time"`
}

type deploymentSummary struct {
	Namespace           string             `json:"namespace"`
	Name                string             `json:"name"`
	Labels              map[string]string  `json:"labels,omitempty"`
	Replicas            int32              `json:"replicas"`
	UpdatedReplicas     int32              `json:"updated_replicas"`
	ReadyReplicas       int32              `json:"ready_replicas"`
	AvailableReplicas   int32              `json:"available_replicas"`
	UnavailableReplicas int32              `json:"unavailable_replicas"`
	Selector            string             `json:"selector,omitempty"`
	Images              []string           `json:"images,omitempty"`
	Strategy            string             `json:"strategy,omitempty"`
	Conditions          []conditionSummary `json:"conditions,omitempty"`
	CreationTimestamp   time.Time          `json:"creation_timestamp"`
}

type rolloutSummary struct {
	Deployment deploymentSummary `json:"deployment"`
	Status     string            `json:"status"`
	Message    string            `json:"message"`
}

type serviceSummary struct {
	Namespace         string               `json:"namespace"`
	Name              string               `json:"name"`
	Type              string               `json:"type"`
	ClusterIP         string               `json:"cluster_ip,omitempty"`
	ClusterIPs        []string             `json:"cluster_ips,omitempty"`
	ExternalIPs       []string             `json:"external_ips,omitempty"`
	LoadBalancerIPs   []string             `json:"load_balancer_ips,omitempty"`
	Ports             []servicePortSummary `json:"ports,omitempty"`
	Selector          map[string]string    `json:"selector,omitempty"`
	CreationTimestamp time.Time            `json:"creation_timestamp"`
}

type servicePortSummary struct {
	Name       string `json:"name,omitempty"`
	Protocol   string `json:"protocol"`
	Port       int32  `json:"port"`
	TargetPort string `json:"target_port"`
	NodePort   int32  `json:"node_port,omitempty"`
}

type ingressSummary struct {
	Namespace         string               `json:"namespace"`
	Name              string               `json:"name"`
	ClassName         string               `json:"class_name,omitempty"`
	Rules             []ingressRuleSummary `json:"rules,omitempty"`
	TLS               []ingressTLSSummary  `json:"tls,omitempty"`
	LoadBalancerIPs   []string             `json:"load_balancer_ips,omitempty"`
	CreationTimestamp time.Time            `json:"creation_timestamp"`
}

type ingressRuleSummary struct {
	Host  string   `json:"host,omitempty"`
	Paths []string `json:"paths,omitempty"`
}

type ingressTLSSummary struct {
	Hosts      []string `json:"hosts,omitempty"`
	SecretName string   `json:"secret_name,omitempty"`
}

type nodeSummary struct {
	Name              string            `json:"name"`
	Ready             bool              `json:"ready"`
	Roles             []string          `json:"roles,omitempty"`
	KubeletVersion    string            `json:"kubelet_version,omitempty"`
	OSImage           string            `json:"os_image,omitempty"`
	ContainerRuntime  string            `json:"container_runtime,omitempty"`
	Capacity          map[string]string `json:"capacity,omitempty"`
	Allocatable       map[string]string `json:"allocatable,omitempty"`
	Taints            []taintSummary    `json:"taints,omitempty"`
	Labels            map[string]string `json:"labels,omitempty"`
	CreationTimestamp time.Time         `json:"creation_timestamp"`
}

type taintSummary struct {
	Key    string `json:"key"`
	Value  string `json:"value,omitempty"`
	Effect string `json:"effect"`
}
