// Copyright 2025 The kpt and Nephio Authors
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

package repoimpltypes

import (
	"context"
	"time"

	configapi "github.com/nephio-project/porch/api/porchconfig/v1alpha1"
	"github.com/nephio-project/porch/pkg/meta"
	"github.com/nephio-project/porch/pkg/repository"
	"k8s.io/apimachinery/pkg/watch"
)

type RepoImplFactory interface {
	NewRepositoryImpl(ctx context.Context, repositorySpec *configapi.Repository, options RepoImplOptions) (repository.Repository, error)
}

type RepoImplOptions struct {
	LocalDirectory         string
	RepoSyncFrequency      time.Duration
	UseUserDefinedCaBundle bool
	CredentialResolver     repository.CredentialResolver
	UserInfoProvider       repository.UserInfoProvider
	MetadataStore          meta.MetadataStore
	RepoPRNotifier         RepoPRNotifier
}

type RepoPRNotifier interface {
	NotifyPackageRevisionChange(eventType watch.EventType, obj repository.PackageRevision) int
}
