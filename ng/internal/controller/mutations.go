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
	"path/filepath"
	"strings"

	"github.com/GoogleContainerTools/kpt-functions-sdk/go/fn"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	porchapi "github.com/nephio-project/porch/api/porch/v1alpha1"
	api "github.com/nephio-project/porch/ng/api/v1alpha1"
	"github.com/nephio-project/porch/ng/internal/utils"
	kptfileapi "github.com/nephio-project/porch/pkg/kpt/api/kptfile/v1"
)

var (
	InjectedByResourceAnnotation = api.PackageVariantGVK.Kind + "." + api.PackageVariantGVK.Group + "/injected-by-resource"
	InjectedByMutationAnnotation = api.PackageVariantGVK.Kind + "." + api.PackageVariantGVK.Group + "/injected-by-mutation"
)

type mutator interface {
	// Apply applies the mutation to the `prr`
	Apply(ctx context.Context, prr *porchapi.PackageRevisionResources) error
}

// ensureMutations applies mutations specified in the PackageVariant to the PackageRevisionResources
// NOTE: this is not a member of the Reconciler for easier unit testing
func ensureMutations(ctx context.Context, cl client.Client, pv *api.PackageVariant, prr *porchapi.PackageRevisionResources) error {
	l := log.FromContext(ctx)
	errors := make([]string, 0)
	for _, mutation := range pv.Spec.Mutations {
		// map Mutation API object to a mutator
		var mutator mutator
		switch mutation.Type {
		case api.MutationTypeInjectPackageRevision, api.MutationTypeInjectLatestPackageRevision:
			mutator = &injectPR{
				mutation: &mutation,
				client:   cl,
				pvKey:    client.ObjectKeyFromObject(pv),
			}
		case api.MutationTypePrependPipeline, api.MutationTypeInjectObject:
			// handled elsewhere

		default:
			l.Info("TODO: unsupported mutation type: %s", mutation.Type)
		}

		// apply mutation
		err := mutator.Apply(ctx, prr)
		if err != nil {
			errors = append(errors, mutation.Id()+": "+err.Error())
		}

	}
	cleanUpOrphanedSubPackages(ctx, pv, prr)
	if len(errors) > 0 {
		return fmt.Errorf("failed to apply some mutations:\n  - %v", strings.Join(errors, "\n  - "))
	}
	return nil
}

// Remove any sub-packages that was injected by this PackageVariant, but by a mutation that no longer exists
func cleanUpOrphanedSubPackages(ctx context.Context, pv *api.PackageVariant, prr *porchapi.PackageRevisionResources) error {
	l := log.FromContext(ctx)
	existingMutationIds := sets.NewString()
	for _, m := range pv.Spec.Mutations {
		if m.Type == api.MutationTypeInjectPackageRevision ||
			m.Type == api.MutationTypeInjectLatestPackageRevision {
			existingMutationIds.Insert(m.Id())
		}
	}

	kobjs, _, err := utils.ReadKubeObjects(prr.Spec.Resources)
	if err != nil {
		return fmt.Errorf("couldn't read KubeObjects from PackageRevisionResources %q: %w", client.ObjectKeyFromObject(prr), err)
	}
	kptfilesInjectedByUs := kobjs.
		Where(fn.IsGroupKind(kptfileapi.KptFileGVK().GroupKind())).
		Where(fn.HasAnnotations(map[string]string{
			InjectedByResourceAnnotation: client.ObjectKeyFromObject(pv).String(),
		}))

	subdirsToDelete := []string{}
	for _, kptfile := range kptfilesInjectedByUs {
		mutationId := kptfile.GetAnnotation(InjectedByMutationAnnotation)
		if !existingMutationIds.Has(mutationId) {
			if !strings.Contains(kptfile.PathAnnotation(), "/") {
				l.Info("WARNING: KptFile resource injected by packagevariant has no / in its path annotation")
				continue
			}
			subdirsToDelete = append(subdirsToDelete, filepath.Dir(kptfile.PathAnnotation()))
		}
	}
	prr.Spec.Resources = deleteSubDirs(subdirsToDelete, prr.Spec.Resources)
	return nil
}

// deleteSubDirs removes all resources that are in any of the subdirectories specified in `subdirsToDelete`
// and returns with the result
func deleteSubDirs(subdirsToDelete []string, resources map[string]string) map[string]string {
	newResources := make(map[string]string)
	for filename, content := range resources {
		toBeDeleted := false
		for _, subdir := range subdirsToDelete {
			if strings.HasPrefix(filename, subdir+"/") {
				toBeDeleted = true
				break
			}
		}
		if !toBeDeleted {
			newResources[filename] = content
		}
	}
	return newResources
}
