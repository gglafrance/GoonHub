package mocks

//go:generate go run go.uber.org/mock/mockgen -destination=mock_user_repository.go -package=mocks goonhub/internal/data UserRepository
//go:generate go run go.uber.org/mock/mockgen -destination=mock_video_repository.go -package=mocks goonhub/internal/data VideoRepository
//go:generate go run go.uber.org/mock/mockgen -destination=mock_revoked_token_repository.go -package=mocks goonhub/internal/data RevokedTokenRepository
//go:generate go run go.uber.org/mock/mockgen -destination=mock_user_settings_repository.go -package=mocks goonhub/internal/data UserSettingsRepository
//go:generate go run go.uber.org/mock/mockgen -destination=mock_role_repository.go -package=mocks goonhub/internal/data RoleRepository
//go:generate go run go.uber.org/mock/mockgen -destination=mock_permission_repository.go -package=mocks goonhub/internal/data PermissionRepository
