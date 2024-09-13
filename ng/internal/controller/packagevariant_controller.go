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
	"reflect"
	"strconv"
	"strings"

	porchapi "github.com/nephio-project/porch/api/porch/v1alpha1"
	api "github.com/nephio-project/porch/ng/api/v1alpha1"
	"github.com/nephio-project/porch/ng/internal/utils"

	kptfilev1 "github.com/nephio-project/porch/pkg/kpt/api/kptfile/v1"
	"github.com/nephio-project/porch/pkg/kpt/kptfileutil"

	"github.com/GoogleContainerTools/kpt-functions-sdk/go/fn"
	"golang.org/x/mod/semver"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// PackageVariantReconciler reconciles a PackageVariant object
type PackageVariantReconciler struct {
	client.Client
}

const (
	fieldOwner          = "ng-packagevariant" // field owner for server-side applies
	workspaceNamePrefix = "packagevariant-"
)

//go:generate go run sigs.k8s.io/controller-tools/cmd/controller-gen@v0.16.1 rbac:headerFile=../../../../../scripts/boilerplate.yaml.txt,roleName=porch-controllers-packagevariants webhook paths="." output:rbac:artifacts:config=../../../config/rbac

//+kubebuilder:rbac:groups=config.porch.kpt.dev,resources=packagevariants,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=config.porch.kpt.dev,resources=packagevariants/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=config.porch.kpt.dev,resources=packagevariants/finalizers,verbs=update
//+kubebuilder:rbac:groups=porch.kpt.dev,resources=packagerevisions,verbs=create;delete;get;list;patch;update;watch
//+kubebuilder:rbac:groups=porch.kpt.dev,resources=packagerevisionresources,verbs=create;delete;get;list;patch;update;watch
//+kubebuilder:rbac:groups=config.porch.kpt.dev,resources=repositories,verbs=get;list;watch

// Reconcile implements the main kubernetes reconciliation loop.
func (r *PackageVariantReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	pv, prList, err := r.init(ctx, req)
	if err != nil {
		return ctrl.Result{}, err
	}
	if pv == nil {
		// maybe the pv was deleted
		return ctrl.Result{}, nil
	}
	l := log.FromContext(ctx)
	l.Info("reconciliation called")
	ctx = utils.WithPackageRevisions(ctx, utils.PackageRevisions(prList.Items))

	defer func() {
		statusErr := r.UpdateStatus(ctx, pv)
		if statusErr != nil {
			if err == nil {
				err = fmt.Errorf("couldn't update status: %w", statusErr)
			} else {
				err = fmt.Errorf("couldn't update status because %q;\nwhile processing this error: %w", statusErr, err)
			}
		}
	}()

	if !pv.ObjectMeta.DeletionTimestamp.IsZero() {
		// This object is being deleted, so we need to make sure the packagerevisions owned by this object
		// are deleted. Normally, garbage collection can handle this, but we have a special case here because
		// (a) we cannot delete published packagerevisions and instead have to propose deletion of them
		// (b) we may want to orphan packagerevisions instead of deleting them.
		for _, pr := range prList.Items {
			if r.hasOurOwnerReference(pv, pr.OwnerReferences) {
				r.deleteOrOrphan(ctx, &pr, pv)
				if pr.Spec.Lifecycle == porchapi.PackageRevisionLifecycleDeletionProposed {
					// We need to orphan this package revision; otherwise it will automatically
					// get deleted after its parent PackageVariant object is deleted.
					r.orphanPackageRevision(ctx, &pr, pv)
				}
			}
		}
		// Remove our finalizer from the list and update it.
		if controllerutil.RemoveFinalizer(pv, api.Finalizer) {
			if err := r.Update(ctx, pv); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to delete finalizer: %w", err)
			}
		}
		return ctrl.Result{}, nil
	}

	// the object is not being deleted, so let's ensure that our finalizer is here
	if controllerutil.AddFinalizer(pv, api.Finalizer) {
		if err := r.Update(ctx, pv); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	if errs := validatePackageVariant(pv); len(errs) > 0 {
		setValidConditionToFalse(pv, combineErrors(errs))
		// do not requeue; failed validation requires a PV change
		return ctrl.Result{}, nil
	}
	upstream, err := r.getUpstreamPR(&pv.Spec.Upstream, prList)
	if err != nil {
		setValidConditionToFalse(pv, err.Error())
		// requeue, as the upstream may appear
		return ctrl.Result{}, err
	}
	meta.SetStatusCondition(&pv.Status.Conditions, metav1.Condition{
		Type:    api.ConditionTypeValid,
		Status:  "True",
		Reason:  "Valid",
		Message: "all validation checks passed",
	})

	_, err = r.ensurePackageVariant(ctx, pv, upstream)
	if err != nil {
		meta.SetStatusCondition(&pv.Status.Conditions, metav1.Condition{
			Type:    api.ConditionTypeReady,
			Status:  "False",
			Reason:  "Error",
			Message: err.Error(),
		})
		// requeue; it may be an intermittent error
		return ctrl.Result{}, err
	}
	meta.SetStatusCondition(&pv.Status.Conditions, metav1.Condition{
		Type:    api.ConditionTypeReady,
		Status:  "True",
		Reason:  "NoErrors",
		Message: "successfully ensured downstream package variant",
	})
	return ctrl.Result{}, nil
}

