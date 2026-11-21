package core

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"goonhub/internal/apperrors"
)

const (
	// maxAliasLength is the maximum length of one alias (same as a name).
	maxAliasLength = 255
	// maxAliases is the maximum number of aliases of an actor or a studio.
	maxAliases = 200
)

// NormalizeAliases trims the aliases, drops the empty ones and those equal to the entity's
// own name, and removes case-insensitive duplicates (the first spelling wins). It never
// returns nil. Aliases that are too long, or too many aliases, are a validation error.
func NormalizeAliases(name string, aliases []string) ([]string, error) {
	out := make([]string, 0, len(aliases))
	seen := make(map[string]bool, len(aliases))
	ownName := strings.ToLower(strings.TrimSpace(name))
	for _, a := range aliases {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		if utf8.RuneCountInString(a) > maxAliasLength {
			return nil, apperrors.NewValidationErrorWithField("aliases",
				fmt.Sprintf("alias '%s...' must be %d characters or less", string([]rune(a)[:32]), maxAliasLength))
		}
		key := strings.ToLower(a)
		if key == ownName || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, a)
	}
	if len(out) > maxAliases {
		return nil, apperrors.NewValidationErrorWithField("aliases",
			fmt.Sprintf("at most %d aliases are allowed", maxAliases))
	}
	return out, nil
}

// MergeAliases returns the aliases of a target entity after merging a source entity into it:
// the target's aliases, then the source's name and aliases, normalized against the target's
// name. added lists those that the target did not have.
func MergeAliases(targetName string, targetAliases []string, sourceName string, sourceAliases []string) (merged, added []string) {
	merged, _ = NormalizeAliases(targetName, append(append(append([]string{}, targetAliases...), sourceName), sourceAliases...))
	if merged == nil {
		// only possible with more than maxAliases: keep everything, the merge must not lose names
		merged = dedupeFold(targetName, append(append(append([]string{}, targetAliases...), sourceName), sourceAliases...))
	}
	had := make(map[string]bool, len(targetAliases))
	for _, a := range targetAliases {
		had[strings.ToLower(strings.TrimSpace(a))] = true
	}
	added = []string{}
	for _, a := range merged {
		if !had[strings.ToLower(a)] {
			added = append(added, a)
		}
	}
	return merged, added
}

func dedupeFold(name string, values []string) []string {
	out := []string{}
	seen := map[string]bool{strings.ToLower(strings.TrimSpace(name)): true}
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" || utf8.RuneCountInString(v) > maxAliasLength || seen[strings.ToLower(v)] {
			continue
		}
		seen[strings.ToLower(v)] = true
		out = append(out, v)
	}
	return out
}

// matchedName tells whether value is the entity's name ("" returned) or one of its aliases
// (the alias returned, with its stored spelling), case-insensitively. ok is false otherwise.
func matchedName(name string, aliases []string, value string) (alias string, ok bool) {
	value = strings.TrimSpace(value)
	if strings.EqualFold(name, value) {
		return "", true
	}
	for _, a := range aliases {
		if strings.EqualFold(a, value) {
			return a, true
		}
	}
	return "", false
}

// MatchedAlias returns the first alias that contains query (case-insensitive) when the
// name does not, for search results ("" when the name matches or nothing does).
func MatchedAlias(name string, aliases []string, query string) string {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" || strings.Contains(strings.ToLower(name), q) {
		return ""
	}
	for _, a := range aliases {
		if strings.Contains(strings.ToLower(a), q) {
			return a
		}
	}
	return ""
}
