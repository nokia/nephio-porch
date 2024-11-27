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

	api "github.com/nephio-project/porch/ng/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (t *PvSuite) TestMutationInjector(ctx context.Context) {
	const (
		downstreamRepository = "target"
		downstreamPackage    = "target-package"
		downstreamPackage2   = "target-package-2"
		upstreamRepository   = "test-blueprints"
	)

	t.RegisterGitRepositoryF(ctx, testBlueprintsRepo, upstreamRepository, "")
	t.RegisterMainGitRepositoryF(ctx, downstreamRepository)

	t.Log("** Creating PackageVariant")
	pv1 := &api.PackageVariant{
		TypeMeta: metav1.TypeMeta{
			APIVersion: api.GroupVersion.String(),
			Kind:       "PackageVariant",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pv",
			Namespace: t.Namespace,
			Labels: map[string]string{
				"app": "test",
			},
		},
		Spec: api.PackageVariantSpec{
			Upstream: &api.PackageRevisionRef{
				Repo:     upstreamRepository,
				Package:  "basens",
				Revision: "v3",
			},
			Downstream: &api.PackageRef{
				Repo:    downstreamRepository,
				Package: downstreamPackage,
			},
		},
	}

	defer t.DeleteE(ctx, pv1)
	t.CreateF(ctx, pv1)
	pv1 = t.WaitUntilPackageVariantIsReady(ctx, client.ObjectKeyFromObject(pv1))

	t.Log("** Creating MutationInjector")
	mi := &api.MutationInjector{
		TypeMeta: metav1.TypeMeta{
			APIVersion: api.GroupVersion.String(),
			Kind:       "MutationInjector",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-mi",
			Namespace: t.Namespace,
		},
		Spec: api.MutationInjectorSpec{
			PackageVariantSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "test",
				},
			},
			Mutations: []api.Mutation{
				{
					Name:    "inject-basens-v2",
					Manager: "this-should-be-overwritten",
					Type:    api.MutationTypeInjectPackageRevision,
					InjectPackageRevision: &api.InjectPackageRevision{
						PackageRevisionRef: api.PackageRevisionRef{
							Repo:     upstreamRepository,
							Package:  "basens",
							Revision: "v2",
						},
					},
				},
			},
		},
	}
	defer t.DeleteE(ctx, mi)
	t.CreateF(ctx, mi)

	t.WaitUntilPackageVariantHasMutations(ctx, client.ObjectKeyFromObject(pv1))
	t.CheckInjectedSubPackages(ctx, pv1, &api.InjectPackageRevision{
		PackageRevisionRef: api.PackageRevisionRef{
			Repo:     upstreamRepository,
			Package:  "basens",
			Revision: "v3",
		},
		Subdir: "basens",
	})

}
