package specimport

import (
	"encoding/json"

	mcp "github.com/daltoniam/switchboard"
)

type importedWire struct {
	Name       string          `json:"name"`
	Kind       SpecKind        `json:"kind"`
	BaseURL    string          `json:"baseURL"`
	Operations []operationWire `json:"operations"`
}

type operationWire struct {
	Tool           mcp.ToolDefinition    `json:"tool"`
	Effect         effect                `json:"effect"`
	HTTPMethod     string                `json:"httpMethod,omitempty"`
	PathTemplate   string                `json:"pathTemplate,omitempty"`
	PathBindings   []openAPIParamBinding `json:"pathBindings,omitempty"`
	QueryBindings  []openAPIParamBinding `json:"queryBindings,omitempty"`
	HeaderBindings []openAPIParamBinding `json:"headerBindings,omitempty"`
	CookieBindings []openAPIParamBinding `json:"cookieBindings,omitempty"`
	GQLDocument    string                `json:"gqlDocument,omitempty"`
	GQLVariables   []string              `json:"gqlVariables,omitempty"`
}

func (im *Imported) MarshalJSON() ([]byte, error) {
	w := importedWire{
		Name:    im.Name,
		Kind:    im.Kind,
		BaseURL: im.BaseURL,
	}
	for _, op := range im.operations {
		w.Operations = append(w.Operations, operationWire{
			Tool:           op.tool,
			Effect:         op.effect,
			HTTPMethod:     op.httpMethod,
			PathTemplate:   op.pathTemplate,
			PathBindings:   op.pathBindings,
			QueryBindings:  op.queryBindings,
			HeaderBindings: op.headerBindings,
			CookieBindings: op.cookieBindings,
			GQLDocument:    op.gqlDocument,
			GQLVariables:   op.gqlVariables,
		})
	}
	return json.Marshal(w)
}

func (im *Imported) UnmarshalJSON(data []byte) error {
	var w importedWire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	im.Name = w.Name
	im.Kind = w.Kind
	im.BaseURL = w.BaseURL
	im.operations = nil
	for _, ow := range w.Operations {
		im.operations = append(im.operations, operation{
			tool:           ow.Tool,
			effect:         ow.Effect,
			httpMethod:     ow.HTTPMethod,
			pathTemplate:   ow.PathTemplate,
			pathBindings:   ow.PathBindings,
			queryBindings:  ow.QueryBindings,
			headerBindings: ow.HeaderBindings,
			cookieBindings: ow.CookieBindings,
			gqlDocument:    ow.GQLDocument,
			gqlVariables:   ow.GQLVariables,
		})
	}
	return nil
}
