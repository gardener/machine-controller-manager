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

	// In this section, we will attempt to scale up the new machine set. Machines with the `node.machine.sapcloud.io/update-successful` label
	// can transfer their ownership to the new machine set.
	// It is crucial to ensure that during the ownership transfer, the machine is not deleted,
	// and the old machine set is not scaled up to recreate the machine.
	scaledUp, err := dc.reconcileNewMachineSetInPlace(ctx, oldISs, newIS, d)
	if err != nil {
		klog.V(3).Infof("this was unexpected error")
		return err
	}
	if scaledUp {
		// Update DeploymentStatus
		return dc.syncRolloutStatus(ctx, allISs, newIS, d)
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

func (dc *controller) reconcileNewMachineSetInPlace(ctx context.Context, oldISs []*v1alpha1.MachineSet, newIS *v1alpha1.MachineSet, deployment *v1alpha1.MachineDeployment) (bool, error) {
	if (newIS.Spec.Replicas) == (deployment.Spec.Replicas) {
		// Scaling not required.
		return false, nil
	}

	if (newIS.Spec.Replicas) > (deployment.Spec.Replicas) {
		// Scale down.
		scaled, _, err := dc.scaleMachineSetAndRecordEvent(ctx, newIS, (deployment.Spec.Replicas), deployment)
		return scaled, err
	}

	klog.V(3).Infof("reconcile new machine set %s", newIS.Name)

	addedNewReplicasCount := int32(0)

	for _, is := range oldISs {
		transferredMachineCount := int32(0)
		// get the machines for the machine set
		machines, err := dc.machineLister.List(labels.SelectorFromSet(is.Spec.Selector.MatchLabels))
		if err != nil {
			return false, err
		}

		klog.V(3).Infof("machine in old machine set %s: %d", is.Name, len(machines))

		for _, machine := range machines {
			nodeName, ok := machine.Labels[v1alpha1.NodeLabelKey]
			if !ok {
				return addedNewReplicasCount > 0, fmt.Errorf("node label not found for machine %s", machine.Name)
			}

			node, err := dc.nodeLister.Get(nodeName)
			if err != nil {
				if apierrors.IsNotFound(err) {
					klog.Warningf("Node %s not found for machine %s", nodeName, machine.Name)
					continue
				}
				return addedNewReplicasCount > 0, fmt.Errorf("failed to get node %s for machine %s: %w", nodeName, machine.Name, err)
			}

			cond := getMachineCondition(machine, v1alpha1.NodeInPlaceUpdate)
			if cond == nil || cond.Reason != v1alpha1.UpdateSuccessful || node.Labels[v1alpha1.LabelKeyNodeUpdateResult] != v1alpha1.LabelValueNodeUpdateSuccessful {
				continue
			}

			klog.V(3).Infof("transferring machine %s to new machine set %s", machine.Name, newIS.Name)

			// removes labels not present in newIS so that the machine is not selected by the old machine set
			machineNewLabels := MergeStringMaps(MergeWithOverwriteAndFilter(machine.Labels, is.Spec.Selector.MatchLabels, newIS.Spec.Selector.MatchLabels), map[string]string{v1alpha1.LabelKeyNodeUpdateResult: v1alpha1.LabelValueNodeUpdateSuccessful})

			// update the owner reference of the machine to the new machine set and update the labels
			addControllerPatch := fmt.Sprintf(
				`{"metadata":{"ownerReferences":[{"apiVersion":"machine.sapcloud.io/v1alpha1","kind":"%s","name":"%s","uid":"%s","controller":true,"blockOwnerDeletion":true}],"labels":{%s},"uid":"%s"}}`,
				v1alpha1.SchemeGroupVersion.WithKind("MachineSet"),
				newIS.GetName(), newIS.GetUID(), labelsutil.GetFormatedLabels(machineNewLabels), machine.UID)

			err = dc.machineControl.PatchMachine(ctx, machine.Namespace, machine.Name, []byte(addControllerPatch))
			if err != nil {
				klog.V(3).Infof("failed to transfer the ownership of machine %s to new machine set %s", machine.Name, err)
				// scale up the new machine set to the already added replicas.
				scaled, _, err2 := dc.scaleMachineSetAndRecordEvent(ctx, newIS, newIS.Spec.Replicas+addedNewReplicasCount, deployment)
				klog.V(3).Infof("scale up after failure failed %s", err2)
				if err2 != nil {
					return addedNewReplicasCount > 0, err2
				}

				klog.V(3).Infof("scale down machine set %s, transferred replicas to new machine set %d", is.Name, transferredMachineCount)
				_, _, err2 = dc.scaleMachineSetAndRecordEvent(ctx, is, is.Spec.Replicas-transferredMachineCount, deployment)
				klog.V(3).Infof("scale down after failure failed %s", err2)
				if err2 != nil {
					klog.V(3).Infof("scale down failed %s", err)
					return addedNewReplicasCount > 0, err
				}

				return scaled, err
			}

			// uncordon the node since the ownership of the machine has been transferred to the new machien set.
			node.Spec.Unschedulable = false
			_, err = dc.targetCoreClient.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
			if err != nil {
				return false, fmt.Errorf("failed to uncordon the node %s: %w", node.Name, err)
			}

			transferredMachineCount++ // scale down the old machine set.
			addedNewReplicasCount++   // scale up the new machine set.
		}

		klog.V(3).Infof("scale down machine set %s, transffered replicas to new machine set %d", is.Name, transferredMachineCount)
		_, _, err = dc.scaleMachineSetAndRecordEvent(ctx, is, is.Spec.Replicas-transferredMachineCount, deployment)
		if err != nil {
			klog.V(3).Infof("scale down failed %s", err)
			return addedNewReplicasCount > 0, err
		}
	}

	klog.V(3).Infof("scale up the new machine set %s by %d", newIS.Name, addedNewReplicasCount)
	scaled, _, err := dc.scaleMachineSetAndRecordEvent(ctx, newIS, newIS.Spec.Replicas+addedNewReplicasCount, deployment)
	return scaled, err
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
