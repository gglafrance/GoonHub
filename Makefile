.PHONY: mocks test test-race test-cover setup-test

setup-test:
	mkdir -p web/dist && touch web/dist/main.html

mocks: setup-test
	go run go.uber.org/mock/mockgen -destination=internal/mocks/mock_user_repository.go -package=mocks goonhub/internal/data UserRepository
	go run go.uber.org/mock/mockgen -destination=internal/mocks/mock_video_repository.go -package=mocks goonhub/internal/data VideoRepository
	go run go.uber.org/mock/mockgen -destination=internal/mocks/mock_revoked_token_repository.go -package=mocks goonhub/internal/data RevokedTokenRepository
	go run go.uber.org/mock/mockgen -destination=internal/mocks/mock_user_settings_repository.go -package=mocks goonhub/internal/data UserSettingsRepository
	go run go.uber.org/mock/mockgen -destination=internal/mocks/mock_role_repository.go -package=mocks goonhub/internal/data RoleRepository
	go run go.uber.org/mock/mockgen -destination=internal/mocks/mock_permission_repository.go -package=mocks goonhub/internal/data PermissionRepository
	go run go.uber.org/mock/mockgen -destination=internal/mocks/mock_job_history_repository.go -package=mocks goonhub/internal/data JobHistoryRepository
	go run go.uber.org/mock/mockgen -destination=internal/mocks/mock_pool_config_repository.go -package=mocks goonhub/internal/data PoolConfigRepository
	go run go.uber.org/mock/mockgen -destination=internal/mocks/mock_processing_config_repository.go -package=mocks goonhub/internal/data ProcessingConfigRepository
	go run go.uber.org/mock/mockgen -destination=internal/mocks/mock_trigger_config_repository.go -package=mocks goonhub/internal/data TriggerConfigRepository
	go run go.uber.org/mock/mockgen -destination=internal/mocks/mock_tag_repository.go -package=mocks goonhub/internal/data TagRepository
	go run go.uber.org/mock/mockgen -destination=internal/mocks/mock_interaction_repository.go -package=mocks goonhub/internal/data InteractionRepository
	go run go.uber.org/mock/mockgen -destination=internal/mocks/mock_watch_history_repository.go -package=mocks goonhub/internal/data WatchHistoryRepository
	go run go.uber.org/mock/mockgen -destination=internal/mocks/mock_dlq_repository.go -package=mocks goonhub/internal/data DLQRepository
	go run go.uber.org/mock/mockgen -destination=internal/mocks/mock_retry_config_repository.go -package=mocks goonhub/internal/data RetryConfigRepository

test: mocks
	go test ./...

test-race: mocks
	go test -race ./...

test-cover: mocks
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
