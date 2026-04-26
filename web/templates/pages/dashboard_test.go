package pages

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSortByName(t *testing.T) {
	items := []IntegrationSummary{
		{Name: "charlie"},
		{Name: "alpha"},
		{Name: "bravo"},
	}
	sorted := sortByName(items)
	assert.Equal(t, "alpha", sorted[0].Name)
	assert.Equal(t, "bravo", sorted[1].Name)
	assert.Equal(t, "charlie", sorted[2].Name)

	assert.Equal(t, "charlie", items[0].Name)
}

func TestLastCheckLabel(t *testing.T) {
	tests := []struct {
		name string
		when time.Time
		want string
	}{
		{"zero", time.Time{}, ""},
		{"just now", time.Now().Add(-5 * time.Second), "just now"},
		{"minutes", time.Now().Add(-15 * time.Minute), "15m ago"},
		{"hours", time.Now().Add(-3 * time.Hour), "3h ago"},
		{"days", time.Now().Add(-48 * time.Hour), time.Now().Add(-48 * time.Hour).Format("Jan 2 15:04")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lastCheckLabel(tt.when)
			assert.Equal(t, tt.want, got)
		})
	}
}
