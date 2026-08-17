package stripe

// Shared parameter helpers used across handlers.

// copyIfPresent copies a key from src into out if the key exists in src.
// This preserves the value as-is so encodeForm can flatten objects/arrays/scalars correctly.
func copyIfPresent(out map[string]any, src map[string]any, keys ...string) {
	for _, k := range keys {
		if v, ok := src[k]; ok {
			out[k] = v
		}
	}
}

// listParamsFrom extracts the standard list-pagination params (limit, starting_after, ending_before)
// plus any additional named keys from args.
func listParamsFrom(args map[string]any, extra ...string) map[string]any {
	out := map[string]any{}
	copyIfPresent(out, args, "limit", "starting_after", "ending_before")
	copyIfPresent(out, args, extra...)
	return out
}

// searchParamsFrom extracts query/limit/page from args for Stripe search endpoints.
func searchParamsFrom(args map[string]any) map[string]any {
	out := map[string]any{}
	copyIfPresent(out, args, "query", "limit", "page")
	return out
}
