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

	porchapi "github.com/nephio-project/porch/api/porch/v1alpha1"
	configapi "github.com/nephio-project/porch/api/porchconfig/v1alpha1"
	api "github.com/nephio-project/porch/ng/api/v1alpha1"

	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// SetupWithManager sets up the controller with the Manager.
func (r *PackageVariantReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := api.AddToScheme(mgr.GetScheme()); err != nil {
		return err
	}
	if err := porchapi.AddToScheme(mgr.GetScheme()); err != nil {
		return err
	}
	if err := configapi.AddToScheme(mgr.GetScheme()); err != nil {
		return err
	}

	r.Client = mgr.GetClient()

	//TODO: establish watches on resource types injected in all the Package Revisions
	//      we own, and use those to generate requests
	return ctrl.NewControllerManagedBy(mgr).
		For(&api.PackageVariant{}).
		Owns(&porchapi.PackageRevision{}).
		Watches(&porchapi.PackageRevision{}, handler.EnqueueRequestsFromMapFunc(mapObjectsToRequests(r.Client))).
		Complete(r)
}

func isPrAndPvRelated(pr *porchapi.PackageRevision, pv *api.PackageVariant) bool {
	if pv.Spec.Upstream.Repo == pr.Spec.RepositoryName &&
		pv.Spec.Upstream.Package == pr.Spec.PackageName &&
		pv.Spec.Upstream.Revision == pr.Spec.Revision {
		return true
	}
	if pv.Spec.Downstream.Repo == pr.Spec.RepositoryName &&
		pv.Spec.Downstream.Package == pr.Spec.PackageName {
		return true
	}
	for _, mutation := range pv.Spec.Mutations {
		switch mutation.Type {
		case api.MutationTypeInjectLatestPackageRevision:
			if pr.Spec.RepositoryName == mutation.InjectLatestPackageRevision.Repo &&
				pr.Spec.PackageName == mutation.InjectLatestPackageRevision.Package {
				return true
			}
		}
	}
	return false
}

func mapObjectsToRequests(mgrClient client.Reader) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		pr := obj.(*porchapi.PackageRevision)

		pvs := &api.PackageVariantList{}
		err := mgrClient.List(ctx, pvs, &client.ListOptions{Namespace: obj.GetNamespace()})
		if err != nil {
			return []reconcile.Request{}
		}

		requests := make([]reconcile.Request, 0, len(pvs.Items))
		for _, pv := range pvs.Items {
			if isPrAndPvRelated(pr, &pv) {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      pv.GetName(),
						Namespace: pv.GetNamespace(),
					},
				})
			}
		}
		return requests
	}
}
