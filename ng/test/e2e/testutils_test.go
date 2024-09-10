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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (t *PvSuite) WaitUntilPackageVariantIsReady(ctx context.Context, pvKey types.NamespacedName) *api.PackageVariant {

	t.Helper()
	t.Logf("Waiting for PackageVariant %q to be ready", pvKey)
	timeout := 120 * time.Second
	var foundPv api.PackageVariant
	err := wait.PollUntilContextTimeout(ctx, time.Second, timeout, true, func(ctx context.Context) (done bool, err error) {
		err = t.Client.Get(ctx, pvKey, &foundPv)
		if err != nil {
			if client.IgnoreNotFound(err) != nil {
				return false, err
			}
		}
		for _, condition := range foundPv.Status.Conditions {
			if condition.Type == api.ConditionTypeReady && condition.Status == metav1.ConditionTrue {
				return true, nil
			}
		}
		return false, nil
	})
	if err != nil {
		t.Fatalf("PackageVariant %q is not ready in time (%v)", pvKey, timeout)
	}
	return &foundPv
}

func (t *PvSuite) CheckInjectedSubPackages(
	ctx context.Context,
	pv *api.PackageVariant,
	injections ...*api.InjectPackageRevision,
) {
	t.Helper()
	for _, mutation := range pv.Spec.Mutations {
		if mutation.Type == api.MutationTypeInjectPackageRevision {
			injections = append(injections, mutation.InjectPackageRevision)
		}
	}

	downstreamKey := types.NamespacedName{Namespace: pv.Namespace, Name: pv.Status.DownstreamTargets[0].Name}
	// get the contents of the downstream PR
	got := t.WaitUntilPackageRevisionResourcesExists(ctx, downstreamKey).Spec.Resources

	// calculate the expected contents of the downstream PR
	want := t.GetContentsOfPackageRevision(ctx, pv.Spec.Upstream.Repo, pv.Spec.Upstream.Package, pv.Spec.Upstream.Revision)
	for _, injection := range injections {
		injected := t.GetContentsOfPackageRevision(ctx, injection.Repo, injection.Package, injection.Revision)
		if injection.Subdir == "" {
			injection.Subdir = injection.Package
		}
		for name, content := range injected {
			want[injection.Subdir+"/"+name] = content
		}
	}

	// Only compare the name of the files, not the contents.
	// The content of the files are changed by normal PackageVariant behavior (i.e. rendering the package).
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