func (r *PackageVariantReconciler) init(ctx context.Context,
	req ctrl.Request) (*api.PackageVariant, *porchapi.PackageRevisionList, error) {
	var pv api.PackageVariant
	if err := r.Client.Get(ctx, req.NamespacedName, &pv); err != nil {
		return nil, nil, client.IgnoreNotFound(err)
	}

	var prList porchapi.PackageRevisionList
	if err := r.Client.List(ctx, &prList, client.InNamespace(pv.Namespace)); err != nil {
		return nil, nil, err
	}

	return &pv, &prList, nil
}

func (r *PackageVariantReconciler) UpdateStatus(ctx context.Context, pv *api.PackageVariant) error {
	// NOTE: using server-side apply here would automatically trigger a new Reconcile call (and then another, ...)
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Update the status of the PackageVariant object
		// get the PV again, since we may have modified it (e.g. by adding a finalizer)
		var newPV api.PackageVariant
		err := r.Client.Get(ctx, client.ObjectKeyFromObject(pv), &newPV)
		if err != nil {
			return client.IgnoreNotFound(err)
		}
		if reflect.DeepEqual(newPV.Status, pv.Status) {
			return nil
		}
		newPV.Status = pv.Status
		return r.Client.Status().Update(ctx, &newPV)
	})
}

func validatePackageVariant(pv *api.PackageVariant) []string {
	var allErrs []string
	if pv.Spec.AdoptionPolicy == "" {
		pv.Spec.AdoptionPolicy = api.AdoptionPolicyAdoptNone
	}
	if pv.Spec.DeletionPolicy == "" {
		pv.Spec.DeletionPolicy = api.DeletionPolicyDelete
	}
	if pv.Spec.AdoptionPolicy != api.AdoptionPolicyAdoptNone && pv.Spec.AdoptionPolicy != api.AdoptionPolicyAdoptExisting {
		allErrs = append(allErrs, fmt.Sprintf("spec.adoptionPolicy field can only be %q or %q",
			api.AdoptionPolicyAdoptNone, api.AdoptionPolicyAdoptExisting))
	}
	if pv.Spec.DeletionPolicy != api.DeletionPolicyOrphan && pv.Spec.DeletionPolicy != api.DeletionPolicyDelete {
		allErrs = append(allErrs, fmt.Sprintf("spec.deletionPolicy can only be %q or %q",
			api.DeletionPolicyOrphan, api.DeletionPolicyDelete))
	}
	return allErrs
}

func combineErrors(errs []string) string {
	var errMsgs []string
	for _, e := range errs {
		if e != "" {
			errMsgs = append(errMsgs, e)
		}
	}
	return strings.Join(errMsgs, "; ")
}

