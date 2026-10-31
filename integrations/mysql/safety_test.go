package mysql

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateQuery(t *testing.T) {
	cases := []struct {
		query string
		valid bool
	}{
		{"SELECT id, name FROM users WHERE id = ?", true},
		{"select count(*), max(id) from `orders`", true},
		{"SELECT 'drop table users', 'it''s safe'", true},
		{"SELECT * FROM users WHERE id IN (SELECT user_id FROM orders)", true},
		{"SELECT CASE WHEN id > 1 THEN 'yes' ELSE 'no' END FROM users", true},
		{"SELECT CASE WHEN (1 = 1) THEN 1 ELSE 0 END", true},
		{"SELECT id FROM t ORDER BY (id)", true},
		{"SELECT CAST(1 AS DECIMAL(10,2))", true},
		{"SELECT a.id FROM a JOIN b USING (id)", true},
		{"SELECT COUNT(DISTINCT(id)) FROM t", true},
		{"SELECT COUNT(ALL(id)) FROM t", true},
		{"SELECT DISTINCT (id) FROM t", true},
		{"SELECT DISTINCTROW (id) FROM t", true},
		{"SELECT 1 WHERE 'a' LIKE ('a')", true},
		{"SELECT 1 WHERE 'a' NOT LIKE ('b')", true},
		{"SELECT 1 WHERE 'a' REGEXP ('a')", true},
		{"SELECT 1 WHERE 'a' RLIKE ('a')", true},
		{"SELECT 1 WHERE 2 BETWEEN (1) AND (3)", true},
		{"SELECT 1 WHERE TRUE XOR (FALSE)", true},
		{"SELECT COUNT(DISTINCT(dangerous_routine())) FROM t", false},
		{"SELECT 1 WHERE 'a' LIKE (dangerous_routine())", false},
		{"SELECT 1 WHERE 'a' REGEXP (app.count(*))", false},
		{"SELECT 1 WHERE 2 BETWEEN (SLEEP(100)) AND (3)", false},
		{"SELECT 1 WHERE TRUE XOR (`dangerous_routine`())", false},
		{"SHOW DATABASES", true},
		{"SHOW FULL TABLES FROM `app`", true},
		{"SHOW CREATE TABLE `app`.`users`", true},
		{"EXPLAIN FORMAT=JSON SELECT * FROM users", true},
		{"", false},
		{"INSERT INTO users VALUES (1)", false},
		{"SELECT 1; DROP TABLE users", false},
		{"SELECT 1;", false},
		{"SELECT 1 /*! INTO OUTFILE '/tmp/data' */", false},
		{"SELECT /*+ SET_VAR(max_execution_time=0) */ 1", false},
		{"SELECT 1 -- ignored", false},
		{"SELECT 1 # ignored", false},
		{"SELECT * INTO OUTFILE '/tmp/data' FROM users", false},
		{"SELECT LOAD_FILE('/etc/passwd')", false},
		{"SELECT dangerous_routine()", false},
		{"SELECT app.count(*)", false},
		{"SELECT `dangerous_routine`()", false},
		{"SELECT count (id) FROM users", false},
		{"SELECT app . count (id) FROM users", false},
		{"SELECT SLEEP(100)", false},
		{"SELECT GET_LOCK('x', 100)", false},
		{"SELECT @x := 1", false},
		{"SELECT * FROM users FOR UPDATE", false},
		{"SELECT * FROM users LOCK IN SHARE MODE", false},
		{"CALL routine()", false},
		{"SET GLOBAL max_connections=100", false},
		{"SHOW BINLOG EVENTS", false},
		{"EXPLAIN ANALYZE SELECT 1", false},
		{"EXPLAIN DELETE FROM users", false},
		{"SELECT 'unterminated", false},
		{"SELECT 'back\\slash'", false},
		{"SELECT 0x4142", true},
		{"WITH x AS (DELETE FROM users) SELECT * FROM x", false},
	}
	for _, test := range cases {
		t.Run(test.query, func(t *testing.T) {
			err := validateQuery(test.query)
			if test.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}
