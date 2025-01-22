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
	kptfile "github.com/nephio-project/porch/pkg/kpt/api/kptfile/v1"
	"github.com/nephio-project/porch/pkg/repository"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// Implementation of the repository.PackageRevision interface for testing.
type FakePackageRevision struct {
	Name               string
	Namespace          string
	Uid                types.UID
	PackageRevisionKey repository.PackageRevisionKey
	PackageLifecycle   v1alpha1.PackageRevisionLifecycle
	PackageRevision    *v1alpha1.PackageRevision
	Resources          *v1alpha1.PackageRevisionResources
	Kptfile            kptfile.KptFile
}

var _ repository.PackageRevision = &FakePackageRevision{}

func (pr *FakePackageRevision) KubeObjectName() string {
	return pr.Name
}

// ToMainPackageRevision implements repository.PackageRevision.
func (pr *FakePackageRevision) ToMainPackageRevision() repository.PackageRevision {
	panic("unimplemented")
}

func (pr *FakePackageRevision) KubeObjectNamespace() string {
	return pr.Namespace
}

func (pr *FakePackageRevision) UID() types.UID {
	return pr.Uid
}

func (pr *FakePackageRevision) ResourceVersion() string {
	return pr.PackageRevision.ResourceVersion
}

func (pr *FakePackageRevision) Key() repository.PackageRevisionKey {
	return pr.PackageRevisionKey
}

func (pr *FakePackageRevision) Lifecycle(ctx context.Context) v1alpha1.PackageRevisionLifecycle {
	return pr.PackageLifecycle
}

func (pr *FakePackageRevision) GetPackageRevision(context.Context) (*v1alpha1.PackageRevision, error) {
	return pr.PackageRevision, nil
}

func (pr *FakePackageRevision) GetResources(context.Context) (*v1alpha1.PackageRevisionResources, error) {
	return pr.Resources, nil
}

func (pr *FakePackageRevision) GetKptfile(context.Context) (kptfile.KptFile, error) {
	return pr.Kptfile, nil
}

func (pr *FakePackageRevision) GetUpstreamLock(context.Context) (kptfile.Upstream, kptfile.UpstreamLock, error) {
	return *pr.Kptfile.Upstream, *pr.Kptfile.UpstreamLock, nil
}

func (pr *FakePackageRevision) GetLock() (kptfile.Upstream, kptfile.UpstreamLock, error) {
	return *pr.Kptfile.Upstream, *pr.Kptfile.UpstreamLock, nil
}

func (pr *FakePackageRevision) UpdateLifecycle(context.Context, v1alpha1.PackageRevisionLifecycle) error {
	return nil
}

func (pr *FakePackageRevision) GetMeta() metav1.ObjectMeta {
	return metav1.ObjectMeta{}
}

func (pr *FakePackageRevision) SetMeta(metav1.ObjectMeta) {
}

// Implementation of the repository.PackageRevisionDraft interface for testing.
type FakePackageRevisionDraft struct {
	Name string
}

var _ repository.PackageRevisionDraft = &FakePackageRevisionDraft{}

func (prd *FakePackageRevisionDraft) UpdateResources(context.Context, *v1alpha1.PackageRevisionResources, *v1alpha1.Task) error {
	return nil
}

func (prd *FakePackageRevisionDraft) UpdateLifecycle(ctx context.Context, new v1alpha1.PackageRevisionLifecycle) error {
	return nil
}

func (prd *FakePackageRevisionDraft) GetName() string {
	return prd.Name
}
