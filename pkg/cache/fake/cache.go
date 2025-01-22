package fake

import (
	"context"

	configapi "github.com/nephio-project/porch/api/porchconfig/v1alpha1"
	"github.com/nephio-project/porch/pkg/cache"
	"github.com/nephio-project/porch/pkg/repository"
	"github.com/nephio-project/porch/pkg/repository/fake"
)

type FakeCache struct{}

var _ cache.Cache = &FakeCache{}

func (m *FakeCache) OpenRepository(context.Context, *configapi.Repository) (repository.Repository, error) {
	return &fake.FakeRepository{}, nil
}

func (m *FakeCache) CloseRepository(context.Context, *configapi.Repository, []configapi.Repository) error {
	return nil
}
