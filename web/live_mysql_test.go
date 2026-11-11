package web

import (
	"database/sql"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/daltoniam/switchboard/integrations/mysql"
	driver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
)

const liveMySQLImage = "mysql:8.4@sha256:a81d043930e51745522bd5c430961cf8286f0a23f0a9a2a6e0108f7ea71b0902"

const liveMySQLSeed = `
CREATE DATABASE app;
CREATE TABLE app.customers (
	id BIGINT UNSIGNED PRIMARY KEY,
	email VARCHAR(255) NOT NULL UNIQUE
);
CREATE TABLE app.orders (
	id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
	customer_id BIGINT UNSIGNED NOT NULL,
	total_cents INT NOT NULL,
	CONSTRAINT orders_customer_fk FOREIGN KEY (customer_id) REFERENCES app.customers(id)
);
CREATE VIEW app.customer_totals AS
	SELECT c.email, SUM(o.total_cents) AS spent_cents
	FROM app.customers c JOIN app.orders o ON o.customer_id = c.id
	GROUP BY c.email;
INSERT INTO app.customers VALUES (9223372036854775809, 'ada@example.test'), (2, 'grace@example.test');
INSERT INTO app.orders (customer_id, total_cents) VALUES (9223372036854775809, 4200), (9223372036854775809, 1300), (2, 999);
CREATE USER 'reader'@'%' IDENTIFIED BY 'live-reader-password';
GRANT SELECT, SHOW VIEW ON app.* TO 'reader'@'%';
GRANT SELECT ON performance_schema.processlist TO 'reader'@'%';
GRANT SELECT ON performance_schema.global_status TO 'reader'@'%';
`

func init() {
	registerLiveSpec(liveSpec{
		Integration: "mysql",
		Tier:        liveContainer,
		Adapter:     mysql.New(),
		Provision:   liveMySQLProvision,
		Cases: []liveCase{
			{Tool: "mysql_list_databases", Expect: []string{"app", "utf8mb4"}},
			{Tool: "mysql_list_tables", Args: map[string]any{"database": "app"}, Expect: []string{"customers", "orders", "customer_totals"}},
			{Tool: "mysql_describe_table", Args: map[string]any{"database": "app", "table": "orders"}, Expect: []string{"customer_id", "orders_customer_fk", "CREATE TABLE"}},
			{Tool: "mysql_list_columns", Args: map[string]any{"database": "app", "table": "orders"}, Expect: []string{"customer_id", "total_cents", "auto_increment"}},
			{Tool: "mysql_list_indexes", Args: map[string]any{"database": "app", "table": "customers"}, Expect: []string{"PRIMARY", "email"}},
			{Tool: "mysql_list_foreign_keys", Args: map[string]any{"database": "app", "table": "orders"}, Expect: []string{"orders_customer_fk", "customers", "customer_id"}},
			{Tool: "mysql_list_views", Args: map[string]any{"database": "app"}, Expect: []string{"customer_totals", "spent_cents"}},
			{Tool: "mysql_table_stats", Args: map[string]any{"database": "app", "table": "orders"}, Expect: []string{"orders", "InnoDB"}},
			{Tool: "mysql_running_queries", Expect: []string{"reader", "SLEEP(600)"}},
			{Tool: "mysql_server_info", Expect: []string{"8.4.", "uptime_seconds", "threads_connected"}},
			{Tool: "mysql_query", Args: map[string]any{"query": "SELECT email, spent_cents FROM app.customer_totals ORDER BY email"}, Expect: []string{"ada@example.test", "5500", "grace@example.test"}},
			{Tool: "mysql_query", Args: map[string]any{"query": "SELECT email FROM app.customers WHERE id = ?", "params": "[9223372036854775809]"}, Expect: []string{"ada@example.test"}},
			{Tool: "mysql_explain", Args: map[string]any{"query": "SELECT * FROM app.orders WHERE customer_id = ?", "params": "[9223372036854775809]"}, Expect: []string{"orders", "query_block"}},
		},
	})
}

func liveMySQLProvision(t *testing.T) liveProvision {
	container := liveStartContainer(t, liveDockerOptions{
		Image: liveMySQLImage,
		Env:   map[string]string{"MYSQL_ROOT_PASSWORD": "live-root-password", "MYSQL_ROOT_HOST": "%"},
		Ports: []string{"3306/tcp"},
	})
	host, port := container.Address(t, "3306/tcp")
	config := driver.NewConfig()
	config.Net, config.Addr = "tcp", net.JoinHostPort(host, port)
	config.User, config.Passwd = "root", "live-root-password"
	config.Timeout, config.ReadTimeout, config.WriteTimeout = time.Second, 5*time.Second, 5*time.Second
	connector, err := driver.NewConnector(config)
	require.NoError(t, err)
	db := sql.OpenDB(connector)
	t.Cleanup(func() { _ = db.Close() })
	liveEventually(t, 90*time.Second, func() error { return db.PingContext(t.Context()) })
	container.Exec(t, liveMySQLSeed, "mysql", "--user=root", "--password=live-root-password")

	config.User, config.Passwd, config.DBName = "reader", "live-reader-password", "app"
	connector, err = driver.NewConnector(config)
	require.NoError(t, err)
	reader := sql.OpenDB(connector)
	t.Cleanup(func() { _ = reader.Close() })
	require.NoError(t, reader.PingContext(t.Context()))
	for _, statement := range []string{"INSERT INTO app.customers VALUES (3, 'denied@example.test')", "CREATE TABLE app.denied (id INT)", "SELECT * FROM mysql.user"} {
		_, err := reader.ExecContext(t.Context(), statement)
		require.Error(t, err, "least-privilege reader must not execute %s", statement)
	}
	container.ExecDetached(t, "mysql", "--user=reader", "--password=live-reader-password", "--database=app", "--execute=SELECT SLEEP(600)")
	liveEventually(t, 30*time.Second, func() error {
		var count int
		err := reader.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM performance_schema.processlist WHERE USER = 'reader' AND INFO = 'SELECT SLEEP(600)'").Scan(&count)
		if err != nil {
			return err
		}
		if count == 0 {
			return fmt.Errorf("seeded reader query is not active yet")
		}
		return nil
	})
	return liveProvision{Credentials: map[string]string{
		"host": host, "port": port, "user": "reader", "password": "live-reader-password", "database": "app", "tls": "false",
	}}
}
