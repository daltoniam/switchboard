# Switchboard PR #320 — Sandbox E2E proof (iMessage adapter)

- Generated: 2026-10-10 17:47 CDT
- PR: https://github.com/daltoniam/switchboard/pull/320 (branch `switchboard-imessage`)
- Head SHA: `d7de0f40b8b1a92e77a782d31192e6a01e2e191d`
- Transport: local SQLite read-only (modernc.org/sqlite, `mode=ro&_pragma=query_only(1)`, WAL) + AppleScript (`/usr/bin/osascript`) send. **No REST/OAuth/token.**
- Harness: PR build in isolated HOME/SWITCHBOARD_DEV_HOME, driven over MCP HTTP on :13852 (read), :13853 (send gating), :13854–13856 (setup failure modes). Linux box; no Mac and no real Messages DB used.
- Fixtures: synthetic chat.db with a Sonoma/Sequoia-shaped schema (newest 5 messages left in un-checkpointed `chat.db-wal`) + AddressBook-v22.abcddb (Sources/<uuid> + root). All names fictional, numbers +1 312/773 555-01xx, emails @example.com. Builder: `fixtures/build_fixtures.py`.

## Summary
- Unit tests: `go test ./integrations/imessage/...` PASS (32 top-level tests, 100 incl. subtests); `go vet` + `go build ./cmd/server` + `go test ./config/...` PASS.
- MCP E2E: **53 PASS / 0 FAIL / 3 INFO**, plus 1 SKIPPED (real send delivery).
- Everything that passed at cec573f3 still passes; all 5 previous FAILs and the allowlist INFO now pass; 3 INFO items are unchanged.

## Per-tool
| Tool / check | Result | Evidence |
|---|---|---|
| search (discovery) | PASS | all 6 imessage_* tools returned by `search` |
| imessage_list_chats | PASS | 9 chats newest first; group `Trail Crew 🥾` flagged as a group with 3 participants; unread counts skip tapbacks and deleted messages; last-message preview from un-checkpointed WAL; name/phone-digit query; pagination; **default render now keeps participant handle + name** |
| imessage_get_chat_messages | PASS | tapbacks love/like/laugh/emphasize/question/🔥 emoji, `bp:`/`p:N/` GUIDs, 3xxx removal; threaded replies; HEIC/PDF/MOV/CAF attachments, U+FFFC stripped; edited/unsent; attributedBody incl. >127-byte + NSMutableString; rename event hidden; by-handle merges iMessage + SMS; a +44 handle no longer resolves to the +1 chat; paging; argument errors; **default render now keeps reaction type/emoji/by=me and full attachment objects** |
| imessage_search_messages | PASS | case-insensitive over text + attributedBody; scope by chat_id / since; handle scope now covers every chat the person is in, incl. my own sent messages; tapback rows excluded; **Recently Deleted canary no longer returned** |
| imessage_list_unread | PASS | newest first incl. WAL rows; unread tapback excluded; **Recently Deleted canary no longer listed** |
| imessage_lookup_contact | PASS | name / partial phone / email / org-only / nickname-only; same person across Sources/<uuid> + root AddressBook merged; **two different 'Jordan Lee' cards stay separate**; 'not configured' without contacts_dir on Linux |
| imessage_send_message — gating | PASS | allow_send unset → refused. With allowlist: non-listed `to` and groups with non-listed members refused before osascript; **+44 lookalike of the allowlisted +1 number refused (with and without service=sms)**; allowlisted chat_id / reformatted / 11-digit `to` get through and fail with `sending messages requires macOS` |
| imessage_send_message — actual delivery | SKIPPED | needs a Mac + Automation→Messages permission + a recipient who agreed to it |
| Read-only safety | PASS | sha256 of chat.db, chat.db-wal, 2× AddressBook unchanged; WAL not checkpointed; only SQLite's -shm index file created; DSN mode=ro + query_only(1) |
| Non-Mac setup | PASS | no db_path → `imessage: requires macOS (set db_path …)`; missing file / non-Messages DB fail clearly; tools return `imessage: not configured` |

