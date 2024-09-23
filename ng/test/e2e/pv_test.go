// Copyright 2024 The kpt and Nephio Authors
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

package e2e

import (
	"context"
	"os"
	"testing"

	api "github.com/nephio-project/porch/ng/api/v1alpha1"
	"github.com/nephio-project/porch/test/e2e"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

const (
	testBlueprintsRepo = "https://github.com/platkrm/test-blueprints.git"
)

type PvSuite struct {
	e2e.TestSuiteWithGit
}

var _ e2e.Initializer = &PvSuite{}

func TestE2E(t *testing.T) {
	if os.Getenv("E2E") == "" {
		t.Skip("set E2E to run this test")
	}

	pvSuite := PvSuite{}
	e2e.RunSuite(&pvSuite, t)
}

func (t *PvSuite) Initialize(ctx context.Context) {
	t.TestSuiteWithGit.Initialize(ctx)
	utilruntime.Must(api.AddToScheme(t.Client.Scheme()))
}

func (t *PvSuite) TestMutationsWithSameName(ctx context.Context) {
	const (
		downstreamRepository = "target"
		downstreamPackage    = "target-package"
		upstreamRepository   = "test-blueprints"
	)

	t.RegisterMainGitRepositoryF(ctx, downstreamRepository)
	t.RegisterGitRepositoryF(ctx, testBlueprintsRepo, upstreamRepository, "")

	pv := &api.PackageVariant{
		TypeMeta: metav1.TypeMeta{
			APIVersion: api.GroupVersion.String(),
			Kind:       "PackageVariant",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pv",
			Namespace: t.Namespace,
		},
		Spec: api.PackageVariantSpec{
			Upstream: api.PackageRevisionRef{
				Repo:     upstreamRepository,
				Package:  "basens",
				Revision: "v3",
			},
			Downstream: api.PackageRef{
				Repo:    downstreamRepository,
				Package: downstreamPackage,
			},
			Mutations: []api.Mutation{
				{
					Name: "my-mutation",
					Type: api.MutationTypeInjectPackageRevision,
					InjectPackageRevision: &api.InjectPackageRevision{
						PackageRevisionRef: api.PackageRevisionRef{
							Repo:     upstreamRepository,
							Package:  "basens",
							Revision: "v2",
						},
					},
				},
				{
					Name: "my-mutation",
					Type: api.MutationTypeInjectPackageRevision,
					InjectPackageRevision: &api.InjectPackageRevision{
						PackageRevisionRef: api.PackageRevisionRef{
							Repo:     upstreamRepository,
							Package:  "empty",
							Revision: "v1",
						},
					},
				},
			},
		},
	}
	t.Log("Trying to create a PackageVariant with two mutations with the same name... expecting error.")
	err := t.Client.Create(ctx, pv)
	if err == nil {
		t.Errorf("expected error while creating PV with non-unique mutation IDs, got nil")
		t.DeleteE(ctx, pv)
	}
	t.Logf("Got expected error: %v", err)
}

func (t *PvSuite) TestMissingMutationValue(ctx context.Context) {
	const (
		downstreamRepository = "target"
		downstreamPackage    = "target-package"
		upstreamRepository   = "test-blueprints"
	)

	t.RegisterMainGitRepositoryF(ctx, downstreamRepository)
	t.RegisterGitRepositoryF(ctx, testBlueprintsRepo, upstreamRepository, "")

	pv := &api.PackageVariant{
		TypeMeta: metav1.TypeMeta{
			APIVersion: api.GroupVersion.String(),
			Kind:       "PackageVariant",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pv",
			Namespace: t.Namespace,
		},
		Spec: api.PackageVariantSpec{
			Upstream: api.PackageRevisionRef{
				Repo:     upstreamRepository,
				Package:  "basens",
				Revision: "v3",
			},
			Downstream: api.PackageRef{
				Repo:    downstreamRepository,
				Package: downstreamPackage,
			},
			Mutations: []api.Mutation{
				{
					Name: "inject-basens-v2",
					Type: api.MutationTypeInjectPackageRevision,
				},
			},
		},
	}
	t.Log("Trying to create a PackageVariant with missing mutation parameters... expecting error.")
	err := t.Client.Create(ctx, pv)
	if err == nil {
		t.Errorf("expected error, got nil")
		t.DeleteE(ctx, pv)
	}
	t.Logf("Got expected error: %v", err)
}