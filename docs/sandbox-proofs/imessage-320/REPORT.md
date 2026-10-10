# Switchboard PR #320 — Sandbox E2E proof (iMessage adapter)

- Generated: 2026-10-10 14:37 CDT
- PR: https://github.com/daltoniam/switchboard/pull/320 (branch `switchboard-imessage`)
- Head SHA: `cec573f3db60b7082a05dbc6fa6be321a9f46305`
- Transport: local SQLite read-only (modernc.org/sqlite, `mode=ro&_pragma=query_only(1)`, WAL) + AppleScript (`/usr/bin/osascript`) send. **No REST/OAuth/token.**
- Harness: PR build in isolated HOME/SWITCHBOARD_DEV_HOME, driven over MCP HTTP on :13852 (read), :13853 (send gating), :13854–13856 (setup failure modes). Linux box; no Mac and no real Messages DB used.
- Fixtures: synthetic chat.db with a Sonoma/Sequoia-shaped schema (newest 5 messages left in un-checkpointed `chat.db-wal`) + AddressBook-v22.abcddb (Sources/<uuid> + root). All names fictional, numbers +1 312/773 555-01xx, emails @example.com. Builder: `fixtures/build_fixtures.py`.

## Summary
- Unit tests: `go test ./integrations/imessage/...` PASS (28 top-level tests, 89 incl. subtests); `go vet` + `go build ./cmd/server` + `go test ./config/...` PASS.
- MCP E2E: **42 PASS / 5 FAIL / 4 INFO**, plus 1 SKIPPED (real send delivery).
- Core read/send behaviour works on realistic schema edge cases. The 5 FAILs are review findings, not crashes: compact rendering ×2 (same root cause), Recently Deleted leak ×2 (same root cause), same-name contact merge ×1.

## Per-tool
| Tool / check | Result | Evidence |
|---|---|---|
| search (discovery) | PASS | all 6 imessage_* tools returned by `search` |
| imessage_list_chats | PASS | 9 chats newest-first; group `Trail Crew 🥾` is_group + 3 participants; unread counts skip tapbacks; last-message preview from un-checkpointed WAL; name/phone-digit query (1:1 before group); pagination |
| imessage_list_chats — default compact render | FAIL | participant handles dropped whenever a contact name exists (compact.yaml nested-array spec) |
| imessage_get_chat_messages | PASS | tapbacks love/like/laugh/emphasize/question/🔥 emoji, `bp:`/`p:N/` GUIDs, 3xxx removal; threaded replies (reply_to); HEIC/PDF/MOV/CAF attachments + U+FFFC stripped; edited/unsent; attributedBody incl. >127-byte + NSMutableString; group rename event hidden; by-handle merges iMessage+SMS; paging via next_before; arg errors |
| imessage_get_chat_messages — default compact render | FAIL | reaction type/emoji and my own reactions lost; attachments reduced to path (compact.yaml); fix verified locally |
| imessage_search_messages | PASS | case-insensitive search over text + attributedBody; scope by chat_id / handle / since; tapback rows excluded; empty-query error |
| imessage_search_messages — Recently Deleted | FAIL | message only in chat_recoverable_message_join (Recently Deleted) is returned, with chat_id=0 |
| imessage_list_unread | PASS | newest-first unread incl. WAL rows; unread tapback excluded |
| imessage_list_unread — Recently Deleted | FAIL | same root cause: deleted unread message listed |
| imessage_lookup_contact | PASS | name / partial phone / email (case-insensitive) / org-only / nickname-only; merges Sources/<uuid> + root AddressBook, de-dupes phone formats; empty-query error; 'not configured' without contacts_dir on Linux |
| imessage_lookup_contact — same-name people | FAIL | two different 'Jordan Lee' cards merged into one (merge key = lowercased name) |
| imessage_send_message — gating | PASS | allow_send unset → refused; allowlist refuses non-listed `to` and group chats with non-listed members before osascript; validation errors; allowlisted chat_id / reformatted `to` pass the gate and reach the runner → `sending messages requires macOS` |
| imessage_send_message — actual delivery | SKIPPED | needs a Mac + Automation→Messages permission + a recipient who agreed to it |
| Read-only safety | PASS | sha256 of chat.db, chat.db-wal, 2× AddressBook unchanged; WAL not checkpointed; only the SQLite -shm index file was created; DSN mode=ro + query_only(1) |
| Non-Mac setup | PASS | no db_path → `imessage: requires macOS (set db_path …)`; missing file / non-Messages DB fail clearly; tools return `imessage: not configured` |

