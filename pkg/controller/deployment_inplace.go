// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/controller/autoscaler"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

// rolloutAutoInPlace implements the logic for rolling  a machine set without replacing it.
func (dc *controller) rolloutAutoInPlace(ctx context.Context, d *v1alpha1.MachineDeployment, isList []*v1alpha1.MachineSet, machineMap map[types.UID]*v1alpha1.MachineList) error {
	clusterAutoscalerScaleDownAnnotations := make(map[string]string)
	clusterAutoscalerScaleDownAnnotations[autoscaler.ClusterAutoscalerScaleDownDisabledAnnotationKey] = autoscaler.ClusterAutoscalerScaleDownDisabledAnnotationValue

	// We do this to avoid accidentally deleting the user provided annotations.
	clusterAutoscalerScaleDownAnnotations[autoscaler.ClusterAutoscalerScaleDownDisabledAnnotationByMCMKey] = autoscaler.ClusterAutoscalerScaleDownDisabledAnnotationByMCMValue

	newIS, oldISs, err := dc.getAllMachineSetsAndSyncRevision(ctx, d, isList, machineMap, true)
	if err != nil {
		return err
	}
	allISs := append(oldISs, newIS)

	err = dc.taintNodesBackingMachineSets(
		ctx,
		oldISs, &v1.Taint{
			Key:    PreferNoScheduleKey,
			Value:  "True",
			Effect: "PreferNoSchedule",
		},
	)

	if dc.autoscalerScaleDownAnnotationDuringRollout {
		// Add the annotation on the all machinesets if there are any old-machinesets and not scaled-to-zero.
		// This also helps in annotating the node under new-machineset, incase the reconciliation is failing in next
		// status-rollout steps.
		if len(oldISs) > 0 && !dc.machineSetsScaledToZero(oldISs) {
			// Annotate all the nodes under this machine-deployment, as roll-out is on-going.
			err := dc.annotateNodesBackingMachineSets(ctx, allISs, clusterAutoscalerScaleDownAnnotations)
			if err != nil {
				klog.Errorf("Failed to add %s on all nodes. Error: %s", clusterAutoscalerScaleDownAnnotations, err)
				return err
			}
		}
	}

	if err != nil {
		klog.Warningf("Failed to add %s on all nodes. Error: %s", PreferNoScheduleKey, err)
	}

	if MachineDeploymentComplete(d, &d.Status) {
		if dc.autoscalerScaleDownAnnotationDuringRollout {
			// Check if any of the machine under this MachineDeployment contains the by-mcm annotation, and
			// remove the original autoscaler annotation only after.
			err := dc.removeAutoscalerAnnotationsIfRequired(ctx, allISs, clusterAutoscalerScaleDownAnnotations)
			if err != nil {
				return err
			}
		}
		if err := dc.cleanupMachineDeployment(ctx, oldISs, d); err != nil {
			return err
		}
	}

	// Sync deployment status
	return dc.syncRolloutStatus(ctx, allISs, newIS, d)
}
