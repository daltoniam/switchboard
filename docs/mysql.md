# MySQL 8.4 integration

The native `mysql` adapter supports **MySQL 8.4 LTS, direct TCP, read-only access**. MariaDB, TiDB, Vitess, other MySQL versions, Unix sockets, hosted tunnels, mutations, and query cancellation by `KILL` are not supported in this first slice.

## Configure

Open **Integrations > mysql** in the local Web UI. The standard credential page is the setup page; there is no OAuth flow. Enable the integration and save either:

- `host`, `user`, `password`, optional `port` (3306 by default), optional `database`, and `tls` (verified TLS by default); or
- `connection_string`, a go-sql-driver DSN such as `reader:password@tcp(db.example:3306)/app?tls=true&parseTime=true`.

`host` and `user` are required when not using a DSN. All fields have optional labels because a DSN replaces the individual connection fields. The password, DSN, and additional-connections JSON are secret inputs. Blank passwords are submitted as blank, not silently preserved.

Remote connections require `tls=true`, verifying the server certificate chain and hostname against system roots. `tls=false` is permitted only for `localhost` or a literal loopback address, for isolated local development. `skip-verify`, `preferred`, custom TLS registry names, plaintext fallback, multi-statements, local-file access, parameter interpolation, and DSN session variables are rejected. Private deployments must install the issuing CA in the operating system trust store. Do not use a privileged production account.

Environment overrides are `MYSQL_DSN`, `MYSQL_HOST`, `MYSQL_PORT`, `MYSQL_USER`, `MYSQL_PASSWORD`, `MYSQL_DATABASE`, and `MYSQL_TLS`. They replace the corresponding saved values.

Additional independent connections can be provided as secret `connections` JSON (at most eight):

```json
[{"alias":"analytics","host":"analytics.example","user":"reader","password":"dedicated-secret","database":"analytics","tls":"true"}]
```

Aliases must be nonempty and unique. `default` is reserved for the primary connection. Every tool accepts `connection` to select an alias; `database` means a MySQL schema name, never an alias. Additional connections do not inherit primary credentials. Configuration validates every entry before opening connections, verifies the server version, and replaces the previous working configuration only after all connections succeed.

## Least privilege and safety boundary

Use a dedicated account with only `SELECT` and `SHOW VIEW` on the databases it should inspect. Have an administrator provision the account and these narrowly scoped grants:

```sql
GRANT SELECT, SHOW VIEW ON app.* TO 'switchboard_reader'@'approved-client-host';
GRANT SELECT ON performance_schema.processlist TO 'switchboard_reader'@'approved-client-host';
GRANT SELECT ON performance_schema.global_status TO 'switchboard_reader'@'approved-client-host';
```

The two performance-schema grants are needed only for the diagnostics tools. `PROCESS` is optional for visibility of other users' sessions. Without it, process visibility is limited to the account's own sessions. No tool requires `SUPER`. Never grant `FILE`, `EXECUTE`, administrative privileges, write privileges, or roles that confer those privileges. Use trusted schemas and views; views can invoke server-side routines under their definer's privileges.

Each tool read runs on one pinned connection inside a server-enforced read-only transaction, always rolled back. The adapter sets session `max_execution_time=10000` and `sql_select_limit=1001` on that same connection before beginning the transaction. A context deadline and driver read/write timeouts also bound the call. The pool has at most two connections per alias and a three-minute connection lifetime.

Caller SQL is restricted to single `SELECT`, `EXPLAIN [FORMAT=JSON] SELECT`, and these SHOW forms:

- `SHOW DATABASES`
- `SHOW [FULL] TABLES [FROM database]`
- `SHOW CREATE TABLE database.table` or `SHOW CREATE VIEW database.view`

Semicolons (including a trailing semicolon), comments (including executable comments and optimizer hints), variables, backslash escapes, double quotes, unsafe clauses, qualified functions, quoted function names, and arbitrary routines are rejected. Function calls must use the explicitly allowlisted built-ins in `integrations/mysql/safety.go` with no whitespace before `(`. CTEs and `EXPLAIN ANALYZE` are not supported. Positional `?` parameters accept a JSON array of scalar values.

**This is not a complete SQL sandbox.** Lexical restrictions and read-only transactions are defense in depth, not a substitute for least-privilege credentials, trusted database objects, and server-side resource governance. Expensive reads can still consume database resources. MySQL's execution-time limit does not cover all statement classes; the context deadline closes a timed-out client connection but is not an administrative server-side kill guarantee.

## Tools and response limits

Start with `mysql_list_databases`, then `mysql_list_tables`, `mysql_describe_table`, and `mysql_query` or `mysql_explain`. The other tools are `mysql_list_columns`, `mysql_list_indexes`, `mysql_list_foreign_keys`, `mysql_list_views`, `mysql_table_stats`, `mysql_running_queries`, and `mysql_server_info`.

Catalog tools require explicit database/table identifiers where relevant. The first slice supports identifiers containing letters, numbers, `_`, `$`, or `-`, up to 64 characters. Foreign keys list outgoing references. Table row counts and sizes are estimates.

Results preserve `columns`, positional `rows`, and `truncated`; table descriptions contain separate `columns`, `indexes`, `foreign_keys`, and `ddl` result sets. `limit` defaults to 100, ranges from 1 to 1000, and truncation is explicit when rows or the 1 MiB encoded response budget are exceeded. SQL input is capped at 64 KiB. Metadata queries request at most `limit+1` rows. Caller SQL can contain its own LIMIT; returned data is still capped. An oversized table description returns an error directing the caller to individual catalog tools. Narrow the SQL projection for large values. These are response limits, not a hard bound on a single upstream row's wire allocation.

## Verification

Unit tests cover configuration, unsafe SQL and DSNs, dispatch parity, compaction shape, aliases, parameter binding, failure paths, cancellation, and row/byte limits. `TestUIMySQL` drives the real binary in isolated Chromium against a credential-checking local wire fixture, including fresh fields, save/health, invalid/missing password, restart persistence, MCP discovery/read, and disable. Screenshots mask every input.

For an explicitly isolated MySQL 8.4 test server with an `app.users` fixture table, run:

```bash
SWITCHBOARD_MYSQL_TEST_DSN='reader:dedicated-test-password@tcp(127.0.0.1:13362)/app?tls=false' go test -v ./integrations/mysql -run TestMySQL84Live
```

The opt-in test reads all 12 tools. Fixture-backed browser acceptance is mandatory in `make test-ui` and `make ci` and needs no external database. Local fixture success is not evidence of authentication to a production server.
