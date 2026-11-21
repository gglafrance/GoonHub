package core

import (
	"testing"

	"goonhub/internal/data"
)

func TestWithDateFormat(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"", "auto", false},
		{"auto", "auto", false},
		{"DD/MM/YYYY", "DD/MM/YYYY", false},
		{"MM/DD/YYYY", "MM/DD/YYYY", false},
		{"YYYY-MM-DD", "YYYY-MM-DD", false},
		{"DD.MM.YYYY", "DD.MM.YYYY", false},
		{"D MMM YYYY", "D MMM YYYY", false},
		{"MMM D, YYYY", "MMM D, YYYY", false},
		{"dd/mm/yyyy", "", true},
		{"%Y-%m-%d", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			settings := &data.UserSettings{DateFormat: "unchanged"}
			err := WithDateFormat(tt.in)(settings)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if settings.DateFormat != "unchanged" {
					t.Fatalf("date format changed on error: %q", settings.DateFormat)
				}
				return
			}
			if err != nil || settings.DateFormat != tt.want {
				t.Fatalf("got %q, %v; want %q", settings.DateFormat, err, tt.want)
			}
		})
	}
}
