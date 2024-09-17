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

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/GoogleContainerTools/kpt-functions-sdk/go/fn"
	porchapi "github.com/nephio-project/porch/api/porch/v1alpha1"
	api "github.com/nephio-project/porch/ng/api/v1alpha1"
	"github.com/nephio-project/porch/ng/utils"
)

type injectInlineObject struct {
	mutation *api.Mutation
	pvKey    types.NamespacedName
}

var _ mutator = &injectInlineObject{}

func (m *injectInlineObject) Apply(ctx context.Context, prr *porchapi.PackageRevisionResources) error {
	toInject, err := fn.ParseKubeObject([]byte(m.mutation.InjectInlineObject))
	if err != nil {
		return err
	}

	// TODO: signal that these are validation errors that can only be recovered from by changing the PackageVariant
	if toInject.GetKind() == "" {
		return fmt.Errorf("kind is missing from the injected object")
	}
	if toInject.GetName() == "" {
		return fmt.Errorf("name is missing from the injected object")
	}

	// parse PRR
	kobjs, _, err := utils.ReadKubeObjects(prr.Spec.Resources)
	if err != nil {
		return fmt.Errorf("couldn't read KubeObjects from PackageRevisionResources %q: %w", prr.Name, err)
	}

	// find injection points
	// TODO: check namespace if it's set
	// TODO: check injection-point annotation
	injectionPoints := kobjs.Where(fn.IsGroupKind(toInject.GroupKind())).Where(fn.IsName(toInject.GetName()))
	if len(injectionPoints) == 0 {
		return fmt.Errorf("couldn't find injection point for object %s/%s", toInject.GroupKind(), toInject.GetName())
	}

	// overwrite injection points
	pathsToCopy := sets.NewString()
	for _, ip := range injectionPoints {
		if ip.GetAnnotation(InjectedByResourceAnnotation) == m.pvKey.String() &&
			ip.GetAnnotation(InjectedByMutationAnnotation) == m.mutation.Id() {
			// already injected by us
			continue
		}

		pathsToCopy.Insert(ip.PathAnnotation())
		if toInject.GetMap("spec") != nil {
			ip.SetMap(toInject.GetMap("spec"), "spec")
		}
		if toInject.GetMap("data") != nil {
			ip.SetMap(toInject.GetMap("data"), "data")
		}
		ip.SetAnnotation(InjectedResourceAnnotation, "from-inline-resource")
		ip.SetAnnotation(InjectedByResourceAnnotation, m.pvKey.String())
		ip.SetAnnotation(InjectedByMutationAnnotation, m.mutation.Id())
	}

	// overwrite necessary files
	resources, err := utils.WriteKubeObjects(kobjs)
	if err != nil {
		return err
	}
	for _, path := range pathsToCopy.List() {
		prr.Spec.Resources[path] = resources[path]
	}
	return nil
}
