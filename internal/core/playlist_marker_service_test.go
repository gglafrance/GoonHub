package core

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"goonhub/internal/apperrors"
	"goonhub/internal/data"
)

// The test service has a limit of 3 markers per playlist and a default marker duration of 30 s
// (newTestPlaylistService).

func markerPlaylist(id, ownerID uint, visibility string) *data.Playlist {
	return &data.Playlist{
		ID:         id,
		UUID:       uuid.New(),
		UserID:     ownerID,
		Name:       "Moments",
		Visibility: visibility,
		Kind:       data.PlaylistKindMarkers,
		User:       data.User{ID: ownerID, Username: "owner"},
	}
}

func TestCreateMarkerPlaylist_WithVisibleMarkers(t *testing.T) {
	svc, repo, _, _ := newTestPlaylistService(t)

	repo.EXPECT().FilterVisibleMarkerIDs(uint(1), []uint{7, 8}).Return([]uint{7, 8}, nil)
	repo.EXPECT().Create(gomock.Any()).DoAndReturn(func(p *data.Playlist) error {
		if p.Kind != data.PlaylistKindMarkers {
			t.Fatalf("kind = %q, want markers", p.Kind)
		}
		p.ID = 4
		return nil
	})
	repo.EXPECT().AddMarkers(uint(4), []uint{7, 8}, 3).Return(2, nil)
	repo.EXPECT().GetByID(uint(4)).Return(markerPlaylist(4, 1, "private"), nil)

	got, err := svc.Create(1, CreatePlaylistInput{Name: "Moments", Kind: "markers", MarkerIDs: []uint{7, 8, 7}})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.Kind != data.PlaylistKindMarkers {
		t.Fatalf("kind = %q", got.Kind)
	}
}

