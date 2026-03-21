package tenant

import (
	"context"
	"testing"
)

func TestWithContext_FromContext(t *testing.T) {
	info := Info{
		TenantID: "tenant-1",
		UserID:   "user-42",
		Scopes:   []string{"read", "write"},
	}

	ctx := WithContext(context.Background(), info)
	got, ok := FromContext(ctx)
	if !ok {
		t.Fatal("expected tenant info in context")
	}
	if got.TenantID != "tenant-1" {
		t.Errorf("TenantID = %q, want %q", got.TenantID, "tenant-1")
	}
	if got.UserID != "user-42" {
		t.Errorf("UserID = %q, want %q", got.UserID, "user-42")
	}
	if len(got.Scopes) != 2 || got.Scopes[0] != "read" || got.Scopes[1] != "write" {
		t.Errorf("Scopes = %v, want [read write]", got.Scopes)
	}
}

func TestFromContext_Missing(t *testing.T) {
	_, ok := FromContext(context.Background())
	if ok {
		t.Fatal("expected no tenant info in bare context")
	}
}

func TestInfo_HasScope(t *testing.T) {
	tests := []struct {
		name   string
		scopes []string
		check  string
		want   bool
	}{
		{name: "present", scopes: []string{"read", "write"}, check: "write", want: true},
		{name: "absent", scopes: []string{"read"}, check: "admin", want: false},
		{name: "empty scopes", scopes: nil, check: "read", want: false},
		{name: "empty check", scopes: []string{"read"}, check: "", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := Info{Scopes: tt.scopes}
			if got := info.HasScope(tt.check); got != tt.want {
				t.Errorf("HasScope(%q) = %v, want %v", tt.check, got, tt.want)
			}
		})
	}
}
