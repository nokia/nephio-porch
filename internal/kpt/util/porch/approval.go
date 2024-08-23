// Copyright 2022 The kpt and Nephio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package porch

import (
	"context"
	"fmt"

	"github.com/nephio-project/porch/api/porch/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func UpdatePackageRevisionApproval(ctx context.Context, client client.Client, pr *v1alpha1.PackageRevision, new v1alpha1.PackageRevisionLifecycle) error {

	switch lifecycle := pr.Spec.Lifecycle; lifecycle {
	case v1alpha1.PackageRevisionLifecycleProposed:
		// Approve - change the package revision kind to 'final'.
		if new != v1alpha1.PackageRevisionLifecyclePublished && new != v1alpha1.PackageRevisionLifecycleDraft {
			return fmt.Errorf("cannot change approval from %s to %s", lifecycle, new)
		}
	case v1alpha1.PackageRevisionLifecycleDeletionProposed:
		if new != v1alpha1.PackageRevisionLifecyclePublished {
			return fmt.Errorf("cannot change approval from %s to %s", lifecycle, new)
		}
	case new:
		// already correct value
		return nil
	default:
		return fmt.Errorf("cannot change approval from %s to %s", lifecycle, new)
	}

	pr.Spec.Lifecycle = new
	return client.SubResource("approval").Update(ctx, pr)
}
