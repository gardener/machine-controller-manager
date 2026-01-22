// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Package controller is used to provide the core functionalities of machine-controller-manager
package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machineutils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	"k8s.io/client-go/tools/cache"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

var (
	errMultipleMachineMatch = errors.New("multiple machines matching node")
	errNoMachineMatch       = errors.New("no machines matching node found")
)

func (c *controller) addNode(obj any) {
	node, ok := obj.(*corev1.Node)
	if !ok {
		klog.Errorf("couldn't convert to node from object")
		return
	}

	// If NotManagedByMCM annotation is present on node, don't process this node object
	if _, annotationPresent := node.ObjectMeta.Annotations[machineutils.NotManagedByMCM]; annotationPresent {
		klog.V(4).Infof("NotManagedByMCM annotation present on node %q, skipping ADD event", node.Name)
		return
	}
	c.enqueueNode(node, "handling ADD event for node")
}

func (c *controller) updateNode(oldObj, newObj any) {
	oldNode := oldObj.(*corev1.Node)
	node := newObj.(*corev1.Node)
	if oldNode == nil || node == nil {
		klog.Errorf("couldn't convert to node from object")
		return
	}

	// If NotManagedByMCM annotation is present on node, don't process this node object
	if _, annotationPresent := node.ObjectMeta.Annotations[machineutils.NotManagedByMCM]; annotationPresent {
		klog.V(4).Infof("NotManagedByMCM annotation present on node %q, skipping UPDATE event", node.Name)
		return
	}

	// Do not process node updates if there is no associated machine
	// In case of transient errors while fetching machine, do not retry
	// as the update handler will be triggered again due to kubelet updates.
	machine, err := c.getMachineFromNode(node.Name)
	if err != nil {
		klog.Errorf("unable to handle update event for node %q, couldn't fetch associated machine. Error: %v", node.Name, err)
		return
	}

	// delete the machine if the node is deleted
	if node.DeletionTimestamp != nil {
		err := c.triggerMachineDeletion(context.Background(), node.Name)
		if err != nil {
			c.enqueueNodeAfter(node, time.Duration(machineutils.ShortRetry), fmt.Sprintf("handling node UPDATE event. Failed to trigger machine deletion for node %q, re-queuing", node.Name))
		}
		return
	}

	if !HasFinalizer(node, NodeFinalizerName) {
		c.enqueueNodeAfter(node, time.Duration(machineutils.MediumRetry), fmt.Sprintf("MCM finalizer missing from node %q, re-queuing", node.Name))
		return
	}
	// to reconcile on addition/removal of essential taints in machine lifecycle, example - critical component taint
	if addedOrRemovedEssentialTaints(oldNode, node, machineutils.EssentialTaints) {
		c.enqueueMachine(machine, fmt.Sprintf("handling node UPDATE event. Atleast one of essential taints on node %q has changed", getNodeName(machine)))
		return
	}
	if inPlaceUpdateLabelsChanged(oldNode, node) {
		c.enqueueMachine(machine, fmt.Sprintf("handling node UPDATE event. in-place update label added or updated for node %q", getNodeName(machine)))
		return
	}
	isMachineCrashLooping := machine.Status.CurrentStatus.Phase == v1alpha1.MachineCrashLoopBackOff
	isMachineTerminating := machine.Status.CurrentStatus.Phase == v1alpha1.MachineTerminating
	_, _, nodeConditionsHaveChanged := nodeConditionsHaveChanged(machine.Status.Conditions, node.Status.Conditions)

	// Enqueue machine if node conditions have changed and machine is not in crashloop or terminating state
	if nodeConditionsHaveChanged && !(isMachineCrashLooping || isMachineTerminating) {
		c.enqueueMachine(machine, fmt.Sprintf("handling node UPDATE event. Conditions of node %q differ from machine status", node.Name))
	}
}

