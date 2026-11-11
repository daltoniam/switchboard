# Argo Workflows status inspection

The `argoworkflows` integration exposes two read-only Argo Server REST operations, tested against Argo Workflows v4.1.6:

- `argoworkflows_list_workflows`: discover workflow names, phases and timestamps in one configured namespace.
- `argoworkflows_get_workflow`: inspect one exact workflow name, progress and at most 100 failed node identifiers and phases.

This is Argo Workflows, not Argo CD. There are no workflow submissions, retries, deletions, logs, artifacts, templates, specs or input/output parameters. Provider failure messages and node display names are deliberately excluded, not merely truncated, because arbitrary text can contain credentials. Failure summaries are generated from the phase. This output allowlist applies before both ordinary MCP responses and raw script results.

## Configure

Open **Integrations → argoworkflows** in the Web UI. The integration is disabled by default. Enter:

| Credential | Environment override | Required value |
|------------|----------------------|----------------|
| `base_url` | `ARGO_WORKFLOWS_BASE_URL` | Trusted HTTPS Argo Server installation URL, optionally including a reverse-proxy prefix |
| `namespace` | `ARGO_WORKFLOWS_NAMESPACE` | One literal Kubernetes namespace |
| `token` | `ARGO_WORKFLOWS_TOKEN` | Dedicated service-account bearer token without the `Bearer ` prefix |
| `ca_cert` | `ARGO_WORKFLOWS_CA_CERT` | Optional PEM CA certificate; use literal `\n` between lines in the single-line Web UI field |

Enable and save. A successful health check lists at most one workflow. Empty namespaces are valid. TLS certificate and hostname verification are mandatory, including with a custom CA. HTTP, URL credentials, query strings, fragments, encoded installation paths and redirects are rejected. Only configure operator-approved server origins; this integration intentionally supports private deployments and is not a general-purpose URL fetcher.

Argo Server must accept client authentication (`--auth-mode client`). SSO cookies and unauthenticated server mode are not supported. The integration never mints or refreshes tokens and never falls back to administrator credentials. A 401 indicates a rejected or expired token; a 403 indicates insufficient namespace permissions.

## Least privilege

Have an administrator create a dedicated service account in the approved namespace and bind this namespaced role to it:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: switchboard-workflow-status
  namespace: approved
rules:
  - apiGroups: [argoproj.io]
    resources: [workflows]
    verbs: [get, list]
```

Do not grant watch, create, update, patch, delete, pods, pod logs, secrets, cluster-wide access or additional role bindings. Obtain an expiring token through the administrator's normal Kubernetes TokenRequest procedure. Kubernetes provides authenticated identities with the self-review permissions Argo client authentication uses; do not grant broad workload permissions to work around authentication errors.

Namespace scoping and read annotations do not certify a token's effective RBAC. Audit inherited bindings. The upstream workflow get permission can expose complete workflow objects, including parameters, outside Switchboard. Use a dedicated namespace whose complete workflow read visibility is acceptable. The disposable live acceptance spec proves that its reader can list/get and cannot submit, delete or read a workflow in another namespace; it does not certify production permissions.

## Pagination and bounds

Lists default to 20 workflows and accept `limit` from 1 through 100, an optional `label_selector`, and the opaque `next_cursor` from the previous response. Keep the limit and label selector identical across pages. `incomplete=true` means more pages remain. HTTP 410 requires restarting without the cursor. The adapter never fabricates a cursor, automatically fans out, or silently slices an oversized page.

Both reads enforce a 1 MiB decompressed upstream response limit and an overall 10-second deadline, including at most two HTTP 429 retries. `Retry-After` waits longer than three seconds fail with retry-later guidance. Oversized bodies/pages fail explicitly; narrow the label selector or page size. Failed nodes are sorted by stable identifier, capped at 100, and marked `nodes_truncated` when capped. Counts and progress are not evidence of workflow correctness.

## Verification

```bash
go test -race ./integrations/argoworkflows ./config
make test-live INTEGRATION=argoworkflows
make test-ui
make ci
```

`TestUIArgoWorkflows` uses an isolated TLS fixture and covers fresh fields, authentication errors, save/enable, restart persistence, MCP search/read, disable, and masked desktop/mobile screenshots. The live spec starts digest-pinned official Argo and K3s containers, installs minimal workflow CRDs, and seeds terminal workflow status through the Kubernetes API. No controller executes jobs or pulls user workload images. It supplies a get/list-only service-account token through the actual setup page, then verifies both tools through MCP and raw scripts. Live evidence is written to `dist/live-artifacts/live-argoworkflows.json`.
