package core

import (
	"fmt"
	"goonhub/internal/data"
	"os"

	"go.uber.org/zap"
)

type StoragePathService struct {
	repo   data.StoragePathRepository
	logger *zap.Logger
}

func NewStoragePathService(repo data.StoragePathRepository, logger *zap.Logger) *StoragePathService {
	return &StoragePathService{
		repo:   repo,
		logger: logger,
	}
}

// ValidatePath checks if a path exists, is a directory, and is readable
func (s *StoragePathService) ValidatePath(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("path does not exist: %s", path)
		}
		return fmt.Errorf("failed to access path: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", path)
	}

	// Check if readable by trying to open
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("path is not readable: %w", err)
	}
	f.Close()

	return nil
}

func (s *StoragePathService) List() ([]data.StoragePath, error) {
	return s.repo.List()
}

func (s *StoragePathService) GetByID(id uint) (*data.StoragePath, error) {
	return s.repo.GetByID(id)
}

func (s *StoragePathService) GetDefault() (*data.StoragePath, error) {
	return s.repo.GetDefault()
}

func (s *StoragePathService) Create(name, path string, isDefault bool) (*data.StoragePath, error) {
	// Validate path exists and is accessible
	if err := s.ValidatePath(path); err != nil {
		return nil, err
	}

	// Check if path already exists
	existing, err := s.repo.GetByPath(path)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing path: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("storage path already exists: %s", path)
	}

	// If setting as default, clear existing default
	if isDefault {
		if err := s.repo.ClearDefault(); err != nil {
			return nil, fmt.Errorf("failed to clear default: %w", err)
		}
	}

	storagePath := &data.StoragePath{
		Name:      name,
		Path:      path,
		IsDefault: isDefault,
	}

	if err := s.repo.Create(storagePath); err != nil {
		return nil, fmt.Errorf("failed to create storage path: %w", err)
	}

	s.logger.Info("Created storage path",
		zap.Uint("id", storagePath.ID),
		zap.String("name", name),
		zap.String("path", path),
		zap.Bool("is_default", isDefault),
	)

	return storagePath, nil
}

func (s *StoragePathService) Update(id uint, name, path string, isDefault bool) (*data.StoragePath, error) {
	existing, err := s.repo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get storage path: %w", err)
	}
	if existing == nil {
		return nil, fmt.Errorf("storage path not found")
	}

	// Validate path if it changed
	if path != existing.Path {
		if err := s.ValidatePath(path); err != nil {
			return nil, err
		}

		// Check if new path already exists
		existingPath, err := s.repo.GetByPath(path)
		if err != nil {
			return nil, fmt.Errorf("failed to check existing path: %w", err)
		}
		if existingPath != nil && existingPath.ID != id {
			return nil, fmt.Errorf("storage path already exists: %s", path)
		}
	}

	// If setting as default, clear existing default
	if isDefault && !existing.IsDefault {
		if err := s.repo.ClearDefault(); err != nil {
			return nil, fmt.Errorf("failed to clear default: %w", err)
		}
	}

	existing.Name = name
	existing.Path = path
	existing.IsDefault = isDefault

	if err := s.repo.Update(existing); err != nil {
		return nil, fmt.Errorf("failed to update storage path: %w", err)
	}

	s.logger.Info("Updated storage path",
		zap.Uint("id", id),
		zap.String("name", name),
		zap.String("path", path),
		zap.Bool("is_default", isDefault),
	)

	return existing, nil
}

func (s *StoragePathService) Delete(id uint) error {
	// Check if this is the only storage path
	count, err := s.repo.Count()
	if err != nil {
		return fmt.Errorf("failed to count storage paths: %w", err)
	}
	if count <= 1 {
		return fmt.Errorf("cannot delete the only storage path")
	}

	existing, err := s.repo.GetByID(id)
	if err != nil {
		return fmt.Errorf("failed to get storage path: %w", err)
	}
	if existing == nil {
		return fmt.Errorf("storage path not found")
	}

	// If deleting default, set another path as default
	if existing.IsDefault {
		paths, err := s.repo.List()
		if err != nil {
			return fmt.Errorf("failed to list storage paths: %w", err)
		}
		for _, p := range paths {
			if p.ID != id {
				p.IsDefault = true
				if err := s.repo.Update(&p); err != nil {
					return fmt.Errorf("failed to set new default: %w", err)
				}
				break
			}
		}
	}

	if err := s.repo.Delete(id); err != nil {
		return fmt.Errorf("failed to delete storage path: %w", err)
	}

	s.logger.Info("Deleted storage path",
		zap.Uint("id", id),
		zap.String("name", existing.Name),
		zap.String("path", existing.Path),
	)

	return nil
}