func (c *controller) deleteNode(obj any) {
	node, ok := obj.(*corev1.Node)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			klog.Errorf("couldn't get object from tombstone %+v", obj)
			return
		}
		node, ok = tombstone.Obj.(*corev1.Node)
		if !ok {
			klog.Errorf("tombstone contained object that is not a node %+v", obj)
			return
		}
	}

	// If NotManagedByMCM annotation is present on node, don't process this node object
	if _, annotationPresent := node.ObjectMeta.Annotations[machineutils.NotManagedByMCM]; annotationPresent {
		klog.V(4).Infof("NotManagedByMCM annotation present on node %q, skipping DELETE event", node.Name)
		return
	}

	err := c.triggerMachineDeletion(context.Background(), node.Name)
	if err != nil {
		klog.Errorf("ClusterNode %q: error triggering machine deletion for deleted node: %v", node.Name, err)
	}
}

func (c *controller) reconcileClusterNodeKey(key string) error {
	ctx := context.Background()
	node, err := c.nodeLister.Get(key)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.Errorf("ClusterNode %q: Node object not found in store, might have been deleted", key)
			return nil
		}
		klog.Errorf("ClusterNode %q: Unable to retrieve object from store: %v", key, err)
		return err
	}

	// If NotManagedByMCM annotation is present on node, don't process this node object
	if _, annotationPresent := node.ObjectMeta.Annotations[machineutils.NotManagedByMCM]; annotationPresent {
		klog.Infof("ClusterNode %q: NotManagedByMCM annotation present, skipping reconciliation", key)
		return nil
	}

	// Ignore node updates without an associated machine. Retry only for errors other than errNoMachineMatch;
	// transient fetch errors will be eventually requeued by the update handler.
	if _, err := c.getMachineFromNode(node.Name); err != nil {
		if errors.Is(err, errNoMachineMatch) {
			klog.Errorf("ClusterNode %q: No machine found matching node, skipping adding finalizers", key)
			return nil
		}
		klog.Errorf("ClusterNode %q: error fetching machine for node: %v", key, err)
		return err
	}

	if node.DeletionTimestamp != nil {
		err := c.triggerMachineDeletion(context.Background(), node.Name)
		if err != nil {
			klog.Errorf("ClusterNode %q: error triggering machine deletion for deleted node: %v", key, err)
			// Rate-limited requeue prevents tight looping when machine object
			// was manually deleted (finalizer removed) before creationTimeout,
			// as orphan VM collector will not take any action.
			return err
		}
		return nil
	}

	//Add finalizers to node object if not present
	err = c.addNodeFinalizers(ctx, node)
	if err != nil {
		klog.Errorf("ClusterNode %q: error adding finalizers to node: %v", key, err)
		c.enqueueNodeAfter(node, time.Duration(machineutils.ShortRetry), err.Error())
	}

	return nil
}

// triggerMachineDeletion triggers deletion for the machine associated with the given node name.
func (c *controller) triggerMachineDeletion(ctx context.Context, nodeName string) error {
	machine, err := c.getMachineFromNode(nodeName)
	if err != nil {
		klog.Errorf("couldn't fetch associated machine for node %s: %v", nodeName, err)
		return err
	}

	if machine.DeletionTimestamp == nil {
		klog.Infof("Node %q for machine %q has been deleted. Triggering machine deletion flow.", nodeName, machine.Name)
		if err := c.controlMachineClient.Machines(c.namespace).Delete(ctx, machine.Name, metav1.DeleteOptions{}); err != nil {
			klog.Errorf("machine object %q backing the deleted node %q could not be marked for deletion. Error: %s", machine.Name, nodeName, err)
			return err
		}
	}
	return nil
}

/*
	SECTION
	Node utils
*/

func (c *controller) enqueueNode(obj any, reason string) {
	if key, ok := c.getKeyForObj(obj); ok {
		klog.Infof("Adding node object to queue %q, reason: %s", key, reason)
		c.nodeQueue.Add(key)
	}
}

func (c *controller) enqueueNodeAfter(obj any, after time.Duration, reason string) {
	if key, ok := c.getKeyForObj(obj); ok {
		klog.Infof("Adding node object to queue %q after %s, reason: %s", key, after, reason)
		c.nodeQueue.AddAfter(key, after)
	}
}