## Caveats / findings
1. **compact.yaml loses nested fields in default output (FAIL, easy fix).** `messages[].reactions[].type/emoji/by/by_name`, `messages[].attachments[].name/mime_type/path`, and `chats[].participants[].handle/name` are nested-array specs. `docs/field-compaction.md` says these overwrite each other, so only one field is kept. In the default `execute` render, msg 13 shows `reactions=["Blake Moreno"]`: the reaction type, the 🔥 emoji and my own reaction are gone, and attachments show only the path. In `list_chats`, participants show only names, with no phone or email. The adapter's raw JSON is correct (checked with script `api.call`). **Fix, checked on a local scratch build:** project the whole array (`messages[].reactions`, `messages[].attachments`, `chats[].participants`). See `compact-fixcheck.json`.
2. **Recently Deleted messages still show up (FAIL).** Since macOS 13, deleting a message removes its `chat_message_join` row and adds a `chat_recoverable_message_join` row; the `message` row stays for about 30 days. `search_messages` and `list_unread` use `LEFT JOIN chat_message_join` and never exclude these rows, so the deleted canary (id 95) comes back with `chat_id=0`. Suggested fix: inner-join `chat_message_join` (or add `NOT EXISTS` on `chat_recoverable_message_join`). This was modelled from Apple's schema in the fixture, not seen on a real Mac.
3. **Contacts with the same name get merged (FAIL, low).** `readAddressBook` merges cards by lowercased display name, so two different "Jordan Lee" cards become one contact with both numbers. That risks texting the wrong Jordan. Keying on `Z_PK` per source and merging only on a shared handle would fix it.
4. **send_allowlist matches on the last 10 digits (INFO, low).** `to:"+44 312 555 0101", service:"sms"` passes an allowlist of `+1 312 555 0101` and reaches the script runner. Without `service`, it goes to the existing +1 chat instead.
5. **Junk/filtered chats are included (INFO).** A chat with `chat.is_filtered=1` (unknown-sender spam) is listed and counts as unread. Arguably fine, but worth a reviewer decision.
6. **Setup errors (INFO, cosmetic).** A missing-file error adds the macOS Full Disk Access hint even on Linux. Agents see only `imessage: not configured`; the details are only in the server log.
7. **WAL reads:** an `-shm` index file gets created next to a copied DB on first read. This is normal SQLite WAL behaviour, and DB/WAL bytes stay the same. Reading needs a writable directory, or an existing `-shm`.
8. Fixture limits: synthetic DB shaped like Sonoma/Sequoia, not a real Mac DB. Not covered: macOS <13 schemas (adapter falls back on missing columns; unit-tested), RCS, edit history in `message_summary_info`, real TCC prompts.

## Dalton blockers (Mac-live only)
- Actual delivery of `imessage_send_message`: needs a Mac running the PR build, Automation→Messages permission for switchboard, and a recipient who has agreed to receive a test message (`allow_send=true`, `send_allowlist=<that handle>`).
- Optional: a read pass against his real `~/Library/Messages/chat.db` with Full Disk Access, to confirm on real data (attributedBody variety, Recently Deleted behaviour).

