package project

import "encoding/json"

var definitionFields = map[string]struct{}{
	"$schema":     {},
	"version":     {},
	"name":        {},
	"description": {},
	"resources":   {},
	"launch":      {},
	"tools":       {},
	"agents":      {},
	"extensions":  {},
}

func (d *Definition) UnmarshalJSON(data []byte) error {
	type definitionAlias Definition
	var known definitionAlias
	if err := json.Unmarshal(data, &known); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	*d = Definition(known)
	d.Additional = make(map[string]json.RawMessage)
	for name, value := range fields {
		if _, ok := definitionFields[name]; !ok {
			d.Additional[name] = value
		}
	}
	if len(d.Additional) == 0 {
		d.Additional = nil
	}
	return nil
}

func (d Definition) MarshalJSON() ([]byte, error) {
	type definitionAlias Definition
	known, err := json.Marshal(definitionAlias(d))
	if err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(known, &fields); err != nil {
		return nil, err
	}
	delete(fields, "Additional")
	delete(fields, "baseDir")
	for name, value := range d.Additional {
		if _, ok := definitionFields[name]; !ok {
			fields[name] = value
		}
	}
	return json.Marshal(fields)
}
