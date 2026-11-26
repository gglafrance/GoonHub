package core

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"

	"go.uber.org/zap"
)

// PrivacyService answers "can this viewer see this item of this owner" (docs/SOCIAL.md). The
// owner always sees everything; there is no administrator bypass: privacy applies to every
// viewer, superusers included.
type PrivacyService struct {
	repo   data.PrivacyRepository
	social data.SocialRepository
	logger *zap.Logger

	listenersMu sync.RWMutex
	listeners   []PrivacyChangeListener
}

// PrivacyChangeListener is called after a user changed the audience of some items (keys of
// the changed items), e.g. to re-evaluate who sees their presence at once.
type PrivacyChangeListener func(userID uint, items []string)

// OnChange registers a listener of the privacy changes (called synchronously after the
// change is saved: keep it short).
func (s *PrivacyService) OnChange(fn PrivacyChangeListener) {
	s.listenersMu.Lock()
	defer s.listenersMu.Unlock()
	s.listeners = append(s.listeners, fn)
}

func (s *PrivacyService) notifyChange(userID uint, items []string) {
	s.listenersMu.RLock()
	listeners := append([]PrivacyChangeListener(nil), s.listeners...)
	s.listenersMu.RUnlock()
	for _, fn := range listeners {
		fn(userID, items)
	}
}

func NewPrivacyService(repo data.PrivacyRepository, social data.SocialRepository, logger *zap.Logger) *PrivacyService {
	return &PrivacyService{repo: repo, social: social, logger: logger.With(zap.String("service", "privacy"))}
}

// PrivacyAudienceDefault resets an item to its catalog default in UpdateSettings.
const PrivacyAudienceDefault = "default"

// PrivacyItemSetting is the effective audience of an item for a user.
type PrivacyItemSetting struct {
	Item     PrivacyItem
	Audience Audience
	// IsDefault: the user did not choose, Audience is the catalog default.
	IsDefault bool
}

// PrivacySettings are the privacy choices of a user, every catalog item included.
type PrivacySettings struct {
	Items               []PrivacyItemSetting
	FriendRequestPolicy string
}

// UpdatePrivacyInput changes some privacy choices: Items maps an item key to an audience or
// to PrivacyAudienceDefault; absent items are unchanged. FriendRequestPolicy nil = unchanged.
type UpdatePrivacyInput struct {
	Items               map[string]string
	FriendRequestPolicy *string
}

// AudienceMembers is who is in the audience of an item of an owner (the owner excluded).
type AudienceMembers struct {
	// Everyone: every signed-in user (public audience); UserIDs is empty then.
	Everyone bool
	// UserIDs: the users in the audience, sorted (empty for private).
	UserIDs []uint
}

// Contains reports whether a user (other than the owner) is in the audience.
func (m AudienceMembers) Contains(userID uint) bool {
	if m.Everyone {
		return true
	}
	i := sort.Search(len(m.UserIDs), func(i int) bool { return m.UserIDs[i] >= userID })
	return i < len(m.UserIDs) && m.UserIDs[i] == userID
}

func catalogItem(key string) (PrivacyItem, error) {
	item, ok := privacyItemByKey(key)
	if !ok {
		return PrivacyItem{}, fmt.Errorf("unknown privacy item %q", key)
	}
	return item, nil
}

// effectiveAudience is the stored audience when valid, else the item default.
func effectiveAudience(item PrivacyItem, stored string) Audience {
	if a := Audience(stored); stored != "" && a.Valid() {
		return a
	}
	return item.Default
}

// AudienceOf returns the effective audience of an item of an owner.
func (s *PrivacyService) AudienceOf(ctx context.Context, ownerID uint, key string) (Audience, error) {
	item, err := catalogItem(key)
	if err != nil {
		return "", err
	}
	stored, err := s.repo.AudiencesFor(ctx, []uint{ownerID}, key)
	if err != nil {
		return "", err
	}
	return effectiveAudience(item, stored[ownerID]), nil
}

// CanView reports whether the viewer can see the item of the owner.
func (s *PrivacyService) CanView(ctx context.Context, viewerID, ownerID uint, key string) (bool, error) {
	visible, err := s.VisibleTo(ctx, viewerID, []uint{ownerID}, key)
	if err != nil {
		return false, err
	}
	return visible[ownerID], nil
}

