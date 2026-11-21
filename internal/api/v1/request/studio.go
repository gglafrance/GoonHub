package request

type CreateStudioRequest struct {
	Name        string   `json:"name" binding:"required"`
	Aliases     []string `json:"aliases"`
	ShortName   string   `json:"short_name"`
	URL         string   `json:"url"`
	Description string   `json:"description"`
	Rating      *float64 `json:"rating"`
	Logo        string   `json:"logo"`
	Favicon     string   `json:"favicon"`
	Poster      string   `json:"poster"`
	PornDBID    string   `json:"porndb_id"`
	ParentID    *uint    `json:"parent_id"`
	NetworkID   *uint    `json:"network_id"`
}

type UpdateStudioRequest struct {
	Name        *string   `json:"name"`
	Aliases     *[]string `json:"aliases"`
	ShortName   *string   `json:"short_name"`
	URL         *string   `json:"url"`
	Description *string   `json:"description"`
	Rating      *float64  `json:"rating"`
	Logo        *string   `json:"logo"`
	Favicon     *string   `json:"favicon"`
	Poster      *string   `json:"poster"`
	PornDBID    *string   `json:"porndb_id"`
	ParentID    *uint     `json:"parent_id"`
	NetworkID   *uint     `json:"network_id"`
}

type SetSceneStudioRequest struct {
	StudioID *uint `json:"studio_id"`
}

// MergeStudioRequest names the studio merged into the studio of the URL (which is kept).
type MergeStudioRequest struct {
	SourceUUID string `json:"source_uuid" binding:"required"`
}