func (r *PackageVariantReconciler) getUpstreamPR(upstream *api.PackageRevisionRef,
	prList *porchapi.PackageRevisionList) (*porchapi.PackageRevision, error) {
	for _, pr := range prList.Items {
		if pr.Spec.RepositoryName == upstream.Repo &&
			pr.Spec.PackageName == upstream.Package &&
			pr.Spec.Revision == upstream.Revision {
			return &pr, nil
		}
	}
	return nil, fmt.Errorf("could not find upstream package revision '%s/%s' in repo '%s'",
		upstream.Package, upstream.Revision, upstream.Repo)
}

func setValidConditionToFalse(pv *api.PackageVariant, message string) {
	meta.SetStatusCondition(&pv.Status.Conditions, metav1.Condition{
		Type:    api.ConditionTypeValid,
		Status:  "False",
		Reason:  "ValidationError",
		Message: message,
	})
	meta.SetStatusCondition(&pv.Status.Conditions, metav1.Condition{
		Type:    api.ConditionTypeReady,
		Status:  "False",
		Reason:  "Error",
		Message: "invalid packagevariant object",
	})
}

// ensurePackageVariant needs to:
//   - Check if the downstream package revision already exists. If not, create it.
//   - If it does already exist, we need to make sure it is up-to-date. If there are
//     downstream package drafts, we look at all drafts. Otherwise, we look at the latest
//     published downstream package revision.
//   - Compare pv.Spec.Upstream.Revision to the revision number that the downstream
//     package is based on. If it is different, we need to do an update (could be an upgrade
//     or a downgrade).
//   - Delete or orphan other package revisions owned by this controller that are no
//     longer needed.
func (r *PackageVariantReconciler) ensurePackageVariant(
	ctx context.Context,
	pv *api.PackageVariant,
	upstreamPR *porchapi.PackageRevision,
) (
	[]*porchapi.PackageRevision,
	error,
) {
	// find or create downstream package revisions
	downstreams, err := r.getDownstreamPRs(ctx, pv, client.ObjectKeyFromObject(upstreamPR))
	if err != nil {
		return nil, err
	}

	// ensure that all downstream package revisions are up-to-date
	var errors []string
	statuses := make([]*api.DownstreamTarget, len(downstreams))
	for i, downstreamPR := range downstreams {
		downstreams[i], statuses[i], err = r.ensureDownstreamTarget(ctx, pv, upstreamPR, downstreamPR)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %s", downstreams[i].Name, err.Error()))
		}
	}
	pv.Status.DownstreamTargets = make([]api.DownstreamTarget, len(statuses))
	for i, status := range statuses {
		pv.Status.DownstreamTargets[i] = *status
	}
	if len(errors) > 0 {
		return nil, fmt.Errorf("failed to ensure some downstream package revisions:\n  - %v", strings.Join(errors, "\n  - "))
	}
	return downstreams, nil
}

