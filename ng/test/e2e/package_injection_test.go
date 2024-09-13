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

	porchapi "github.com/nephio-project/porch/api/porch/v1alpha1"
	api "github.com/nephio-project/porch/ng/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (t *PvSuite) TestInjectPackage(ctx context.Context) {
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
						Subdir: "nothing",
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

func (t *PvSuite) TestInjectLatestRevision(ctx context.Context) {
	const (
		downstreamRepository = "target"
		downstreamPackage    = "target-package"
		downstreamPackage2   = "target-package-2"
		upstreamRepository   = "test-blueprints"
	)

	t.RegisterMainGitRepositoryF(ctx, downstreamRepository)
	t.RegisterGitRepositoryF(ctx, testBlueprintsRepo, upstreamRepository, "")

	t.Log("** Creating PackageVariant (PV1) with InjectLatestPackageRevision mutations")
	pv1 := &api.PackageVariant{
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

	defer t.DeleteE(ctx, pv1)
	t.CreateF(ctx, pv1)

	pv1 = t.WaitUntilPackageVariantIsReady(ctx, client.ObjectKeyFromObject(pv1))
	t.CheckInjectedSubPackages(ctx, pv1, &api.InjectPackageRevision{
		PackageRevisionRef: api.PackageRevisionRef{
			Repo:     upstreamRepository,
			Package:  "basens",
			Revision: "v3",
		},
		Subdir: "basens",
	})

	t.Log("** Deleting the the injection of the 'basens' package, expecting it to be deleted from the downstream package.")
	pv1.Spec.Mutations = nil
	t.UpdateF(ctx, pv1)
	// give it some time for the reconciler to be called at least once
	time.Sleep(2 * time.Second)
	pv1 = t.WaitUntilPackageVariantIsReady(ctx, client.ObjectKeyFromObject(pv1))
	t.CheckInjectedSubPackages(ctx, pv1)

	t.Log("** Approve downstream PR of PV1")
	downstreamPrKey := types.NamespacedName{Namespace: pv1.Namespace, Name: pv1.Status.DownstreamTargets[0].Name}
	var pr porchapi.PackageRevision
	t.MustExist(ctx, downstreamPrKey, &pr)
	pr.Spec.Lifecycle = porchapi.PackageRevisionLifecycleProposed
	t.UpdateF(ctx, &pr)
	pr.Spec.Lifecycle = porchapi.PackageRevisionLifecyclePublished
	downstream1_v1 := t.UpdateApprovalF(ctx, &pr, metav1.UpdateOptions{})
	if downstream1_v1.Spec.Revision != "v1" {
		t.Fatalf("expected downstream package to be at revision v1, got %q", downstream1_v1.Spec.Revision)
	}
	if !porchapi.LifecycleIsPublished(downstream1_v1.Spec.Lifecycle) {
		t.Fatalf("expected downstream package to be published, got %q", downstream1_v1.Spec.Lifecycle)
	}

	t.Log("** Create another PackageVariant (PV2) that injects the latest revision of PV1's downstream package")
	pv2 := &api.PackageVariant{
		TypeMeta: metav1.TypeMeta{
			APIVersion: api.GroupVersion.String(),
			Kind:       "PackageVariant",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pv-2",
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
				Package: downstreamPackage2,
			},
			Mutations: []api.Mutation{
				{
					Name: "inject-latest",
					Type: api.MutationTypeInjectLatestPackageRevision,
					InjectLatestPackageRevision: &api.InjectLatestPackageRevision{
						PackageRef: api.PackageRef{
							Repo:    downstreamRepository,
							Package: downstreamPackage,
						},
						Subdir: "latest",
					},
				},
			},
		},
	}

	defer t.DeleteE(ctx, pv2)
	t.CreateF(ctx, pv2)

	t.Log("** Check if the downstream package of PV2 is correct")
	pv2 = t.WaitUntilPackageVariantIsReady(ctx, client.ObjectKeyFromObject(pv2))
	t.CheckInjectedSubPackages(ctx, pv2, &api.InjectPackageRevision{
		PackageRevisionRef: api.PackageRevisionRef{
			Repo:     downstreamRepository,
			Package:  downstreamPackage,
			Revision: "v1",
		},
		Subdir: "latest",
	})

	t.Log("** Create a new revision of PV1's downstream package by injecting basens into it")
	pv1.Spec.Mutations = []api.Mutation{
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
	}
	t.UpdateF(ctx, pv1)
	// give it some time for the reconciler to be called at least once
	time.Sleep(2 * time.Second)
	pv1 = t.WaitUntilPackageVariantIsReady(ctx, client.ObjectKeyFromObject(pv1))
	t.CheckInjectedSubPackages(ctx, pv1, &api.InjectPackageRevision{
		PackageRevisionRef: api.PackageRevisionRef{
			Repo:     upstreamRepository,
			Package:  "basens",
			Revision: "v3",
		},
		Subdir: "basens",
	})

	t.Log("** Approve downstream PR of PV1")
	downstreamPrKey = types.NamespacedName{Namespace: pv1.Namespace, Name: pv1.Status.DownstreamTargets[0].Name}
	t.MustExist(ctx, downstreamPrKey, &pr)
	pr.Spec.Lifecycle = porchapi.PackageRevisionLifecycleProposed
	t.UpdateF(ctx, &pr)
	pr.Spec.Lifecycle = porchapi.PackageRevisionLifecyclePublished
	downstream1_v2 := t.UpdateApprovalF(ctx, &pr, metav1.UpdateOptions{})
	if downstream1_v2.Spec.Revision != "v2" {
		t.Fatalf("expected downstream package to be at revision v1, got %q", downstream1_v1.Spec.Revision)
	}
	if !porchapi.LifecycleIsPublished(downstream1_v2.Spec.Lifecycle) {
		t.Fatalf("expected downstream package to be published, got %q", downstream1_v1.Spec.Lifecycle)
	}

	t.Log("** Check if the downstream package of PV2 was automatically updated")
	time.Sleep(5 * time.Second)
	pv2 = t.WaitUntilPackageVariantIsReady(ctx, client.ObjectKeyFromObject(pv2))
	t.CheckInjectedSubPackages(ctx, pv2, &api.InjectPackageRevision{
		PackageRevisionRef: api.PackageRevisionRef{
			Repo:     downstreamRepository,
			Package:  downstreamPackage,
			Revision: "v2",
		},
		Subdir: "latest",
	})
}
