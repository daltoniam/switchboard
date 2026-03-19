package mcp

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// ArgInt extracts an integer from args[key].
// Handles float64 (JSON default), int, int64, json.Number, and string.
// Returns (0, nil) for missing or nil keys.
// Returns (0, error) for present-but-unconvertible values.
func ArgInt(args map[string]any, key string) (int, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return 0, nil
	}
	switch v := v.(type) {
	case float64:
		return int(v), nil
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case json.Number:
		n, err := v.Int64()
		if err != nil {
			return 0, fmt.Errorf("parameter %q: cannot convert json.Number %q to int: %w", key, v.String(), err)
		}
		return int(n), nil
	case string:
		n, err := strconv.Atoi(v)
		if err != nil {
			return 0, fmt.Errorf("parameter %q: cannot convert string %q to int: %w", key, v, err)
		}
		return n, nil
	default:
		return 0, fmt.Errorf("parameter %q: cannot convert %T to int", key, v)
	}
}

// ArgInt64 extracts an int64 from args[key].
// Handles float64 (JSON default), int, int64, json.Number, and string.
// Returns (0, nil) for missing or nil keys.
// Returns (0, error) for present-but-unconvertible values.
func ArgInt64(args map[string]any, key string) (int64, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return 0, nil
	}
	switch v := v.(type) {
	case float64:
		return int64(v), nil
	case int:
		return int64(v), nil
	case int64:
		return v, nil
	case json.Number:
		n, err := v.Int64()
		if err != nil {
			return 0, fmt.Errorf("parameter %q: cannot convert json.Number %q to int64: %w", key, v.String(), err)
		}
		return n, nil
	case string:
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parameter %q: cannot convert string %q to int64: %w", key, v, err)
		}
		return n, nil
	default:
		return 0, fmt.Errorf("parameter %q: cannot convert %T to int64", key, v)
	}
}

// ArgFloat64 extracts a float64 from args[key].
// Handles float64 (JSON default), int, int64, json.Number, and string.
// Returns (0, nil) for missing or nil keys.
// Returns (0, error) for present-but-unconvertible values.
func ArgFloat64(args map[string]any, key string) (float64, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return 0, nil
	}
	switch v := v.(type) {
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case json.Number:
		n, err := v.Float64()
		if err != nil {
			return 0, fmt.Errorf("parameter %q: cannot convert json.Number %q to float64: %w", key, v.String(), err)
		}
		return n, nil
	case string:
		n, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0, fmt.Errorf("parameter %q: cannot convert string %q to float64: %w", key, v, err)
		}
		return n, nil
	default:
		return 0, fmt.Errorf("parameter %q: cannot convert %T to float64", key, v)
	}
}

// ArgStr extracts a string from args[key].
// Handles string (direct), fmt.Stringer, numeric types (float64, int, int64, json.Number),
// and bool. Returns ("", nil) for missing or nil keys.
// Returns ("", error) for truly unconvertible types (slices, maps, etc.).
func ArgStr(args map[string]any, key string) (string, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return "", nil
	}
	switch v := v.(type) {
	case string:
		return v, nil
	case float64:
		return formatFloat(v), nil
	case int:
		return strconv.Itoa(v), nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case json.Number:
		return v.String(), nil
	case bool:
		return strconv.FormatBool(v), nil
	case fmt.Stringer:
		return v.String(), nil
	default:
		return "", fmt.Errorf("parameter %q: cannot convert %T to string", key, v)
	}
}

// formatFloat formats a float64 as a string, dropping trailing ".0" for integers.
func formatFloat(f float64) string {
	if f == math.Trunc(f) && !math.IsInf(f, 0) && !math.IsNaN(f) {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}

// ArgInt32 extracts an int32 from args[key].
// Handles float64 (JSON default), int, int64, json.Number, and string.
// Returns (0, nil) for missing or nil keys.
// Returns (0, error) for present-but-unconvertible values.
func ArgInt32(args map[string]any, key string) (int32, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return 0, nil
	}
	switch v := v.(type) {
	case float64:
		if v > math.MaxInt32 || v < math.MinInt32 {
			return 0, fmt.Errorf("parameter %q: float64 %v overflows int32", key, v)
		}
		return int32(v), nil
	case int:
		if v > math.MaxInt32 || v < math.MinInt32 {
			return 0, fmt.Errorf("parameter %q: int %d overflows int32", key, v)
		}
		return int32(v), nil
	case int64:
		if v > math.MaxInt32 || v < math.MinInt32 {
			return 0, fmt.Errorf("parameter %q: int64 %d overflows int32", key, v)
		}
		return int32(v), nil
	case json.Number:
		n, err := v.Int64()
		if err != nil {
			return 0, fmt.Errorf("parameter %q: cannot convert json.Number %q to int32: %w", key, v.String(), err)
		}
		if n > math.MaxInt32 || n < math.MinInt32 {
			return 0, fmt.Errorf("parameter %q: json.Number %q overflows int32", key, v.String())
		}
		return int32(n), nil
	case string:
		n, err := strconv.ParseInt(v, 10, 32)
		if err != nil {
			return 0, fmt.Errorf("parameter %q: cannot convert string %q to int32: %w", key, v, err)
		}
		return int32(n), nil
	default:
		return 0, fmt.Errorf("parameter %q: cannot convert %T to int32", key, v)
	}
}

