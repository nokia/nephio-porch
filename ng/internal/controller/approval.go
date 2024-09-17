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

	porchapi "github.com/nephio-project/porch/api/porch/v1alpha1"
	api "github.com/nephio-project/porch/ng/api/v1alpha1"
	kptfileapi "github.com/nephio-project/porch/pkg/kpt/api/kptfile/v1"
	"k8s.io/client-go/util/retry"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *PackageVariantReconciler) ensureApproval(ctx context.Context, pv *api.PackageVariant, target *downstreamTarget) error {
	l := log.FromContext(ctx)

	if !approvalNeeded(pv) {
		return nil
	}
	if porchapi.LifecycleIsPublished(target.pr.Spec.Lifecycle) {
		return nil
	}
	if !PackageRevisionIsReady(target.pr) {
		l.Info(fmt.Sprintf("package revision %q needs approval, but it isn't ready --> requeue", target.pr.Name))
		target.requeueNeeded = true
		return nil
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var newPR porchapi.PackageRevision
		err := r.Client.Get(ctx, client.ObjectKeyFromObject(target.pr), &newPR)
		if err != nil {
			return client.IgnoreNotFound(err)
		}

		switch newPR.Spec.Lifecycle {

		case porchapi.PackageRevisionLifecycleDraft:
			l.Info(fmt.Sprintf("proposing package revision %q", target.pr.Name))
			newPR.Spec.Lifecycle = porchapi.PackageRevisionLifecycleProposed
			err = r.Client.Update(ctx, &newPR)
			if err != nil {
				return err
			}
			fallthrough

		case porchapi.PackageRevisionLifecycleProposed:
			l.Info(fmt.Sprintf("approving package revision %q", target.pr.Name))
			newPR.Spec.Lifecycle = porchapi.PackageRevisionLifecyclePublished
			return r.Client.SubResource("approval").Update(ctx, &newPR)

		case porchapi.PackageRevisionLifecyclePublished, porchapi.PackageRevisionLifecycleDeletionProposed:
			l.Info(fmt.Sprintf("package revision %q already approved", target.pr.Name))
			return nil

		default:
			return fmt.Errorf("invalid lifecycle value for package revision %s: %s", target.pr.Name, target.pr.Spec.Lifecycle)
		}
	})
}

func readinessGates(pv *api.PackageVariant) []porchapi.ReadinessGate {
	result := pv.Spec.ReadinessGates
	for _, m := range pv.Spec.Mutations {
		result = append(result, porchapi.ReadinessGate{
			ConditionType: m.ConditionType(pvPrefix(client.ObjectKeyFromObject(pv))),
		})
	}
	if pv.Spec.ApprovalPolicy == api.ApprovalPolicyAlwaysWithManualEdits {
		result = append(result, porchapi.ReadinessGate{
			ConditionType: ConditionTypeManualEdits,
		})
	}
	// TODO: add readiness gates for mandatory injection points
	return result
}

func approvalNeeded(pv *api.PackageVariant) bool {
	return pv.Spec.ApprovalPolicy != api.ApprovalPolicyNever
	// TODO[kispaljr]: implement initial policy
}

// Check if package revision has met all readiness gates
func PackageRevisionIsReady(pr *porchapi.PackageRevision) bool {
	for _, gate := range pr.Spec.ReadinessGates {
		if !ConditionIsTrue(pr, gate.ConditionType) {
			return false
		}
	}
	return true
}

func ConditionIsTrue(pr *porchapi.PackageRevision, conditionType string) bool {
	for _, condition := range pr.Status.Conditions {
		if condition.Type == conditionType {
			return condition.Status == porchapi.ConditionTrue
		}
	}
	return false
}

func ConditionFromStatus(conditionType string, status bool, message string) kptfileapi.Condition {
	if status {
		return kptfileapi.Condition{
			Type:    conditionType,
			Status:  kptfileapi.ConditionTrue,
			Reason:  "Ready",
			Message: message,
		}
	} else {
		return kptfileapi.Condition{
			Type:    conditionType,
			Status:  kptfileapi.ConditionFalse,
			Reason:  "Error",
			Message: message,
		}
	}
}

func SetConditionFromError(pr *porchapi.PackageRevision, conditionType string, errValue error) {
	var condition porchapi.Condition
	if errValue == nil {
		condition = porchapi.Condition{
			Type:    conditionType,
			Status:  porchapi.ConditionTrue,
			Reason:  "Ready",
			Message: "Operation succeeded",
		}
	} else {
		condition = porchapi.Condition{
			Type:    conditionType,
			Status:  porchapi.ConditionFalse,
			Reason:  "Error",
			Message: errValue.Error(),
		}
	}
	SetCondition(pr, condition)
}

func SetCondition(pr *porchapi.PackageRevision, newCondition porchapi.Condition) {
	for i, condition := range pr.Status.Conditions {
		if condition.Type == newCondition.Type {
			pr.Status.Conditions[i] = newCondition
			return
		}
	}
	pr.Status.Conditions = append(pr.Status.Conditions, newCondition)
}
