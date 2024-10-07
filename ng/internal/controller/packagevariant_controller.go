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
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	porchapi "github.com/nephio-project/porch/api/porch/v1alpha1"
	api "github.com/nephio-project/porch/ng/api/v1alpha1"
	"github.com/nephio-project/porch/ng/utils"

	kptfilev1 "github.com/nephio-project/porch/pkg/kpt/api/kptfile/v1"
	"github.com/nephio-project/porch/pkg/kpt/kptfileutil"

	"github.com/nephio-project/porch/third_party/GoogleContainerTools/kpt-functions-sdk/go/fn"
	"golang.org/x/mod/semver"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// PackageVariantReconciler reconciles a PackageVariant object
const (
	fieldOwner               = "ng-packagevariant" // field owner for server-side applies
	workspaceNamePrefix      = "packagevariant-"
	ConditionTypeManualEdits = "ManualEditsReady"
)

type PackageVariantReconciler struct {
	client.Client
	log logr.Logger
}

// +kubebuilder:rbac:groups=ng.porch.kpt.dev,resources=packagevariants,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ng.porch.kpt.dev,resources=packagevariants/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ng.porch.kpt.dev,resources=packagevariants/finalizers,verbs=update
// +kubebuilder:rbac:groups=porch.kpt.dev,resources=packagerevisions,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=porch.kpt.dev,resources=packagerevisions/approval,verbs=get;patch;update
// +kubebuilder:rbac:groups=porch.kpt.dev,resources=packagerevisionresources,verbs=create;delete;get;list;patch;update
// +kubebuilder:rbac:groups=config.porch.kpt.dev,resources=repositories,verbs=get;list;watch

