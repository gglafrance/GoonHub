.PHONY: mocks test test-race test-cover

mocks:
	go run go.uber.org/mock/mockgen -destination=internal/mocks/mock_user_repository.go -package=mocks goonhub/internal/data UserRepository
	go run go.uber.org/mock/mockgen -destination=internal/mocks/mock_video_repository.go -package=mocks goonhub/internal/data VideoRepository
	go run go.uber.org/mock/mockgen -destination=internal/mocks/mock_revoked_token_repository.go -package=mocks goonhub/internal/data RevokedTokenRepository
	go run go.uber.org/mock/mockgen -destination=internal/mocks/mock_user_settings_repository.go -package=mocks goonhub/internal/data UserSettingsRepository
	go run go.uber.org/mock/mockgen -destination=internal/mocks/mock_role_repository.go -package=mocks goonhub/internal/data RoleRepository
	go run go.uber.org/mock/mockgen -destination=internal/mocks/mock_permission_repository.go -package=mocks goonhub/internal/data PermissionRepository
	go run go.uber.org/mock/mockgen -destination=internal/mocks/mock_job_history_repository.go -package=mocks goonhub/internal/data JobHistoryRepository

test: mocks
	go test ./...

test-race: mocks
	go test -race ./...

test-cover: mocks
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