// ensureDownstreamTarget needs to:
//   - Compare pv.Spec.Upstream.Revision to the revision number that the downstream
//     package is based on. If it is different, we need to do an update (could be an upgrade
//     or a downgrade).
//   - Make sure that all of pv's mutations are applied to the downstream package revision.
//   - If the downstream package revision is published, but its contents needed to be changed,
//     then copy it into a new draft revision and apply the changes to that draft. In this case
//     the new draft revision will be returned.
func (r *PackageVariantReconciler) ensureDownstreamTarget(
	ctx context.Context,
	pv *api.PackageVariant,
	upstreamPR *porchapi.PackageRevision,
	downstreamPR *porchapi.PackageRevision,
) (
	*porchapi.PackageRevision,
	*api.DownstreamTarget,
	error,
) {
	var err error
	status := downstreamStatusOf(downstreamPR, pv)

	if downstreamPR.Spec.Lifecycle == porchapi.PackageRevisionLifecycleDeletionProposed {
		// We proposed this package revision for deletion in the past, but now it
		// matches our target, so we no longer want it to be deleted.
		downstreamPR.Spec.Lifecycle = porchapi.PackageRevisionLifecyclePublished
		// We update this now, because later we may use a Porch call to clone or update
		// and we want to make sure the server is in sync with us
		if err := r.Client.Update(ctx, downstreamPR); err != nil {
			klog.Errorf("error updating package revision lifecycle: %v", err)
			return nil, nil, err
		}
	}

	// see if the package needs updating due to an upstream change
	if !r.isUpToDate(pv, downstreamPR) {
		// we need to copy a published package to a new draft before updating
		if porchapi.LifecycleIsPublished(downstreamPR.Spec.Lifecycle) {
			downstreamPR, err = r.copyPublished(ctx, downstreamPR, pv)
			if err != nil {
				return nil, nil, err
			}
		}
		downstreamPR, err = r.updateDraft(ctx, downstreamPR, upstreamPR)
		if err != nil {
			return nil, nil, err
		}
		klog.Infoln(fmt.Sprintf("package variant %q updated package revision %q to upstream revision %s", pv.Name, downstreamPR.Name, upstreamPR.Spec.Revision))
	}

	// finally, see if any other changes are needed to the resources
	prr, changed, err := r.calculateDraftResources(ctx, pv, downstreamPR, status)
	if err != nil {
		return nil, nil, err
	}

	// if there are changes, save them
	if changed {
		// if no pkg update was needed, we may still be a published package
		// so, clone to a new Draft if that's the case
		if porchapi.LifecycleIsPublished(downstreamPR.Spec.Lifecycle) {

			downstreamPR, err = r.copyPublished(ctx, downstreamPR, pv)
			if err != nil {
				return nil, nil, err
			}
			// recalculate from the new Draft
			prr, _, err = r.calculateDraftResources(ctx, pv, downstreamPR, status)
			if err != nil {
				return nil, nil, err
			}

		}
		// Save the updated PackageRevisionResources
		if err := r.updatePackageResources(ctx, prr, status); err != nil {
			return nil, nil, err
		}
	}
	status.Name = downstreamPR.Name
	return downstreamPR, status, nil
}

func downstreamStatusOf(downstreamPR *porchapi.PackageRevision, pv *api.PackageVariant) *api.DownstreamTarget {
	status := api.DownstreamTarget{
		Name: downstreamPR.Name,
	}
	for _, s := range pv.Status.DownstreamTargets {
		if s.Name == downstreamPR.Name {
			status.RenderStatus = s.RenderStatus
		}
	}
	return &status
}

