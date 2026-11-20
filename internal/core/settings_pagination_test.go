package core

import (
	"testing"

	"goonhub/internal/data"
)

func TestWithPaginationPosition(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"", "bottom", false},
		{"top", "top", false},
		{"bottom", "bottom", false},
		{"both", "both", false},
		{"left", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			settings := &data.UserSettings{}
			err := WithPaginationPosition(tt.in)(settings)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil || settings.PaginationPosition != tt.want {
				t.Fatalf("got %q, %v; want %q", settings.PaginationPosition, err, tt.want)
			}
		})
	}
}
