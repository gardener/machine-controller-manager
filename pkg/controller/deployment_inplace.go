// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"sort"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/controller/autoscaler"
	labelsutil "github.com/gardener/machine-controller-manager/pkg/util/labels"
	"github.com/gardener/machine-controller-manager/pkg/util/nodeops"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machineutils"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"k8s.io/utils/integer"
)

// rolloutAutoInPlace implements the logic for rolling  a machine set without replacing it.
func (dc *controller) rolloutAutoInPlace(ctx context.Context, d *v1alpha1.MachineDeployment, isList []*v1alpha1.MachineSet, machineMap map[types.UID]*v1alpha1.MachineList) error {
	clusterAutoscalerScaleDownAnnotations := make(map[string]string)
	clusterAutoscalerScaleDownAnnotations[autoscaler.ClusterAutoscalerScaleDownDisabledAnnotationKey] = autoscaler.ClusterAutoscalerScaleDownDisabledAnnotationValue

	// We do this to avoid accidentally deleting the user provided annotations.
	clusterAutoscalerScaleDownAnnotations[autoscaler.ClusterAutoscalerScaleDownDisabledAnnotationByMCMKey] = autoscaler.ClusterAutoscalerScaleDownDisabledAnnotationByMCMValue

	newMachineSet, oldMachineSets, err := dc.getAllMachineSetsAndSyncRevision(ctx, d, isList, machineMap, true)
	if err != nil {
		return err
	}
	allMachineSets := append(oldMachineSets, newMachineSet)

	if len(oldMachineSets) > 0 && !dc.machineSetsScaledToZero(oldMachineSets) {
		// Label all the old machine sets to disable the scale up.
		err := dc.labelMachineSets(ctx, oldMachineSets, map[string]string{machineutils.LabelKeyMachineSetScaleUpDisabled: "true"})
		if err != nil {
			klog.Errorf("failed to add label %s on all machine sets. Error: %v", machineutils.LabelKeyMachineSetScaleUpDisabled, err)
			return err
		}

		// Add the annotation on the all machinesets if there are any old-machinesets and not scaled-to-zero.
		// This also helps in annotating the node under new-machineset, incase the reconciliation is failing in next
		// status-rollout steps.
		if dc.autoscalerScaleDownAnnotationDuringRollout {
			// Annotate all the nodes under this machine-deployment, as roll-out is on-going.
			err := dc.annotateNodesBackingMachineSets(ctx, allMachineSets, clusterAutoscalerScaleDownAnnotations)
			if err != nil {
				klog.Errorf("failed to add annotations %s on all nodes. Error: %v", clusterAutoscalerScaleDownAnnotations, err)
				return err
			}
		}
	}

	err = dc.taintNodesBackingMachineSets(
		ctx,
		oldMachineSets, &v1.Taint{
			Key:    PreferNoScheduleKey,
			Value:  "True",
			Effect: "PreferNoSchedule",
		},
	)
	if err != nil {
		klog.Warningf("failed to add taint %s on all nodes. Error: %v", PreferNoScheduleKey, err)
	}

	// label all nodes backing old machine sets as candidate for update
	if err := dc.labelNodesBackingMachineSets(ctx, oldMachineSets, v1alpha1.LabelKeyNodeCandidateForUpdate, "true"); err != nil {
		return fmt.Errorf("failed to label nodes backing old machine sets as candidate for update: %v", err)
	}

	if err := dc.syncMachineSets(ctx, oldMachineSets, newMachineSet, d); err != nil {
		return err
	}

	// In this section, we will attempt to scale up the new machine set. Machines with the `node.machine.sapcloud.io/update-successful` label
	// can transfer their ownership to the new machine set.
	// It is crucial to ensure that during the ownership transfer, the machine is not deleted,
	// and the old machine set is not scaled up to recreate the machine.
	scaledUp, err := dc.reconcileNewMachineSetInPlace(ctx, oldMachineSets, newMachineSet, d)
	if err != nil {
		klog.Errorf("failed to reconcile new machine set in place %s", err)
		return err
	}
	if scaledUp {
		// Update DeploymentStatus
		return dc.syncRolloutStatus(ctx, allMachineSets, newMachineSet, d)
	}

	// prepare old ISs for update
	workDone, err := dc.reconcileOldMachineSetsInPlace(ctx, allMachineSets, FilterActiveMachineSets(oldMachineSets), newMachineSet, d)
	if err != nil {
		return err
	}
	if workDone {
		// Update DeploymentStatus
		return dc.syncRolloutStatus(ctx, allMachineSets, newMachineSet, d)
	}

	if MachineDeploymentComplete(d, &d.Status) {
		if dc.autoscalerScaleDownAnnotationDuringRollout {
			// Check if any of the machine under this MachineDeployment contains the by-mcm annotation, and
			// remove the original autoscaler annotation only after.
			err := dc.removeAutoscalerAnnotationsIfRequired(ctx, allMachineSets, clusterAutoscalerScaleDownAnnotations)
			if err != nil {
				return err
			}
		}
		if err := dc.cleanupMachineDeployment(ctx, oldMachineSets, d); err != nil {
			return err
		}
	}

	// Sync deployment status
	return dc.syncRolloutStatus(ctx, allMachineSets, newMachineSet, d)
}