// CanViewAll reports whether the viewer can see every one of the items of the owner (e.g. the
// profile page and one of its blocks).
func (s *PrivacyService) CanViewAll(ctx context.Context, viewerID, ownerID uint, keys ...string) (bool, error) {
	visible, err := s.Visibility(ctx, viewerID, ownerID, keys)
	if err != nil {
		return false, err
	}
	for _, k := range keys {
		if !visible[k] {
			return false, nil
		}
	}
	return true, nil
}

// Visibility returns, for one owner, whether the viewer can see each item (two queries at
// most, whatever the number of items).
func (s *PrivacyService) Visibility(ctx context.Context, viewerID, ownerID uint, keys []string) (map[string]bool, error) {
	out := make(map[string]bool, len(keys))
	items := make([]PrivacyItem, 0, len(keys))
	for _, k := range keys {
		item, err := catalogItem(k)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if viewerID == ownerID {
		for _, k := range keys {
			out[k] = true
		}
		return out, nil
	}
	rows, err := s.repo.ListSettings(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	stored := make(map[string]string, len(rows))
	for _, row := range rows {
		stored[row.Item] = row.Audience
	}
	var rel *Relationship
	for _, item := range items {
		a := effectiveAudience(item, stored[item.Key])
		if a == AudiencePublic {
			out[item.Key] = true
			continue
		}
		if rel == nil {
			rels, err := loadRelationships(ctx, s.social, viewerID, []uint{ownerID})
			if err != nil {
				return nil, err
			}
			r := rels[ownerID]
			rel = &r
		}
		out[item.Key] = rel.Passes(a)
	}
	return out, nil
}

// VisibleTo returns, for each owner, whether the viewer can see the item: every requested id
// is a key of the map. Two queries at most whatever the number of owners (the relations are
// only read for the owners whose audience is not public).
func (s *PrivacyService) VisibleTo(ctx context.Context, viewerID uint, ownerIDs []uint, key string) (map[uint]bool, error) {
	item, err := catalogItem(key)
	if err != nil {
		return nil, err
	}
	out := make(map[uint]bool, len(ownerIDs))
	others := make([]uint, 0, len(ownerIDs))
	for _, id := range ownerIDs {
		if id == viewerID {
			out[id] = true
		} else {
			others = append(others, id)
		}
	}
	if len(others) == 0 {
		return out, nil
	}
	stored, err := s.repo.AudiencesFor(ctx, others, key)
	if err != nil {
		return nil, err
	}
	audiences := make(map[uint]Audience, len(others))
	var restricted []uint
	for _, id := range others {
		a := effectiveAudience(item, stored[id])
		audiences[id] = a
		if a == AudiencePublic {
			out[id] = true
		} else if a != AudiencePrivate {
			restricted = append(restricted, id)
		} else {
			out[id] = false
		}
	}
	if len(restricted) > 0 {
		rels, err := loadRelationships(ctx, s.social, viewerID, restricted)
		if err != nil {
			return nil, err
		}
		for _, id := range restricted {
			out[id] = rels[id].Passes(audiences[id])
		}
	}
	return out, nil
}

// FilterVisible returns the owners whose item the viewer can see, in the given order.
func (s *PrivacyService) FilterVisible(ctx context.Context, viewerID uint, ownerIDs []uint, key string) ([]uint, error) {
	visible, err := s.VisibleTo(ctx, viewerID, ownerIDs, key)
	if err != nil {
		return nil, err
	}
	out := make([]uint, 0, len(ownerIDs))
	for _, id := range ownerIDs {
		if visible[id] {
			out = append(out, id)
		}
	}
	return out, nil
}

// AudienceMembers returns who can see the item of the owner, to push a change to them only
// (e.g. presence): everyone for public, else the followers, mutuals or friends (friends are
// in every audience but private).
func (s *PrivacyService) AudienceMembers(ctx context.Context, ownerID uint, key string) (AudienceMembers, error) {
	a, err := s.AudienceOf(ctx, ownerID, key)
	if err != nil {
		return AudienceMembers{}, err
	}
	var ids []uint
	switch a {
	case AudiencePublic:
		return AudienceMembers{Everyone: true}, nil
	case AudiencePrivate:
		return AudienceMembers{}, nil
	case AudienceFollowers:
		if ids, err = s.social.FollowerIDs(ctx, ownerID); err != nil {
			return AudienceMembers{}, err
		}
	case AudienceMutuals:
		if ids, err = s.social.MutualIDs(ctx, ownerID); err != nil {
			return AudienceMembers{}, err
		}
	}
	friends, err := s.social.FriendIDs(ctx, ownerID)
	if err != nil {
		return AudienceMembers{}, err
	}
	return AudienceMembers{UserIDs: mergeSortedIDs(ids, friends)}, nil
}

func mergeSortedIDs(a, b []uint) []uint {
	seen := make(map[uint]bool, len(a)+len(b))
	out := make([]uint, 0, len(a)+len(b))
	for _, list := range [][]uint{a, b} {
		for _, id := range list {
			if !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// GetSettings returns the privacy settings of a user, every catalog item included.
func (s *PrivacyService) GetSettings(ctx context.Context, userID uint) (*PrivacySettings, error) {
	rows, err := s.repo.ListSettings(ctx, userID)
	if err != nil {
		return nil, err
	}
	stored := make(map[string]string, len(rows))
	for _, row := range rows {
		stored[row.Item] = row.Audience
	}
	policy, err := s.social.GetFriendRequestPolicy(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to read the friend request policy: %w", err)
	}
	out := &PrivacySettings{Items: make([]PrivacyItemSetting, 0, len(PrivacyCatalog)), FriendRequestPolicy: policy}
	for _, item := range PrivacyCatalog {
		v, ok := stored[item.Key]
		isDefault := !ok || !Audience(v).Valid()
		out.Items = append(out.Items, PrivacyItemSetting{Item: item, Audience: effectiveAudience(item, v), IsDefault: isDefault})
	}
	return out, nil
}

// UpdateSettings applies some privacy choices of a user and returns the new settings.
// Unknown items, audiences or policies are refused (validation error) and nothing changes.
func (s *PrivacyService) UpdateSettings(ctx context.Context, userID uint, in UpdatePrivacyInput) (*PrivacySettings, error) {
	set := make(map[string]string)
	var reset []string
	for key, value := range in.Items {
		if _, ok := privacyItemByKey(key); !ok {
			return nil, apperrors.NewValidationErrorWithField("items", fmt.Sprintf("unknown privacy item %q", key))
		}
		if value == PrivacyAudienceDefault {
			reset = append(reset, key)
			continue
		}
		if !Audience(value).Valid() {
			return nil, apperrors.NewValidationErrorWithField("items", fmt.Sprintf("unknown audience %q for %s", value, key))
		}
		set[key] = value
	}
	sort.Strings(reset)
	if in.FriendRequestPolicy != nil && !validFriendRequestPolicy(*in.FriendRequestPolicy) {
		return nil, apperrors.NewValidationErrorWithField("friend_request_policy", fmt.Sprintf("unknown friend request policy %q", *in.FriendRequestPolicy))
	}
	if len(set) > 0 || len(reset) > 0 {
		if err := s.repo.SaveSettings(ctx, userID, set, reset); err != nil {
			return nil, err
		}
		changed := append([]string(nil), reset...)
		for key := range set {
			changed = append(changed, key)
		}
		sort.Strings(changed)
		s.notifyChange(userID, changed)
	}
	if in.FriendRequestPolicy != nil {
		if err := s.social.SetFriendRequestPolicy(ctx, userID, *in.FriendRequestPolicy); err != nil {
			return nil, fmt.Errorf("failed to save the friend request policy: %w", err)
		}
	}
	return s.GetSettings(ctx, userID)
}

// loadRelationships returns the relation of the viewer toward each user (one query; the
// viewer toward themselves is Self).
func loadRelationships(ctx context.Context, repo data.SocialRepository, viewerID uint, userIDs []uint) (map[uint]Relationship, error) {
	out := make(map[uint]Relationship, len(userIDs))
	others := make([]uint, 0, len(userIDs))
	for _, id := range userIDs {
		if id == viewerID {
			out[id] = selfRelationship
		} else {
			others = append(others, id)
		}
	}
	if len(others) == 0 {
		return out, nil
	}
	rows, err := repo.Relations(ctx, viewerID, others)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.UserID] = relationshipFrom(row)
	}
	return out, nil
}
