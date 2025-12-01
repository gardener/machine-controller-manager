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
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

/*
SECTION
Machine controller - nodeToMachine
*/
var (
	errMultipleMachineMatch = errors.New("multiple machines matching node")
	errNoMachineMatch       = errors.New("no machines matching node found")
)

func (c *controller) addNode(obj any) {
	node := obj.(*corev1.Node)
	if node == nil {
		klog.Errorf("Couldn't convert to node from object")
		return
	}

	// If NotManagedByMCM annotation is present on node, don't process this node object
	if _, annotationPresent := node.ObjectMeta.Annotations[machineutils.NotManagedByMCM]; annotationPresent {
		return
	}

	if node.DeletionTimestamp != nil {
		delRetry, err := c.triggerMachineDeletion(context.Background(), node.Name)
		if err != nil {
			c.enqueueNodeAfter(node, time.Duration(delRetry), fmt.Sprintf("handling node UPDATE event. node %q marked for deletion", node.Name))
		}
		return
	}

	c.enqueueNode(node, "handling ADD event for node")
}

func (c *controller) updateNode(oldObj, newObj any) {
	oldNode := oldObj.(*corev1.Node)
	node := newObj.(*corev1.Node)
	if oldNode == nil || node == nil {
		klog.Errorf("Couldn't convert to node from object")
		return
	}

	// If NotManagedByMCM annotation is present on node, don't process this node object
	if _, annotationPresent := node.ObjectMeta.Annotations[machineutils.NotManagedByMCM]; annotationPresent {
		return
	}

	machine, err := c.getMachineFromNode(node.Name)
	if err != nil {
		klog.Errorf("Unable to handle update event for node %s, couldn't fetch associated machine. Error: %s", node.Name, err)
		return
	}

	// delete the machine if the node is deleted
	if node.DeletionTimestamp != nil {
		delRetry, err := c.triggerMachineDeletion(context.Background(), node.Name)
		if err != nil {
			c.enqueueNodeAfter(node, time.Duration(delRetry), fmt.Sprintf("handling node UPDATE event. backing node %q marked for deletion", node.Name))
		}
		return
	}

	// to reconcile on addition/removal of essential taints in machine lifecycle, example - critical component taint
	if addedOrRemovedEssentialTaints(oldNode, node, machineutils.EssentialTaints) {
		c.enqueueMachine(machine, fmt.Sprintf("handling node UPDATE event. atleast one of essential taints on backing node %q has changed", getNodeName(machine)))
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
		c.enqueueMachine(machine, fmt.Sprintf("handling node UPDATE event. conditions of backing node %q differ from machine status", node.Name))
	}
}

func (c *controller) deleteNode(obj any) {
	node, ok := obj.(*corev1.Node)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			klog.Errorf("Couldn't get object from tombstone %+v", obj)
			return
		}
		node, ok = tombstone.Obj.(*corev1.Node)
		if !ok {
			klog.Errorf("Tombstone contained object that is not a Node %+v", obj)
			return
		}
	}

	// If NotManagedByMCM annotation is present on node, don't process this node object
	if _, annotationPresent := node.ObjectMeta.Annotations[machineutils.NotManagedByMCM]; annotationPresent {
		return
	}

	machine, err := c.getMachineFromNode(node.Name)
	if err != nil {
		klog.V(4).Infof("Couldn't fetch associated machine for deleted node %s: %v", node.Name, err)
		return
	}

	// Trigger machine deletion if not already marked for deletion
	if machine.DeletionTimestamp == nil {
		klog.Infof("Node %s deleted, triggering deletion of machine %s", node.Name, machine.Name)
		if err := c.controlMachineClient.Machines(c.namespace).Delete(context.Background(), machine.Name, metav1.DeleteOptions{}); err != nil {
			klog.Errorf("Failed to delete machine %s for deleted node %s: %v", machine.Name, node.Name, err)
		}
	}
}