// syncMachineSets syncs the machine sets by scaling up the new machine set and scaling down the old machine sets to the required replicas.
func (dc *controller) syncMachineSets(ctx context.Context, oldMachineSets []*v1alpha1.MachineSet, newMachineSet *v1alpha1.MachineSet, deployment *v1alpha1.MachineDeployment) error {
	newMachines, err := dc.machineLister.List(labels.SelectorFromSet(newMachineSet.Spec.Selector.MatchLabels))
	if err != nil {
		return err
	}

	machinesWithUpdateSuccessfulLabel := filterMachinesWithUpdateSuccessfulLabel(newMachines)
	klog.V(3).Infof("Found %d machine(s) with label %q=%q in new machine set", len(machinesWithUpdateSuccessfulLabel), v1alpha1.LabelKeyNodeUpdateResult, v1alpha1.LabelValueNodeUpdateSuccessful)

	if len(newMachines) > int(newMachineSet.Spec.Replicas) && len(machinesWithUpdateSuccessfulLabel) > 0 {
		// scale up the new machine set to the number of machines with the update successful label.
		// This is to ensure that the machines with moved to the new machine set when ownership is transferred is accounted.
		scaleUpBy := min(len(machinesWithUpdateSuccessfulLabel), len(newMachines)-int(newMachineSet.Spec.Replicas))
		_, _, err := dc.scaleMachineSetAndRecordEvent(ctx, newMachineSet, newMachineSet.Spec.Replicas+int32(scaleUpBy), deployment) // #nosec G115 (CWE-190) -- value already validated
		if err != nil {
			return err
		}
	}

	// remove labels from the machines related to the inplace update.
	for _, machine := range machinesWithUpdateSuccessfulLabel {
		labelsToRemove := []string{
			v1alpha1.LabelKeyNodeUpdateResult,
		}

		patchBytes, err := labelsutil.RemoveLabels(labelsToRemove)
		if err != nil {
			return err
		}

		klog.V(3).Infof("removing label %v from machine %s", labelsToRemove, machine.Name)
		if err := dc.machineControl.PatchMachine(ctx, machine.Namespace, machine.Name, patchBytes); err != nil {
			klog.Errorf("error while removing label  %v : %v", labelsToRemove, err)
			return err
		}
	}

	// updates nodes associated with the machines to remove update-related labels and annotations, and uncordons them.
	for _, machine := range newMachines {
		nodeName, ok := machine.Labels[v1alpha1.NodeLabelKey]
		if !ok {
			return fmt.Errorf("node label not found for machine %s: %w", machine.Name, err)
		}

		node, err := dc.nodeLister.Get(nodeName)
		if err != nil {
			return fmt.Errorf("failed to get node %s: %w", nodeName, err)
		}

		cond := nodeops.GetCondition(node, v1alpha1.NodeInPlaceUpdate)
		if isUpdateNotSuccessful(cond, node.Labels) {
			continue
		}

		// remove labels related to the inplace update.
		delete(node.Labels, v1alpha1.LabelKeyNodeCandidateForUpdate)
		delete(node.Labels, v1alpha1.LabelKeyNodeSelectedForUpdate)
		delete(node.Labels, v1alpha1.LabelKeyNodeUpdateResult)
		// remove annotations related to the inplace update.
		delete(node.Annotations, v1alpha1.AnnotationKeyMachineUpdateFailedReason)

		// uncordon the node since the inplace update is successful.
		node.Spec.Unschedulable = false

		_, err = dc.targetCoreClient.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to remove inplace labels/annotations and uncordon node %s: %w", node.Name, err)
		}
	}

	for _, machineSet := range oldMachineSets {
		// scale down the old machine set to the number of machines which is having the labelselector of the machine set.
		oldMachines, err := dc.machineLister.List(labels.SelectorFromSet(machineSet.Spec.Selector.MatchLabels))
		if err != nil {
			return fmt.Errorf("failed to list machines for machine set %s: %w", machineSet.Name, err)
		}

		if len(oldMachines) < int(machineSet.Spec.Replicas) {
			_, _, err := dc.scaleMachineSetAndRecordEvent(ctx, machineSet, int32(len(oldMachines)), deployment) // #nosec G115 (CWE-190) -- value already validated
			if err != nil {
				return fmt.Errorf("failed to scale down machine set %s: %w", machineSet.Name, err)
			}
			klog.V(3).Infof("scaled down machine set %s to %d", machineSet.Name, len(oldMachines))
		}
	}

	return nil
}

