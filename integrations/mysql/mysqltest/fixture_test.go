package mysqltest

import (
	"context"
	"database/sql"
	"testing"
	"time"

	driver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
)

func TestFixture(t *testing.T) {
	fixture := Start(t)
	for _, password := range []string{Password, "invalid", ""} {
		t.Run(password, func(t *testing.T) {
			config := driver.NewConfig()
			config.Net, config.Addr = "tcp", "127.0.0.1:"+fixture.Port
			config.User, config.Passwd = "reader", password
			connector, err := driver.NewConnector(config)
			require.NoError(t, err)
			db := sql.OpenDB(connector)
			defer db.Close()
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			err = db.PingContext(ctx)
			if password != Password {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			var version string
			require.NoError(t, db.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version))
			require.Equal(t, "8.4.11-fixture", version)
		})
	}
}
