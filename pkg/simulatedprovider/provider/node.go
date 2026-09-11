// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"context"
	"fmt"
	"maps"
	"time"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machinecodes/codes"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machinecodes/status"
	controller "github.com/gardener/machine-controller-manager/pkg/util/provider/machinecontroller"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machineutils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
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
	node.Finalizers = []string{controller.NodeFinalizerName}
	node.Labels = addNodeLabels(mc, mcc)
	node.Status = buildNodeStatus(mcc)
	node.Spec.ProviderID = fmt.Sprintf("fake://%s:%s", mcc.NodeTemplate.Region, mc.Name)
	return &node
}

func (d *DriverImpl) transitionNodeToReady(ctx context.Context, name string) (err error) {
	backoff := wait.Backoff{
		Duration: 500 * time.Millisecond,
		Factor:   1.5,
		Jitter:   0.1,
		Steps:    10,
		Cap:      10 * time.Second,
	}

	var node *corev1.Node
	err = retry.OnError(backoff, func(error) bool { return true }, func() error {
		node, err = d.client.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("cannot get node with name %q: %w", name, err)

		}
		for i, cond := range node.Status.Conditions {
			if cond.Type == corev1.NodeReady {
				node.Status.Conditions[i].Status = corev1.ConditionTrue
			}
		}
		node.Status.Phase = corev1.NodeRunning

		_, err = d.client.CoreV1().Nodes().UpdateStatus(ctx, node, metav1.UpdateOptions{})
		return err
	})

	if err != nil {
		// If the node cannot be transitioned to 'Ready' state, return an 'Internal' error.
		return status.Error(codes.Internal, err.Error())
	}

	klog.Infof("Node %q is ready", node.Name)
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
