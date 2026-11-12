package specimport

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestImported_JSONRoundTrip(t *testing.T) {
	im, err := Parse(KindOpenAPI, "demo", []byte(openAPIFixture), "")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	raw, err := json.Marshal(im)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var back Imported
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(back.Tools()) != len(im.Tools()) {
		t.Fatalf("tools = %d, want %d", len(back.Tools()), len(im.Tools()))
	}
	in := NewIntegration(&back)
	if len(in.Tools()) != len(im.Tools()) {
		t.Fatalf("integration tools = %d, want %d", len(in.Tools()), len(im.Tools()))
	}
}

// A rehydrated import must still know how to send every argument, not just
// list its tools.
func TestImported_JSONRoundTripKeepsBindings(t *testing.T) {
	im, err := Parse(KindOpenAPI, "demo", []byte(openAPIFixture), "")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	raw, err := json.Marshal(im)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var back Imported
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	bindings := 0
	for i, op := range im.operations {
		got := back.operations[i]
		for _, pair := range [][2][]openAPIParamBinding{
			{op.pathBindings, got.pathBindings},
			{op.queryBindings, got.queryBindings},
			{op.headerBindings, got.headerBindings},
			{op.cookieBindings, got.cookieBindings},
		} {
			if !reflect.DeepEqual(pair[0], pair[1]) {
				t.Fatalf("%s: bindings changed in round trip: %+v -> %+v", op.tool.Name, pair[0], pair[1])
			}
			bindings += len(pair[0])
		}
	}
	if bindings == 0 {
		t.Fatal("fixture has no parameter bindings to check")
	}
}
