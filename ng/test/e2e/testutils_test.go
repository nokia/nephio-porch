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

func (t *PvSuite) WaitUntilPackageVariantIsReady(
	ctx context.Context,
	key types.NamespacedName,
) *api.PackageVariant {

	t.Helper()
	t.Logf("Waiting for PackageVariant %v to be ready", key)
	timeout := 120 * time.Second
	var foundPv api.PackageVariant
	err := wait.PollUntilContextTimeout(ctx, time.Second, timeout, true, func(ctx context.Context) (done bool, err error) {
		err = t.Client.Get(ctx, key, &foundPv)
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
		t.Fatalf("PackageVariant (%v) is not ready in time (%v)", key, timeout)
	}
	return &foundPv
}
