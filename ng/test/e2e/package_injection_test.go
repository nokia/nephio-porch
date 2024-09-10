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
	"time"

	api "github.com/nephio-project/porch/ng/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (t *PvSuite) TestPackageVariantMutationInjectPackage(ctx context.Context) {
	const (
		downstreamRepository = "target"
		downstreamPackage    = "target-package"
		upstreamRepository   = "test-blueprints"
	)

	t.RegisterMainGitRepositoryF(ctx, downstreamRepository)
	t.RegisterGitRepositoryF(ctx, testBlueprintsRepo, upstreamRepository, "")

	t.Log("Testing PackageVariant with InjectPackageRevision mutations")
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
					InjectPackageRevision: &api.InjectPackageRevision{
						PackageRevisionRef: api.PackageRevisionRef{
							Repo:     upstreamRepository,
							Package:  "basens",
							Revision: "v2",
						},
					},
				},
				{
					Name: "inject-empty-v1",
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

	defer t.DeleteE(ctx, pv)
	t.CreateF(ctx, pv)

	pv = t.WaitUntilPackageVariantIsReady(ctx, client.ObjectKeyFromObject(pv))
	t.CheckInjectedSubPackages(ctx, pv)

	t.Log("Deleting the the injection of the 'empty' package, expecting it to be deleted from the downstream package.")
	pv.Spec.Mutations = pv.Spec.Mutations[:1]
	t.UpdateF(ctx, pv)
	// give it some time for the reconciler to be called at least once
	time.Sleep(2 * time.Second)
	pv = t.WaitUntilPackageVariantIsReady(ctx, client.ObjectKeyFromObject(pv))
	t.CheckInjectedSubPackages(ctx, pv)

	t.Log("changing the basens injection, expecting the downstream package to be updated.")
	pv.Spec.Mutations[0].InjectPackageRevision.Subdir = "new-basens"
	t.UpdateF(ctx, pv)
	// give it some time for the reconciler to be called at least once
	time.Sleep(2 * time.Second)
	pv = t.WaitUntilPackageVariantIsReady(ctx, client.ObjectKeyFromObject(pv))
	t.CheckInjectedSubPackages(ctx, pv)

	t.Log("changing the basens injection, expecting the downstream package to be updated.")
	pv.Spec.Mutations[0].InjectPackageRevision.Package = "empty"
	pv.Spec.Mutations[0].InjectPackageRevision.Revision = "v1"
	t.UpdateF(ctx, pv)
	// give it some time for the reconciler to be called at least once
	time.Sleep(2 * time.Second)
	pv = t.WaitUntilPackageVariantIsReady(ctx, client.ObjectKeyFromObject(pv))
	t.CheckInjectedSubPackages(ctx, pv)
}

func (t *PvSuite) TestPackageVariantMutationInjectLatestRevision(ctx context.Context) {
	const (
		downstreamRepository = "target"
		downstreamPackage    = "target-package"
		upstreamRepository   = "test-blueprints"
	)

	t.RegisterMainGitRepositoryF(ctx, downstreamRepository)
	t.RegisterGitRepositoryF(ctx, testBlueprintsRepo, upstreamRepository, "")

	t.Log("Testing PackageVariant with InjectPackageRevision mutations")
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
					Name: "inject-basens-v3",
					Type: api.MutationTypeInjectLatestPackageRevision,
					InjectLatestPackageRevision: &api.InjectLatestPackageRevision{
						PackageRef: api.PackageRef{
							Repo:    upstreamRepository,
							Package: "basens",
						},
					},
				},
			},
		},
	}

	defer t.DeleteE(ctx, pv)
	t.CreateF(ctx, pv)

	pv = t.WaitUntilPackageVariantIsReady(ctx, client.ObjectKeyFromObject(pv))
	t.CheckInjectedSubPackages(ctx, pv, &api.InjectPackageRevision{
		PackageRevisionRef: api.PackageRevisionRef{
			Repo:     upstreamRepository,
			Package:  "basens",
			Revision: "v3",
		},
		Subdir: "basens",
	})

	t.Log("Deleting the the injection of the 'empty' package, expecting it to be deleted from the downstream package.")
	pv.Spec.Mutations = nil
	t.UpdateF(ctx, pv)
	// give it some time for the reconciler to be called at least once
	time.Sleep(2 * time.Second)
	pv = t.WaitUntilPackageVariantIsReady(ctx, client.ObjectKeyFromObject(pv))
	t.CheckInjectedSubPackages(ctx, pv)

}