// If there are any drafts that are owned by us and match the target package revision, return them all.
// If there are no drafts, return the latest published package revision owned by us.
// If there are no drafts or published package revisions, create a new package revision.
// getDownstreamPRs also deletes or orphans package revisions owned by `pv`, but doesn't match the current downstream target.
func (r *PackageVariantReconciler) getDownstreamPRs(
	ctx context.Context,
	pv *api.PackageVariant,
	upstreamPrKey client.ObjectKey,
) (
	[]*porchapi.PackageRevision,
	error,
) {
	l := log.FromContext(ctx)

	var drafts []*porchapi.PackageRevision
	var latestPublished *porchapi.PackageRevision
	// the first package revision number that porch assigns is "v1",
	// so use v0 as a placeholder for comparison
	latestVersion := "v0"

	for _, pr := range utils.PackageRevisionsFromContextOrDie(ctx) {
		// TODO: When we have a way to find the upstream packagerevision without
		//   listing all packagerevisions, we should add a label to the resources we
		//   own so that we can fetch only those packagerevisions. (A caveat here is
		//   that if the adoptionPolicy is set to adoptExisting, we will still have
		//   to fetch all the packagerevisions so that we can determine which ones
		//   we need to adopt. A mechanism to filter packagerevisions by repo/package
		//   would be helpful for that.)
		owned := r.hasOurOwnerReference(pv, pr.ObjectMeta.OwnerReferences)
		if !owned && pv.Spec.AdoptionPolicy != api.AdoptionPolicyAdoptExisting {
			// this package revision doesn't belong to us
			continue
		}

		// check that the repo and package name match
		if pr.Spec.RepositoryName != pv.Spec.Downstream.Repo ||
			pr.Spec.PackageName != pv.Spec.Downstream.Package {
			if owned {
				// We own this package, but it isn't a match for our downstream target,
				// which means that we created it but no longer need it.
				r.deleteOrOrphan(ctx, &pr, pv)
			}
			continue
		}

		// this package matches, check if we need to adopt it
		if !owned && pv.Spec.AdoptionPolicy == api.AdoptionPolicyAdoptExisting {
			klog.Infoln(fmt.Sprintf("package variant %q is adopting package revision %q", pv.Name, pr.Name))
			if err := r.adoptPackageRevision(ctx, &pr, pv); err != nil {
				klog.Errorf("error adopting package revision: %v", err)
			}
		}

		if porchapi.LifecycleIsPublished(pr.Spec.Lifecycle) {
			latestPublished, latestVersion = compare(&pr, latestPublished, latestVersion)
		} else {
			drafts = append(drafts, pr.DeepCopy())
		}
	}

	if len(drafts) > 0 {
		return drafts, nil
	}
	if latestPublished != nil {
		return []*porchapi.PackageRevision{latestPublished}, nil
	}

	// No downstream package created by this controller exists. Create one.
	readinessGates := pv.Spec.ReadinessGates
	newPR := &porchapi.PackageRevision{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PackageRevision",
			APIVersion: porchapi.SchemeGroupVersion.Identifier(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       pv.Namespace,
			OwnerReferences: []metav1.OwnerReference{constructOwnerReference(pv)},
			Labels:          pv.Spec.Labels,
			Annotations:     pv.Spec.Annotations,
		},
		Spec: porchapi.PackageRevisionSpec{
			ReadinessGates: readinessGates,
			PackageName:    pv.Spec.Downstream.Package,
			RepositoryName: pv.Spec.Downstream.Repo,
			WorkspaceName:  newWorkspaceName(ctx, pv.Spec.Downstream.Package, pv.Spec.Downstream.Repo),
			Tasks: []porchapi.Task{
				{
					Type: porchapi.TaskTypeClone,
					Clone: &porchapi.PackageCloneTaskSpec{
						Upstream: porchapi.UpstreamPackage{
							UpstreamRef: &porchapi.PackageRevisionRef{
								Name: upstreamPrKey.Name,
							},
						},
					},
				},
			},
		},
	}

	if err := r.Client.Create(ctx, newPR); err != nil {
		return nil, err
	}
	l.Info(fmt.Sprintf("new package revision %q was created", newPR.Name))

	return []*porchapi.PackageRevision{newPR}, nil
}

func compare(pr, latestPublished *porchapi.PackageRevision, latestVersion string) (*porchapi.PackageRevision, string) {
	switch cmp := semver.Compare(pr.Spec.Revision, latestVersion); {
	case cmp == 0:
		// Same revision.
	case cmp < 0:
		// current < latest; no change
	case cmp > 0:
		// current > latest; update latest
		latestVersion = pr.Spec.Revision
		latestPublished = pr.DeepCopy()
	}
	return latestPublished, latestVersion
}

// check that the downstream package was created by this PackageVariant object
func (r *PackageVariantReconciler) hasOurOwnerReference(pv *api.PackageVariant, owners []metav1.OwnerReference) bool {
	for _, owner := range owners {
		if owner.UID == pv.UID {
			return true
		}
	}
	return false
}

func (r *PackageVariantReconciler) deleteOrOrphan(ctx context.Context,
	pr *porchapi.PackageRevision,
	pv *api.PackageVariant) {
	switch pv.Spec.DeletionPolicy {
	case "", api.DeletionPolicyDelete:
		klog.Infoln(fmt.Sprintf("package variant %q is deleting package revision %q", pv.Name, pr.Name))
		r.deletePackageRevision(ctx, pr)
	case api.DeletionPolicyOrphan:
		klog.Infoln(fmt.Sprintf("package variant %q is orphaning package revision %q", pv.Name, pr.Name))
		r.orphanPackageRevision(ctx, pr, pv)
	default:
		// this should never happen, because the pv should already be validated beforehand
		klog.Errorf("invalid deletion policy %s", pv.Spec.DeletionPolicy)
	}
}