// ArgBool extracts a bool from args[key].
// Handles bool (direct) and string via strconv.ParseBool (accepts "true", "false",
// "1", "0", "t", "f", "TRUE", "FALSE"). This is intentionally broader than the
// previous per-adapter helpers which only accepted "true" exactly.
// Returns (false, nil) for missing or nil keys.
// Returns (false, error) for present-but-unconvertible values.
func ArgBool(args map[string]any, key string) (bool, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return false, nil
	}
	switch v := v.(type) {
	case bool:
		return v, nil
	case string:
		b, err := strconv.ParseBool(v)
		if err != nil {
			return false, fmt.Errorf("parameter %q: cannot convert string %q to bool: %w", key, v, err)
		}
		return b, nil
	default:
		return false, fmt.Errorf("parameter %q: cannot convert %T to bool", key, v)
	}
}

// ArgStrSlice extracts a []string from args[key].
// Handles []any (extracts strings, errors on non-string elements), []string (direct),
// and string (comma-split; empty string returns nil).
// Returns (nil, nil) for missing or nil keys.
// Returns (nil, error) for present-but-unconvertible values.
//
// Note: previous per-adapter helpers silently skipped non-string elements in []any.
// This version fails fast with an error, which surfaces malformed input to the caller.
func ArgStrSlice(args map[string]any, key string) ([]string, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return nil, nil
	}
	switch v := v.(type) {
	case []any:
		out := make([]string, 0, len(v))
		for i, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("parameter %q: element %d is %T, not string", key, i, item)
			}
			out = append(out, s)
		}
		return out, nil
	case []string:
		return v, nil
	case string:
		if v == "" {
			return nil, nil
		}
		return strings.Split(v, ","), nil
	default:
		return nil, fmt.Errorf("parameter %q: cannot convert %T to []string", key, v)
	}
}

// ArgMap extracts a map[string]any from args[key].
// Returns (nil, nil) for missing or nil keys.
// Returns (nil, error) for present-but-unconvertible values.
func ArgMap(args map[string]any, key string) (map[string]any, error) {
	v, ok := args[key]
	if !ok || v == nil {
		return nil, nil
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("parameter %q: cannot convert %T to map", key, v)
	}
	return m, nil
}

// OptInt extracts an int from args[key], returning def if the value is <= 0 or missing.
// Intended for pagination limits and similar positive-integer parameters.
// Type coercion errors are silently ignored (returns def).
func OptInt(args map[string]any, key string, def int) int {
	v, _ := ArgInt(args, key)
	if v > 0 {
		return v
	}
	return def
}

// OptInt64 extracts an int64 from args[key], returning def if the value is <= 0 or missing.
func OptInt64(args map[string]any, key string, def int64) int64 {
	v, _ := ArgInt64(args, key)
	if v > 0 {
		return v
	}
	return def
}

// Args is an accumulating argument reader (bufio.Scanner pattern).
// It stores the first error encountered; subsequent calls return zero values.
// Check Err() once after extracting all parameters.
type Args struct {
	m   map[string]any
	err error
}

// NewArgs creates an Args reader for the given parameter map.
func NewArgs(args map[string]any) *Args {
	return &Args{m: args}
}

// Err returns the first extraction error, or nil if all extractions succeeded.
func (a *Args) Err() error { return a.err }

// Str extracts a string parameter. Returns "" after the first error.
func (a *Args) Str(key string) string {
	if a.err != nil {
		return ""
	}
	v, err := ArgStr(a.m, key)
	if err != nil {
		a.err = err
		return ""
	}
	return v
}

// Int extracts an int parameter. Returns 0 after the first error.
func (a *Args) Int(key string) int {
	if a.err != nil {
		return 0
	}
	v, err := ArgInt(a.m, key)
	if err != nil {
		a.err = err
		return 0
	}
	return v
}

// Int32 extracts an int32 parameter. Returns 0 after the first error.
func (a *Args) Int32(key string) int32 {
	if a.err != nil {
		return 0
	}
	v, err := ArgInt32(a.m, key)
	if err != nil {
		a.err = err
		return 0
	}
	return v
}

// Int64 extracts an int64 parameter. Returns 0 after the first error.
func (a *Args) Int64(key string) int64 {
	if a.err != nil {
		return 0
	}
	v, err := ArgInt64(a.m, key)
	if err != nil {
		a.err = err
		return 0
	}
	return v
}

// Float64 extracts a float64 parameter. Returns 0 after the first error.
func (a *Args) Float64(key string) float64 {
	if a.err != nil {
		return 0
	}
	v, err := ArgFloat64(a.m, key)
	if err != nil {
		a.err = err
		return 0
	}
	return v
}

// Bool extracts a bool parameter. Returns false after the first error.
func (a *Args) Bool(key string) bool {
	if a.err != nil {
		return false
	}
	v, err := ArgBool(a.m, key)
	if err != nil {
		a.err = err
		return false
	}
	return v
}

// StrSlice extracts a []string parameter. Returns nil after the first error.
func (a *Args) StrSlice(key string) []string {
	if a.err != nil {
		return nil
	}
	v, err := ArgStrSlice(a.m, key)
	if err != nil {
		a.err = err
		return nil
	}
	return v
}

// Map extracts a map[string]any parameter. Returns nil after the first error.
func (a *Args) Map(key string) map[string]any {
	if a.err != nil {
		return nil
	}
	v, err := ArgMap(a.m, key)
	if err != nil {
		a.err = err
		return nil
	}
	return v
}
