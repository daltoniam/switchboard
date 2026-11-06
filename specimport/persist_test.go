package specimport

import (
	"encoding/json"
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