func (r *PackageVariantReconciler) orphanPackageRevision(ctx context.Context,
	pr *porchapi.PackageRevision,
	pv *api.PackageVariant) {
	pr.ObjectMeta.OwnerReferences = removeOwnerRefByUID(pr.OwnerReferences, pv.UID)
	if err := r.Client.Update(ctx, pr); err != nil {
		klog.Errorf("error orphaning package revision: %v", err)
	}
}

func removeOwnerRefByUID(ownerRefs []metav1.OwnerReference,
	ownerToRemove types.UID) []metav1.OwnerReference {
	var result []metav1.OwnerReference
	for _, owner := range ownerRefs {
		if owner.UID != ownerToRemove {
			result = append(result, owner)
		}
	}
	return result
}

// When we adopt a package revision, we need to make sure that the package revision
// has our owner reference and also the labels/annotations specified in pv.Spec.
func (r *PackageVariantReconciler) adoptPackageRevision(ctx context.Context,
	pr *porchapi.PackageRevision,
	pv *api.PackageVariant) error {
	pr.ObjectMeta.OwnerReferences = append(pr.OwnerReferences, constructOwnerReference(pv))
	if len(pv.Spec.Labels) > 0 && pr.ObjectMeta.Labels == nil {
		pr.ObjectMeta.Labels = make(map[string]string)
	}
	for k, v := range pv.Spec.Labels {
		pr.ObjectMeta.Labels[k] = v
	}
	if len(pv.Spec.Annotations) > 0 && pr.ObjectMeta.Annotations == nil {
		pr.ObjectMeta.Annotations = make(map[string]string)
	}
	for k, v := range pv.Spec.Annotations {
		pr.ObjectMeta.Annotations[k] = v
	}
	return r.Client.Update(ctx, pr)
}

func (r *PackageVariantReconciler) deletePackageRevision(ctx context.Context, pr *porchapi.PackageRevision) {
	switch pr.Spec.Lifecycle {
	case "", porchapi.PackageRevisionLifecycleDraft, porchapi.PackageRevisionLifecycleProposed:
		if err := r.Client.Delete(ctx, pr); err != nil {
			klog.Errorf("error deleting package revision: %v", err)
		}
	case porchapi.PackageRevisionLifecyclePublished:
		pr.Spec.Lifecycle = porchapi.PackageRevisionLifecycleDeletionProposed
		if err := r.Client.Update(ctx, pr); err != nil {
			klog.Errorf("error proposing deletion for published package revision: %v", err)
		}
	case porchapi.PackageRevisionLifecycleDeletionProposed:
		// we don't have to do anything
	default:
		// if this ever happens, there's something going wrong with porch
		klog.Errorf("invalid lifecycle value for package revision %s: %s", pr.Name, pr.Spec.Lifecycle)
	}
}

// determine if the downstream PR needs to be updated
func (r *PackageVariantReconciler) isUpToDate(pv *api.PackageVariant, downstream *porchapi.PackageRevision) bool {
	upstreamLock := downstream.Status.UpstreamLock
	lastIndex := strings.LastIndex(upstreamLock.Git.Ref, "/")
	if strings.HasPrefix(upstreamLock.Git.Ref, "drafts") {
		// The current upstream is a draft, and the target upstream
		// will always be a published revision, so we will need to do an update.
		return false
	}
	currentUpstreamRevision := upstreamLock.Git.Ref[lastIndex+1:]
	return currentUpstreamRevision == pv.Spec.Upstream.Revision
}

