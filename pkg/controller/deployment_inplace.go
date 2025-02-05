// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/controller/autoscaler"
	labelsutil "github.com/gardener/machine-controller-manager/pkg/util/labels"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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

	// label all the machines and nodes backing the old machine sets as candidate for update
	err = dc.labelNodesBackingMachineSets(ctx, oldISs, v1alpha1.LabelKeyNodeCandidateForUpdate, "true")
	if err != nil {
		return fmt.Errorf("failed to label nodes backing old machine sets as candidate for update: %v", err)
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

// labelNodesBackingMachineSets annotates all nodes backing the machineSets
func (dc *controller) labelNodesBackingMachineSets(ctx context.Context, machineSets []*v1alpha1.MachineSet, labelKey, labelValue string) error {
	for _, machineSet := range machineSets {

		if machineSet == nil {
			continue
		}

		klog.V(4).Infof("Trying to label nodes under the MachineSet object %q with %v", machineSet.Name, labelKey)
		filteredMachines, err := dc.machineLister.List(labels.SelectorFromSet(machineSet.Spec.Selector.MatchLabels))
		if err != nil {
			return err
		}

		for _, machine := range filteredMachines {
			if err := dc.labelMachineAndBackingNode(ctx, machine, labelKey, labelValue); err != nil {
				return err
			}
		}

		klog.V(3).Infof("Labeled the nodes backed by MachineSet %q with %v", machineSet.Name, labelKey)
	}

	return nil
}

func (dc *controller) labelMachineBackingNode(ctx context.Context, machine *v1alpha1.Machine, labelKey, labelValue string) error {
	if machine.Labels[v1alpha1.NodeLabelKey] != "" {
		node, err := dc.nodeLister.Get(machine.Labels[v1alpha1.NodeLabelKey])
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil // Node is not found, continue to the next machine
			}
			klog.Errorf("Error occurred while trying to fetch node object - err: %s", err)
			return err
		}
		if node.Labels[labelKey] == labelValue {
			return nil
		}
		nodeCopy := node.DeepCopy()
		if nodeCopy.Labels == nil {
			nodeCopy.Labels = make(map[string]string)
		}
		nodeCopy.Labels[labelKey] = labelValue
		if _, err := dc.targetCoreClient.CoreV1().Nodes().Update(ctx, nodeCopy, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}

	return nil
}