func (c *controller) reconcileClusterNodeKey(key string) error {
	ctx := context.Background()
	node, err := c.nodeLister.Get(key)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.Infof("ClusterNode %q: Node object not found in store, might have been deleted", key)
			return nil
		}
		klog.Errorf("ClusterNode %q: Unable to retrieve object from store: %v", key, err)
		return err
	}

	// If NotManagedByMCM annotation is present on node, don't process this node object
	if _, annotationPresent := node.ObjectMeta.Annotations[machineutils.NotManagedByMCM]; annotationPresent {
		return nil
	}

	//Add finalizers to node object if not present
	retryPeriod, err := c.addNodeFinalizers(ctx, node)

	var reEnqueReason = "periodic reconcile"
	if err != nil {
		reEnqueReason = err.Error()
	}
	c.enqueueNodeAfter(node, time.Duration(retryPeriod), reEnqueReason)

	return nil
}

// triggerMachineDeletion triggers deletion for the machine associated with the given node name.
func (c *controller) triggerMachineDeletion(ctx context.Context, nodeName string) (machineutils.RetryPeriod, error) {
	machine, err := c.getMachineFromNode(nodeName)
	if err != nil {
		klog.Infof("Couldn't fetch associated machine for node %s: %v", nodeName, err)
		return machineutils.LongRetry, nil
	}

	if machine.DeletionTimestamp == nil {
		klog.Infof("Node %s for machine %s has been deleted. Triggering machine deletion flow.", nodeName, machine.Name)
		if err := c.controlMachineClient.Machines(c.namespace).Delete(ctx, machine.Name, metav1.DeleteOptions{}); err != nil {
			klog.Errorf("Machine object %s backing the deleted node %s could not be marked for deletion. Error: %s", machine.Name, nodeName, err)
			return machineutils.ShortRetry, err
		}
		klog.Infof("Machine object %s backing the deleted node %s marked for deletion.", machine.Name, nodeName)
	} else {
		// Enqueue for termination if already marked for deletion
		c.enqueueMachineTermination(machine, fmt.Sprintf("backing node obj %q got deleted", nodeName))
	}
	return machineutils.LongRetry, nil
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

func (c *controller) addNodeFinalizers(ctx context.Context, node *corev1.Node) (machineutils.RetryPeriod, error) {
	if finalizers := sets.NewString(node.Finalizers...); !finalizers.Has(NodeFinalizer) {
		finalizers.Insert(NodeFinalizer)
		if err := c.updateNodeFinalizers(ctx, node, finalizers.List()); err != nil {
			return machineutils.ShortRetry, err
		}
		klog.Infof("Added finalizer to node %q", node.Name)
		return machineutils.LongRetry, nil
	}
	// Do not treat case where finalizer is already present as an error
	return machineutils.LongRetry, nil
}

func (c *controller) removeNodeFinalizers(ctx context.Context, node *corev1.Node) (machineutils.RetryPeriod, error) {
	if finalizers := sets.NewString(node.Finalizers...); finalizers.Has(NodeFinalizer) {
		finalizers.Delete(NodeFinalizer)
		if err := c.updateNodeFinalizers(ctx, node, finalizers.List()); err != nil {
			return machineutils.ShortRetry, err
		}
		klog.Infof("Removed finalizer from node %q", node.Name)
		return machineutils.ShortRetry, nil
	}
	return machineutils.ShortRetry, fmt.Errorf("node finalizer not found on node %q", node.Name)
}

// updateNodeFinalizers updates the node finalizers using strategic merge patch
func (c *controller) updateNodeFinalizers(ctx context.Context, node *corev1.Node, finalizers []string) error {
	oldData, err := json.Marshal(node)
	if err != nil {
		return fmt.Errorf("failed to marshal old node %#v for node %q: %v", node, node.Name, err)
	}

	newNode := node.DeepCopy()
	newNode.Finalizers = finalizers
	newData, err := json.Marshal(newNode)
	if err != nil {
		return fmt.Errorf("failed to marshal new node %#v for node %q: %v", newNode, node.Name, err)
	}

	// Create the strategic merge patch
	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, corev1.Node{})
	if err != nil {
		return fmt.Errorf("failed to create patch for node %q: %v", node.Name, err)
	}
	_, err = c.targetCoreClient.CoreV1().Nodes().Patch(ctx, node.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		klog.Errorf("Failed to patch finalizers for node %q: %s", node.Name, err)
		return err
	}

	return nil
}
