package core

import (
	"fmt"

	"goonhub/internal/data"
)

// folder_latest: latest added scenes of a storage path and/or folder.
// Config: storage_path_id, folder (absolute path prefix, or a folder name matched anywhere in
// the path). At least one of them is required.
// Widget payload: {"storage_path": {"id", "name"} | null, "folder": "..."}; without
// files:view_paths the widget gives nothing ({"forbidden": true}): it filters by path.

type folderLatestStoragePath struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type folderLatestPayload struct {
	StoragePath *folderLatestStoragePath `json:"storage_path"`
	Folder      string                   `json:"folder"`
	Forbidden   bool                     `json:"forbidden,omitempty"`
}

func init() {
	registerHomepageWidget("folder_latest", HomepageWidgetSpec{
		Fetch: fetchFolderLatestWidget,
		Validate: func(cfg map[string]interface{}) error {
			if err := validateSceneWidgetFilter(cfg); err != nil {
				return err
			}
			if widgetConfigInt(cfg, "storage_path_id", 0) <= 0 && widgetConfigString(cfg, "folder", "") == "" {
				return fmt.Errorf("choose a storage path or a folder")
			}
			return nil
		},
	})
}

func fetchFolderLatestWidget(s *HomepageService, userID uint, section data.HomepageSection) (*HomepageSectionData, error) {
	if !s.userHas(userID, PermFilesViewPaths) {
		return &HomepageSectionData{Section: section, Scenes: []data.Scene{}, Widget: folderLatestPayload{Forbidden: true}}, nil
	}
	queries, err := sceneWidgetQueries(s)
	if err != nil {
		return nil, err
	}
	filter := sceneWidgetFilter(s, userID, section.Config)
	payload := folderLatestPayload{Folder: filter.Folder}
	if filter.StoragePathID > 0 && s.WidgetDeps().StoragePaths != nil {
		if sp, err := s.WidgetDeps().StoragePaths.GetByID(filter.StoragePathID); err == nil && sp != nil {
			payload.StoragePath = &folderLatestStoragePath{ID: sp.ID, Name: sp.Name}
		}
	}
	if filter.StoragePathID == 0 && filter.Folder == "" {
		return &HomepageSectionData{Section: section, Scenes: []data.Scene{}, Widget: payload}, nil
	}
	ids, total, err := queries.SceneIDs(userID, filter, false, data.HomepageSceneOrderNewest, sceneWidgetLimit(section, 12))
	if err != nil {
		return nil, fmt.Errorf("failed to list folder scenes: %w", err)
	}
	scenes, err := loadWidgetScenes(s, ids)
	if err != nil {
		return nil, err
	}
	return &HomepageSectionData{Section: section, Scenes: scenes, Total: total, Widget: payload}, nil
}
