package data

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/lib/pq"
	"gorm.io/gorm"
)

// SocialRepository stores the social graph: follows, friendships and pending friend requests
// (docs/SOCIAL.md). Self relations are refused by the schema (CHECK constraints); the service
// validates them first.
type SocialRepository interface {
	// GetUser returns the card of a user (gorm.ErrRecordNotFound when unknown).
	GetUser(ctx context.Context, userID uint) (*SocialUser, error)
	// GetUserByUsername returns the card of a user by exact username.
	GetUserByUsername(ctx context.Context, username string) (*SocialUser, error)
	// SearchUsers lists the users other than the viewer whose username contains the query
	// (every user when empty), prefix matches first, then by username.
	SearchUsers(ctx context.Context, viewerID uint, query string, page, limit int) ([]SocialUser, int64, error)

	// Follow makes followerID follow followedID; created is false when it already did.
	Follow(ctx context.Context, followerID, followedID uint) (created bool, err error)
	// Unfollow removes the follow; removed is false when there was none.
	Unfollow(ctx context.Context, followerID, followedID uint) (removed bool, err error)
	// ListFollowers lists the users following userID, most recent first (Since = follow date).
	ListFollowers(ctx context.Context, userID uint, page, limit int) ([]SocialUser, int64, error)
	// ListFollowing lists the users userID follows, most recent first.
	ListFollowing(ctx context.Context, userID uint, page, limit int) ([]SocialUser, int64, error)
	// ListFriends lists the friends of userID, most recent first (Since = friendship date).
	ListFriends(ctx context.Context, userID uint, page, limit int) ([]SocialUser, int64, error)
	// RemoveFriend ends the friendship of a and b; removed is false when they were not friends.
	RemoveFriend(ctx context.Context, a, b uint) (removed bool, err error)
	// FriendIDs, FollowerIDs and MutualIDs return the ids of the friends, followers and mutuals
	// (users following and followed by userID) of a user.
	FriendIDs(ctx context.Context, userID uint) ([]uint, error)
	FollowerIDs(ctx context.Context, userID uint) ([]uint, error)
	MutualIDs(ctx context.Context, userID uint) ([]uint, error)
	// Counts returns the number of friends, followers and followed users of a user.
	Counts(ctx context.Context, userID uint) (SocialCounts, error)
	// Relations returns the relation of the viewer toward each user (one query, one row per
	// distinct user id, unknown ids included with every flag false).
	Relations(ctx context.Context, viewerID uint, userIDs []uint) ([]UserRelation, error)

	// SendFriendRequest creates a pending request from senderID to receiverID, atomically for
	// the pair: when the receiver already requested the sender, the request is accepted
	// instead (FriendRequestAccepted); nothing is created when they are friends or the request
	// is already pending.
	SendFriendRequest(ctx context.Context, senderID, receiverID uint) (FriendRequestOutcome, error)
	// AcceptFriendRequest accepts the pending request of senderID to receiverID: they become
	// friends and the pending requests of the pair are deleted. accepted is false when no such
	// request is pending.
	AcceptFriendRequest(ctx context.Context, receiverID, senderID uint) (accepted bool, err error)
	// DeleteFriendRequest deletes the pending request of senderID to receiverID (decline or
	// cancel); deleted is false when there was none.
	DeleteFriendRequest(ctx context.Context, senderID, receiverID uint) (deleted bool, err error)
	// ListIncomingRequests lists the senders of the requests pending for userID, most recent
	// first (Since = request date); ListOutgoingRequests the receivers of userID's requests.
	ListIncomingRequests(ctx context.Context, userID uint, page, limit int) ([]SocialUser, int64, error)
	ListOutgoingRequests(ctx context.Context, userID uint, page, limit int) ([]SocialUser, int64, error)
	CountIncomingRequests(ctx context.Context, userID uint) (int64, error)
	CountOutgoingRequests(ctx context.Context, userID uint) (int64, error)

	// GetFriendRequestPolicy returns the friend request policy of a user (the default,
	// everyone, when not set).
	GetFriendRequestPolicy(ctx context.Context, userID uint) (string, error)
	SetFriendRequestPolicy(ctx context.Context, userID uint, policy string) error
}

type SocialRepositoryImpl struct {
	DB *gorm.DB
}

func NewSocialRepository(db *gorm.DB) *SocialRepositoryImpl {
	return &SocialRepositoryImpl{DB: db}
}

func socialOffset(page, limit int) int {
	if page < 1 {
		page = 1
	}
	return (page - 1) * limit
}