func (dc *controller) reconcileNewMachineSetInPlace(ctx context.Context, oldMachineSets []*v1alpha1.MachineSet, newMachineSet *v1alpha1.MachineSet, deployment *v1alpha1.MachineDeployment) (bool, error) {
	if newMachineSet.Spec.Replicas == deployment.Spec.Replicas {
		// Scaling not required.
		return false, nil
	}

	if newMachineSet.Spec.Replicas > deployment.Spec.Replicas {
		// Scale down.
		scaled, _, err := dc.scaleMachineSetAndRecordEvent(ctx, newMachineSet, deployment.Spec.Replicas, deployment)
		return scaled, err
	}

	klog.V(3).Infof("reconcile new machine set %s", newMachineSet.Name)

	oldMachinesCount := GetReplicaCountForMachineSets(oldMachineSets)
	if oldMachinesCount == 0 {
		scaled, _, err := dc.scaleMachineSetAndRecordEvent(ctx, newMachineSet, deployment.Spec.Replicas, deployment)
		if err != nil {
			return false, fmt.Errorf("failed to scale up machine set %s: %w", newMachineSet.Name, err)
		}
		klog.V(3).Infof("scaled up machine set %s to %d", newMachineSet.Name, deployment.Spec.Replicas)
		return scaled, err
	}

	addedNewReplicasCount := int32(0)

	for _, machineSet := range oldMachineSets {
		transferredMachineCount := int32(0)
		// get the machines for the machine set
		oldMachines, err := dc.machineLister.List(labels.SelectorFromSet(machineSet.Spec.Selector.MatchLabels))
		if err != nil {
			return false, err
		}

		klog.V(3).Infof("Found %d machine(s) in old machine set %s", len(oldMachines), machineSet.Name)

		for _, oldMachine := range oldMachines {
			nodeName, ok := oldMachine.Labels[v1alpha1.NodeLabelKey]
			if !ok {
				return false, fmt.Errorf("node label not found for machine %s", oldMachine.Name)
			}

			node, err := dc.nodeLister.Get(nodeName)
			if err != nil {
				if apierrors.IsNotFound(err) {
					klog.Warningf("Node %s not found for machine %s", nodeName, oldMachine.Name)
					continue
				}
				return false, fmt.Errorf("failed to get node %s for machine %s: %w", nodeName, oldMachine.Name, err)
			}

			cond := getMachineCondition(oldMachine, v1alpha1.NodeInPlaceUpdate)
			if isUpdateNotSuccessful(cond, node.Labels) {
				continue
			}

			klog.V(3).Infof("Attempting to transfer machine %s to new machine set %s", oldMachine.Name, newMachineSet.Name)

			// removes labels not present in newMachineSet so that the machine is not selected by the old machine set
			machineNewLabels := MergeStringMaps(
				MergeWithOverwriteAndFilter(oldMachine.Labels, machineSet.Spec.Selector.MatchLabels, newMachineSet.Spec.Selector.MatchLabels),
				map[string]string{v1alpha1.LabelKeyNodeUpdateResult: v1alpha1.LabelValueNodeUpdateSuccessful})

			formattedLabels, err := labelsutil.GetFormattedLabels(machineNewLabels)
			if err != nil {
				return false, err
			}
			// update the owner reference of the machine to the new machine set and update the labels
			addControllerPatch := fmt.Sprintf(
				`{"metadata":{"ownerReferences":[{"apiVersion":"machine.sapcloud.io/v1alpha1","kind":"%s","name":"%s","uid":"%s","controller":true,"blockOwnerDeletion":true}],"labels":%s,"uid":"%s"}}`,
				v1alpha1.SchemeGroupVersion.WithKind("MachineSet"),
				newMachineSet.GetName(), newMachineSet.GetUID(), string(formattedLabels), oldMachine.UID)

			err = dc.machineControl.PatchMachine(ctx, oldMachine.Namespace, oldMachine.Name, []byte(addControllerPatch))
			if err != nil {
				klog.Errorf("failed to transfer the ownership of machine %s to new machine set. Err: %v", oldMachine.Name, err)
				return false, err
			}

			// uncordon the node since the ownership of the machine has been transferred to the new machine set.
			node.Spec.Unschedulable = false
			_, err = dc.targetCoreClient.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
			if err != nil {
				return false, fmt.Errorf("failed to uncordon the node %s: %w", node.Name, err)
			}

			transferredMachineCount++ // scale down the old machine set.
			addedNewReplicasCount++   // scale up the new machine set.
		}

		if transferredMachineCount == 0 {
			klog.V(3).Infof("no machines transferred from machine set %s", machineSet.Name)
			continue
		}

		klog.V(3).Infof("%d machine(s) transferred to new machine set. scaling down machine set %s", transferredMachineCount, machineSet.Name)
		_, _, err = dc.scaleMachineSetAndRecordEvent(ctx, machineSet, machineSet.Spec.Replicas-transferredMachineCount, deployment)
		if err != nil {
			klog.Errorf("scale down failed %s", err)
			return false, err
		}
	}

	if addedNewReplicasCount == 0 {
		klog.V(3).Infof("no machines transferred to new machine set %s", newMachineSet.Name)
		return false, nil
	}

	klog.V(3).Infof("scale up the new machine set %s by %d", newMachineSet.Name, addedNewReplicasCount)
	scaled, _, err := dc.scaleMachineSetAndRecordEvent(ctx, newMachineSet, newMachineSet.Spec.Replicas+addedNewReplicasCount, deployment)
	return scaled, err
}

