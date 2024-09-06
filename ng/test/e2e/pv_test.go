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
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	_ = api.AddToScheme(t.Client.Scheme())
}

func (t *PvSuite) TestPackageVariantMutationInjectPackage(ctx context.Context) {
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
			Upstream: &api.Upstream{
				Repo:     upstreamRepository,
				Package:  "basens",
				Revision: "v3",
			},
			Downstream: &api.Downstream{
				Repo:    downstreamRepository,
				Package: downstreamPackage,
			},
			Mutations: []api.Mutation{
				{
					Name: "inject-basens-v2",
					Type: api.MutationTypeInjectPackage,
					InjectPackage: &api.InjectPackage{
						Package: api.Upstream{
							Repo:     upstreamRepository,
							Package:  "basens",
							Revision: "v2",
						},
					},
				},
				{
					Name: "inject-empty-v1",
					Type: api.MutationTypeInjectPackage,
					InjectPackage: &api.InjectPackage{
						Package: api.Upstream{
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

	// get the contents of the downstream PR
	downstreamPR := t.WaitUntilDraftPackageRevisionExists(ctx, downstreamRepository, downstreamPackage)
	downstreamPRR := t.WaitUntilPackageRevisionResourcesExists(ctx, client.ObjectKeyFromObject(downstreamPR))

	// calculate the expected contents of the downstream PR
	expectedContents := t.GetContentsOfPackageRevision(ctx, upstreamRepository, "basens", "v3")
	injected1 := t.GetContentsOfPackageRevision(ctx, upstreamRepository, "basens", "v2")
	injected2 := t.GetContentsOfPackageRevision(ctx, upstreamRepository, "empty", "v1")
	for name, content := range injected1 {
		expectedContents["basens/"+name] = content
	}
	for name, content := range injected2 {
		expectedContents["empty/"+name] = content
	}

	got, want := downstreamPRR.Spec.Resources, expectedContents
	// Only compare the name of the files, not the contents.
	// The content of the files are changed by normal PackageVariant behavior.
	// NOTE: the comparison below is obviously not efficient, but it is intended to produce a useful error message.
	for name := range want {
		if _, found := got[name]; !found {
			t.Errorf("Resource %s is missing from the downstream package", name)
		}
	}
	for name := range got {
		if _, found := want[name]; !found {
			t.Errorf("Resource %s is not expected to be in the downstream package", name)
		}
	}
}
