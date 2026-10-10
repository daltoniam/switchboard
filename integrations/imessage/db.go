package imessage

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"modernc.org/sqlite"
)

// appleEpochOffset is the number of seconds between the Unix epoch and the
// Core Data reference date (2001-01-01T00:00:00Z) that Messages uses.
const appleEpochOffset = 978307200

// matchFuncName is a SQLite scalar function that decodes a message body
// (text column or attributedBody archive) and reports whether it contains a
// lowercased needle. It lets search scan without loading every blob into Go.
const matchFuncName = "switchboard_imessage_match"

func init() {
	if err := sqlite.RegisterDeterministicScalarFunction(matchFuncName, 3, matchFunc); err != nil {
		panic(fmt.Sprintf("imessage: register %s: %v", matchFuncName, err))
	}
}

func matchFunc(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
	needle, _ := args[2].(string)
	if needle == "" {
		return int64(0), nil
	}
	body := messageBody(valueString(args[0]), valueBytes(args[1]))
	if strings.Contains(strings.ToLower(body), needle) {
		return int64(1), nil
	}
	return int64(0), nil
}

func valueString(v driver.Value) string {
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	}
	return ""
}

func valueBytes(v driver.Value) []byte {
	switch t := v.(type) {
	case []byte:
		return t
	case string:
		return []byte(t)
	}
	return nil
}

// messageBody returns the best available plain text for a message.
func messageBody(text string, attributedBody []byte) string {
	if strings.TrimSpace(text) != "" {
		return strings.ReplaceAll(text, "\ufffc", "")
	}
	if s, ok := decodeAttributedBody(attributedBody); ok {
		return s
	}
	return ""
}

func openDB(ctx context.Context, path string) (*sql.DB, error) {
	dsn := (&url.URL{
		Scheme:   "file",
		Path:     path,
		RawQuery: "mode=ro&_pragma=busy_timeout(5000)&_pragma=query_only(1)",
	}).String()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(4)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// permissionHint wraps open/query failures with guidance for the macOS
// privacy (TCC) prompt that blocks reading ~/Library/Messages.
func permissionHint(path string, err error) error {
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "unable to open") || strings.Contains(msg, "authorization denied") ||
		strings.Contains(msg, "operation not permitted") {
		return fmt.Errorf("cannot read %s: %w. Grant Full Disk Access to the switchboard binary (or the terminal running it) in System Settings > Privacy & Security > Full Disk Access, then restart switchboard", path, err)
	}
	return fmt.Errorf("cannot read %s: %w", path, err)
}

// tableColumns returns the set of columns present on a table so queries can
// degrade gracefully on older macOS schema versions.
func tableColumns(ctx context.Context, db *sql.DB, table string) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, "SELECT name FROM pragma_table_info(?)", table)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	cols := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		cols[name] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(cols) == 0 {
		return nil, errors.New("table " + table + " not found; is this a Messages chat.db?")
	}
	return cols, nil
}

// appleTime converts a Messages timestamp to time.Time. Modern databases store
// nanoseconds since 2001-01-01; pre-High Sierra databases store seconds.
func appleTime(v int64) time.Time {
	if v == 0 {
		return time.Time{}
	}
	if v > 1e11 || v < -1e11 {
		return time.Unix(appleEpochOffset+v/1e9, v%1e9)
	}
	return time.Unix(appleEpochOffset+v, 0)
}

func toAppleTime(t time.Time) int64 {
	return (t.Unix()-appleEpochOffset)*1e9 + int64(t.Nanosecond())
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format(time.RFC3339)
}

func parseTimeArg(name, v string) (time.Time, error) {
	if v == "" {
		return time.Time{}, nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, v, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("%s must be an RFC3339 timestamp or YYYY-MM-DD date, got %q", name, v)
}
