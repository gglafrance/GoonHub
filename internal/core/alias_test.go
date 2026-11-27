package core

import (
	"fmt"
	"strings"
	"testing"

	"goonhub/internal/apperrors"
)

func TestNormalizeAliases(t *testing.T) {
	cases := []struct {
		name    string
		in      []string
		want    []string
		wantErr bool
	}{
		{"nil gives an empty list", nil, []string{}, false},
		{"trims and drops empty", []string{"  Jane D ", "", "   "}, []string{"Jane D"}, false},
		{"drops the own name, case-insensitive", []string{"jane doe", "JD"}, []string{"JD"}, false},
		{"case-insensitive duplicates, first spelling wins", []string{"Janie", "JANIE", "janie "}, []string{"Janie"}, false},
		{"keeps order", []string{"b", "a", "c"}, []string{"b", "a", "c"}, false},
		{"255 characters is fine", []string{strings.Repeat("é", 255)}, []string{strings.Repeat("é", 255)}, false},
		{"256 characters is too long", []string{strings.Repeat("a", 256)}, nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := NormalizeAliases("Jane Doe", c.in)
			if c.wantErr {
				if !apperrors.IsValidation(err) {
					t.Fatalf("want a validation error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got == nil || !sameStrings(got, c.want) {
				t.Fatalf("got %#v, want %#v", got, c.want)
			}
		})
	}
}

func TestNormalizeAliasesTooMany(t *testing.T) {
	many := make([]string, maxAliases+1)
	for i := range many {
		many[i] = fmt.Sprintf("alias %d", i)
	}
	if _, err := NormalizeAliases("n", many); !apperrors.IsValidation(err) {
		t.Fatalf("want a validation error for %d aliases, got %v", len(many), err)
	}
	if _, err := NormalizeAliases("n", many[:maxAliases]); err != nil {
		t.Fatalf("%d aliases must be accepted: %v", maxAliases, err)
	}
}

func TestMergeAliases(t *testing.T) {
	merged, added := MergeAliases("Jane Doe", []string{"JD", "Janie"}, "Jane D.", []string{"jd", "Jane Doe", "J. Doe"})
	if want := []string{"JD", "Janie", "Jane D.", "J. Doe"}; !sameStrings(merged, want) {
		t.Fatalf("merged = %#v, want %#v", merged, want)
	}
	if want := []string{"Jane D.", "J. Doe"}; !sameStrings(added, want) {
		t.Fatalf("added = %#v, want %#v", added, want)
	}

	// same name: nothing new
	merged, added = MergeAliases("Jane Doe", nil, "jane doe", nil)
	if len(merged) != 0 || len(added) != 0 || added == nil {
		t.Fatalf("same name: merged %#v, added %#v", merged, added)
	}
}

func TestMatchedAlias(t *testing.T) {
	cases := []struct {
		name, query string
		aliases     []string
		want        string
	}{
		{"Jane Doe", "jane", []string{"JD"}, ""},
		{"Jane Doe", "", []string{"JD"}, ""},
		{"Jane Doe", "janie", []string{"JD", "Janie Doe"}, "Janie Doe"},
		{"Jane Doe", "xyz", []string{"JD"}, ""},
	}
	for _, c := range cases {
		if got := MatchedAlias(c.name, c.aliases, c.query); got != c.want {
			t.Fatalf("MatchedAlias(%q, %v, %q) = %q, want %q", c.name, c.aliases, c.query, got, c.want)
		}
	}
}

func TestMatchedName(t *testing.T) {
	if alias, ok := matchedName("Jane Doe", []string{"JD"}, " jane doe "); !ok || alias != "" {
		t.Fatalf("name match: %q %v", alias, ok)
	}
	if alias, ok := matchedName("Jane Doe", []string{"JD"}, "jd"); !ok || alias != "JD" {
		t.Fatalf("alias match: %q %v", alias, ok)
	}
	if _, ok := matchedName("Jane Doe", []string{"JD"}, "Jane"); ok {
		t.Fatal("substring must not match")
	}
}
