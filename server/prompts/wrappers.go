package prompts

func SearchSummary(ctx Context, total int, query string) string {
	return render(dynamicTmpl, "search_summary.md.tmpl", struct {
		Ctx   Context
		Total int
		Query string
	}{ctx, total, query})
}

func SearchHintMulti(ctx Context) string {
	return render(dynamicTmpl, "search_hint_multi.md.tmpl", struct{ Ctx Context }{ctx})
}

func SearchHintSingle(ctx Context) string {
	return render(dynamicTmpl, "search_hint_single.md.tmpl", struct{ Ctx Context }{ctx})
}

func ResponseTooLargeHint(ctx Context) string {
	return render(dynamicTmpl, "response_too_large_hint.md.tmpl", struct{ Ctx Context }{ctx})
}

func CircuitBreaker(ctx Context, integration string, cooldownSeconds int) string {
	return render(dynamicTmpl, "circuit_breaker.md.tmpl", struct {
		Ctx             Context
		Integration     string
		CooldownSeconds int
	}{ctx, integration, cooldownSeconds})
}
