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

package controller

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/kyaml/kio/kioutil"

	porchapi "github.com/nephio-project/porch/api/porch/v1alpha1"
	api "github.com/nephio-project/porch/ng/api/v1alpha1"
	"github.com/nephio-project/porch/ng/utils"
	"github.com/nephio-project/porch/third_party/GoogleContainerTools/kpt-functions-sdk/go/fn"
)

type injectObjectFromPackage struct {
	mutation *api.Mutation
	client   client.Client
	pvKey    types.NamespacedName
}

var _ mutator = &injectObjectFromPackage{}

func (m *injectObjectFromPackage) Apply(ctx context.Context, prr *porchapi.PackageRevisionResources) error {
	// l := log.FromContext(ctx)
	prs := utils.PackageRevisionsFromContextOrDie(ctx)

	// get the parameters of injection
	cfg := *m.mutation.InjectObjectFromPackage
	ApplyDefaults(cfg.Destination, cfg.Source)

	// find destination object, if exists
	kobjs, _, err := utils.ReadKubeObjects(prr.Spec.Resources)
	if err != nil {
		return fmt.Errorf("couldn't read KubeObjects from PackageRevisionResources %q: %w", client.ObjectKeyFromObject(prr), err)
	}
	dsts := kobjs.Where(MatchesWithRef(cfg.Destination))
	if len(dsts) > 1 {
		return fmt.Errorf("too many objects matching the destination reference %v in the package", cfg.Destination)
	}
	var dst *fn.KubeObject
	if len(dsts) == 1 {
		dst = dsts[0]
		if IsInjectedByMutation(dst, m.pvKey, m.mutation) {
			// the object has already been injected by this mutation
			return nil
		}
	}

	// Fetch the source package to inject the object from
	srcPkg := prs.OfPackage(cfg.Repo, cfg.Package).Revision(cfg.Revision)
	if srcPkg == nil {
		return fmt.Errorf("couldn't find package revision to inject object from: %v/%v/%v", cfg.Repo, cfg.Package, cfg.Revision)
	}
	if !porchapi.LifecycleIsPublished(srcPkg.Spec.Lifecycle) {
		return fmt.Errorf("package revision to inject object from (%v/%v/%v) must be published, but it's lifecycle state is %s", cfg.Repo, cfg.Package, cfg.Revision, srcPkg.Spec.Lifecycle)
	}
	var srcPrr porchapi.PackageRevisionResources
	if err := m.client.Get(ctx, client.ObjectKeyFromObject(srcPkg), &srcPrr); err != nil {
		return fmt.Errorf("couldn't read the contents of  source package revision (%s/%s/%s): %w", cfg.Repo, cfg.Package, cfg.Revision, err)
	}
	srcObjs, _, err := utils.ReadKubeObjects(srcPrr.Spec.Resources)
	if err != nil {
		return fmt.Errorf("couldn't read KubeObjects from source package revision (%s/%s/%s): %w", cfg.Repo, cfg.Package, cfg.Revision, err)
	}
	src, err := utils.SingleItem(srcObjs.Where(MatchesWithRef(cfg.Source)))
	if err != nil {
		return fmt.Errorf("couldn't find object to inject in source package revision (%s/%s/%s): %w", cfg.Repo, cfg.Package, cfg.Revision, err)
	}
	// Inject the resources
	if dst == nil {
		dst = src.Copy()
		if cfg.Destination.Group != nil || cfg.Destination.Version != nil {
			if cfg.Destination.Group == nil {
				cfg.Destination.Group = &src.GetId().Group
			}
			if cfg.Destination.Version == nil {
				cfg.Destination.Version = &src.GetId().Version
			}
			dst.SetAPIVersion(*cfg.Destination.Group + "/" + *cfg.Destination.Version)
		}
		if cfg.Destination.Kind != nil {
			dst.SetKind(*cfg.Destination.Kind)
		}
		if cfg.Destination.Namespace != nil {
			dst.SetNamespace(*cfg.Destination.Namespace)
		}
		if cfg.Destination.Name != nil {
			dst.SetName(*cfg.Destination.Name)
		}
		dst.SetAnnotation(kioutil.PathAnnotation, "injected_objects.yaml")
		kobjs = append(kobjs, dst)
	} else {
		CopyData(src, dst)
	}

	pathsUpdated := sets.NewString(dst.PathAnnotation())
	dst.SetAnnotation(InjectedResourceAnnotation, src.GetName())
	dst.SetAnnotation(InjectedByResourceAnnotation, m.pvKey.String())
	dst.SetAnnotation(InjectedByMutationAnnotation, m.mutation.Id())

	// overwrite necessary files
	resources, err := utils.WriteKubeObjects(kobjs)
	if err != nil {
		return err
	}
	for path := range pathsUpdated {
		prr.Spec.Resources[path] = resources[path]
	}
	return nil
}

func MatchesWithRef(ref api.FullObjectRef) func(*fn.KubeObject) bool {
	return func(o *fn.KubeObject) bool {
		id := o.GetId()
		if ref.Group != nil && ref.Group != &id.Group {
			return false
		}
		if ref.Version != nil && ref.Version != &id.Version {
			return false
		}
		if ref.Kind != nil && ref.Kind != &id.Kind {
			return false
		}
		if ref.Namespace != nil && ref.Namespace != &id.Namespace {
			return false
		}
		if ref.Name != nil && ref.Name != &id.Name {
			return false
		}
		return true
	}
}

func ApplyDefaults(value, defaults api.FullObjectRef) {
	if value.Group == nil {
		value.Group = defaults.Group
	}
	if value.Version == nil {
		value.Version = defaults.Version
	}
	if value.Kind == nil {
		value.Kind = defaults.Kind
	}
	if value.Namespace == nil {
		value.Namespace = defaults.Namespace
	}
	if value.Name == nil {
		value.Name = defaults.Name
	}
}