## All steps
| Phase | Step | Tool | Result | Note |
|---|---|---|---|---|
| A-read | search | `search` | PASS | 6 imessage tools listed: imessage_get_chat_messages, imessage_list_chats, imessage_list_unread, imessage_lookup_contact, imessage_search_messages, imessage_send_message |
| A-read | list_chats | `imessage_list_chats` | PASS | total=9 order=[4, 1, 2, 7, 9, 8, 6, 5, 3]; group 'Trail Crew 🥾' is_group, 3 participants, unread=1 (reaction excluded), last msg from WAL; unnamed group → 'Blake Moreno, Dev'; nickname-only contact → 'Dev' |
| A-read | list_chats_junk_filter | `imessage_list_chats` | INFO | chat 6 (chat.is_filtered=1, unknown sender spam) is listed with unread_count=1 — adapter does not consult chat.is_filtered |
| A-read | list_chats_rendered_participants | `imessage_list_chats (default compact render)` | FAIL | default rendered participants for chat 1 = ['Avery Quinn']: participants[].handle is dropped whenever a contact name exists (only participants[].name survives) — same compact.yaml nested-array cause; raw data has both |
| A-read | list_chats_query_name | `imessage_list_chats` | PASS | query=Avery → [1, 5, 2] (1:1 iMessage+SMS before group 2) |
| A-read | list_chats_query_phone | `imessage_list_chats` | PASS | phone-digit query → [2, 9] |
| A-read | list_chats_pagination | `imessage_list_chats` | PASS | limit=2 offset=2 → [2, 7] has_more=True |
| A-read | get_chat_messages_group | `imessage_get_chat_messages` | PASS | all checks: visible ids (no tapback rows, no rename event); 13 reactions laugh(Blake)+🔥(me); Casey emphasize removed by 3004; 12 question via bp: guid; 23 love + 2 attachments (mov, caf); 13 photo attachment heic, text empty (U+FFFC stripped); 18 pdf + U+FFFC stripped from text; threaded replies 12→MSG-0011, 19→MSG-0018; 22 long attributedBody (>127 bytes, 0x81 len) decoded; 25 NSMutableString dec |
| A-read | get_chat_messages_rendered_fidelity | `imessage_get_chat_messages (default compact render)` | FAIL | default rendered row for msg 13: reactions=['Blake Moreno'] attachments=['IMG_4821.HEIC'] — reaction type/emoji and my own (by=me) reactions are lost, attachments reduced to path only (name/mime_type dropped). Cause: compact.yaml lists several fields under the same nested array (messages[].reactions[].type/emoji/by/by_name, attachments[].name/mime_type/path); docs/field-compaction.md says these ov |
| A-read | get_chat_messages_direct | `imessage_get_chat_messages` | PASS | edited/unsent/tapbacks/attributedBody: OK |
| A-read | get_chat_messages_by_handle | `imessage_get_chat_messages` | PASS | handle → chat_ids=[1, 5] (iMessage + SMS threads merged, 7 msgs) |
| A-read | get_chat_messages_paging | `imessage_get_chat_messages` | PASS | page1=[23, 25, 201] next_before=2026-10-09T09:43:00-05:00 → page2=[18, 19, 22] |
| A-read | get_chat_messages_missing_args | `imessage_get_chat_messages` | PASS | chat_id or handle is required |
| A-read | get_chat_messages_unknown_chat | `imessage_get_chat_messages` | PASS | chat 999 not found; use imessage_list_chats to find a chat_id |
| A-read | get_chat_messages_bad_date | `imessage_get_chat_messages` | PASS | since must be an RFC3339 timestamp or YYYY-MM-DD date, got "last tuesday" |
| A-read | search_text | `imessage_search_messages` | PASS | attributedBody text search; tapback quoting it (202) excluded → ids=[201, 18, 10] (expected [201, 18, 10]) |
| A-read | search_text_rendered | `imessage_search_messages` | PASS | rendered result includes chat_name |
| A-read | search_case_insensitive_wal | `imessage_search_messages` | PASS | case-insensitive, WAL-only row → ids=[203] (expected [203]) |
| A-read | search_handle_scope | `imessage_search_messages` | PASS | SMS short-code scope → ids=[204, 40] (expected [204, 40]) |
| A-read | search_chat_scope | `imessage_search_messages` | PASS | chat_id scope (email 1:1) → ids=[31, 30] (expected [31, 30]) |
| A-read | search_date_scope | `imessage_search_messages` | PASS | since bound → ids=[201] (expected [201]) |
| A-read | search_no_hits | `imessage_search_messages` | PASS | no hits → empty list → ids=[] (expected []) |
| A-read | search_empty_query | `imessage_search_messages` | PASS | query is required |
| A-read | search_recently_deleted | `imessage_search_messages` | FAIL | recently-deleted message returned (id=95, chat_id=0): adapter LEFT JOINs chat_message_join and never excludes rows that only live in chat_recoverable_message_join |
| A-read | list_unread | `imessage_list_unread` | PASS | ids=[204, 203, 201, 200, 95, 60]; newest-first [204, 203, 201, 200] incl. WAL rows; unread tapback 202 excluded |
| A-read | list_unread_recently_deleted | `imessage_list_unread` | FAIL | recently-deleted unread message 95 listed (chat_id=0, no chat_name) — same root cause as search_recently_deleted |
| A-read | list_unread_junk | `imessage_list_unread` | INFO | filtered/junk sender message 60 is listed as unread |
| A-read | lookup_name_merge_sources | `imessage_lookup_contact` | PASS | case-insensitive name; Sources/<uuid> + root AddressBook merged, phone de-duplicated across formats → [{"emails": ["avery.quinn@example.com", "aq@example.com"], "name": "Avery Quinn", "phones": ["(312) 555-0101"]}] |
| A-read | lookup_partial_phone | `imessage_lookup_contact` | PASS | partial phone → nickname-only contact → [{"name": "Dev", "phones": ["773.555.0104"]}] |
| A-read | lookup_email | `imessage_lookup_contact` | PASS | email, case-insensitive → [{"emails": ["Casey.Rivera@example.com"], "name": "Casey Rivera"}] |
| A-read | lookup_org | `imessage_lookup_contact` | PASS | organization-only card → [{"name": "Example Bank", "phones": ["55501"]}] |
| A-read | lookup_no_match | `imessage_lookup_contact` | PASS | no match → empty → [] |
| A-read | lookup_same_name_people | `imessage_lookup_contact` | FAIL | two distinct cards both named 'Jordan Lee' (different numbers) should stay separate → [{"emails": ["jordan.lee.two@example.com"], "name": "Jordan Lee", "phones": ["(312) 555-0107", "(773) 555-0108"]}] |
| A-read | lookup_empty_query | `imessage_lookup_contact` | PASS | query is required |
| A-read | send_disabled_default | `imessage_send_message` | PASS | sending is disabled; set allow_send to true in the imessage integration settings to enable imessage_send_message |
| A-read | read_only_hash | `sha256 fixtures before/after` | PASS | 5 files unchanged byte-for-byte (chat.db, chat.db-wal, 2× AddressBook); WAL not checkpointed into chat.db; new sidecar files: ['Messages/chat.db-shm'] (SQLite WAL shared-memory index created by the read-only reader; contents of DB/WAL untouched) |
| B-send | send_to_not_allowlisted | `imessage_send_message` | PASS | refused before osascript: recipient +13125550102 is not in send_allowlist |
| B-send | send_group_not_allowlisted | `imessage_send_message` | PASS | group has non-allowlisted members → refused before osascript: recipient +13125550102 is not in send_allowlist |
| B-send | send_unknown_chat | `imessage_send_message` | PASS | validation: chat 999 not found; use imessage_list_chats to find a chat_id |
| B-send | send_empty_text | `imessage_send_message` | PASS | validation: text is required |
| B-send | send_bad_service | `imessage_send_message` | PASS | validation: service must be 'imessage' or 'sms', got "fax" |
| B-send | send_missing_target | `imessage_send_message` | PASS | validation: chat_id or to is required |
| B-send | send_allowlisted_chat | `imessage_send_message` | PASS | passed gate → reached script runner → platform error (no osascript on Linux); delivery SKIPPED: messages app rejected the send: sending messages requires macOS with the Messages app (on first use, allow switchboard to control Messages in System Settings > Privacy &  |
| B-send | send_allowlisted_to_reformatted | `imessage_send_message` | PASS | allowlist normalizes formatting; reached runner → platform error; delivery SKIPPED: messages app rejected the send: sending messages requires macOS with the Messages app (on first use, allow switchboard to control Messages in System Settings > Privacy &  |
| B-send | send_allowlist_last10_digits | `imessage_send_message` | INFO | a +44 number sharing the allowlisted number's last 10 digits PASSES the allowlist and reaches the script runner (normalizeHandle compares last 10 digits): messages app rejected the send: sending messages requires macOS with the Messages app (on first use, allow switchboard t |
| B-send | lookup_without_contacts_dir_linux | `imessage_lookup_contact` | PASS | non-Mac without contacts_dir: contacts lookup is not configured |
| B-send | names_absent_without_contacts | `imessage_list_chats` | PASS | no contacts_dir → participants keep handles, no names (best-effort enrichment) |
| C-setup | setup_no_db_path_linux | `Configure + imessage_list_chats` | PASS | log: "WARN: failed to configure \"imessage\": imessage: requires macOS (set db_path to read a copied chat.db on other platforms)" / tool call: imessage: not configured |
| C-setup | setup_missing_db_file | `Configure + imessage_list_chats` | PASS | log: "WARN: failed to configure \"imessage\": imessage: cannot read /workspace/sb-imessage-proof-20261010/nope/chat.db: unable to open database file (14). Grant Full Disk Access to the switchboard binary (or the terminal running it) in / tool call: imessage: not configured |
| C-setup | setup_missing_db_hint | `Configure` | INFO | missing-file error appends the macOS Full Disk Access hint even on Linux / for a plain typo in db_path (cosmetic); Configure errors reach agents only as 'imessage: not configured' (detail is in the server log) |
| C-setup | setup_wrong_sqlite_file | `Configure + imessage_list_chats` | PASS | log: "WARN: failed to configure \"imessage\": imessage: cannot read /workspace/sb-imessage-proof-20261010/fixtures/AddressBook/AddressBook-v22.abcddb: table message not found; is this a Messages chat.db?" / tool call: imessage: not configured |

## Files
REPORT.md, pass-fail.md, handoff-snippet.md, responses.json (steps with rendered + raw adapter JSON), mcp-responses.json, proof.mp4, screenshots/, fixtures/ (chat.db, chat.db-wal, AddressBook/, build_fixtures.py, manifest), hash-before.json / hash-after.json, compact-fixcheck.json, go-test.log, build.log, harness-run.log, switchboard-*.log, run-e2e.py, leak-scan.txt.