func (r *PackageVariantReconciler) copyPublished(
	ctx context.Context,
	source *porchapi.PackageRevision,
	pv *api.PackageVariant,
) (
	*porchapi.PackageRevision,
	error,
) {
	l := log.FromContext(ctx)
	l.Info(fmt.Sprintf("Must mutate published package revision %q, creating new draft", source.Name))
	newPR := &porchapi.PackageRevision{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PackageRevision",
			APIVersion: porchapi.SchemeGroupVersion.Identifier(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       source.Namespace,
			OwnerReferences: []metav1.OwnerReference{constructOwnerReference(pv)},
			Labels:          pv.Spec.Labels,
			Annotations:     pv.Spec.Annotations,
		},
		Spec: source.Spec,
	}

	newPR.Spec.Revision = ""
	newPR.Spec.WorkspaceName = newWorkspaceName(ctx, newPR.Spec.PackageName, newPR.Spec.RepositoryName)
	newPR.Spec.Lifecycle = porchapi.PackageRevisionLifecycleDraft

	if err := r.Client.Create(ctx, newPR); err != nil {
		l.Error(err, fmt.Sprintf("failed to copy %q", source.Name))
		return nil, err
	}
	l.Info(fmt.Sprintf("created package revision %q based on %q", newPR.Name, source.Name))
	return newPR, nil
}

func newWorkspaceName(
	ctx context.Context,
	packageName string,
	repo string,
) porchapi.WorkspaceName {
	wsNum := 0
	for _, pr := range utils.PackageRevisionsFromContextOrDie(ctx) {
		if pr.Spec.PackageName != packageName || pr.Spec.RepositoryName != repo {
			continue
		}
		oldWorkspaceName := string(pr.Spec.WorkspaceName)
		if !strings.HasPrefix(oldWorkspaceName, workspaceNamePrefix) {
			continue
		}
		wsNumStr := strings.TrimPrefix(oldWorkspaceName, workspaceNamePrefix)
		newWsNum, _ := strconv.Atoi(wsNumStr)
		if newWsNum > wsNum {
			wsNum = newWsNum
		}
	}
	wsNum++
	return porchapi.WorkspaceName(fmt.Sprintf(workspaceNamePrefix+"%d", wsNum))
}

func constructOwnerReference(pv *api.PackageVariant) metav1.OwnerReference {
	tr := true
	return metav1.OwnerReference{
		APIVersion:         pv.APIVersion,
		Kind:               pv.Kind,
		Name:               pv.Name,
		UID:                pv.UID,
		Controller:         &tr,
		BlockOwnerDeletion: nil,
	}
}

func (r *PackageVariantReconciler) updateDraft(ctx context.Context,
	draft *porchapi.PackageRevision,
	newUpstreamPR *porchapi.PackageRevision) (*porchapi.PackageRevision, error) {

	draft = draft.DeepCopy()
	tasks := draft.Spec.Tasks

	updateTask := porchapi.Task{
		Type: porchapi.TaskTypeUpdate,
		Update: &porchapi.PackageUpdateTaskSpec{
			Upstream: tasks[0].Clone.Upstream,
		},
	}
	updateTask.Update.Upstream.UpstreamRef.Name = newUpstreamPR.Name
	draft.Spec.Tasks = append(tasks, updateTask)

	err := r.Client.Update(ctx, draft)
	if err != nil {
		return nil, err
	}
	return draft, nil
}