## The 4 cec573f3 findings: before vs after
| Finding | Before (cec573f3) | After (d7de0f40) | Fix |
|---|---|---|---|
| 1. compact.yaml nested arrays | FAIL: default rendered row for msg 13: reactions=['Blake Moreno'] attachments=['IMG_4821.HEIC'] — reaction type/emoji and my own (by=me) reactions are lost, attachments reduced to path only (name/mime_type dropped). Cause: compact.yaml <br>FAIL: default rendered participants for chat 1 = ['Avery Quinn']: participants[].handle is dropped whenever a contact name exists (only participants[].name survives) — same compact.yaml nested-array cause; raw data has both | PASS: default rendered row for msg 13: reactions=[{'by': '+13125550102', 'by_name': 'Blake Moreno', 'type': 'laugh'}, {'by': 'me', 'emoji': '🔥', 'type': 'emoji'}] attachments=[{'bytes': 2481152, 'mime_type': 'image/heic', 'name': 'IMG_4<br>PASS: default rendered participants for chat 1 = [{'handle': '+13125550101', 'name': 'Avery Quinn'}]: handle kept | compact.yaml now lists the whole arrays: `messages[].reactions`, `messages[].attachments`, `chats[].participants`, `participants` |
| 2. Recently Deleted in search/unread | FAIL: recently-deleted message returned (id=95, chat_id=0): adapter LEFT JOINs chat_message_join and never excludes rows that only live in chat_recoverable_message_join<br>FAIL: recently-deleted unread message 95 listed (chat_id=0, no chat_name) — same root cause as search_recently_deleted | PASS: recently-deleted message (only in chat_recoverable_message_join) not returned<br>PASS: deleted canary not listed | new `liveFilter()`: `cmj.chat_id IS NOT NULL` + `NOT EXISTS chat_recoverable_message_join` (search, unread, unread badges, last-message preview, transcripts, reactions) |
| 3. Same-name contact merge | FAIL: two distinct cards both named 'Jordan Lee' (different numbers) should stay separate → [{"emails": ["jordan.lee.two@example.com"], "name": "Jordan Lee", "phones": ["(312) 555-0107", "(773) 555-0108"]}] | PASS: two distinct cards both named 'Jordan Lee' (different numbers) should stay separate → [{"name": "Jordan Lee", "phones": ["(312) 555-0107"]}, {"emails": ["jordan.lee.two@example.com"], "name": "Jordan Lee", "phones": ["(773) 555-01 | records keyed by Z_PK; merged only when the name matches AND they share a phone/email (`mergeContacts`/`sameHandle`) |
| 4. Allowlist last-10-digit match | INFO: a +44 number sharing the allowlisted number's last 10 digits PASSES the allowlist and reaches the script runner (normalizeHandle compares last 10 digits): messages app rejected the send: sending messages requires macOS with the Me<br>(not tested at cec573f3)<br>(not tested at cec573f3) | PASS: +44 number sharing the allowlisted +1 number's last 10 digits is refused before osascript: recipient +44 312 555 0101 is not in send_allowlist<br>PASS: +44 number sharing the allowlisted +1 number's last 10 digits is refused before osascript: recipient +44 312 555 0101 is not in send_allowlist<br>PASS: +44 number does not resolve to the +1 Avery chat: no conversation found with +44 312 555 0101; use imessage_list_chats to find a chat_id | `sameHandle`: exact digits, except a bare 10-digit number matches the 1-prefixed form; used for the allowlist, existing-chat lookup and handle lookups |

Old results are kept in `prior-cec573f3/` for comparison.

## Caveats / findings
1. **All 4 findings from cec573f3 are fixed** (see the before/after table). **No regressions:** all 42 steps that passed at cec573f3 still pass (none regressed). The old `send_allowlist_last10_digits` INFO step is now `send_allowlist_foreign_cc_sms` and passes. 6 new steps cover the changed code.
2. **Still open (INFO, unchanged, a reviewer decision):**
   - Junk/filtered chats (`chat.is_filtered=1`) are still listed and count as unread.
   - A missing `db_path` file still shows the macOS Full Disk Access hint on Linux.
   - Setup errors still reach agents only as `imessage: not configured`; details are only in the server log.
3. **New, from reading the code (low, not exercised):**
   - Handle matching for reads is now exact too (`findDirectChats`, `chatsWithHandle`). A non-US number typed in national format (e.g. `07700 900123` vs stored `+447700900123`) no longer finds the chat. For the allowlist this fails safe (send refused); for reads it's a usability gap.
   - Display-name enrichment (`contactBook.name`) still uses the fuzzy last-10-digit key, so a +44 lookalike could be labelled with a +1 contact's name. Display only; send and lookups use `sameHandle`.
   - `NOT EXISTS (… chat_recoverable_message_join WHERE message_id = …)` can't use that table's (chat_id, message_id) primary key, so it scans per row. That table is usually small (30-day retention), so this is a performance note only.
4. Configure/Execute now snapshot state behind an inflight WaitGroup, so the old DB closes after in-flight calls finish. `go test -race ./integrations/imessage/...` passes; I didn't stress-test reconfiguring under load.
5. Reading a copied DB creates a `-shm` file next to it (normal SQLite WAL behaviour; DB/WAL bytes unchanged), so the folder must be writable.
6. Fixture limits: the chat.db is synthetic, shaped like Sonoma/Sequoia. Not covered: schemas older than macOS 13 (unit tests cover the column fallbacks), RCS, edit history in `message_summary_info`, real permission prompts.

## Dalton blockers (Mac-live only)
- Actual delivery of `imessage_send_message`: needs a Mac running the PR build, Automation→Messages permission for switchboard, and a recipient who has agreed to receive a test message (`allow_send=true`, `send_allowlist=<that handle>`).
- Optional: a read pass against his real `~/Library/Messages/chat.db` with Full Disk Access, to confirm the Recently Deleted filter and attributedBody decoding on real data.

## All steps
| Phase | Step | Tool | Result | Note |
|---|---|---|---|---|
| A-read | search | `search` | PASS | 6 imessage tools listed: imessage_get_chat_messages, imessage_list_chats, imessage_list_unread, imessage_lookup_contact, imessage_search_messages, imessage_send_message |
| A-read | list_chats | `imessage_list_chats` | PASS | total=9 order=[4, 1, 2, 7, 9, 8, 6, 5, 3]; group 'Trail Crew 🥾' is_group, 3 participants, unread=1 (reaction excluded), last msg from WAL; unnamed group → 'Blake Moreno, Dev'; nickname-only contact → 'Dev' |
| A-read | list_chats_junk_filter | `imessage_list_chats` | INFO | chat 6 (chat.is_filtered=1, unknown sender spam) is listed with unread_count=1 — adapter does not consult chat.is_filtered |
| A-read | list_chats_rendered_participants | `imessage_list_chats (default compact render)` | PASS | default rendered participants for chat 1 = [{'handle': '+13125550101', 'name': 'Avery Quinn'}]: handle kept |
| A-read | list_chats_query_name | `imessage_list_chats` | PASS | query=Avery → [1, 5, 2] (1:1 iMessage+SMS before group 2) |
| A-read | list_chats_query_phone | `imessage_list_chats` | PASS | phone-digit query → [2, 9] |
| A-read | list_chats_pagination | `imessage_list_chats` | PASS | limit=2 offset=2 → [2, 7] has_more=True |
| A-read | get_chat_messages_group | `imessage_get_chat_messages` | PASS | all checks: visible ids (no tapback rows, no rename event); 13 reactions laugh(Blake)+🔥(me); Casey emphasize removed by 3004; 12 question via bp: guid; 23 love + 2 attachments (mov, caf); 13 photo attachment heic, text empty (U+FFFC stripped); 18 pdf + U+FFFC stripped from text; threaded replies 12→MSG-0011, 19→MSG-0018; 22 long attributedBody (>127 bytes, 0x81 len) decoded; 25 NSMutableString dec |
| A-read | get_chat_messages_rendered_fidelity | `imessage_get_chat_messages (default compact render)` | PASS | default rendered row for msg 13: reactions=[{'by': '+13125550102', 'by_name': 'Blake Moreno', 'type': 'laugh'}, {'by': 'me', 'emoji': '🔥', 'type': 'emoji'}] attachments=[{'bytes': 2481152, 'mime_type': 'image/heic', 'name': 'IMG_4821.HEIC', 'path': '/workspace/sb-imessage-proof-20261010/homes/A-read/Library/Messages/Attachments/3a/10/AB12CD34-0001/IMG_4821.HEIC'}] — reaction types kept |
| A-read | get_chat_messages_direct | `imessage_get_chat_messages` | PASS | edited/unsent/tapbacks/attributedBody: OK |
| A-read | get_chat_messages_by_handle | `imessage_get_chat_messages` | PASS | handle → chat_ids=[1, 5] (iMessage + SMS threads merged, 7 msgs) |
| A-read | get_chat_messages_by_foreign_handle | `imessage_get_chat_messages` | PASS | +44 number does not resolve to the +1 Avery chat: no conversation found with +44 312 555 0101; use imessage_list_chats to find a chat_id |
| A-read | get_chat_messages_paging | `imessage_get_chat_messages` | PASS | page1=[23, 25, 201] next_before=2026-10-09T09:43:00-05:00 → page2=[18, 19, 22] |
| A-read | get_chat_messages_missing_args | `imessage_get_chat_messages` | PASS | chat_id or handle is required |
| A-read | get_chat_messages_unknown_chat | `imessage_get_chat_messages` | PASS | chat 999 not found; use imessage_list_chats to find a chat_id |
| A-read | get_chat_messages_bad_date | `imessage_get_chat_messages` | PASS | since must be an RFC3339 timestamp or YYYY-MM-DD date, got "last tuesday" |
| A-read | search_text | `imessage_search_messages` | PASS | attributedBody text search; tapback quoting it (202) excluded → ids=[201, 18, 10] (expected [201, 18, 10]) |
| A-read | search_text_rendered | `imessage_search_messages` | PASS | rendered result includes chat_name |
| A-read | search_case_insensitive_wal | `imessage_search_messages` | PASS | case-insensitive, WAL-only row → ids=[203] (expected [203]) |
| A-read | search_handle_scope | `imessage_search_messages` | PASS | SMS short-code scope → ids=[204, 40] (expected [204, 40]) |
| A-read | search_handle_scope_group | `imessage_search_messages` | PASS | handle scope now = every chat the person is in (group incl. others' messages) → ids=[201, 18, 10] (expected [201, 18, 10]) |
| A-read | search_handle_scope_my_msgs | `imessage_search_messages` | PASS | handle scope includes my own sent messages → ids=[31, 30] (expected [31, 30]) |
| A-read | search_chat_scope | `imessage_search_messages` | PASS | chat_id scope (email 1:1) → ids=[31, 30] (expected [31, 30]) |
| A-read | search_date_scope | `imessage_search_messages` | PASS | since bound → ids=[201] (expected [201]) |
| A-read | search_no_hits | `imessage_search_messages` | PASS | no hits → empty list → ids=[] (expected []) |
| A-read | search_empty_query | `imessage_search_messages` | PASS | query is required |
| A-read | search_recently_deleted | `imessage_search_messages` | PASS | recently-deleted message (only in chat_recoverable_message_join) not returned |
| A-read | list_unread | `imessage_list_unread` | PASS | ids=[204, 203, 201, 200, 60]; newest-first [204, 203, 201, 200] incl. WAL rows; unread tapback 202 excluded |
| A-read | list_unread_recently_deleted | `imessage_list_unread` | PASS | deleted canary not listed |
| A-read | list_unread_junk | `imessage_list_unread` | INFO | filtered/junk sender message 60 is listed as unread |
| A-read | lookup_name_merge_sources | `imessage_lookup_contact` | PASS | case-insensitive name; Sources/<uuid> + root AddressBook merged, phone de-duplicated across formats → [{"emails": ["avery.quinn@example.com", "aq@example.com"], "name": "Avery Quinn", "phones": ["(312) 555-0101"]}] |
| A-read | lookup_partial_phone | `imessage_lookup_contact` | PASS | partial phone → nickname-only contact → [{"name": "Dev", "phones": ["773.555.0104"]}] |
| A-read | lookup_email | `imessage_lookup_contact` | PASS | email, case-insensitive → [{"emails": ["Casey.Rivera@example.com"], "name": "Casey Rivera"}] |
| A-read | lookup_org | `imessage_lookup_contact` | PASS | organization-only card → [{"name": "Example Bank", "phones": ["55501"]}] |
| A-read | lookup_no_match | `imessage_lookup_contact` | PASS | no match → empty → [] |
| A-read | lookup_same_name_people | `imessage_lookup_contact` | PASS | two distinct cards both named 'Jordan Lee' (different numbers) should stay separate → [{"name": "Jordan Lee", "phones": ["(312) 555-0107"]}, {"emails": ["jordan.lee.two@example.com"], "name": "Jordan Lee", "phones": ["(773) 555-0108"]}] |
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
| B-send | send_allowlist_foreign_cc_sms | `imessage_send_message` | PASS | +44 number sharing the allowlisted +1 number's last 10 digits is refused before osascript: recipient +44 312 555 0101 is not in send_allowlist |
| B-send | send_allowlist_foreign_cc_existing_chat | `imessage_send_message` | PASS | +44 number sharing the allowlisted +1 number's last 10 digits is refused before osascript: recipient +44 312 555 0101 is not in send_allowlist |
| B-send | send_allowlist_us_cc_variant | `imessage_send_message` | PASS | 11-digit 1-prefixed form of the allowlisted number still passes the gate (no over-tightening) → platform error: messages app rejected the send: sending messages requires macOS with the Messages app (on  |
| B-send | lookup_without_contacts_dir_linux | `imessage_lookup_contact` | PASS | non-Mac without contacts_dir: contacts lookup is not configured |
| B-send | names_absent_without_contacts | `imessage_list_chats` | PASS | no contacts_dir → participants keep handles, no names (best-effort enrichment) |
| C-setup | setup_no_db_path_linux | `Configure + imessage_list_chats` | PASS | log: "WARN: failed to configure \"imessage\": imessage: requires macOS (set db_path to read a copied chat.db on other platforms)" / tool call: imessage: not configured |
| C-setup | setup_missing_db_file | `Configure + imessage_list_chats` | PASS | log: "WARN: failed to configure \"imessage\": imessage: cannot read /workspace/sb-imessage-proof-20261010/nope/chat.db: unable to open database file (14). Grant Full Disk Access to the switchboard binary (or the terminal running it) in / tool call: imessage: not configured |
| C-setup | setup_missing_db_hint | `Configure` | INFO | missing-file error appends the macOS Full Disk Access hint even on Linux / for a plain typo in db_path (cosmetic); Configure errors reach agents only as 'imessage: not configured' (detail is in the server log) |
| C-setup | setup_wrong_sqlite_file | `Configure + imessage_list_chats` | PASS | log: "WARN: failed to configure \"imessage\": imessage: cannot read /workspace/sb-imessage-proof-20261010/fixtures/AddressBook/AddressBook-v22.abcddb: table message not found; is this a Messages chat.db?" / tool call: imessage: not configured |

## Files
REPORT.md, pass-fail.md, handoff-snippet.md, responses.json (steps with rendered + raw adapter JSON), mcp-responses.json, proof.mp4, screenshots/, fixtures/ (chat.db, chat.db-wal, AddressBook/, build_fixtures.py, manifest), hash-before.json / hash-after.json, go-test.log, build.log, harness-run.log, switchboard-*.log, run-e2e.py, leak-scan.txt.
