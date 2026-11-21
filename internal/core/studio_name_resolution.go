package core

import "strings"

// StudioNameResolver resolves a studio name or alias to its studio (StudioService).
type StudioNameResolver interface {
	Resolve(name string) (*StudioResolution, error)
}

// SetStudioResolver sets the resolver that turns a studio alias given by name (PornDB
// metadata apply) into the studio's name.
func (s *SceneService) SetStudioResolver(r StudioNameResolver) {
	s.studioResolver = r
}

// SetStudioResolver sets the resolver that turns a studio alias given by name (bulk studio
// edit) into the studio's name.
func (s *ExplorerService) SetStudioResolver(r StudioNameResolver) {
	s.studioResolver = r
}

// canonicalStudioName returns the name of the studio that name (a studio name or alias,
// case-insensitive) resolves to, or name unchanged when nothing matches.
func canonicalStudioName(r StudioNameResolver, name string) string {
	if r == nil || strings.TrimSpace(name) == "" {
		return name
	}
	res, err := r.Resolve(name)
	if err != nil || res == nil || res.Studio == nil {
		return name
	}
	return res.Studio.Name
}
