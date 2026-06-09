package s3

import "context"

type ObjectStorageRepository struct{}

func NewObjectStorageRepository() *ObjectStorageRepository {
	return &ObjectStorageRepository{}
}

func (repository *ObjectStorageRepository) Store(ctx context.Context, key string) error {
	return nil
}
