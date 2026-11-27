package data

import (
	"time"

	"github.com/google/uuid"
)

// Lobby visibilities (lobbies.visibility).
const (
	LobbyVisibilityPublic  = "public"
	LobbyVisibilityPrivate = "private"
)

// Auto-next modes (lobbies.auto_next): what happens when the item playing reaches its end.
const (
	AutoNextStop        = "stop"         // pause at the end of the item
	AutoNextNext        = "next"         // the next queue item starts (the item leaves the queue)
	AutoNextRepeatOne   = "repeat_one"   // the item (or its range) starts over
	AutoNextRepeatQueue = "repeat_queue" // the item goes to the end of the queue, the next one starts
)

// ValidAutoNext tells whether mode is an auto-next mode.
func ValidAutoNext(mode string) bool {
	switch mode {
	case AutoNextStop, AutoNextNext, AutoNextRepeatOne, AutoNextRepeatQueue:
		return true
	}
	return false
}

// Queue item statuses (lobby_queue_items.status).
const (
	QueueItemQueued    = "queued"
	QueueItemSuggested = "suggested"
)

// Lobby is a Goon2gether watch lobby (docs/GOON2GETHER.md). The live playback state is held
// in memory by the hub; CurrentItemID, PositionS and PlaybackRate are its last saved values.
type Lobby struct {
	ID            uint      `gorm:"primarykey"`
	UUID          uuid.UUID `gorm:"type:uuid;uniqueIndex"`
	InviteCode    string    `gorm:"size:16;not null"`
	Name          string    `gorm:"size:100;not null"`
	Visibility    string    `gorm:"size:10;not null;default:'private'"`
	HostUserID    *uint
	CreatedBy     *uint
	ControlOpen   bool
	QueueOpen     bool
	WaitForAll    bool
	MaxMembers    int
	CurrentItemID *uint
	PositionS     float64
	PlaybackRate  float64 `gorm:"not null;default:1"`
	AutoNext      string  `gorm:"size:16;not null;default:'next'"`
	AnonSeq       int     // last anonymous number given in the lobby
	// The play of the current item (history): when it first played (nil: not yet) and whether
	// it reached its end
	PlayStartedAt *time.Time
	PlayCompleted bool
	LastActiveAt  time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (Lobby) TableName() string { return "lobbies" }

// LobbyMember is a user who joined a lobby. A banned member keeps its row (BannedAt set). An
// anonymous member (public lobbies) is shown to the others as "Anonymous #AnonNumber".
type LobbyMember struct {
	LobbyID    uint `gorm:"primaryKey;autoIncrement:false"`
	UserID     uint `gorm:"primaryKey;autoIncrement:false"`
	CanControl bool
	Anonymous  bool
	AnonNumber *int
	BannedAt   *time.Time
	JoinedAt   time.Time
	Username   string `gorm:"->;-:migration"` // read only, filled by the member queries
}

func (LobbyMember) TableName() string { return "lobby_members" }

// LobbyQueueItem is a scene (or a range of it) in a lobby queue, or a suggestion.
type LobbyQueueItem struct {
	ID       uint `gorm:"primarykey"`
	LobbyID  uint `gorm:"not null"`
	SceneID  uint `gorm:"not null"`
	Status   string
	Position int
	StartS   *float64
	EndS     *float64
	Label    *string
	AddedBy  *uint
	// AddedByAnon is the anonymous number of the member who added it (nil: under their name)
	AddedByAnon *int
	// read only, filled by the queue queries
	SceneTitle    string `gorm:"->;-:migration"`
	SceneDuration int    `gorm:"->;-:migration"`
	AddedByName   string `gorm:"->;-:migration"`
	CreatedAt     time.Time
}

func (LobbyQueueItem) TableName() string { return "lobby_queue_items" }

// Outcomes of a lobby history entry (lobby_history.outcome).
const (
	HistoryCompleted = "completed" // the item reached its end at least once
	HistorySkipped   = "skipped"   // skipped, removed or replaced before its end
)

// LobbyHistoryEntry is an item a lobby played (one per play: a repeated item counts once until
// the item changes).
type LobbyHistoryEntry struct {
	ID      uint `gorm:"primarykey"`
	LobbyID uint `gorm:"not null"`
	SceneID uint `gorm:"not null"`
	StartS  *float64
	EndS    *float64
	Label   *string
	AddedBy *uint
	// AddedByAnon is the anonymous number of the member who added the item (nil: under their name)
	AddedByAnon *int
	StartedAt   time.Time
	EndedAt     time.Time
	Outcome     string
	// read only, filled by ListHistory
	SceneTitle     string `gorm:"->;-:migration"`
	SceneDuration  int    `gorm:"->;-:migration"`
	SceneAvailable bool   `gorm:"->;-:migration"` // the scene is not in the trash
	AddedByName    string `gorm:"->;-:migration"`
}

func (LobbyHistoryEntry) TableName() string { return "lobby_history" }

// LobbyPlayback is the saved playback of a lobby (SavePlayback).
type LobbyPlayback struct {
	ItemID        *uint
	PositionS     float64
	Rate          float64
	PlayStartedAt *time.Time
	PlayCompleted bool
}

// LobbyListEntry is a lobby with what the lobby browser shows.
type LobbyListEntry struct {
	Lobby
	HostUsername    string
	QueueLength     int
	NowSceneID      *uint
	NowSceneTitle   string
	NowLabel        *string
	IsMember        bool
	MemberAnonymous bool
	MemberBannedAt  *time.Time
}

// Emote is a server custom emote (Goon2gether chat), its image in goon2gether.emote_dir.
type Emote struct {
	ID          uint   `gorm:"primarykey"`
	Name        string `gorm:"size:32;not null"`
	Filename    string `gorm:"size:64;not null"`
	ContentType string `gorm:"size:32;not null"`
	Width       int
	Height      int
	SizeBytes   int
	Animated    bool
	CreatedBy   *uint
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (Emote) TableName() string { return "emotes" }