func uintsToInt64Array(ids []uint) pq.Int64Array {
	out := make(pq.Int64Array, 0, len(ids))
	seen := make(map[uint]bool, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, int64(id))
		}
	}
	return out
}

func orderedPair(a, b uint) (uint, uint) {
	if a < b {
		return a, b
	}
	return b, a
}

func (r *SocialRepositoryImpl) GetUser(ctx context.Context, userID uint) (*SocialUser, error) {
	var u SocialUser
	res := r.DB.WithContext(ctx).Raw(`SELECT `+socialUserColumns+`, u.created_at AS since FROM users u`+socialUserProfileJoin+` WHERE u.id = ?`, userID).Scan(&u)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &u, nil
}

func (r *SocialRepositoryImpl) GetUserByUsername(ctx context.Context, username string) (*SocialUser, error) {
	var u SocialUser
	res := r.DB.WithContext(ctx).Raw(`SELECT `+socialUserColumns+`, u.created_at AS since FROM users u`+socialUserProfileJoin+` WHERE u.username = ?`, username).Scan(&u)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &u, nil
}

func (r *SocialRepositoryImpl) SearchUsers(ctx context.Context, viewerID uint, query string, page, limit int) ([]SocialUser, int64, error) {
	query = strings.TrimSpace(query)
	where := "id <> ?"
	args := []any{viewerID}
	prefix := ""
	if query != "" {
		where += ` AND username ILIKE ? ESCAPE '\'`
		args = append(args, "%"+escapeLike(query)+"%")
		prefix = escapeLike(query) + "%"
	}
	var total int64
	if err := r.DB.WithContext(ctx).Raw("SELECT COUNT(*) FROM users WHERE "+where, args...).Scan(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count users: %w", err)
	}
	order := "lower(username), id"
	listArgs := append([]any{}, args...)
	if prefix != "" {
		order = `(username ILIKE ? ESCAPE '\') DESC, ` + order
		listArgs = append(listArgs, prefix)
	}
	listArgs = append(listArgs, limit, socialOffset(page, limit))
	var users []SocialUser
	if err := r.DB.WithContext(ctx).Raw("SELECT "+socialUserColumns+", u.created_at AS since FROM users u"+socialUserProfileJoin+" WHERE "+where+
		" ORDER BY "+order+" LIMIT ? OFFSET ?", listArgs...).Scan(&users).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to search users: %w", err)
	}
	return users, total, nil
}

