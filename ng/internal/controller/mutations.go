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
	"strings"

	porchapi "github.com/nephio-project/porch/api/porch/v1alpha1"
	api "github.com/nephio-project/porch/ng/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
		case api.MutationTypeInjectPackage:
			if mutation.InjectPackage == nil {
				return fmt.Errorf("mutation type %s requires a non-empty InjectPackage field", api.MutationTypeInjectPackage)
			}
			mutator = &injectPackage{
				config: mutation.InjectPackage,
				client: r.Client,
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

// injectPackage is a mutator that injects a whole package into the target package revision
type injectPackage struct {
	config *api.InjectPackage
	client client.Client
}

var _ mutator = &injectPackage{}

func (i *injectPackage) Apply(ctx context.Context, prr *porchapi.PackageRevisionResources) error {

	newResources := make(map[string]string)

	if i.config.Subdir == "" {
		i.config.Subdir = i.config.Package.Package
	}

	// delete everything from the target subdir
	for filename, content := range prr.Spec.Resources {
		if !strings.HasPrefix(filename, i.config.Subdir+"/") {
			newResources[filename] = content
		}
	}

	// Fetch the package to inject
	prs := PackageRevisionsFromContext(ctx)
	prToInject := prs.OfPackage(i.config.Package.Repo, i.config.Package.Package).Revision(i.config.Package.Revision)
	if prToInject == nil {
		return fmt.Errorf("couldn't find package revision to inject: %v/%v/%v", i.config.Package.Repo, i.config.Package.Package, i.config.Package.Revision)
	}
	if !porchapi.LifecycleIsPublished(prToInject.Spec.Lifecycle) {
		return fmt.Errorf("package revision to inject (%v/%v/%v) must be published, but it's lifecycle state is %s", i.config.Package.Repo, i.config.Package.Package, i.config.Package.Revision, prToInject.Spec.Lifecycle)
	}

	// Load the PackageRevisionResources
	var prrToInject porchapi.PackageRevisionResources
	if err := i.client.Get(ctx, client.ObjectKeyFromObject(prToInject), &prrToInject); err != nil {
		return fmt.Errorf("couldn't read the package revision that should be inserted (%s/%s/%s): %w", i.config.Package.Repo, i.config.Package.Package, i.config.Package.Revision, err)
	}

	// Inject the resources
	for filename, content := range prrToInject.Spec.Resources {
		newResources[i.config.Subdir+"/"+filename] = content
	}

	prr.Spec.Resources = newResources

	return nil
}
