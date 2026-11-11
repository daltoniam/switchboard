package web

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/daltoniam/switchboard/integrations/postgres"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"
)

const livePostgresImage = "postgres:17-alpine@sha256:266d3c257594e195b309ae9d1805e54c788f1d7b375f56bf6c367aac6c917e7b"

const livePostgresSeed = `
CREATE TYPE public.customer_tier AS ENUM ('free', 'pro', 'enterprise');
CREATE TABLE public.customers (
	id serial PRIMARY KEY,
	email text NOT NULL UNIQUE CHECK (email LIKE '%@%'),
	tier public.customer_tier NOT NULL DEFAULT 'free'
);
CREATE TABLE public.orders (
	id serial PRIMARY KEY,
	customer_id int NOT NULL REFERENCES public.customers (id),
	total_cents int NOT NULL,
	updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX orders_customer_idx ON public.orders (customer_id);
CREATE TABLE public.job_queue (id serial PRIMARY KEY, name text NOT NULL);
CREATE VIEW public.big_spenders AS
	SELECT c.email, sum(o.total_cents) AS spent_cents
	FROM public.customers c JOIN public.orders o ON o.customer_id = c.id
	GROUP BY c.email;
CREATE FUNCTION public.touch_order() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
	NEW.updated_at := now();
	RETURN NEW;
END $$;
CREATE TRIGGER orders_touch BEFORE UPDATE ON public.orders FOR EACH ROW EXECUTE FUNCTION public.touch_order();
CREATE EXTENSION pg_trgm;
CREATE ROLE live_reporter LOGIN;
GRANT SELECT ON public.orders TO live_reporter;
INSERT INTO public.customers (email, tier) VALUES ('ada@example.test', 'enterprise'), ('grace@example.test', 'pro');
INSERT INTO public.orders (customer_id, total_cents) VALUES (1, 4200), (1, 1300), (2, 999);
INSERT INTO public.job_queue (name) VALUES ('nightly-export');
ANALYZE;
`

func init() {
	registerLiveSpec(liveSpec{
		Integration: "postgres",
		Tier:        liveContainer,
		Adapter:     postgres.New(),
		Provision:   livePostgresProvision,
		Cases: []liveCase{
			{Tool: "postgres_list_databases", Expect: []string{"default", "livedb"}},
			{Tool: "postgres_list_schemas", Expect: []string{"public"}},
			{Tool: "postgres_list_tables", Args: map[string]any{"schema": "public"}, Expect: []string{"customers", "orders", "job_queue"}},
			{Tool: "postgres_describe_table", Args: map[string]any{"table": "customers"}, Expect: []string{"email", "tier"}},
			{Tool: "postgres_list_columns", Args: map[string]any{"table": "orders"}, Expect: []string{"customer_id", "total_cents"}},
			{Tool: "postgres_list_indexes", Args: map[string]any{"table": "orders"}, Expect: []string{"orders_customer_idx"}},
			{Tool: "postgres_list_constraints", Args: map[string]any{"table": "customers"}, Expect: []string{"customers_pkey", "customers_email_key"}},
			{Tool: "postgres_list_foreign_keys", Args: map[string]any{"table": "orders"}, Expect: []string{"orders_customer_id_fkey"}},
			{Tool: "postgres_list_views", Expect: []string{"big_spenders"}},
			{Tool: "postgres_list_functions", Expect: []string{"touch_order"}},
			{Tool: "postgres_list_triggers", Args: map[string]any{"table": "orders"}, Expect: []string{"orders_touch"}},
			{Tool: "postgres_list_enums", Expect: []string{"customer_tier", "enterprise"}},
			{Tool: "postgres_query", Args: map[string]any{"sql": "SELECT email, spent_cents FROM public.big_spenders ORDER BY email"}, Expect: []string{"ada@example.test", "5500"}},
			{Tool: "postgres_execute", Args: map[string]any{"sql": "INSERT INTO public.customers (email) VALUES ('linus@example.test')"}, Expect: []string{"1"}},
			{Tool: "postgres_select", Args: map[string]any{"table": "customers", "where": "email = 'linus@example.test'"}, Expect: []string{"linus@example.test", "free"}},
			{Tool: "postgres_explain", Args: map[string]any{"sql": "SELECT * FROM public.orders WHERE customer_id = 1", "analyze": true}, Expect: []string{"orders", "actual time"}},
			{Tool: "postgres_database_info", Expect: []string{"PostgreSQL 17", "livedb"}},
			{Tool: "postgres_database_size", Expect: []string{"customers"}},
			{Tool: "postgres_table_stats", Args: map[string]any{"table": "orders"}, Expect: []string{"orders"}},
			{Tool: "postgres_list_roles", Expect: []string{"live_reporter"}},
			{Tool: "postgres_list_grants", Args: map[string]any{"table": "orders"}, Expect: []string{"live_reporter", "SELECT"}},
			{Tool: "postgres_list_extensions", Expect: []string{"pg_trgm"}},
			{Tool: "postgres_list_active_connections", Expect: []string{"livedb", "pg_sleep"}},
			{Tool: "postgres_list_locks", Expect: []string{"nightly-export-reader"}},
			{Tool: "postgres_running_queries", Expect: []string{"pg_sleep"}},
		},
	})
}

func livePostgresProvision(t *testing.T) liveProvision {
	container := liveStartContainer(t, liveDockerOptions{
		Image: livePostgresImage,
		Env:   map[string]string{"POSTGRES_USER": "live", "POSTGRES_PASSWORD": "live-password", "POSTGRES_DB": "livedb"},
		Ports: []string{"5432/tcp"},
	})
	host, port := container.Address(t, "5432/tcp")
	dsn := fmt.Sprintf("postgres://live:live-password@%s:%s/livedb?sslmode=disable", host, port)
	db, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	// The image's initdb server only listens on a socket, so a TCP ping
	// succeeds once the final server is up.
	liveEventually(t, 60*time.Second, func() error { return db.PingContext(t.Context()) })
	_, err = db.ExecContext(t.Context(), livePostgresSeed)
	require.NoError(t, err)

	// Hold a row lock on job_queue and queue a writer behind it so the lock
	// and running-query tools have real activity to report. A row lock keeps
	// catalog and size queries from waiting on it.
	psql := []string{"psql", "--username", "live", "--dbname", "livedb", "--command"}
	container.ExecDetached(t, append(psql, "BEGIN; UPDATE public.job_queue SET name = 'held' WHERE id = 1; SELECT pg_sleep(600);")...)
	liveEventually(t, 30*time.Second, func() error { return livePostgresActivity(t, db, "pg_sleep(600)") })
	container.ExecDetached(t, append(psql, "UPDATE public.job_queue SET name = 'nightly-export-reader' WHERE id = 1;")...)
	liveEventually(t, 30*time.Second, func() error { return livePostgresActivity(t, db, "nightly-export-reader") })

	return liveProvision{Credentials: map[string]string{
		"host":      host,
		"port":      port,
		"user":      "live",
		"password":  "live-password",
		"database":  "livedb",
		"sslmode":   "disable",
		"read_only": "false",
	}}
}

func livePostgresActivity(t *testing.T, db *sql.DB, fragment string) error {
	var count int
	err := db.QueryRowContext(t.Context(), `SELECT count(*) FROM pg_stat_activity WHERE state = 'active' AND query LIKE '%' || $1 || '%' AND pid != pg_backend_pid()`, fragment).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("no active query containing %q yet", fragment)
	}
	return nil
}
