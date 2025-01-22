package fake

import (
	"github.com/nephio-project/porch/api/porch/v1alpha1"
	"github.com/nephio-project/porch/pkg/repository"
)

type FakePackage struct {
	Name           string
	PackageKey     repository.PackageKey
	LatestRevision string
}

var _ repository.Package = &FakePackage{}

func (pkg *FakePackage) KubeObjectName() string {
	return pkg.Name
}

func (pkg *FakePackage) Key() repository.PackageKey {
	return pkg.PackageKey
}

func (pkg *FakePackage) GetPackage() *v1alpha1.PorchPackage {
	panic("not implemented")
}

func (pkg *FakePackage) GetLatestRevision() string {
	return pkg.LatestRevision
}
