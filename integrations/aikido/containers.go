package aikido

import (
	"net/http"
	"net/url"

	mcp "github.com/daltoniam/switchboard"
)

func containerRepoID() param {
	return integer("container_repo_id", "Aikido container repository id (id from aikido_list_containers).")
}

var containerEndpoints = []endpoint{
	{
		tool: "aikido_list_containers", method: http.MethodGet, path: "/containers", scope: "repositories:read", access: mcp.AccessRead,
		desc: "List container images (container repositories) Aikido scans for vulnerabilities, with registry, scanned tag, OS distro, last scan, active state and optional running state, internet exposure, sensitivity and labels. Start here for Aikido container image security and Docker image CVEs.",
		params: ps(
			str("filter_name", "Only containers whose name matches."),
			str("filter_tag", "Only containers with this tag."),
			integer("filter_team_id", "Only containers owned by this team id."),
			str("filter_status", "Activation status (default active).").oneOf("all", "active", "inactive"),
			str("filter_reachability", "Only containers whose running images have this internet reachability.").oneOf("unknown", "direct", "lb", "limited_ips", "none"),
			boolean("include_is_running", "Include whether each image is running anywhere."),
			boolean("include_sensitivity", "Include sensitivity."),
			boolean("include_connectivity", "Include internet connectivity."),
			boolean("include_labels", "Include labels."),
		),
		page: pages(10, 100, 20),
	},
	{
		tool: "aikido_get_container", method: http.MethodGet, path: "/containers/{container_repo_id}", scope: "containers:read", access: mcp.AccessRead,
		desc:   "Get one Aikido container repository: registry, tag, distro and version, last scanned tag and digest, linked code repository, labels and optionally owning teams. Use after aikido_list_containers.",
		params: ps(containerRepoID(), boolean("include_teams", "Include the teams responsible for the container.")),
	},
	{
		tool: "aikido_list_container_runners", method: http.MethodGet, path: "/containers/{container_repo_id}/runners", scope: "containers:read", access: mcp.AccessRead,
		desc:   "List where an Aikido container image runs (AWS Lambda, ECS, Azure jobs, Kubernetes pods and more) with region, tag and digest. Use after aikido_get_container to judge exposure.",
		params: ps(containerRepoID()),
	},
	{
		tool: "aikido_get_container_registry", method: http.MethodGet, path: "/containers/registries/{registry_id}", scope: "containers:read", access: mcp.AccessRead,
		desc:   "Get one container registry connected to Aikido (name, kind, username). Registry ids come from aikido_list_containers.",
		params: ps(integer("registry_id", "Container registry id.")),
	},
	{
		tool: "aikido_export_container_sbom", method: http.MethodGet, path: "/containers/{container_repo_id}/licenses/export", scope: "repositories:read", access: mcp.AccessRead,
		desc: "Export the SBOM (software bill of materials) of one Aikido container image: OS and application packages, versions and licenses as CycloneDX JSON (default), SPDX or CSV.",
		params: ps(
			containerRepoID(),
			sbomFormat(),
			boolean("include_vex", "Include VEX exploitability analysis (CycloneDX only).").asInt(),
			boolean("include_risk_reason", "Include the license risk reason (CSV only).").asInt(),
		),
		fixed: url.Values{"format": {"sbom"}},
	},
	{
		tool: "aikido_scan_container", method: http.MethodPost, path: "/containers/{container_repo_id}/scan", scope: "containers:write", access: mcp.AccessWrite,
		desc:   "Start a new Aikido vulnerability scan of one container image.",
		params: ps(containerRepoID()),
	},
	{
		tool: "aikido_activate_container", method: http.MethodPost, path: "/containers/activate", scope: "containers:write", access: mcp.AccessWrite,
		desc:   "Activate an Aikido container repository so it is scanned. Reversible with aikido_deactivate_container.",
		params: ps(containerRepoID().req()),
	},
	{
		tool: "aikido_deactivate_container", method: http.MethodPost, path: "/containers/deactivate", scope: "containers:write", access: mcp.AccessWrite,
		desc:   "Deactivate an Aikido container repository so it is no longer scanned. Reversible with aikido_activate_container.",
		params: ps(containerRepoID().req()),
	},
	{
		tool: "aikido_add_public_container", method: http.MethodPost, path: "/containers/public", scope: "containers:write", access: mcp.AccessWrite,
		desc: "Add a public container image (for example nginx:latest from Docker Hub) for Aikido to scan for vulnerabilities.",
		params: ps(
			str("registry_kind", "Registry hosting the image.").req().oneOf("docker-hub", "aws-ecr", "github", "mcr", "elastic-co-public", "gcr", "quay", "gcp-artifact", "k8s-public"),
			str("container_repo_name", "Full image name, for example library/nginx.").req(),
			str("tag", "Image tag.").req(),
		),
	},
	{
		tool: "aikido_update_container_tag_filter", method: http.MethodPost, path: "/containers/updateTagFilter", scope: "containers:write", access: mcp.AccessWrite,
		desc: "Set which tags of an Aikido container repository are scanned, with * wildcards or semver-production style filters.",
		params: ps(
			containerRepoID().req(),
			str("tag_filter", "Tag filter, for example v* or latest."),
		),
	},
	{
		tool: "aikido_update_container_sensitivity", method: http.MethodPut, path: "/containers/{container_repo_id}/sensitivity", scope: "containers:write", access: mcp.AccessWrite,
		desc:   "Set how sensitive the data handled by an Aikido container is, which weighs issue severity.",
		params: ps(containerRepoID(), str("sensitivity", "Sensitivity level.").req().oneOf(sensitivityLevels...)),
	},
	{
		tool: "aikido_update_container_connectivity", method: http.MethodPut, path: "/containers/{container_repo_id}/internetConnection", scope: "containers:write", access: mcp.AccessWrite,
		desc:   "Set whether an Aikido container is exposed to the internet, which weighs issue severity.",
		params: ps(containerRepoID(), str("internet_exposed", "Internet exposure.").req().oneOf(connectivityLevels...)),
	},
	{
		tool: "aikido_link_container_code_repo", method: http.MethodPost, path: "/containers/linkCodeRepo", scope: "containers:write", access: mcp.AccessWrite,
		desc:   "Link an Aikido container image to the code repository that builds it, so findings map back to source.",
		params: ps(containerRepoID().req(), codeRepoID().req()),
	},
	{
		tool: "aikido_unlink_container_code_repo", method: http.MethodPost, path: "/containers/unlinkCodeRepo", scope: "containers:write", access: mcp.AccessWrite,
		desc:   "Remove the link between an Aikido container image and its code repository.",
		params: ps(containerRepoID().req()),
	},
	{
		tool: "aikido_delete_container", method: http.MethodDelete, path: "/containers/{container_repo_id}", scope: "containers:write", access: mcp.AccessDestructive,
		desc:   "Delete a public or self-reported SBOM container from Aikido with its findings. Registry-synced containers are deactivated instead.",
		params: ps(containerRepoID()),
	},
}
