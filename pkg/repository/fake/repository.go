// Copyright 2022, 2024 The kpt and Nephio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fake

import (
	"context"

	"github.com/nephio-project/porch/api/porch/v1alpha1"
	"github.com/nephio-project/porch/pkg/repository"
)

// Implementation of the repository.Repository interface for testing.
// TODO(mortent): Implement stub functionality for all functions from the interface.
type FakeRepository struct {
	PackageRevisions []repository.PackageRevision
	Packages         []repository.Package
	RepoVersion      string
}

var _ repository.Repository = &FakeRepository{}

func (r *FakeRepository) Close() error {
	return nil
}

func (r *FakeRepository) Version(context.Context) (string, error) {
	return r.RepoVersion, nil
}

func (r *FakeRepository) ListPackageRevisions(_ context.Context, filter repository.ListPackageRevisionFilter) ([]repository.PackageRevision, error) {
	var revs []repository.PackageRevision
	for _, rev := range r.PackageRevisions {
		if filter.KubeObjectName != "" && filter.KubeObjectName == rev.KubeObjectName() {
			revs = append(revs, rev)
		}
		if filter.Package != "" && filter.Package == rev.Key().Package {
			revs = append(revs, rev)
		}
		if filter.Revision != "" && filter.Revision == rev.Key().Revision {
			revs = append(revs, rev)
		}
		if filter.WorkspaceName != "" && filter.WorkspaceName == rev.Key().WorkspaceName {
			revs = append(revs, rev)
		}
	}
	return revs, nil
}

func (r *FakeRepository) CreatePackageRevision(context.Context, *v1alpha1.PackageRevision) (repository.PackageRevisionDraft, error) {
	return &FakePackageRevisionDraft{}, nil
}

func (r *FakeRepository) ClosePackageRevisionDraft(context.Context, repository.PackageRevisionDraft, string) (repository.PackageRevision, error) {
	return &FakePackageRevision{}, nil
}

func (r *FakeRepository) DeletePackageRevision(context.Context, repository.PackageRevision) error {
	return nil
}

func (r *FakeRepository) UpdatePackageRevision(context.Context, repository.PackageRevision) (repository.PackageRevisionDraft, error) {
	return &FakePackageRevisionDraft{}, nil
}

func (r *FakeRepository) ListPackages(context.Context, repository.ListPackageFilter) ([]repository.Package, error) {
	return r.Packages, nil
}

func (r *FakeRepository) CreatePackage(context.Context, *v1alpha1.PorchPackage) (repository.Package, error) {
	return &FakePackage{}, nil
}

func (r *FakeRepository) DeletePackage(context.Context, repository.Package) error {
	return nil
}

func (r *FakeRepository) Refresh(context.Context) error {
	return nil
}
