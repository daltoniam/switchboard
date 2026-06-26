package prompts

var Meta = metaAccessors{}

type metaAccessors struct{}

func (metaAccessors) Search() string  { return render(metaTmpl, "search.md.tmpl", nil) }
func (metaAccessors) Execute() string { return render(metaTmpl, "execute.md.tmpl", nil) }
func (metaAccessors) Session() string { return render(metaTmpl, "session.md.tmpl", nil) }
func (metaAccessors) History() string { return render(metaTmpl, "history.md.tmpl", nil) }
func (metaAccessors) Pin() string     { return render(metaTmpl, "pin.md.tmpl", nil) }