func (dc *controller) reconcileOldMachineSetsInPlace(ctx context.Context, allMachineSets []*v1alpha1.MachineSet, oldMachineSets []*v1alpha1.MachineSet, newMachineSet *v1alpha1.MachineSet, deployment *v1alpha1.MachineDeployment) (workDone bool, err error) {
	oldMachinesCount := GetReplicaCountForMachineSets(oldMachineSets)
	if oldMachinesCount == 0 {
		// Can't scale down further
		return false, nil
	}

	// If maxSurge is defined, there will be machines left in the old machine set that do not require an update
	// because the new machine set already has the required replicas due to the additional machines added as per maxSurge.
	// In that case we simply scale down the old machine set to zero.
	if newMachineSet.Spec.Replicas == deployment.Spec.Replicas {
		// Scale down old machine sets to zero.
		for _, machineSet := range oldMachineSets {
			_, _, err := dc.scaleMachineSetAndRecordEvent(ctx, machineSet, 0, deployment)
			if err != nil {
				return false, err
			}
		}
		return true, nil
	}

	allMachinesCount := GetReplicaCountForMachineSets(allMachineSets)
	klog.V(3).Infof("New machine set %s has %d available machines.", newMachineSet.Name, newMachineSet.Status.AvailableReplicas)
	maxUnavailable := MaxUnavailable(*deployment)

	minAvailable := deployment.Spec.Replicas - maxUnavailable
	newISUnavailableMachineCount := newMachineSet.Spec.Replicas - newMachineSet.Status.AvailableReplicas
	oldISsMachinesUndergoingUpdate, err := dc.getMachinesUndergoingUpdate(oldMachineSets)
	if err != nil {
		return false, err
	}

	klog.V(3).Infof("allMachinesCount:%d,  minAvailable:%d,  newISUnavailableMachineCount:%d,  oldISsMachineInUpdateProcess:%d", allMachinesCount, minAvailable, newISUnavailableMachineCount, oldISsMachinesUndergoingUpdate)

	// maxUpdatePossible is calculated as the total number of machines (allMachinesCount)
	// minus the minimum number of machines that must remain available (minAvailable),
	// minus the number of machines in the new instance set that are currently unavailable (newISUnavailableMachineCount),
	// minus the number of machines in the old instance sets that are undergoing updates (oldISsMachinesUndergoingUpdate).
	// here unavailable machines of old machine sets are not considered as first we want to check if we can select machines for update from old machine sets
	// after fulfilling all the constraints.
	maxUpdatePossible := allMachinesCount - minAvailable - newISUnavailableMachineCount - oldISsMachinesUndergoingUpdate
	if maxUpdatePossible <= 0 {
		klog.V(3).Infof("no machines can be selected for update from old machine sets")
		return false, nil
	}

	// prepare machines from old machine sets for update, need to check maxUnavailable to ensure we can select machines for update.
	numOfMachinesSelectedForUpdate, err := dc.selectNumOfMachineForUpdate(ctx, allMachineSets, oldMachineSets, newMachineSet, deployment, oldISsMachinesUndergoingUpdate)
	if err != nil {
		return false, err
	}

	return numOfMachinesSelectedForUpdate > 0, nil
}