func (r *SocialRepositoryImpl) Follow(ctx context.Context, followerID, followedID uint) (bool, error) {
	res := r.DB.WithContext(ctx).Exec(`INSERT INTO user_follows (follower_id, followed_id) VALUES (?, ?) ON CONFLICT DO NOTHING`, followerID, followedID)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

func (r *SocialRepositoryImpl) Unfollow(ctx context.Context, followerID, followedID uint) (bool, error) {
	res := r.DB.WithContext(ctx).Exec(`DELETE FROM user_follows WHERE follower_id = ? AND followed_id = ?`, followerID, followedID)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// listUsers runs a paginated list of users joined to a relation: from is the FROM/WHERE part
// giving the user id as rel.uid and the relation date as rel.since.
func (r *SocialRepositoryImpl) listUsers(ctx context.Context, from string, args []any, page, limit int) ([]SocialUser, int64, error) {
	var total int64
	if err := r.DB.WithContext(ctx).Raw("SELECT COUNT(*) FROM "+from, args...).Scan(&total).Error; err != nil {
		return nil, 0, err
	}
	listArgs := append(append([]any{}, args...), limit, socialOffset(page, limit))
	var users []SocialUser
	if err := r.DB.WithContext(ctx).Raw("SELECT "+socialUserColumns+", rel.since FROM "+from+
		" JOIN users u ON u.id = rel.uid"+socialUserProfileJoin+" ORDER BY rel.since DESC, u.id LIMIT ? OFFSET ?", listArgs...).Scan(&users).Error; err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

func (r *SocialRepositoryImpl) ListFollowers(ctx context.Context, userID uint, page, limit int) ([]SocialUser, int64, error) {
	return r.listUsers(ctx, `(SELECT follower_id AS uid, created_at AS since FROM user_follows WHERE followed_id = ?) rel`, []any{userID}, page, limit)
}

func (r *SocialRepositoryImpl) ListFollowing(ctx context.Context, userID uint, page, limit int) ([]SocialUser, int64, error) {
	return r.listUsers(ctx, `(SELECT followed_id AS uid, created_at AS since FROM user_follows WHERE follower_id = ?) rel`, []any{userID}, page, limit)
}

func (r *SocialRepositoryImpl) ListFriends(ctx context.Context, userID uint, page, limit int) ([]SocialUser, int64, error) {
	return r.listUsers(ctx, `(SELECT friend_id AS uid, created_at AS since FROM user_friends WHERE user_id = ?) rel`, []any{userID}, page, limit)
}

func (r *SocialRepositoryImpl) ListIncomingRequests(ctx context.Context, userID uint, page, limit int) ([]SocialUser, int64, error) {
	return r.listUsers(ctx, `(SELECT sender_id AS uid, created_at AS since FROM friend_requests WHERE receiver_id = ?) rel`, []any{userID}, page, limit)
}

func (r *SocialRepositoryImpl) ListOutgoingRequests(ctx context.Context, userID uint, page, limit int) ([]SocialUser, int64, error) {
	return r.listUsers(ctx, `(SELECT receiver_id AS uid, created_at AS since FROM friend_requests WHERE sender_id = ?) rel`, []any{userID}, page, limit)
}

func (r *SocialRepositoryImpl) RemoveFriend(ctx context.Context, a, b uint) (bool, error) {
	low, high := orderedPair(a, b)
	res := r.DB.WithContext(ctx).Exec(`DELETE FROM user_friendships WHERE user_low_id = ? AND user_high_id = ?`, low, high)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

func (r *SocialRepositoryImpl) ids(ctx context.Context, query string, args ...any) ([]uint, error) {
	var ids []uint
	if err := r.DB.WithContext(ctx).Raw(query, args...).Scan(&ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *SocialRepositoryImpl) FriendIDs(ctx context.Context, userID uint) ([]uint, error) {
	return r.ids(ctx, `SELECT friend_id FROM user_friends WHERE user_id = ? ORDER BY friend_id`, userID)
}

func (r *SocialRepositoryImpl) FollowerIDs(ctx context.Context, userID uint) ([]uint, error) {
	return r.ids(ctx, `SELECT follower_id FROM user_follows WHERE followed_id = ? ORDER BY follower_id`, userID)
}

func (r *SocialRepositoryImpl) MutualIDs(ctx context.Context, userID uint) ([]uint, error) {
	return r.ids(ctx, `SELECT f.follower_id FROM user_follows f
		JOIN user_follows b ON b.follower_id = f.followed_id AND b.followed_id = f.follower_id
		WHERE f.followed_id = ? ORDER BY f.follower_id`, userID)
}

func (r *SocialRepositoryImpl) Counts(ctx context.Context, userID uint) (SocialCounts, error) {
	var c SocialCounts
	err := r.DB.WithContext(ctx).Raw(`SELECT
		(SELECT COUNT(*) FROM user_friends WHERE user_id = ?) AS friends,
		(SELECT COUNT(*) FROM user_follows WHERE followed_id = ?) AS followers,
		(SELECT COUNT(*) FROM user_follows WHERE follower_id = ?) AS following`, userID, userID, userID).Scan(&c).Error
	return c, err
}

func (r *SocialRepositoryImpl) Relations(ctx context.Context, viewerID uint, userIDs []uint) ([]UserRelation, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	var rows []UserRelation
	v := viewerID
	err := r.DB.WithContext(ctx).Raw(`SELECT o.id AS user_id,
		EXISTS (SELECT 1 FROM user_follows f WHERE f.follower_id = ? AND f.followed_id = o.id) AS following,
		EXISTS (SELECT 1 FROM user_follows f WHERE f.follower_id = o.id AND f.followed_id = ?) AS followed_by,
		EXISTS (SELECT 1 FROM user_friendships fr WHERE fr.user_low_id = LEAST(?::bigint, o.id) AND fr.user_high_id = GREATEST(?::bigint, o.id)) AS friend,
		EXISTS (SELECT 1 FROM friend_requests rq WHERE rq.sender_id = ? AND rq.receiver_id = o.id) AS request_sent,
		EXISTS (SELECT 1 FROM friend_requests rq WHERE rq.sender_id = o.id AND rq.receiver_id = ?) AS request_received,
		COALESCE(ss.friend_request_policy, 'everyone') AS request_policy,
		COALESCE(ss.message_policy, 'friends') AS message_policy
		FROM unnest(?::bigint[]) AS o(id)
		LEFT JOIN user_social_settings ss ON ss.user_id = o.id`,
		v, v, v, v, v, v, uintsToInt64Array(userIDs)).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("failed to read relations: %w", err)
	}
	return rows, nil
}

// lockPair serializes the friend request operations of a pair of users inside a transaction
// (two concurrent crossed requests must give one friendship, not two pending requests).
func lockPair(tx *gorm.DB, a, b uint) error {
	low, high := orderedPair(a, b)
	// one key per pair (a collision only serializes two unrelated pairs)
	key := int64(low)<<32 ^ int64(high)
	return tx.Exec(`SELECT pg_advisory_xact_lock(?)`, key).Error
}

func (r *SocialRepositoryImpl) SendFriendRequest(ctx context.Context, senderID, receiverID uint) (FriendRequestOutcome, error) {
	var outcome FriendRequestOutcome
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockPair(tx, senderID, receiverID); err != nil {
			return err
		}
		low, high := orderedPair(senderID, receiverID)
		var friends int64
		if err := tx.Raw(`SELECT COUNT(*) FROM user_friendships WHERE user_low_id = ? AND user_high_id = ?`, low, high).Scan(&friends).Error; err != nil {
			return err
		}
		if friends > 0 {
			outcome = FriendRequestAlreadyFriends
			return nil
		}
		reverse := tx.Exec(`DELETE FROM friend_requests WHERE sender_id = ? AND receiver_id = ?`, receiverID, senderID)
		if reverse.Error != nil {
			return reverse.Error
		}
		if reverse.RowsAffected > 0 {
			if err := tx.Exec(`DELETE FROM friend_requests WHERE sender_id = ? AND receiver_id = ?`, senderID, receiverID).Error; err != nil {
				return err
			}
			if err := tx.Exec(`INSERT INTO user_friendships (user_low_id, user_high_id) VALUES (?, ?) ON CONFLICT DO NOTHING`, low, high).Error; err != nil {
				return err
			}
			outcome = FriendRequestAccepted
			return nil
		}
		res := tx.Exec(`INSERT INTO friend_requests (sender_id, receiver_id) VALUES (?, ?) ON CONFLICT DO NOTHING`, senderID, receiverID)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			outcome = FriendRequestAlreadySent
		} else {
			outcome = FriendRequestSent
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to send friend request: %w", err)
	}
	return outcome, nil
}

func (r *SocialRepositoryImpl) AcceptFriendRequest(ctx context.Context, receiverID, senderID uint) (bool, error) {
	accepted := false
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockPair(tx, senderID, receiverID); err != nil {
			return err
		}
		res := tx.Exec(`DELETE FROM friend_requests WHERE sender_id = ? AND receiver_id = ?`, senderID, receiverID)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return nil
		}
		if err := tx.Exec(`DELETE FROM friend_requests WHERE sender_id = ? AND receiver_id = ?`, receiverID, senderID).Error; err != nil {
			return err
		}
		low, high := orderedPair(senderID, receiverID)
		if err := tx.Exec(`INSERT INTO user_friendships (user_low_id, user_high_id) VALUES (?, ?) ON CONFLICT DO NOTHING`, low, high).Error; err != nil {
			return err
		}
		accepted = true
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("failed to accept friend request: %w", err)
	}
	return accepted, nil
}

func (r *SocialRepositoryImpl) DeleteFriendRequest(ctx context.Context, senderID, receiverID uint) (bool, error) {
	res := r.DB.WithContext(ctx).Exec(`DELETE FROM friend_requests WHERE sender_id = ? AND receiver_id = ?`, senderID, receiverID)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

func (r *SocialRepositoryImpl) CountIncomingRequests(ctx context.Context, userID uint) (int64, error) {
	var n int64
	err := r.DB.WithContext(ctx).Raw(`SELECT COUNT(*) FROM friend_requests WHERE receiver_id = ?`, userID).Scan(&n).Error
	return n, err
}

func (r *SocialRepositoryImpl) CountOutgoingRequests(ctx context.Context, userID uint) (int64, error) {
	var n int64
	err := r.DB.WithContext(ctx).Raw(`SELECT COUNT(*) FROM friend_requests WHERE sender_id = ?`, userID).Scan(&n).Error
	return n, err
}

func (r *SocialRepositoryImpl) GetFriendRequestPolicy(ctx context.Context, userID uint) (string, error) {
	var s UserSocialSettings
	err := r.DB.WithContext(ctx).Where("user_id = ?", userID).Take(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return FriendRequestPolicyEveryone, nil
	}
	if err != nil {
		return "", err
	}
	return s.FriendRequestPolicy, nil
}

func (r *SocialRepositoryImpl) SetFriendRequestPolicy(ctx context.Context, userID uint, policy string) error {
	return r.DB.WithContext(ctx).Exec(`INSERT INTO user_social_settings (user_id, friend_request_policy, updated_at) VALUES (?, ?, NOW())
		ON CONFLICT (user_id) DO UPDATE SET friend_request_policy = EXCLUDED.friend_request_policy, updated_at = NOW()`, userID, policy).Error
}
