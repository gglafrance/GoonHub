package request

type CreateStoragePathRequest struct {
	Name      string `json:"name" binding:"required,min=1,max=100"`
	Path      string `json:"path" binding:"required,min=1,max=500"`
	IsDefault bool   `json:"is_default"`
}

type UpdateStoragePathRequest struct {
	Name      string `json:"name" binding:"required,min=1,max=100"`
	Path      string `json:"path" binding:"required,min=1,max=500"`
	IsDefault bool   `json:"is_default"`
}

type ValidatePathRequest struct {
	Path string `json:"path" binding:"required,min=1,max=500"`
}