func (dc *controller) selectNumOfMachineForUpdate(ctx context.Context, allMachineSets []*v1alpha1.MachineSet, oldMachineSets []*v1alpha1.MachineSet, newMachineSet *v1alpha1.MachineSet, deployment *v1alpha1.MachineDeployment, oldISsMachinesUndergoingUpdate int32) (int32, error) {
	maxUnavailable := MaxUnavailable(*deployment)

	// Check if we can pick machines from old ISes for updating to new IS.
	minAvailable := deployment.Spec.Replicas - maxUnavailable

	// Find the number of available machines.
	availableMachineCount := GetAvailableReplicaCountForMachineSets(allMachineSets) - oldISsMachinesUndergoingUpdate
	if availableMachineCount <= minAvailable {
		// Cannot pick for updating.
		return 0, nil
	}

	sort.Sort(MachineSetsByCreationTimestamp(oldMachineSets))

	totalSelectedForUpdate := int32(0)
	maxSelectableForUpdate := min(availableMachineCount-minAvailable, max(deployment.Spec.Replicas-newMachineSet.Spec.Replicas, 0))
	for _, targetIS := range oldMachineSets {
		if totalSelectedForUpdate >= maxSelectableForUpdate {
			// No further updating required.
			break
		}
		if (targetIS.Spec.Replicas) == 0 {
			// cannot pick this ReplicaSet.
			continue
		}
		// prepare for update
		readyForUpdateCount := integer.Int32Min(targetIS.Spec.Replicas, maxSelectableForUpdate-totalSelectedForUpdate) // #nosec G115 (CWE-190) -- value already validated
		newReplicasCount := targetIS.Spec.Replicas - readyForUpdateCount

		if newReplicasCount > targetIS.Spec.Replicas {
			return 0, fmt.Errorf("when selecting machine from old IS for update, got invalid request %s %d -> %d", targetIS.Name, targetIS.Spec.Replicas, newReplicasCount)
		}
		selectedFromCurrentMachineSet, err := dc.labelMachinesToSelectedForUpdate(ctx, targetIS, readyForUpdateCount)
		if err != nil {
			return totalSelectedForUpdate + selectedFromCurrentMachineSet, err
		}

		totalSelectedForUpdate += selectedFromCurrentMachineSet
	}

	return totalSelectedForUpdate, nil
}

// labelNodesBackingMachineSets annotates all nodes backing the machineSets
func (dc *controller) labelNodesBackingMachineSets(ctx context.Context, machineSets []*v1alpha1.MachineSet, labelKey, labelValue string) error {
	for _, machineSet := range machineSets {

		if machineSet == nil {
			continue
		}

		klog.V(4).Infof("Attempting to label nodes belonging to MachineSet object %q with %v", machineSet.Name, labelKey)
		filteredMachines, err := dc.machineLister.List(labels.SelectorFromSet(machineSet.Spec.Selector.MatchLabels))
		if err != nil {
			return err
		}

		for _, machine := range filteredMachines {
			if err := dc.labelNodeForMachine(ctx, machine, labelKey, labelValue); err != nil {
				return err
			}
		}

		klog.V(3).Infof("Labeled nodes belonging to MachineSet %q with %v", machineSet.Name, labelKey)
	}

	return nil
}

func (dc *controller) labelNodeForMachine(ctx context.Context, machine *v1alpha1.Machine, labelKey, labelValue string) error {
	if machine.Labels[v1alpha1.NodeLabelKey] == "" {
		klog.V(3).Infof("Node label not found for machine %s", machine.Name)
		return nil
	}

	node, err := dc.nodeLister.Get(machine.Labels[v1alpha1.NodeLabelKey])
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil // Node is not found, continue to the next machine
		}
		klog.Errorf("Error occurred while trying to fetch node object: %v", err)
		return err
	}
	if node.Labels[labelKey] == labelValue {
		return nil
	}

	nodeCopy := node.DeepCopy()
	nodeCopy.Labels = labelsutil.AddLabel(nodeCopy.Labels, labelKey, labelValue)
	if _, err := dc.targetCoreClient.CoreV1().Nodes().Update(ctx, nodeCopy, metav1.UpdateOptions{}); err != nil {
		return err
	}

	return nil
}

