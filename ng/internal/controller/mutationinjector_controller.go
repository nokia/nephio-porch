/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
	api "github.com/nephio-project/porch/ng/api/v1alpha1"
	"github.com/nephio-project/porch/ng/utils"
)

// MutationInjectorReconciler reconciles a MutationInjector object
type MutationInjectorReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	log    logr.Logger
}

// SetupWithManager sets up the controller with the Manager.
func (r *MutationInjectorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&api.MutationInjector{}).
		Watches(&api.PackageVariant{}, handler.EnqueueRequestsFromMapFunc(
			func(ctx context.Context, obj client.Object) []reconcile.Request {
				pv := obj.(*api.PackageVariant)
				pvLabels := labels.Set(pv.Spec.Labels)
				// find all MutationInjectors that target this PackageVariant
				// and return a reconcile request for each

				var injectors api.MutationInjectorList
				err := mgr.GetClient().List(ctx, &injectors, client.InNamespace(pv.Namespace))
				if err != nil {
					return nil
				}
				var results []reconcile.Request
				for _, injector := range injectors.Items {
					targetSelector, err := metav1.LabelSelectorAsSelector(injector.Spec.PackageVariantSelector)
					if err != nil {
						continue
					}
					if targetSelector.Matches(pvLabels) {
						results = append(results, reconcile.Request{
							NamespacedName: client.ObjectKeyFromObject(&injector),
						})
					}
				}
				return results
			}),
		).
		Complete(r)
}

// +kubebuilder:rbac:groups=ng.porch.kpt.dev,resources=mutationinjectors,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ng.porch.kpt.dev,resources=mutationinjectors/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ng.porch.kpt.dev,resources=mutationinjectors/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *MutationInjectorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var injector *api.MutationInjector
	var err error
	ctx, injector, err = r.init(ctx, req)
	if err != nil {
		return ctrl.Result{}, err
	}
	if injector == nil {
		return ctrl.Result{}, nil
	}
	l := log.FromContext(ctx)
	l.Info("reconciliation called")

	owned, targets, err := r.getOwnedAndTargetPackageVariants(ctx, injector)
	if err != nil {
		return ctrl.Result{}, err
	}

	if !injector.DeletionTimestamp.IsZero() {
		// This object is being deleted, so we need to make sure that the mutations injected by us
		// and our ownerReference are removed from the owned PackageVariants.
		errlist := utils.NewErrorCollector("")
		for _, pv := range owned {
			errlist.AddWithPrefix(r.applyMutations(ctx, pv, nil), pv.Name)
		}
		if !errlist.IsEmpty() {
			return ctrl.Result{}, errlist.Combined("failed to orphan owned PackageVariants, while deleting MutationInjection:")
		}

		// Remove our finalizer from the list and update it.
		if controllerutil.RemoveFinalizer(injector, api.Finalizer) {
			if err := r.Update(ctx, injector); client.IgnoreNotFound(err) != nil {
				return ctrl.Result{}, fmt.Errorf("failed to delete finalizer: %w", err)
			}
		}
		return ctrl.Result{}, nil
	}

	// the object is not being deleted, so let's ensure that our finalizer is here
	if controllerutil.AddFinalizer(injector, api.Finalizer) {
		if err := r.Update(ctx, injector); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	// TODO: validate the spec of the MutationInjector

	// orphan all owned PackageVariants that no longer matches our target selector
	errlist := utils.NewErrorCollector("")
	for _, owned := range owned {
		isTarget := false
		for _, target := range targets {
			if target.UID == owned.UID {
				isTarget = true
				break
			}
		}
		if !isTarget {
			errlist.AddWithPrefix(r.applyMutations(ctx, owned, nil), owned.Name)
		}
	}
	if !errlist.IsEmpty() {
		return ctrl.Result{}, errlist.Combined("failed to orphan PackageVariants:")
	}

	// rewrite mutation manager fields to point to the reconciled MutationInjector
	for i := range injector.Spec.Mutations {
		injector.Spec.Mutations[i].Manager = fmt.Sprintf("%s/%s", api.MutationInjectorGVK.GroupKind(), req)
	}

	// apply mutations to target PackageVariants (and add an owner reference)
	errlist = utils.NewErrorCollector("")
	for _, target := range targets {
		errlist.AddWithPrefix(r.applyMutations(ctx, target, injector), target.Name)
	}
	if !errlist.IsEmpty() {
		return ctrl.Result{}, errlist.Combined("failed to apply mutations:")
	}
	// TODO: set "Valid" and "Ready" conditions based on the result of the above operations
	return ctrl.Result{}, nil
}

// collect the reconciled object and add session-specific variables to context
func (r *MutationInjectorReconciler) init(
	ctx context.Context,
	req ctrl.Request,
) (
	context.Context,
	*api.MutationInjector,
	error,
) {
	// get reconciled object
	var injector api.MutationInjector
	if err := r.Client.Get(ctx, req.NamespacedName, &injector); err != nil {
		return ctx, nil, client.IgnoreNotFound(err)
	}

	// replace the logger put into ctx by the controller-runtime
	// with one that has less clutter (values) in its log messages
	l := r.log.WithValues(api.MutationInjectorGVK.Kind, req.String())
	ctx = log.IntoContext(ctx, l)

	return ctx, &injector, nil
}

// list of PackageVariants that have `injector` as owner
func (r *MutationInjectorReconciler) getOwnedAndTargetPackageVariants(
	ctx context.Context,
	injector *api.MutationInjector,
) (
	owned []*api.PackageVariant,
	targets []*api.PackageVariant,
	err error,
) {
	selector, err := metav1.LabelSelectorAsSelector(injector.Spec.PackageVariantSelector)
	if err != nil {
		// TODO: this is a validation error: signal this via a "Valid" condition type
		return nil, nil, err
	}

	// get all PackageVariants in the same namespace
	var pvs api.PackageVariantList
	err = r.Client.List(ctx, &pvs, client.InNamespace(injector.Namespace))
	if err != nil {
		return
	}

	for _, pv := range pvs.Items {
		// filter PackageVariants that have the injector as owner
		if utils.IsOwnedBy(&pv, injector) {
			owned = append(owned, &pv)
		}
		// filter PackageVariants that match the selector
		if selector.Matches(labels.Set(pv.Spec.Labels)) {
			targets = append(targets, &pv)
		}
	}
	return
}

// add mutations and an ownerReference to the target PackageVariant
func (r *MutationInjectorReconciler) applyMutations(
	ctx context.Context,
	target *api.PackageVariant,
	injector *api.MutationInjector,
) error {
	var patch *api.PackageVariant
	if injector == nil {
		// remove previously added mutations and ownerReference by an empty patch
		patch = &api.PackageVariant{
			ObjectMeta: metav1.ObjectMeta{
				Name:      target.Name,
				Namespace: target.Namespace,
			},
		}
	} else {
		// add mutations and ownerReference
		patch = &api.PackageVariant{
			ObjectMeta: metav1.ObjectMeta{
				Name:      target.Name,
				Namespace: target.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         api.GroupVersion.String(),
						Kind:               api.MutationInjectorGVK.Kind,
						Name:               injector.Name,
						UID:                injector.UID,
						Controller:         ptr.To(false),
						BlockOwnerDeletion: ptr.To(false),
					},
				},
			},
			Spec: api.PackageVariantSpec{
				Mutations: injector.Spec.Mutations,
			},
		}
	}
	// NOTE: not forcing the apply to detect conflicts
	return r.Patch(ctx, patch, client.Apply, client.FieldOwner("mutation-injector"))
}
