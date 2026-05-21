package ui

import "testing"

func TestPageJumpIndex(t *testing.T) {
	tests := []struct {
		name     string
		selected int
		total    int
		delta    int
		want     int
	}{
		{"empty list", 0, 0, 20, 0},
		{"down one page from top", 0, 50, 20, 20},
		{"down clamps at end", 45, 50, 20, 49},
		{"up one page from middle", 25, 50, -20, 5},
		{"up clamps at top", 3, 50, -20, 0},
		{"single row no move", 0, 1, 20, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pageJumpIndex(tt.selected, tt.total, tt.delta)
			if got != tt.want {
				t.Fatalf("pageJumpIndex(%d, %d, %d) = %d, want %d",
					tt.selected, tt.total, tt.delta, got, tt.want)
			}
		})
	}
}