func (dc *controller) labelMachinesToSelectedForUpdate(ctx context.Context, machineSet *v1alpha1.MachineSet, drainCount int32) (int32, error) {
	numOfMachinesSelectedForUpdate := int32(0)

	machines, err := dc.getMachinesForDrain(machineSet, drainCount)
	if err != nil {
		return numOfMachinesSelectedForUpdate, err
	}

	klog.V(3).Infof("machines selected for drain %v", machines)

	for _, machine := range machines {
		// labels on the node are added cumulatively and we can find both candidate-for-update and selected-for-update labels on the node.
		if err := dc.labelNodeForMachine(ctx, machine, v1alpha1.LabelKeyNodeSelectedForUpdate, "true"); err != nil {
			return numOfMachinesSelectedForUpdate, err
		}
		numOfMachinesSelectedForUpdate++
	}

	return numOfMachinesSelectedForUpdate, nil
}

func (dc *controller) getMachinesUndergoingUpdate(oldMachineSets []*v1alpha1.MachineSet) (int32, error) {
	machineInUpdateProcess := int32(0)
	for _, machineSet := range oldMachineSets {
		machines, err := dc.machineLister.List(labels.SelectorFromSet(machineSet.Spec.Selector.MatchLabels))
		if err != nil {
			return 0, err
		}

		for _, machine := range machines {
			if machine.Labels[v1alpha1.NodeLabelKey] == "" {
				continue
			}

			node, err := dc.nodeLister.Get(machine.Labels[v1alpha1.NodeLabelKey])
			if err != nil {
				return machineInUpdateProcess, err
			}

			if _, ok := node.Labels[v1alpha1.LabelKeyNodeSelectedForUpdate]; ok {
				machineInUpdateProcess++
			}
		}
	}

	return machineInUpdateProcess, nil
}

func (dc *controller) getMachinesForDrain(machineSet *v1alpha1.MachineSet, readyForDrain int32) ([]*v1alpha1.Machine, error) {
	machines, err := dc.machineLister.List(labels.SelectorFromSet(machineSet.Spec.Selector.MatchLabels))
	if err != nil {
		return nil, err
	}

	var candidateForUpdateMachines []*v1alpha1.Machine
	for _, machine := range machines {
		if machine.Labels[v1alpha1.NodeLabelKey] == "" {
			continue
		}

		node, err := dc.nodeLister.Get(machine.Labels[v1alpha1.NodeLabelKey])
		if err != nil {
			return candidateForUpdateMachines, err
		}

		if _, ok := node.Labels[v1alpha1.LabelKeyNodeCandidateForUpdate]; ok {
			if _, ok := node.Labels[v1alpha1.LabelKeyNodeSelectedForUpdate]; !ok {
				candidateForUpdateMachines = append(candidateForUpdateMachines, machine)
			}
			if len(candidateForUpdateMachines) == int(readyForDrain) {
				return candidateForUpdateMachines, nil
			}
		}
	}

	return candidateForUpdateMachines, nil
}

// labelMachineSets label all the machineSets with the given label
func (dc *controller) labelMachineSets(ctx context.Context, MachineSets []*v1alpha1.MachineSet, labels map[string]string) error {
	for _, machineSet := range MachineSets {

		if machineSet == nil {
			continue
		}

		labels := MergeStringMaps(machineSet.Labels, labels)
		formattedLabels, err := labelsutil.GetFormattedLabels(labels)
		if err != nil {
			return err
		}

		addLabelPatch := fmt.Sprintf(`{"metadata":{"labels":%s}}`, string(formattedLabels))

		if err := dc.machineSetControl.PatchMachineSet(ctx, machineSet.Namespace, machineSet.Name, []byte(addLabelPatch)); err != nil {
			return fmt.Errorf("failed to label MachineSet %s: %w", machineSet.Name, err)
		}
	}

	return nil
}

func isUpdateNotSuccessful(condition *v1.NodeCondition, labels map[string]string) bool {
	return condition == nil || condition.Reason != v1alpha1.UpdateSuccessful || labels[v1alpha1.LabelKeyNodeUpdateResult] != v1alpha1.LabelValueNodeUpdateSuccessful
}
