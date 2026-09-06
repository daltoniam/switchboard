# Phase 6: Change Subscriptions and External Consistency

## Goal

Make cache invalidation observable through MCP `subscriptions/listen` for both catalog API mutations and out-of-process filesystem edits. Notifications are advisory; revisions and cache TTLs remain the truth.

## BDD Success Criteria

#### Scenario: Create and delete change the resource list

- **Given** a modern client listens for `resourcesListChanged`
- **When** a project is created or deleted through canonical tools or compatibility tools
- **Then** the client receives `notifications/resources/list_changed` on the acknowledged subscription
- **And** a subsequent `resources/list` reflects the filesystem truth.

#### Scenario: Definition patch updates subscribed resources

- **Given** a client subscribes to the catalog URI and one current-definition URI
- **When** that project is patched
- **Then** it receives `notifications/resources/updated` only for subscribed changed URIs
- **And** each notification carries the subscription ID
- **And** rereading returns the new revision.

#### Scenario: External atomic file replacement is detected

- **Given** Switchboard is running and another process atomically replaces a valid project definition
- **When** the catalog watcher observes the change
- **Then** the same definition/catalog notifications are emitted
- **And** the next read reflects the file
- **And** duplicate filesystem events are coalesced.

#### Scenario: Invalid external edits do not publish false valid state

- **Given** an external process writes a malformed project definition
- **When** the watcher processes it
- **Then** the catalog summary/diagnostics change is observable
- **And** no immutable valid revision is created for malformed content
- **And** the last valid revision resource remains readable.

#### Scenario: Reconnect does not assume notification replay

- **Given** the subscription stream disconnects
- **When** the client reconnects and opens a new `subscriptions/listen`
- **Then** it rereads resources using TTL/revision rather than expecting lost events to replay
- **And** subsequent updates are received on the new subscription ID.

## Implementation Instructions

### Required files

- Create `project/events.go`:
  - Define transport-neutral catalog event types (`ProjectAdded`, `ProjectRemoved`, `DefinitionChanged`, `ContextChanged`, `DiagnosticsChanged`, `ProjectValidityChanged`) carrying project ID, old/new revision when valid, source kind, canonical root URI when context is root-specific, and changed context paths. Do not put `project://` URIs in the domain package; the MCP adapter maps semantic events to URIs.
  - Expose a bounded subscription port. Events are immutable values: publishers deep-copy path slices/maps before enqueue, and each subscriber receives independent data so one consumer cannot mutate another or catalog state. Slow consumers must coalesce/drop duplicate invalidations rather than block writes; emit metrics/logs for dropped invalidations without losing source state.
- Create `project/watch.go` and `project/watch_test.go`:
  - Watch `projects/`, configured `context/`, and known repo-local definition paths using a maintained watcher dependency or bounded polling if cross-platform watcher semantics cannot be made deterministic.
  - Maintain root-keyed idle-TTL leases, not client/subscription ownership: every successful explicit-root resolve/read refreshes a canonical-root watch for 10 minutes; a background reaper releases it after 10 minutes without another access. If a client opens a resource subscription for that exact root-qualified URI, refresh the same lease while notifications are delivered but do not require connection identity. Default/base-root watches remain process-scoped.
  - Debounce atomic rename/write bursts.
  - Re-resolve from disk before emitting semantic events.
  - Stop cleanly on context cancellation.
- Modify catalog create/patch/delete to emit the same semantic events after successful durable writes.
- Modify `server/project_catalog_server.go`:
  - Advertise resources `listChanged=true`, `subscribe=true`.
  - Bridge semantic events to SDK resource-list/resource-updated notification APIs.
  - Do not advertise tool-list changes because tools are static.
- Create `server/project_catalog_subscriptions_test.go` for official SDK and raw Streamable HTTP tests.
- Modify `cmd/server/main.go` to start/stop the watcher with daemon context.
- Add metrics in `metrics.go`/`metrics_test.go` only if existing project metrics conventions support them; minimally add structured logs with project ID, event kind, and revision but no contents.

### Notification mapping

- Global user/default-repository add/remove or valid↔invalid transition: `resources/list_changed`; update global catalog and diagnostics URI for subscribers.
- A root-local overlay validity/diagnostics change never changes global `resources/list` or the unqualified current-definition URI; update only that root-qualified context/diagnostics notification target (add optional `rootUri` to the diagnostics template) and require callers to re-run explicit-root resolve.
- Global definition/provenance/diagnostics change: update current definition (when valid), diagnostics, and catalog URI.
- Context set or content change: update the exact root-qualified context manifest/content URI from the event's canonical `rootUri`; never notify an unqualified/default URI for another worktree.
- Immutable revision resources never emit update notifications.
- Compatibility-tool mutations and canonical-tool mutations must traverse one event source; adapters must not manually synthesize inconsistent notifications.

### TDD sequence

1. Add event contract tests around real catalog mutation.
2. Add watcher tests for atomic replace, add, delete, valid↔invalid transitions, diagnostics updates, event coalescing, two out-of-tree worktrees with the same context path, root-qualified notifications, and watcher lease/TTL cleanup.
3. Add in-memory official SDK `subscriptions/listen` tests.
4. Add real Streamable HTTP disconnect/reconnect test.
5. Add slow-consumer test proving writes finish and a later read remains correct.
6. Add aliasing/race tests with two subscribers: mutate one received event's path collection and prove the other event and catalog state remain unchanged.

## End-to-End Test Plan

1. Start production catalog endpoint and watcher over a temporary config root.
2. Connect an Phase 1 pinned Go MCP client with resource-list and resource-update handlers; wait for subscription acknowledgment before mutation.
3. Create/patch/delete through canonical and compatibility MCP tools, and assert expected notifications plus reread state.
4. Atomically replace a project file using `os.WriteFile(temp)` + `os.Rename`; assert one semantic invalidation set.
5. Write malformed JSON externally; assert diagnostics change and no valid immutable revision.
6. Close the subscription HTTP response, mutate while disconnected, reconnect, reread, then mutate again and observe the new subscription.
7. Run:

```bash
go test -race ./project ./server ./cmd/server -run 'Watch|Subscription|ResourceUpdated' -count=1
```

## Anti-Cheating Audit

- Ensure notifications originate only after rereading durable filesystem truth; no event should be emitted before a write succeeds.
- Confirm watcher tests use actual file operations and rename, not direct calls to event methods.
- Verify notifications are filtered by real subscription URI and include SDK-generated subscription IDs.
- Check no test assumes replay of events during disconnect.
- Ensure event queues cannot block catalog writes indefinitely and dropped events cannot make reads stale.
- Inspect event construction/delivery for slice/map aliasing; require independent copies and the two-subscriber race test.
- Verify malformed content does not create a revision or overwrite the last valid revision file.
- Ensure context contents and definitions are not logged.
- Confirm no legacy `resources/subscribe` API is newly implemented; use 2026 `subscriptions/listen` while relying on SDK legacy support only for older clients.

## Completion Gate

- [ ] Catalog mutation events are emitted after durable writes.
- [ ] External edits produce equivalent semantic events.
- [ ] Modern list/resource subscriptions pass over Streamable HTTP.
- [ ] Disconnect/reconnect and no-replay behavior is proven.
- [ ] Invalid edits change diagnostics without minting valid revisions.
- [ ] Slow consumer cannot block writes or corrupt state.
- [ ] Resource capabilities accurately advertise listChanged/subscribe.
- [ ] Focused race tests pass with no goroutine leaks.
- [ ] `make ci` passes for this phase commit; focused tests and RED→GREEN evidence are recorded additionally.