func TestCreatePlaylist_KindValidation(t *testing.T) {
	tests := []struct {
		name  string
		input CreatePlaylistInput
		want  error
	}{
		{"unknown kind", CreatePlaylistInput{Name: "x", Kind: "actors"}, apperrors.ErrPlaylistInvalidKind},
		{"scenes in a marker playlist", CreatePlaylistInput{Name: "x", Kind: "markers", SceneIDs: []uint{1}}, apperrors.ErrPlaylistKindMismatch},
		{"markers in a scene playlist", CreatePlaylistInput{Name: "x", MarkerIDs: []uint{1}}, apperrors.ErrPlaylistKindMismatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _, _, _ := newTestPlaylistService(t)
			if _, err := svc.Create(1, tt.input); !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestCreateMarkerPlaylist_HiddenMarkerRefused(t *testing.T) {
	svc, repo, _, _ := newTestPlaylistService(t)
	// Marker 9 belongs to a user who does not share markers: not visible
	repo.EXPECT().FilterVisibleMarkerIDs(uint(1), []uint{7, 9}).Return([]uint{7}, nil)

	_, err := svc.Create(1, CreatePlaylistInput{Name: "x", Kind: "markers", MarkerIDs: []uint{7, 9}})
	if !errors.Is(err, apperrors.ErrPlaylistMarkerNotFound) || !apperrors.IsNotFound(err) {
		t.Fatalf("err = %v, want marker not found", err)
	}
}

func TestCreateMarkerPlaylist_OverLimit(t *testing.T) {
	svc, repo, _, _ := newTestPlaylistService(t)
	repo.EXPECT().FilterVisibleMarkerIDs(uint(1), []uint{1, 2, 3, 4}).Return([]uint{1, 2, 3, 4}, nil)

	_, err := svc.Create(1, CreatePlaylistInput{Name: "x", Kind: "markers", MarkerIDs: []uint{1, 2, 3, 4}})
	if !errors.Is(err, apperrors.ErrPlaylistFull) {
		t.Fatalf("err = %v, want playlist full", err)
	}
}

func TestAddMarkers(t *testing.T) {
	t.Run("dedupes and counts skipped", func(t *testing.T) {
		svc, repo, _, _ := newTestPlaylistService(t)
		p := markerPlaylist(2, 1, "private")
		repo.EXPECT().GetByUUID(p.UUID.String()).Return(p, nil)
		repo.EXPECT().FilterVisibleMarkerIDs(uint(1), []uint{5, 6}).Return([]uint{5, 6}, nil)
		// 5 was already in the playlist
		repo.EXPECT().AddMarkers(uint(2), []uint{5, 6}, 3).Return(1, nil)

		res, err := svc.AddMarkers(1, p.UUID.String(), []uint{5, 5, 6})
		if err != nil {
			t.Fatalf("AddMarkers: %v", err)
		}
		if res.Added != 1 || res.Skipped != 1 {
			t.Fatalf("result = %+v, want 1 added 1 skipped", res)
		}
	})

	t.Run("full playlist", func(t *testing.T) {
		svc, repo, _, _ := newTestPlaylistService(t)
		p := markerPlaylist(2, 1, "private")
		repo.EXPECT().GetByUUID(p.UUID.String()).Return(p, nil)
		repo.EXPECT().FilterVisibleMarkerIDs(uint(1), []uint{5}).Return([]uint{5}, nil)
		repo.EXPECT().AddMarkers(uint(2), []uint{5}, 3).Return(0, data.ErrPlaylistFullSentinel())

		if _, err := svc.AddMarkers(1, p.UUID.String(), []uint{5}); !errors.Is(err, apperrors.ErrPlaylistFull) {
			t.Fatalf("err = %v, want playlist full", err)
		}
	})

	t.Run("hidden marker", func(t *testing.T) {
		svc, repo, _, _ := newTestPlaylistService(t)
		p := markerPlaylist(2, 1, "private")
		repo.EXPECT().GetByUUID(p.UUID.String()).Return(p, nil)
		repo.EXPECT().FilterVisibleMarkerIDs(uint(1), []uint{5}).Return([]uint{}, nil)

		if _, err := svc.AddMarkers(1, p.UUID.String(), []uint{5}); !errors.Is(err, apperrors.ErrPlaylistMarkerNotFound) {
			t.Fatalf("err = %v, want marker not found", err)
		}
	})

	t.Run("empty list", func(t *testing.T) {
		svc, repo, _, _ := newTestPlaylistService(t)
		p := markerPlaylist(2, 1, "private")
		repo.EXPECT().GetByUUID(p.UUID.String()).Return(p, nil)

		if _, err := svc.AddMarkers(1, p.UUID.String(), []uint{}); !errors.Is(err, apperrors.ErrPlaylistMarkersRequired) {
			t.Fatalf("err = %v, want markers required", err)
		}
	})

	t.Run("not the owner", func(t *testing.T) {
		svc, repo, _, _ := newTestPlaylistService(t)
		p := markerPlaylist(2, 1, "public")
		repo.EXPECT().GetByUUID(p.UUID.String()).Return(p, nil)

		if _, err := svc.AddMarkers(2, p.UUID.String(), []uint{5}); !errors.Is(err, apperrors.ErrPlaylistForbidden) {
			t.Fatalf("err = %v, want forbidden", err)
		}
	})

	t.Run("scene playlist", func(t *testing.T) {
		svc, repo, _, _ := newTestPlaylistService(t)
		p := markerPlaylist(2, 1, "private")
		p.Kind = data.PlaylistKindScenes
		repo.EXPECT().GetByUUID(p.UUID.String()).Return(p, nil)

		if _, err := svc.AddMarkers(1, p.UUID.String(), []uint{5}); !errors.Is(err, apperrors.ErrPlaylistKindMismatch) {
			t.Fatalf("err = %v, want kind mismatch", err)
		}
	})
}

func TestAddScenes_MarkerPlaylistRefused(t *testing.T) {
	svc, repo, _, _ := newTestPlaylistService(t)
	p := markerPlaylist(2, 1, "private")
	repo.EXPECT().GetByUUID(p.UUID.String()).Return(p, nil)

	if err := svc.AddScenes(1, p.UUID.String(), []uint{1}); !errors.Is(err, apperrors.ErrPlaylistKindMismatch) {
		t.Fatalf("err = %v, want kind mismatch", err)
	}
}

func TestAddMarkersFromFilter(t *testing.T) {
	filter := PlaylistMarkerFilter{Label: "Kiss", Options: data.MarkerListOptions{RangesOnly: true}}

	t.Run("adds the listing", func(t *testing.T) {
		svc, repo, _, _ := newTestPlaylistService(t)
		p := markerPlaylist(2, 1, "private")
		repo.EXPECT().GetByUUID(p.UUID.String()).Return(p, nil)
		// limit + 1 detects a listing that cannot fit
		repo.EXPECT().ResolveVisibleMarkerIDs(uint(1), "Kiss", filter.Options, 4).Return([]uint{3, 1}, nil)
		repo.EXPECT().AddMarkers(uint(2), []uint{3, 1}, 3).Return(2, nil)

		res, err := svc.AddMarkersFromFilter(1, p.UUID.String(), filter)
		if err != nil {
			t.Fatalf("AddMarkersFromFilter: %v", err)
		}
		if res.Added != 2 || res.Skipped != 0 {
			t.Fatalf("result = %+v", res)
		}
	})

	t.Run("listing larger than the limit", func(t *testing.T) {
		svc, repo, _, _ := newTestPlaylistService(t)
		p := markerPlaylist(2, 1, "private")
		repo.EXPECT().GetByUUID(p.UUID.String()).Return(p, nil)
		repo.EXPECT().ResolveVisibleMarkerIDs(uint(1), "Kiss", filter.Options, 4).Return([]uint{1, 2, 3, 4}, nil)

		if _, err := svc.AddMarkersFromFilter(1, p.UUID.String(), filter); !errors.Is(err, apperrors.ErrPlaylistFull) {
			t.Fatalf("err = %v, want playlist full", err)
		}
	})

	t.Run("empty listing", func(t *testing.T) {
		svc, repo, _, _ := newTestPlaylistService(t)
		p := markerPlaylist(2, 1, "private")
		repo.EXPECT().GetByUUID(p.UUID.String()).Return(p, nil)
		repo.EXPECT().ResolveVisibleMarkerIDs(uint(1), "Kiss", filter.Options, 4).Return(nil, nil)

		res, err := svc.AddMarkersFromFilter(1, p.UUID.String(), filter)
		if err != nil || res.Added != 0 {
			t.Fatalf("result = %+v, err = %v", res, err)
		}
	})
}

func TestRemoveAndReorderMarkers(t *testing.T) {
	t.Run("remove", func(t *testing.T) {
		svc, repo, _, _ := newTestPlaylistService(t)
		p := markerPlaylist(2, 1, "private")
		repo.EXPECT().GetByUUID(p.UUID.String()).Return(p, nil)
		repo.EXPECT().RemoveMarkers(uint(2), []uint{5}).Return(int64(1), nil)
		if err := svc.RemoveMarker(1, p.UUID.String(), 5); err != nil {
			t.Fatalf("RemoveMarker: %v", err)
		}
	})

	t.Run("remove a marker not in the playlist", func(t *testing.T) {
		svc, repo, _, _ := newTestPlaylistService(t)
		p := markerPlaylist(2, 1, "private")
		repo.EXPECT().GetByUUID(p.UUID.String()).Return(p, nil)
		repo.EXPECT().RemoveMarkers(uint(2), []uint{5, 6}).Return(int64(0), nil)
		if err := svc.RemoveMarkers(1, p.UUID.String(), []uint{5, 6}); !errors.Is(err, apperrors.ErrPlaylistMarkerNotInPlaylist) {
			t.Fatalf("err = %v, want not in playlist", err)
		}
	})

	t.Run("reorder", func(t *testing.T) {
		svc, repo, _, _ := newTestPlaylistService(t)
		p := markerPlaylist(2, 1, "private")
		repo.EXPECT().GetByUUID(p.UUID.String()).Return(p, nil)
		repo.EXPECT().ReorderMarkers(uint(2), []uint{6, 5}).Return(nil)
		if err := svc.ReorderMarkers(1, p.UUID.String(), []uint{6, 5, 6}); err != nil {
			t.Fatalf("ReorderMarkers: %v", err)
		}
	})
}

func TestGetMarkerPlaylist_EntriesAndRanges(t *testing.T) {
	svc, repo, _, _ := newTestPlaylistService(t)
	p := markerPlaylist(2, 1, "public")
	updated := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	end := 70

	repo.EXPECT().GetByUUID(p.UUID.String()).Return(p, nil)
	repo.EXPECT().GetMarkerStats(uint(2), 30).Return(data.PlaylistMarkerStats{MarkerCount: 3, SceneCount: 2, TotalDuration: 60}, nil)
	repo.EXPECT().GetThumbnailMarkers(uint(2), 4).Return([]data.MarkerThumbnailRef{{ID: 10, UpdatedAt: updated}}, nil)
	repo.EXPECT().GetPlaylistTags(uint(2)).Return(nil, nil)
	repo.EXPECT().GetLikeStatus(uint(9), uint(2)).Return(false, nil)
	repo.EXPECT().GetLikeCount(uint(2)).Return(int64(0), nil)
	repo.EXPECT().GetPlaylistMarkers(uint(2)).Return([]data.PlaylistMarkerRow{
		// range marker: plays its range
		{Position: 0, MarkerID: 10, MarkerUserID: 3, SceneID: 100, Timestamp: 40, EndTimestamp: &end, ThumbnailPath: "x.webp", UpdatedAt: updated, SceneTitle: "A", SceneDuration: 600},
		// point marker: default duration
		{Position: 1, MarkerID: 11, MarkerUserID: 3, SceneID: 100, Timestamp: 100, SceneDuration: 600},
		// point marker near the end: cut at the end of the scene
		{Position: 2, MarkerID: 12, MarkerUserID: 1, SceneID: 101, Timestamp: 590, SceneDuration: 600},
	}, nil)
	repo.EXPECT().GetProgress(uint(9), uint(2)).Return(nil, nil)

	// A viewer of the public playlist (user 9) sees its markers, including those of user 3
	detail, err := svc.GetByUUID(9, p.UUID.String())
	if err != nil {
		t.Fatalf("GetByUUID: %v", err)
	}
	if detail.Kind != data.PlaylistKindMarkers || detail.MarkerCount != 3 || detail.SceneCount != 2 {
		t.Fatalf("item = %+v", detail.PlaylistListItem)
	}
	if len(detail.ThumbnailMarkers) != 1 || detail.ThumbnailMarkers[0].Version != updated.UnixMilli() {
		t.Fatalf("thumbnail markers = %+v", detail.ThumbnailMarkers)
	}
	if detail.MarkerDefaultDuration != 30 || len(detail.Scenes) != 0 {
		t.Fatalf("default duration %d, scenes %d", detail.MarkerDefaultDuration, len(detail.Scenes))
	}
	want := [][2]int{{40, 70}, {100, 130}, {590, 600}}
	if len(detail.Markers) != len(want) {
		t.Fatalf("got %d markers", len(detail.Markers))
	}
	for i, w := range want {
		m := detail.Markers[i]
		if m.Start != w[0] || m.End != w[1] {
			t.Fatalf("marker %d range = %d-%d, want %d-%d", i, m.Start, m.End, w[0], w[1])
		}
	}
	if !detail.Markers[0].Marker.HasThumbnail || detail.Markers[1].Marker.HasThumbnail {
		t.Fatal("has_thumbnail follows the thumbnail path")
	}
}

func TestGetMarkerPlaylist_PrivateDenied(t *testing.T) {
	svc, repo, _, _ := newTestPlaylistService(t)
	p := markerPlaylist(2, 1, "private")
	repo.EXPECT().GetByUUID(p.UUID.String()).Return(p, nil)

	if _, err := svc.GetByUUID(9, p.UUID.String()); !errors.Is(err, apperrors.ErrPlaylistForbidden) {
		t.Fatalf("err = %v, want forbidden", err)
	}
}

func TestUpdateProgress_MarkerPlaylist(t *testing.T) {
	markerID := uint(11)

	t.Run("stores the marker and its scene", func(t *testing.T) {
		svc, repo, _, _ := newTestPlaylistService(t)
		p := markerPlaylist(2, 1, "public")
		repo.EXPECT().GetByUUID(p.UUID.String()).Return(p, nil)
		repo.EXPECT().HasMarker(uint(2), markerID).Return(true, uint(100), nil)
		repo.EXPECT().UpsertProgress(uint(9), uint(2), uint(100), &markerID, 12.5).Return(nil)

		// the scene sent by the client is ignored: the marker's scene is stored
		if err := svc.UpdateProgress(9, p.UUID.String(), 55, &markerID, 12.5); err != nil {
			t.Fatalf("UpdateProgress: %v", err)
		}
	})

	t.Run("marker required", func(t *testing.T) {
		svc, repo, _, _ := newTestPlaylistService(t)
		p := markerPlaylist(2, 1, "public")
		repo.EXPECT().GetByUUID(p.UUID.String()).Return(p, nil)
		if err := svc.UpdateProgress(9, p.UUID.String(), 100, nil, 1); !apperrors.IsValidation(err) {
			t.Fatalf("err = %v, want validation", err)
		}
	})

	t.Run("marker not in the playlist", func(t *testing.T) {
		svc, repo, _, _ := newTestPlaylistService(t)
		p := markerPlaylist(2, 1, "public")
		repo.EXPECT().GetByUUID(p.UUID.String()).Return(p, nil)
		repo.EXPECT().HasMarker(uint(2), markerID).Return(false, uint(0), nil)
		if err := svc.UpdateProgress(9, p.UUID.String(), 0, &markerID, 1); !errors.Is(err, apperrors.ErrPlaylistMarkerNotInPlaylist) {
			t.Fatalf("err = %v, want not in playlist", err)
		}
	})

	t.Run("private playlist of another user", func(t *testing.T) {
		svc, repo, _, _ := newTestPlaylistService(t)
		p := markerPlaylist(2, 1, "private")
		repo.EXPECT().GetByUUID(p.UUID.String()).Return(p, nil)
		if err := svc.UpdateProgress(9, p.UUID.String(), 0, &markerID, 1); !errors.Is(err, apperrors.ErrPlaylistForbidden) {
			t.Fatalf("err = %v, want forbidden", err)
		}
	})
}

func TestListPlaylists_KindFilter(t *testing.T) {
	svc, repo, _, _ := newTestPlaylistService(t)
	if _, _, err := svc.List(1, data.PlaylistListParams{Kind: "bogus"}); !errors.Is(err, apperrors.ErrPlaylistInvalidKind) {
		t.Fatalf("err = %v, want invalid kind", err)
	}

	repo.EXPECT().List(gomock.Any()).DoAndReturn(func(params data.PlaylistListParams) ([]data.Playlist, int64, error) {
		if params.Kind != data.PlaylistKindMarkers || params.UserID != 1 {
			t.Fatalf("params = %+v", params)
		}
		return nil, 0, nil
	})
	if _, _, err := svc.List(1, data.PlaylistListParams{Kind: "markers"}); err != nil {
		t.Fatalf("List: %v", err)
	}
}