func (c *controller) getMachineFromNode(nodeName string) (*v1alpha1.Machine, error) {
	var (
		list     = []string{nodeName}
		selector = labels.NewSelector()
		req, _   = labels.NewRequirement("node", selection.Equals, list)
	)

	selector = selector.Add(*req)
	machines, _ := c.machineLister.List(selector)

	if len(machines) > 1 {
		return nil, errMultipleMachineMatch
	} else if len(machines) < 1 {
		return nil, errNoMachineMatch
	}

	return machines[0], nil
}

func (c *controller) addNodeFinalizers(ctx context.Context, node *corev1.Node) error {
	if !HasFinalizer(node, NodeFinalizerName) {
		finalizers := sets.NewString(node.Finalizers...)
		finalizers.Insert(NodeFinalizerName)
		if err := c.updateNodeFinalizers(ctx, node, finalizers.List()); err != nil {
			return err
		}
		klog.Infof("Added finalizer %q to node %q", NodeFinalizerName, node.Name)
		return nil
	}
	// Do not treat case where finalizer is already present as an error
	return nil
}

func (c *controller) removeNodeFinalizers(ctx context.Context, node *corev1.Node) error {
	if HasFinalizer(node, NodeFinalizerName) {
		finalizers := sets.NewString(node.Finalizers...)
		finalizers.Delete(NodeFinalizerName)
		if err := c.updateNodeFinalizers(ctx, node, finalizers.List()); err != nil {
			return err
		}
		klog.Infof("Removed finalizer %q from node %q", NodeFinalizerName, node.Name)
		return nil
	}
	return nil
}

// updateNodeFinalizers updates the node finalizers using merge patch
func (c *controller) updateNodeFinalizers(ctx context.Context, node *corev1.Node, finalizers []string) error {
	patch := map[string]any{
		"metadata": map[string]any{
			"finalizers": finalizers,
		},
	}
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("failed to marshal patch for node %q: %v", node.Name, err)
	}

	_, err = c.targetCoreClient.CoreV1().Nodes().Patch(ctx, node.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		klog.Errorf("failed to patch finalizers for node %q: %s", node.Name, err)
		return err
	}

	return nil
}

// HasFinalizer checks if the given object has the specified finalizer.
func HasFinalizer(o metav1.Object, finalizer string) bool {
	return sets.NewString(o.GetFinalizers()...).Has(finalizer)
}

func inPlaceUpdateLabelsChanged(oldNode, node *corev1.Node) bool {
	if oldNode == nil || node == nil {
		return false
	}

	labelKeys := []string{
		v1alpha1.LabelKeyNodeCandidateForUpdate,
		v1alpha1.LabelKeyNodeSelectedForUpdate,
		v1alpha1.LabelKeyNodeUpdateResult,
	}

	for _, key := range labelKeys {
		oldVal, oldOk := oldNode.Labels[key]
		newVal, newOk := node.Labels[key]
		if (!oldOk && newOk) || (key == v1alpha1.LabelKeyNodeUpdateResult && oldVal != newVal) {
			return true
		}
	}

	return false
}

func addedOrRemovedEssentialTaints(oldNode, node *corev1.Node, taintKeys []string) bool {
	mapOldNodeTaintKeys := make(map[string]bool)
	mapNodeTaintKeys := make(map[string]bool)

	for _, t := range oldNode.Spec.Taints {
		mapOldNodeTaintKeys[t.Key] = true
	}

	for _, t := range node.Spec.Taints {
		mapNodeTaintKeys[t.Key] = true
	}

	for _, tk := range taintKeys {
		_, oldNodeHasTaint := mapOldNodeTaintKeys[tk]
		_, newNodeHasTaint := mapNodeTaintKeys[tk]
		if oldNodeHasTaint != newNodeHasTaint {
			klog.V(2).Infof("Taint with key %q has been added/removed from the node %q", tk, node.Name)
			return true
		}
	}
	return false
}
