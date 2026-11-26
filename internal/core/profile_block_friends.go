package core

import (
	"context"
	"fmt"
	"time"
)

// friends: a preview of the owner's friends (avatars and names), behind friends_list like the
// friends list itself.

// ProfileFriendItem is a friend of the friends block.
type ProfileFriendItem struct {
	ID            uint      `json:"id"`
	Username      string    `json:"username"`
	DisplayName   string    `json:"display_name"`
	AvatarVersion int64     `json:"avatar_version"`
	Since         time.Time `json:"since"`
}

// ProfileFriendsPayload is the payload of the friends block.
type ProfileFriendsPayload struct {
	Items []ProfileFriendItem `json:"items"`
}

func init() {
	registerProfileBlock(ProfileBlockSpec{
		Type:           "friends",
		Label:          "Friends",
		Description:    "A preview of your friends",
		PrivacyItem:    PrivacyFriendsList,
		Options:        []ProfileBlockOption{profileCountOption(8)},
		DefaultVisible: true,
		Order:          100,
		Fetch:          fetchFriendsBlock,
	})
}

func fetchFriendsBlock(ctx context.Context, deps *ProfileBlockDeps, req ProfileBlockRequest) (*ProfileBlockData, error) {
	users, total, err := deps.Social.ListFriends(ctx, req.OwnerID, 1, req.Config.Int("count"))
	if err != nil {
		return nil, fmt.Errorf("failed to read the friends: %w", err)
	}
	payload := ProfileFriendsPayload{Items: make([]ProfileFriendItem, len(users))}
	for i, u := range users {
		payload.Items[i] = ProfileFriendItem{ID: u.ID, Username: u.Username, DisplayName: u.DisplayName, AvatarVersion: u.AvatarVersion, Since: u.Since}
	}
	return &ProfileBlockData{Widget: payload, Total: &total, Empty: len(users) == 0}, nil
}
