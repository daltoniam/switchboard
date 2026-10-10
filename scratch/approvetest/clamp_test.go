package scratch

import "testing"

func TestClamp(t *testing.T) {
	tests := []struct {
		name                   string
		value, low, high, want int
	}{
		{"inside", 5, 1, 10, 5},
		{"below", -3, 1, 10, 1},
		{"above", 42, 1, 10, 10},
		{"on low bound", 1, 1, 10, 1},
		{"on high bound", 10, 1, 10, 10},
		{"swapped bounds", 42, 10, 1, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Clamp(tt.value, tt.low, tt.high); got != tt.want {
				t.Fatalf("Clamp(%d, %d, %d) = %d, want %d", tt.value, tt.low, tt.high, got, tt.want)
			}
		})
	}
}
