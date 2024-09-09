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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	porchapi "github.com/nephio-project/porch/api/porch/v1alpha1"
	api "github.com/nephio-project/porch/ng/api/v1alpha1"
	"github.com/nephio-project/porch/ng/internal/utils"
	kptfileapi "github.com/nephio-project/porch/pkg/kpt/api/kptfile/v1"
)

const (
	InjectedByResourceAnnotation = "PackageVariant.porch.kpt.dev/injected-by-resource"
	InjectedByMutationAnnotation = "PackageVariant.porch.kpt.dev/injected-by-mutation"
)

type mutator interface {
	// Apply applies the mutation to the KRM resources in prr
	Apply(ctx context.Context, prr *porchapi.PackageRevisionResources) error
}

func ensureMutations(ctx context.Context, client client.Client, pv *api.PackageVariant, prr *porchapi.PackageRevisionResources) error {
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
				client:   client,
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
	cleanUpOrphanedSubPackages(ctx, pv, prr)
	return nil
}

// Remove any sub-packages that was injected by this PackageVariant, but by a mutation that no longer exists
func cleanUpOrphanedSubPackages(ctx context.Context, pv *api.PackageVariant, prr *porchapi.PackageRevisionResources) error {
	l := log.FromContext(ctx)
	pvId := fmt.Sprintf("%s/%s", pv.Namespace, pv.Name)
	existingMutationIds := make([]string, 0)
	for _, m := range pv.Spec.Mutations {
		if m.Type == api.MutationTypeInjectPackageRevision {
			existingMutationIds = append(existingMutationIds, fmt.Sprintf("%s/%s", m.Manager, m.Name))
		}
	}

	kobjs, _, err := utils.ReadKubeObjects(prr.Spec.Resources)
	if err != nil {
		return fmt.Errorf("couldn't read KubeObjects from PackageRevisionResources %q: %w", client.ObjectKeyFromObject(prr), err)
	}
	kptfilesInjectedByUs := kobjs.
		Where(fn.IsGroupKind(kptfileapi.KptFileGVK().GroupKind())).
		Where(fn.HasAnnotations(map[string]string{
			InjectedByResourceAnnotation: pvId,
		}))

	subdirsToDelete := make([]string, 0)
	for _, kptfile := range kptfilesInjectedByUs {
		mutationId := kptfile.GetAnnotation(InjectedByMutationAnnotation)
		orphan := true
		for _, existingMutationId := range existingMutationIds {
			if mutationId == existingMutationId {
				orphan = false
				break
			}
		}
		if orphan {
			if !strings.Contains(kptfile.PathAnnotation(), "/") {
				l.Info("WARNING: KptFile resource injected by packagevariant has no / in its path annotation")
				continue
			}
			subdirsToDelete = append(subdirsToDelete, filepath.Dir(kptfile.PathAnnotation()))
		}
	}

	// delete every package previously injected by us that doesn't match the current injection parameters
	prr.Spec.Resources = deleteSubDirs(subdirsToDelete, prr.Spec.Resources)
	return nil
}

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
