package data

import (
	"time"

	"gorm.io/gorm"
)

type StoragePath struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	Name      string    `gorm:"not null;size:100" json:"name"`
	Path      string    `gorm:"not null;uniqueIndex;size:500" json:"path"`
	IsDefault bool      `gorm:"not null;default:false" json:"is_default"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (StoragePath) TableName() string {
	return "storage_paths"
}

type StoragePathRepository interface {
	List() ([]StoragePath, error)
	GetByID(id uint) (*StoragePath, error)
	GetByPath(path string) (*StoragePath, error)
	GetDefault() (*StoragePath, error)
	Create(storagePath *StoragePath) error
	Update(storagePath *StoragePath) error
	Delete(id uint) error
	ClearDefault() error
	Count() (int64, error)
}

type StoragePathRepositoryImpl struct {
	DB *gorm.DB
}

func NewStoragePathRepository(db *gorm.DB) *StoragePathRepositoryImpl {
	return &StoragePathRepositoryImpl{DB: db}
}

func (r *StoragePathRepositoryImpl) List() ([]StoragePath, error) {
	var paths []StoragePath
	err := r.DB.Order("is_default DESC, name ASC").Find(&paths).Error
	return paths, err
}

func (r *StoragePathRepositoryImpl) GetByID(id uint) (*StoragePath, error) {
	var path StoragePath
	err := r.DB.First(&path, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &path, nil
}

func (r *StoragePathRepositoryImpl) GetByPath(path string) (*StoragePath, error) {
	var storagePath StoragePath
	err := r.DB.Where("path = ?", path).First(&storagePath).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &storagePath, nil
}

func (r *StoragePathRepositoryImpl) GetDefault() (*StoragePath, error) {
	var path StoragePath
	err := r.DB.Where("is_default = ?", true).First(&path).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &path, nil
}

func (r *StoragePathRepositoryImpl) Create(storagePath *StoragePath) error {
	storagePath.CreatedAt = time.Now()
	storagePath.UpdatedAt = time.Now()
	return r.DB.Create(storagePath).Error
}

func (r *StoragePathRepositoryImpl) Update(storagePath *StoragePath) error {
	storagePath.UpdatedAt = time.Now()
	return r.DB.Save(storagePath).Error
}

func (r *StoragePathRepositoryImpl) Delete(id uint) error {
	return r.DB.Delete(&StoragePath{}, id).Error
}

func (r *StoragePathRepositoryImpl) ClearDefault() error {
	return r.DB.Model(&StoragePath{}).Where("is_default = ?", true).Update("is_default", false).Error
}

func (r *StoragePathRepositoryImpl) Count() (int64, error) {
	var count int64
	err := r.DB.Model(&StoragePath{}).Count(&count).Error
	return count, err
}
