package core

import (
	"net"
	"net/url"
	"strings"
)

// sourceSiteVariantLabels are the leading host labels that only pick a variant of the same
// site (www.pornhub.com, m.pornhub.com, mobile.example.com), stripped from the source site.
// Two-letter language or country labels (fr.pornhub.com, de.xhamster.com) and locale labels
// (en-us.example.com) are stripped too, see isLanguageLabel.
var sourceSiteVariantLabels = map[string]bool{
	"www":    true,
	"m":      true,
	"mobile": true,
}

// isLanguageLabel reports whether a host label looks like a language or country variant:
// two ASCII letters ("fr", "de") or a locale ("en-us", "pt-br"). This is a heuristic, not a
// list of ISO 639 codes: on a host of three labels or more, any leading two-letter label is
// taken as a variant.
func isLanguageLabel(label string) bool {
	isLetters := func(s string) bool {
		for i := 0; i < len(s); i++ {
			if s[i] < 'a' || s[i] > 'z' {
				return false
			}
		}
		return true
	}
	switch len(label) {
	case 2:
		return isLetters(label)
	case 5:
		return label[2] == '-' && isLetters(label[:2]) && isLetters(label[3:])
	}
	return false
}

// SourceSiteFromURL returns the site a scene comes from, from its source URL: the host,
// lowercased, without port, trailing dot, and leading variant labels (www, m, mobile, and
// language codes), so that https://fr.pornhub.com/view_video.php?..., https://www.pornhub.com/...
// and https://m.pornhub.com/... all give "pornhub.com". A host always keeps at least two
// labels ("m.ok.ru" gives "ok.ru", "ok.ru" stays). IP addresses are kept as they are.
// Returns "" when there is no source or the URL has no host.
//
// No public suffix list is used: hosts under a multi-label suffix keep their own label
// (www.bbc.co.uk gives bbc.co.uk), which is what grouping by site needs.
func SourceSiteFromURL(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return ""
	}
	return normalizeSourceHost(parsed.Hostname())
}

// NormalizeSourceSite normalizes a source site given by a client (a search filter value):
// a bare host ("www.PornHub.com") or a full URL, normalized like SourceSiteFromURL.
func NormalizeSourceSite(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.Contains(value, "://") {
		return SourceSiteFromURL(value)
	}
	// A bare host, possibly with a path or a port ("pornhub.com/view", "example.com:8080")
	if i := strings.IndexAny(value, "/?#"); i >= 0 {
		value = value[:i]
	}
	return SourceSiteFromURL("http://" + value)
}

// normalizeSourceHost lowercases a host and strips its variant labels.
func normalizeSourceHost(host string) string {
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	if host == "" {
		return ""
	}
	if net.ParseIP(host) != nil {
		return host
	}
	labels := strings.Split(host, ".")
	for len(labels) > 2 && (sourceSiteVariantLabels[labels[0]] || isLanguageLabel(labels[0])) {
		labels = labels[1:]
	}
	return strings.Join(labels, ".")
}

// ParseSourceSites splits a comma-separated list of source sites (search filter) and
// normalizes it with NormalizeSourceSites.
func ParseSourceSites(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	return NormalizeSourceSites(strings.Split(raw, ","))
}

// NormalizeSourceSites normalizes source sites with NormalizeSourceSite and drops empty
// values and duplicates, keeping the first-seen order. Returns nil when nothing is left.
func NormalizeSourceSites(values []string) []string {
	seen := make(map[string]bool, len(values))
	var sites []string
	for _, value := range values {
		site := NormalizeSourceSite(value)
		if site == "" || seen[site] {
			continue
		}
		seen[site] = true
		sites = append(sites, site)
	}
	return sites
}
