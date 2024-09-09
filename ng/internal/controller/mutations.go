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

package packagevariant

import (
	"context"
	"fmt"

	porchapi "github.com/nephio-project/porch/api/porch/v1alpha1"
	api "github.com/nephio-project/porch/ng/api/v1alpha1"
)

const (
	InjectedByAnnotation = "PackageVariant.porch.kpt.dev/injected-by"
)

type mutator interface {
	// Apply applies the mutation to the KRM resources in prr
	Apply(ctx context.Context, prr *porchapi.PackageRevisionResources) error
}

func (r *PackageVariantReconciler) ensureMutations(ctx context.Context, pv *api.PackageVariant, prr *porchapi.PackageRevisionResources) error {
	// Check if there are any mutations specified in the PackageVariant
	for _, mutation := range pv.Spec.Mutations {

		// map Mutation API object to a mutator
		var mutator mutator
		switch mutation.Type {
		case api.MutationTypeInjectPackageRevision:
			if mutation.InjectPackageRevision == nil {
				return fmt.Errorf("mutation type %s requires a non-empty InjectPackage field", api.MutationTypeInjectPackageRevision)
			}
			mutator = &injectPR{
				mutation: &mutation,
				client:   r.Client,
				pv:       pv,
			}
		default:
			return fmt.Errorf("unsupported mutation type: %s", mutation.Type)
		}

		// apply mutation
		if err := mutator.Apply(ctx, prr); err != nil {
			return err
		}

	}
	return nil
}
