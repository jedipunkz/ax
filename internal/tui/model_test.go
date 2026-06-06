package tui

import "testing"

func TestClampScroll(t *testing.T) {
	tests := []struct {
		name      string
		cursor    int
		offset    int
		availRows int
		want      int
	}{
		{name: "cursor within window keeps offset", cursor: 3, offset: 2, availRows: 5, want: 2},
		{name: "cursor above window scrolls up", cursor: 1, offset: 4, availRows: 5, want: 1},
		{name: "cursor below window scrolls down", cursor: 10, offset: 0, availRows: 5, want: 6},
		{name: "negative offset clamped to zero", cursor: 0, offset: -3, availRows: 5, want: 0},
		{name: "zero availRows leaves offset", cursor: 9, offset: 2, availRows: 0, want: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clampScroll(tt.cursor, tt.offset, tt.availRows); got != tt.want {
				t.Errorf("clampScroll(%d, %d, %d) = %d, want %d",
					tt.cursor, tt.offset, tt.availRows, got, tt.want)
			}
		})
	}
}