// Reconcile implements the main kubernetes reconciliation loop.
func (r *PackageVariantReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	ctx, pv, err := r.init(ctx, req)
	if err != nil {
		return ctrl.Result{}, err
	}
	if pv == nil {
		// pv was deleted
		return ctrl.Result{}, nil
	}
	l := log.FromContext(ctx)
	l.Info("reconciliation called")

	defer func() {
		statusErr := r.UpdateStatus(ctx, pv)
		if statusErr != nil {
			if err == nil {
				err = fmt.Errorf("couldn't update status: %w", statusErr)
			} else {
				err = fmt.Errorf("couldn't update status because: %s;\nwhile processing this error: %w", statusErr, err)
			}
		}
	}()

	if !pv.ObjectMeta.DeletionTimestamp.IsZero() {
		// This object is being deleted, so we need to make sure the packagerevisions owned by this object
		// are deleted. Normally, garbage collection can handle this, but we have a special case here because
		// (a) we cannot delete published packagerevisions and instead have to propose deletion of them
		// (b) we may want to orphan packagerevisions instead of deleting them.
		for _, pr := range utils.PackageRevisionsFromContextOrDie(ctx) {
			if r.hasOurOwnerReference(pv, pr.OwnerReferences) {
				err := r.deleteOrOrphan(ctx, &pr, pv)
				if err != nil {
					setReadyCondition(pv, err)
					return ctrl.Result{}, err
				}
				if pr.Spec.Lifecycle == porchapi.PackageRevisionLifecycleDeletionProposed {
					// We need to orphan this package revision; otherwise it will automatically
					// get deleted after its parent PackageVariant object is deleted.
					err := r.orphanPackageRevision(ctx, &pr, pv)
					if err != nil {
						setReadyCondition(pv, err)
						return ctrl.Result{}, err
					}
				}
			}
		}
		// Remove our finalizer from the list and update it.
		if controllerutil.RemoveFinalizer(pv, api.Finalizer) {
			if err := r.Update(ctx, pv); client.IgnoreNotFound(err) != nil {
				// TODO[kispaljr]: HACK ALERT! this error seems to be recurring, but seems harmless
				if !strings.Contains(err.Error(), "StorageError: invalid object, Code: 4") {
					return ctrl.Result{}, fmt.Errorf("failed to delete finalizer: %w", err)
				}
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

	if err := validatePackageVariant(pv); err != nil {
		setValidCondition(pv, err)
		// do not requeue; failed validation requires a PV change
		return ctrl.Result{}, nil
	}
	upstream, err := r.getUpstreamPR(ctx, pv.Spec.Upstream)
	if err != nil {
		setValidCondition(pv, err)
		l.Info(err.Error() + "... retrying later")
		// requeue, as the upstream may appear
		return ctrl.Result{Requeue: true}, nil
	}
	setValidCondition(pv, nil)

	requeueNeeded, err := r.ensurePackageVariant(ctx, pv, upstream)
	setReadyCondition(pv, err)
	return ctrl.Result{Requeue: requeueNeeded}, err
}

func (r *PackageVariantReconciler) init(
	ctx context.Context,
	req ctrl.Request,
) (
	context.Context,
	*api.PackageVariant,
	error,
) {
	// get reconciled object
	var pv api.PackageVariant
	if err := r.Client.Get(ctx, req.NamespacedName, &pv); err != nil {
		return ctx, nil, client.IgnoreNotFound(err)
	}
	// save list of package revisions in the context
	var prList porchapi.PackageRevisionList
	if err := r.Client.List(ctx, &prList, client.InNamespace(pv.Namespace)); err != nil {
		return ctx, nil, err
	}
	ctx = utils.WithPackageRevisions(ctx, utils.PackageRevisions(prList.Items))

	// replace the logger put into ctx by the controller-runtime, with one that has less clutter in its log messages
	l := r.log.WithValues(api.PackageVariantGVK.Kind, req.String())
	ctx = log.IntoContext(ctx, l)

	return ctx, &pv, nil
}

func (r *PackageVariantReconciler) UpdateStatus(ctx context.Context, pv *api.PackageVariant) error {
	// NOTE: using server-side apply here would automatically trigger a new Reconcile call (and then another, forever)
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

// NOTE: in theory all of this is taken care of by CRD field validations in the kube-apiserver
// leave it here for now, in case we need to add more validation in the future
func validatePackageVariant(pv *api.PackageVariant) error {
	errors := utils.ErrorCollector{Joiner: "; "}
	if pv.Spec.AdoptionPolicy == "" {
		pv.Spec.AdoptionPolicy = api.AdoptionPolicyAdoptNone
	}
	if pv.Spec.DeletionPolicy == "" {
		pv.Spec.DeletionPolicy = api.DeletionPolicyProposeDeletion
	}
	if pv.Spec.AdoptionPolicy != api.AdoptionPolicyAdoptNone && pv.Spec.AdoptionPolicy != api.AdoptionPolicyAdoptExisting {
		errors.Addf("spec.adoptionPolicy field can only be %q or %q", api.AdoptionPolicyAdoptNone, api.AdoptionPolicyAdoptExisting)
	}
	switch pv.Spec.DeletionPolicy {
	case api.DeletionPolicyDelete, api.DeletionPolicyOrphan, api.DeletionPolicyProposeDeletion:
		// ok
	default:
		errors.Addf("spec.deletionPolicy field can only be %q, %q, or %q", api.DeletionPolicyDelete, api.DeletionPolicyOrphan, api.DeletionPolicyProposeDeletion)
	}
	return errors.Combined("")
}

func (r *PackageVariantReconciler) getUpstreamPR(ctx context.Context, upstream *api.PackageRevisionRef) (*porchapi.PackageRevision, error) {
	for _, pr := range utils.PackageRevisionsFromContextOrDie(ctx) {
		if pr.Spec.RepositoryName == upstream.Repo &&
			pr.Spec.PackageName == upstream.Package &&
			pr.Spec.Revision == upstream.Revision {
			return &pr, nil
		}
	}
	return nil, fmt.Errorf("upstream package revision '%s/%s/%s' is missing", upstream.Repo, upstream.Package, upstream.Revision)
}

func setValidCondition(pv *api.PackageVariant, err error) {
	if err != nil {
		meta.SetStatusCondition(&pv.Status.Conditions, metav1.Condition{
			Type:    api.ConditionTypeValid,
			Status:  "False",
			Reason:  "ValidationError",
			Message: err.Error(),
		})
		meta.SetStatusCondition(&pv.Status.Conditions, metav1.Condition{
			Type:    api.ConditionTypeReady,
			Status:  "False",
			Reason:  "Error",
			Message: "invalid packagevariant object",
		})
	} else {
		meta.SetStatusCondition(&pv.Status.Conditions, metav1.Condition{
			Type:    api.ConditionTypeValid,
			Status:  "True",
			Reason:  "Valid",
			Message: "all validation checks passed",
		})
	}
}

func setReadyCondition(pv *api.PackageVariant, err error) {
	if err == nil {
		meta.SetStatusCondition(&pv.Status.Conditions, metav1.Condition{
			Type:    api.ConditionTypeReady,
			Status:  "True",
			Reason:  "NoErrors",
			Message: "successfully ensured downstream package variant",
		})
	} else {
		meta.SetStatusCondition(&pv.Status.Conditions, metav1.Condition{
			Type:    api.ConditionTypeReady,
			Status:  "False",
			Reason:  "Error",
			Message: err.Error(),
		})
	}
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
	requeueNeeded bool,
	_ error,
) {
	// find or create downstream package revisions
	downstreams, err := r.getDownstreamPRs(ctx, pv, client.ObjectKeyFromObject(upstreamPR))
	if err != nil {
		return false, err
	}

	// ensure that all downstream package revisions are up-to-date
	results := make([]*downstreamTarget, len(downstreams))
	for i, downstreamPR := range downstreams {
		results[i] = r.ensureDownstreamTarget(ctx, pv, upstreamPR, downstreamPR)
	}

	// aggregate status and errors
	requeueNeeded = false
	errors := &utils.ErrorCollector{Joiner: "\n  --- "}
	pv.Status.DownstreamTargets = make([]api.DownstreamTargetStatus, len(results))
	for i, result := range results {
		if result.err != nil {
			errors.Addf("%s: %s", result.pr.Name, result.err)
		}
		if result.requeueNeeded {
			requeueNeeded = true
		}
		pv.Status.DownstreamTargets[i] = result.status()
	}
	return requeueNeeded, errors.Combined("failed to ensure some downstream package revisions:")
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

func newWorkspaceName(ctx context.Context, packageName string, repo string) porchapi.WorkspaceName {
	return utils.PackageRevisionsFromContextOrDie(ctx).OfPackage(repo, packageName).NewWorkspaceName(workspaceNamePrefix)
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
