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
	"github.com/nephio-project/porch/ng/utils"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type downstreamTarget struct {
	pr             *porchapi.PackageRevision
	renderStatus   porchapi.RenderStatus
	mutationStatus []api.MutationStatus
	err            error
	requeueNeeded  bool
}

func newDownstreamTarget(downstreamPR *porchapi.PackageRevision, pv *api.PackageVariant) *downstreamTarget {
	result := &downstreamTarget{
		pr:            downstreamPR,
		requeueNeeded: false,
	}
	// preserve some fields from pv.Status
	for _, s := range pv.Status.DownstreamTargets {
		if s.Name == downstreamPR.Name {
			result.renderStatus = s.RenderStatus
		}
	}
	return result
}

func (dt downstreamTarget) status() api.DownstreamTargetStatus {
	return api.DownstreamTargetStatus{
		Name:         dt.pr.Name,
		RenderStatus: dt.renderStatus,
		Mutations:    dt.mutationStatus,
	}
}

// getDownstreamPRs returns the list of either existing or newly created downstream package revisions,
// according to the following rules:
// - If there are any drafts that are owned by us and match the target package revision, return them all.
// - If there are no drafts, return the latest published package revision owned by us.
// - If there are no drafts or published package revisions, create a new package revision.
// getDownstreamPRs deletes or orphans package revisions that are owned by `pv`, but doesn't match the current downstream target.
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
				err := r.deleteOrOrphan(ctx, &pr, pv)
				if err != nil {
					return nil, err
				}
			}
			continue
		}

		// this package matches, check if we need to adopt it
		if !owned && pv.Spec.AdoptionPolicy == api.AdoptionPolicyAdoptExisting {
			if err := r.adoptPackageRevision(ctx, &pr, pv); err != nil {
				l.Error(err, fmt.Sprintf("couldn't adopt package revision %q", pr.Name))
			} else {
				l.Info(fmt.Sprintf("adopted package revision %q", pr.Name))
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
			ReadinessGates: readinessGates(pv),
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
) (target *downstreamTarget) {
	l := log.FromContext(ctx)
	target = newDownstreamTarget(downstreamPR, pv)

	// if target.pr.Spec.Lifecycle == porchapi.PackageRevisionLifecycleDeletionProposed {
	// 	// We proposed this package revision for deletion in the past, but now it
	// 	// matches our target, so we no longer want it to be deleted.
	// 	target.pr.Spec.Lifecycle = porchapi.PackageRevisionLifecyclePublished
	// 	// We update this now, because later we may use a Porch call to clone or update
	// 	// and we want to make sure the server is in sync with us
	// 	target.err = r.Client.Update(ctx, target.pr)
	// 	if target.err != nil {
	// 		l.Error(target.err, "couldn't update package revision lifecycle"))
	// 		return
	// 	}
	// }

	// see if the package needs updating due to an upstream change
	if !r.isUpToDate(pv, target.pr) {
		// we need to copy a published package to a new draft before updating
		if porchapi.LifecycleIsPublished(target.pr.Spec.Lifecycle) {
			target.pr, target.err = r.copyPublished(ctx, target.pr, pv)
			if target.err != nil {
				return
			}
		}
		target.pr, target.err = r.updateDraft(ctx, target.pr, upstreamPR)
		if target.err != nil {
			return
		}
		l.Info(fmt.Sprintf("updated package revision %q to upstream revision %s", target.pr.Name, upstreamPR.Spec.Revision))
	}

	// finally, see if any other changes are needed to the resources
	var prr *porchapi.PackageRevisionResources
	var changed bool
	prr, changed, target.err = r.calculateDraftResources(ctx, pv, target)
	if target.err != nil {
		return
	}

	// if there are changes, save them
	if changed {
		// if no pkg update was needed, we may still be a published package
		// so, clone to a new Draft if that's the case
		if porchapi.LifecycleIsPublished(target.pr.Spec.Lifecycle) {

			target.pr, target.err = r.copyPublished(ctx, target.pr, pv)
			if target.err != nil {
				return
			}
			// recalculate from the new Draft
			prr, _, target.err = r.calculateDraftResources(ctx, pv, target)
			if target.err != nil {
				return
			}

		}
		// Save the updated PackageRevisionResources
		target.err = r.Update(ctx, prr)
		target.renderStatus = prr.Status.RenderStatus
		if target.err != nil {
			return
		}
	}

	target.err = r.ensureApproval(ctx, pv, target)
	if target.err != nil {
		return
	}
	return
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

// calculateDraftResources applies mutations to the downstream target and checks if any resources have changed.
// Returns with the new resources and a boolean indicating if any changes were made.
func (r *PackageVariantReconciler) calculateDraftResources(
	ctx context.Context,
	pv *api.PackageVariant,
	target *downstreamTarget,
) (
	prr *porchapi.PackageRevisionResources,
	changed bool,
	err error,
) {
	l := log.FromContext(ctx)
	// Load the PackageRevisionResources
	prr = &porchapi.PackageRevisionResources{}
	if err := r.Client.Get(ctx, client.ObjectKeyFromObject(target.pr), prr); err != nil {
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

	if err = ensureMutations(ctx, r.Client, pv, prr, target); err != nil {
		return nil, false, err
	}

	// check if any resources have changed after applying mutations
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

func (r *PackageVariantReconciler) deletePackageRevision(ctx context.Context, pr *porchapi.PackageRevision, proposeOnly bool) error {
	l := log.FromContext(ctx)

	switch pr.Spec.Lifecycle {
	case porchapi.PackageRevisionLifecyclePublished:
		l.Info(fmt.Sprintf("proposing published package revision %q for deletion", pr.Name))
		pr.Spec.Lifecycle = porchapi.PackageRevisionLifecycleDeletionProposed
		if err := r.Client.Update(ctx, pr); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("couldn't propose package revision %q for deletion: %w", pr.Name, err)
		}
		fallthrough
	case porchapi.PackageRevisionLifecycleDeletionProposed:
		if proposeOnly {
			// we don't have to do anything
			return nil
		}
		fallthrough
	case "", porchapi.PackageRevisionLifecycleDraft, porchapi.PackageRevisionLifecycleProposed:
		l.Info(fmt.Sprintf("deleting package revision %q", pr.Name))
		err := r.Client.Delete(ctx, pr)
		if client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("couldn't delete package revision %q: %w", pr.Name, err)
		}
	default:
		// if this ever happens, there's something going wrong with porch
		l.Error(nil, fmt.Sprintf("invalid lifecycle value for package revision %s: %s", pr.Name, pr.Spec.Lifecycle))
	}
	return nil
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
		Spec: porchapi.PackageRevisionSpec{
			RepositoryName: source.Spec.RepositoryName,
			PackageName:    source.Spec.PackageName,
			Revision:       "",
			WorkspaceName:  newWorkspaceName(ctx, source.Spec.PackageName, source.Spec.RepositoryName),
			Lifecycle:      porchapi.PackageRevisionLifecycleDraft,
			ReadinessGates: source.Spec.ReadinessGates,
			Tasks: []porchapi.Task{
				{
					Type: porchapi.TaskTypeEdit,
					Edit: &porchapi.PackageEditTaskSpec{
						Source: &porchapi.PackageRevisionRef{
							Name: source.Name,
						},
					},
				},
			},
		},
	}

	// newPR.Spec.Revision = ""
	// newPR.Spec.WorkspaceName = newWorkspaceName(ctx, newPR.Spec.PackageName, newPR.Spec.RepositoryName)
	// newPR.Spec.Lifecycle = porchapi.PackageRevisionLifecycleDraft

	if err := r.Client.Create(ctx, newPR); err != nil {
		l.Error(err, fmt.Sprintf("failed to copy %q", source.Name))
		return source, err
	}
	l.Info(fmt.Sprintf("created package revision %q based on %q", newPR.Name, source.Name))
	return newPR, nil
}

func (r *PackageVariantReconciler) deleteOrOrphan(
	ctx context.Context,
	pr *porchapi.PackageRevision,
	pv *api.PackageVariant,
) error {
	l := log.FromContext(ctx)
	switch pv.Spec.DeletionPolicy {
	case api.DeletionPolicyProposeDeletion:
		return r.deletePackageRevision(ctx, pr, true)
	case "", api.DeletionPolicyDelete:
		return r.deletePackageRevision(ctx, pr, false)
	case api.DeletionPolicyOrphan:
		l.Info(fmt.Sprintf("orphaning package revision %q", pr.Name))
		return r.orphanPackageRevision(ctx, pr, pv)
	default:
		// this should never happen, because the pv should already be validated beforehand
		l.Error(nil, fmt.Sprintf("invalid deletion policy %s", pv.Spec.DeletionPolicy))
	}
	return nil
}

func (r *PackageVariantReconciler) orphanPackageRevision(
	ctx context.Context,
	pr *porchapi.PackageRevision,
	pv *api.PackageVariant,
) error {
	pr.ObjectMeta.OwnerReferences = removeOwnerRefByUID(pr.OwnerReferences, pv.UID)
	if err := r.Client.Update(ctx, pr); client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("error orphaning package revision %q: %w", pr.Name, err)
	}
	return nil
}
