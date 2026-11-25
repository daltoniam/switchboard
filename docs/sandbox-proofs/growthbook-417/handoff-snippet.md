### Sandbox E2E proof: GrowthBook writes (rules + experiments, native REST)

**READY TO MERGE from the sandbox side.** On `58681b8`, all 37 tools pass end to end through Switchboard MCP (native REST over HTTPS) against self-hosted GrowthBook 5.2.0. The 9 new write tools cover the full draft → publish and discard rule lifecycle and the experiment create → update → start (409 checklist, then success) → stop-with-rollout flow. Every gate override, ramp schedule, status change and unscoped new rule is refused before any request leaves Switchboard. All writes hit sandbox data only. Findings: 2 low (L1 update can unscope a rule, which is inert; L2 the audit comment isn't visible through list_audit_logs) and 1 nit, none blocking. Human review is still required, and nothing was merged.

Head `58681b8a24a7240384074c67d3c6f907f9331b7f`. Self-hosted GrowthBook **5.2.0+ffe6dab** (the image the PR's live test pins, run without Docker), MongoDB 8.0.4, PostgreSQL 17 warehouse, HTTPS via a sandbox CA.

210 PASS / 0 FAIL / 6 INFO

| Tool | Access | Result | Calls | Evidence |
|---|---|---|---|---|
| `growthbook_list_features` | read | PASS | 8 | new flags visible after writes |
| `growthbook_get_feature` | read | PASS | 9 | published NA rule visible after writes; discarded rule absent |
| `growthbook_get_feature_staleness` | read | PASS | 4 | same checks as #397, run before and after the writes |
| `growthbook_list_feature_revisions` | read | PASS | 8 | draft listed with rules before publish; published v3, discarded v4 and the start-published experiment flag revision listed after |
| `growthbook_list_experiments` | read | PASS | 8 | new experiment listed; status=stopped filter includes it |
| `growthbook_get_experiment` | read | PASS | 6 | won, released variation, analysis and linked flag after stop |
| `growthbook_get_experiment_results` | read | PASS | 2 | same checks as #397, run before and after the writes |
| `growthbook_refresh_experiment_results` | write | PASS | 2 | snapshot started against the warehouse (twice) and reached success |
| `growthbook_get_snapshot` | read | PASS | 3 | same checks as #397, run before and after the writes |
| `growthbook_list_fact_metrics` | read | PASS | 4 | same checks as #397, run before and after the writes |
| `growthbook_get_fact_metric` | read | PASS | 2 | same checks as #397, run before and after the writes |
| `growthbook_list_metrics` | read | PASS | 2 | same checks as #397, run before and after the writes |
| `growthbook_list_fact_tables` | read | PASS | 2 | same checks as #397, run before and after the writes |
| `growthbook_list_projects` | read | PASS | 2 | same checks as #397, run before and after the writes |
| `growthbook_list_environments` | read | PASS | 2 | same checks as #397, run before and after the writes |
| `growthbook_list_sdk_connections` | read | PASS | 2 | same checks as #397, run before and after the writes |
| `growthbook_list_saved_groups` | read | PASS | 2 | same checks as #397, run before and after the writes |
| `growthbook_get_saved_group` | read | PASS | 3 | same checks as #397, run before and after the writes |
| `growthbook_list_attributes` | read | PASS | 2 | same checks as #397, run before and after the writes |
| `growthbook_list_data_sources` | read | PASS | 2 | same checks as #397, run before and after the writes |
| `growthbook_list_segments` | read | PASS | 2 | same checks as #397, run before and after the writes |
| `growthbook_list_audit_logs` | read | PASS | 10 | feature.publish + feature.revision.update events and experiment start/stop listed; cursor paging; publish comment not in compact output (finding L2) |
| `growthbook_list_code_refs` | read | PASS | 2 | same checks as #397, run before and after the writes |
| `growthbook_get_feature_code_refs` | read | PASS | 2 | same checks as #397, run before and after the writes |
| `growthbook_get_settings` | read | PASS | 2 | same checks as #397, run before and after the writes |
| `growthbook_list_members` | read | PASS | 2 | same checks as #397, run before and after the writes |
| `growthbook_toggle_feature` | write | PASS | 6 | production off→on→off (direct REST) |
| `growthbook_create_feature` | write | PASS | 6 | flags created disabled, duplicate rejected |
| `growthbook_add_feature_rule` | write (new) | PASS | 21 | new draft (title/comment) force rule; 2nd rule into same version; JSON-string rule; experiment-ref rule; draft not live (direct REST). Rejected locally with 0 requests to GrowthBook: no environments / [] / allEnvironments:false, bad type, all 7 gate overrides, rampSchedule, schedule, non-object rule. GrowthBook 400s surfaced: unknown env, published version |
| `growthbook_update_feature_rule` | write (new) | PASS | 15 | condition/value/description change; rollout coverage 0.25→0.5. Rejected: 7 overrides + rampSchedule, type change, empty patch, version 0, edit of a published version. Edge: environments:[] accepted (finding L1) |
| `growthbook_delete_feature_rule` | write (new) | PASS | 3 | temp rollout removed from the draft and never went live; version "abc" and `../` rule_id rejected |
| `growthbook_publish_feature_revision` | write (new) | PASS | 5 | v3 published with audit comment; rule live in production (direct REST + UI); blank comment, discarded draft, unknown ignoreWarnings arg rejected; comment stored in audit details.context |
| `growthbook_discard_feature_revision` | write (new) | PASS | 1 | draft v4 discarded, live revision unchanged (direct REST + UI), discarded draft cannot be published |
| `growthbook_create_experiment` | write (new) | PASS | 11 | draft with project/datasource/assignmentQuery/variations; status:running, 7 overrides, 1 variation, missing trackingKey rejected with 0 requests to GrowthBook |
| `growthbook_update_experiment` | write (new) | PASS | 14 | goal + guardrail metric, hypothesis, description persisted (direct REST + UI); edit while running; status running/stopped/draft, linkedChanges, 7 overrides, empty changes rejected with 0 requests |
| `growthbook_start_experiment` | write (new) | PASS | 4 | 409 checklist_incomplete (goal metric + linked flag) surfaced, experiment stays draft; after update + experiment-ref draft, start → running and publishes the linked draft (publishedFeatureDrafts); extra skipChecklist arg rejected; unknown id 404 |
| `growthbook_stop_experiment` | write (new) | PASS | 4 | won + winner + enable_temporary_rollout of NA Banner → stopped, resultSummary.releasedVariationId set, excludeFromPayload false; invalid results, rollout without released id, `../` id rejected |

**Findings**

No blocking bugs. Two low-severity findings and one nit, none of which let an agent bypass a GrowthBook gate:

- **L1 (low) `growthbook_update_feature_rule` doesn't re-check rule scope.** `add_feature_rule` requires `environments` or `allEnvironments: true`, but an update patch of `{"environments": []}` is forwarded. GrowthBook 5.2.0 accepts it, and the draft publishes with no `allowUnscopedRules` gate. The published rule is inert: it isn't in any environment's SDK definition, so production isn't targeted. It does break the "rule must name its environments" rule the tool states. Repro: add a staging rule, call update_feature_rule with `rule: {"environments": []}`, then publish. Suggested fix: reject a patch that leaves `environments` empty without `allEnvironments: true`, matching the add check.
- **L2 (low, compact spec) the publish audit comment can't be read back through MCP.** GrowthBook records the publish `comment` in the `feature.publish` audit event under `details.context.comment` (checked by direct REST). The `growthbook_list_audit_logs` compact spec keeps id, event, entity, user.name, reason and dateCreated and drops `details`, so neither the comment nor the rule.add/update/delete context shows. The spec predates this PR, but the new publish tool says the comment is "recorded in the audit log". Suggested fix: surface `details.context` (it is small) without the large pre/post blobs.
- **Nit:** the 409 from `growthbook_start_experiment` passes GrowthBook's text through word for word, including `(retry with "skipChecklist": true)`. The tool can't send that flag (it rejects it as an unknown parameter), so the agent may be confused. Adding "a person must resolve this in GrowthBook" to the error would match the tool description.

**Caveats**

- **Hosting:** the official `growthbook/growthbook` image `5.2.0+ffe6dab` (the digest the PR's live test pins), with its layers re-extracted from the cached Docker Hub blobs and run under `chroot` (the box has no Docker). Also MongoDB 8.0.4, a PostgreSQL 17 warehouse, and a local TLS proxy with a sandbox CA (`ca_cert`). Native REST only; remotemcp was not used.
- **Fresh state:** Mongo was wiped and re-seeded for the final run (`harness/reset-and-seed.sh`). Warehouse timestamps were moved to the last 24 hours so the seeded experiment window covers them, because the #397 data had aged out of the 5-day phase.
- **Admin key:** self-hosted OSS only issues admin-role Secret Keys (`readonly` → HTTP 402).
- **Not reachable on OSS (INFO):** the `pending_confirmation` 202 path, required reviews and approvals. The gate-as-error behavior was shown live through the launch checklist 409, and the unit tests cover the 202 path.
- **GrowthBook limitation (INFO):** revision titles are stored (the UI shows "US market launch") but GrowthBook's REST revision payload leaves them out, so the adapter can't echo them.
- **Guardrails are client-side:** for every rejected override, ramp, status and scope call, the TLS proxy log shows **0** requests reaching GrowthBook. Unknown top-level arguments (such as `skipChecklist` on start, or `ignoreWarnings` on publish) are rejected by Switchboard's parameter validation.
- **Secrets:** the artifacts are scrubbed (`secret_…`, `sdk-…`, passwords). See leak-scan.txt.

Artifacts: REPORT.md, pass-fail.md, proof.mp4, responses.json, mcp-responses.json, responses/, screenshots/.
