// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package driver

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machineutils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

const labelMachineClassName = "machine-class"

func (d *DriverImpl) initializeMCMManagedNodes(ctx context.Context) error {
	// Add only existing nodes having a non-empty 'machine-name' label on them
	nodeList, err := d.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf(
			"%s,%s!=", machineutils.MachineLabelKey, machineutils.MachineLabelKey,
		),
	})
	if err != nil {
		return err
	}
	for _, n := range nodeList.Items {
		d.managedNodes.Store(n.Name, managedNodeInfo{
			ProviderID:       n.Spec.ProviderID,
			MachineClassName: n.Labels[labelMachineClassName],
		})
	}
	return nil
}

func (d *DriverImpl) buildNode(mc *v1alpha1.Machine, mcc *v1alpha1.MachineClass) *corev1.Node {
	var node corev1.Node
	node.Name = mc.Name
	node.Labels = addNodeLabels(mc, mcc)
	node.Status = buildNodeStatus(mcc)
	node.Spec.ProviderID = fmt.Sprintf("fake://%s:%s", mcc.NodeTemplate.Region, mc.Name)
	return &node
}

func (d *DriverImpl) transitionNodeToReady(ctx context.Context, node *corev1.Node) (updatedNode *corev1.Node, err error) {
	node.Spec.Taints = slices.DeleteFunc(node.Spec.Taints, func(taint corev1.Taint) bool {
		return taint.Key == corev1.TaintNodeNotReady
	})

	updatedNode, err = d.client.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
	if err != nil {
		err = fmt.Errorf("cannot update node with name %q: %w", node.Name, err)
		return
	}
	updatedNode.Status.Conditions = buildNodeReadyConditions(corev1.ConditionTrue)
	updatedNode.Status.Phase = corev1.NodeRunning

	updatedNode, err = d.client.CoreV1().Nodes().UpdateStatus(ctx, updatedNode, metav1.UpdateOptions{})
	if err != nil {
		err = fmt.Errorf("cannot update the status of node with name %q: %w", node.Name, err)
	}
	return
}

func addNodeLabels(mc *v1alpha1.Machine, mcc *v1alpha1.MachineClass) map[string]string {
	labels := map[string]string{
		machineutils.MachineLabelKey:   mc.Name,
		corev1.LabelHostname:           mc.Name,
		corev1.LabelOSStable:           "linux",
		corev1.LabelInstanceTypeStable: mcc.NodeTemplate.InstanceType,
		corev1.LabelArchStable:         ptr.Deref(mcc.NodeTemplate.Architecture, ""),
		corev1.LabelTopologyRegion:     mcc.NodeTemplate.Region,
		// This is required so that `ListMachines` can filter the tracked machines based on
		// requested `MachineClass`
		labelMachineClassName: mcc.Name,
	}

	maps.Copy(labels, mc.Spec.NodeTemplateSpec.Labels)
	return labels
}

func buildNodeStatus(mcc *v1alpha1.MachineClass) corev1.NodeStatus {
	status := corev1.NodeStatus{
		Capacity: maps.Clone(mcc.NodeTemplate.Capacity),
	}
	// Prevents the scheduler from throwing `NodeResourcesFit` scheduling failures
	status.Capacity[corev1.ResourcePods] = resource.MustParse("110")

	status.Allocatable = maps.Clone(status.Capacity)

	// TODO: (takoverflow) when simulation failures are introduced, this would be initially
	// False (i.e. node is NotReady) and depending on the scenario may/may not become True.
	status.Conditions = buildNodeReadyConditions(corev1.ConditionFalse)
	status.Phase = corev1.NodePending

	return status
}

func buildNodeReadyConditions(readyConditionStatus corev1.ConditionStatus) []corev1.NodeCondition {
	currentTime := metav1.NewTime(time.Now())
	return []corev1.NodeCondition{
		{
			Type:               corev1.NodeReady,
			Status:             readyConditionStatus,
			LastTransitionTime: currentTime,
			LastHeartbeatTime:  currentTime,
		},
		{
			Type:               corev1.NodeNetworkUnavailable,
			Status:             corev1.ConditionFalse,
			LastTransitionTime: currentTime,
			LastHeartbeatTime:  currentTime,
		},
		{
			Type:               corev1.NodeDiskPressure,
			Status:             corev1.ConditionFalse,
			LastTransitionTime: currentTime,
			LastHeartbeatTime:  currentTime,
		},
		{
			Type:               corev1.NodeMemoryPressure,
			Status:             corev1.ConditionFalse,
			LastTransitionTime: currentTime,
			LastHeartbeatTime:  currentTime,
		},
	}
}
