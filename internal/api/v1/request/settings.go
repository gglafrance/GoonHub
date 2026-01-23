package request

type UpdatePlayerSettingsRequest struct {
	Autoplay      bool `json:"autoplay"`
	DefaultVolume int  `json:"default_volume" binding:"min=0,max=100"`
	Loop          bool `json:"loop"`
}

type UpdateAppSettingsRequest struct {
	VideosPerPage    int    `json:"videos_per_page" binding:"required,min=1,max=100"`
	DefaultSortOrder string `json:"default_sort_order" binding:"required"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=6"`
}

type ChangeUsernameRequest struct {
	Username string `json:"username" binding:"required,min=3"`
}
