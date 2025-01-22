package fake

import (
	"context"

	"github.com/nephio-project/porch/pkg/engine/utils"
	"github.com/nephio-project/porch/pkg/repository"
	"k8s.io/apimachinery/pkg/watch"
)

type FakeWatcherManager struct{}

var _ utils.WatcherManager = &FakeWatcherManager{}

func (m *FakeWatcherManager) WatchPackageRevisions(context.Context, repository.ListPackageRevisionFilter, utils.ObjectWatcher) error {
	return nil
}

func (m *FakeWatcherManager) NotifyPackageRevisionChange(watch.EventType, repository.PackageRevision) int {
	return 0
}
