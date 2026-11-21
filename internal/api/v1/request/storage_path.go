package request

type CreateStoragePathRequest struct {
	Name      string `json:"name" binding:"required,min=1,max=100"`
	Path      string `json:"path" binding:"required,min=1,max=500"`
	IsDefault bool   `json:"is_default"`
	ReadOnly  bool   `json:"read_only"`
}

type UpdateStoragePathRequest struct {
	Name      string `json:"name" binding:"required,min=1,max=100"`
	Path      string `json:"path" binding:"required,min=1,max=500"`
	IsDefault bool   `json:"is_default"`
	// ReadOnly is optional: omitted keeps the current value
	ReadOnly *bool `json:"read_only"`
}

type ValidatePathRequest struct {
	Path string `json:"path" binding:"required,min=1,max=500"`
}