func (r *PackageVariantReconciler) calculateDraftResources(
	ctx context.Context,
	pv *api.PackageVariant,
	draft *porchapi.PackageRevision,
	status *api.DownstreamTarget,
) (
	prr *porchapi.PackageRevisionResources,
	changed bool,
	err error,
) {
	l := log.FromContext(ctx)
	// Load the PackageRevisionResources
	prr = &porchapi.PackageRevisionResources{}
	prrKey := types.NamespacedName{Name: draft.GetName(), Namespace: draft.GetNamespace()}
	if err := r.Client.Get(ctx, prrKey, prr); err != nil {
		return nil, false, err
	}

	// Check if it's a valid PRR
	if prr.Spec.Resources == nil {
		return nil, false, fmt.Errorf("nil resources found for PackageRevisionResources '%s/%s'", prr.Namespace, prr.Name)
	}

	origResources := make(map[string]string, len(prr.Spec.Resources))
	for k, v := range prr.Spec.Resources {
		origResources[k] = v
	}

	if err := ensureConfigInjection(ctx, r.Client, pv, prr); err != nil {
		return nil, false, err
	}

	if err = ensureMutations(ctx, r.Client, pv, prr, status); err != nil {
		return nil, false, err
	}

	if len(prr.Spec.Resources) != len(origResources) {
		// files were added or deleted
		l.Info(fmt.Sprintf("PackageRevision %q changed: number of files: %d original files, %d new files", prr.Name, len(origResources), len(prr.Spec.Resources)))
		return prr, true, nil
	}

	for file, oldContent := range origResources {
		newContent, ok := prr.Spec.Resources[file]
		if !ok {
			l.Info(fmt.Sprintf("PackageRevision %q changed: file %q was deleted", prr.Name, file))
			return prr, true, nil
		}

		if newContent != oldContent {
			// HACK ALERT - TODO(jbelamaric): Fix this
			// Currently nephio controllers and package variant controller are rendering Kptfiles slightly differently in YAML
			// not sure why, need to investigate more. It may be due to different versions of kyaml. So, here, just for Kptfiles,
			// we will parse and compare semantically.
			//
			if file == "Kptfile" && kptfilesEqual(oldContent, newContent) {
				l.Info(fmt.Sprintf("PackageRevision %q changed: Kptfiles differ, but not semantically", prr.Name))
				continue
			}

			// a file was changed
			l.Info(fmt.Sprintf("PackageRevision %q changed: contents of file %q changed", prr.Name, file))
			return prr, true, nil
		}
	}

	// all files in orig are in new, no new files, and all contents match
	// so no change
	l.Info(fmt.Sprintf("PackageRevision %q: resources unchanged", prr.Name))
	return prr, false, nil
}

func parseKptfile(kf string) (*kptfilev1.KptFile, error) {
	ko, err := fn.ParseKubeObject([]byte(kf))
	if err != nil {
		return nil, err
	}
	var kptfile kptfilev1.KptFile
	err = ko.As(&kptfile)
	if err != nil {
		return nil, err
	}

	return &kptfile, nil
}

func kptfilesEqual(a, b string) bool {
	akf, err := parseKptfile(a)
	if err != nil {
		return false
	}

	bkf, err := parseKptfile(b)
	if err != nil {
		return false
	}

	equal, err := kptfileutil.Equal(akf, bkf)
	if err != nil {
		return false
	}
	return equal
}

func getFileKubeObject(prr *porchapi.PackageRevisionResources, file, kind, name string) (*fn.KubeObject, error) {
	if prr.Spec.Resources == nil {
		return nil, fmt.Errorf("nil resources found for PackageRevisionResources '%s/%s'", prr.Namespace, prr.Name)
	}

	if _, ok := prr.Spec.Resources[file]; !ok {
		return nil, fmt.Errorf("%q not found in PackageRevisionResources '%s/%s'", file, prr.Namespace, prr.Name)
	}

	ko, err := fn.ParseKubeObject([]byte(prr.Spec.Resources[file]))
	if err != nil {
		return nil, fmt.Errorf("failed to parse %q of PackageRevisionResources %s/%s: %w", file, prr.Namespace, prr.Name, err)
	}
	if kind != "" && ko.GetKind() != kind {
		return nil, fmt.Errorf("%q does not contain kind %q in PackageRevisionResources '%s/%s'", file, kind, prr.Namespace, prr.Name)
	}
	if name != "" && ko.GetName() != name {
		return nil, fmt.Errorf("%q does not contain resource named %q in PackageRevisionResources '%s/%s'", file, name, prr.Namespace, prr.Name)
	}

	return ko, nil
}

func (r *PackageVariantReconciler) updatePackageResources(
	ctx context.Context,
	prr *porchapi.PackageRevisionResources,
	status *api.DownstreamTarget,
) error {
	if err := r.Update(ctx, prr); err != nil {
		return err
	}
	status.RenderStatus = prr.Status.RenderStatus
	return nil
}
