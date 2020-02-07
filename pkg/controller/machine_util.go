/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

This file was copied and modified from the kubernetes/kubernetes project
https://github.com/kubernetes/kubernetes/release-1.8/pkg/controller/deployment/util/pod_util.go

Modifications Copyright (c) 2017 SAP SE or an SAP affiliate company. All rights reserved.
*/

// Package controller is used to provide the core functionalities of machine-controller-manager
package controller

import (
	"encoding/json"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/validation"
	"k8s.io/klog"

	machineapi "github.com/gardener/machine-controller-manager/pkg/apis/machine"
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	v1alpha1client "github.com/gardener/machine-controller-manager/pkg/client/clientset/versioned/typed/machine/v1alpha1"
	v1alpha1listers "github.com/gardener/machine-controller-manager/pkg/client/listers/machine/v1alpha1"
	v1 "k8s.io/api/core/v1"
	errorsutil "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/util/retry"
)

const (
	// LastAppliedALTAnnotation contains the last configuration of annotations, labels & taints applied on the node object
	LastAppliedALTAnnotation = "node.machine.sapcloud.io/last-applied-anno-labels-taints"
)

var (
	// emptyMap is a dummy emptyMap to compare with
	emptyMap = make(map[string]string)
)

// TODO: use client library instead when it starts to support update retries
//       see https://github.com/kubernetes/kubernetes/issues/21479
type updateMachineFunc func(machine *v1alpha1.Machine) error

// UpdateMachineWithRetries updates a machine with given applyUpdate function. Note that machine not found error is ignored.
// The returned bool value can be used to tell if the machine is actually updated.
func UpdateMachineWithRetries(machineClient v1alpha1client.MachineInterface, machineLister v1alpha1listers.MachineLister, namespace, name string, applyUpdate updateMachineFunc) (*v1alpha1.Machine, error) {
	var machine *v1alpha1.Machine

	retryErr := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		var err error
		machine, err = machineLister.Machines(namespace).Get(name)
		if err != nil {
			return err
		}
		machine = machine.DeepCopy()
		// Apply the update, then attempt to push it to the apiserver.
		if applyErr := applyUpdate(machine); applyErr != nil {
			return applyErr
		}
		machine, err = machineClient.Update(machine)
		return err
	})

	// Ignore the precondition violated error, this machine is already updated
	// with the desired label.
	if retryErr == errorsutil.ErrPreconditionViolated {
		klog.V(4).Infof("Machine %s precondition doesn't hold, skip updating it.", name)
		retryErr = nil
	}

	return machine, retryErr
}

func (c *controller) validateMachineClass(classSpec *v1alpha1.ClassSpec) (interface{}, *v1.Secret, error) {

	var MachineClass interface{}
	var secretRef *v1.Secret

	switch classSpec.Kind {
	case "AWSMachineClass":
		AWSMachineClass, err := c.awsMachineClassLister.AWSMachineClasses(c.namespace).Get(classSpec.Name)
		if err != nil {
			klog.V(2).Infof("AWSMachineClass %q/%q not found. Skipping. %v", c.namespace, classSpec.Name, err)
			return MachineClass, secretRef, err
		}
		MachineClass = AWSMachineClass

		// Validate AWSMachineClass
		internalAWSMachineClass := &machineapi.AWSMachineClass{}
		err = c.internalExternalScheme.Convert(AWSMachineClass, internalAWSMachineClass, nil)
		if err != nil {
			klog.V(2).Info("Error in scheme conversion")
			return MachineClass, secretRef, err
		}

		validationerr := validation.ValidateAWSMachineClass(internalAWSMachineClass)
		if validationerr.ToAggregate() != nil && len(validationerr.ToAggregate().Errors()) > 0 {
			klog.V(2).Infof("Validation of AWSMachineClass failed %s", validationerr.ToAggregate().Error())
			return MachineClass, secretRef, nil
		}

		// Get secretRef
		secretRef, err = c.getSecret(AWSMachineClass.Spec.SecretRef, AWSMachineClass.Name)
		if err != nil || secretRef == nil {
			klog.V(2).Info("Secret reference not found")
			return MachineClass, secretRef, err
		}
	case "AzureMachineClass":
		AzureMachineClass, err := c.azureMachineClassLister.AzureMachineClasses(c.namespace).Get(classSpec.Name)
		if err != nil {
			klog.V(2).Infof("AzureMachineClass %q not found. Skipping. %v", classSpec.Name, err)
			return MachineClass, secretRef, err
		}
		MachineClass = AzureMachineClass

		// Validate AzureMachineClass
		internalAzureMachineClass := &machineapi.AzureMachineClass{}
		err = c.internalExternalScheme.Convert(AzureMachineClass, internalAzureMachineClass, nil)
		if err != nil {
			klog.V(2).Info("Error in scheme conversion")
			return MachineClass, secretRef, err
		}

		validationerr := validation.ValidateAzureMachineClass(internalAzureMachineClass)
		if validationerr.ToAggregate() != nil && len(validationerr.ToAggregate().Errors()) > 0 {
			klog.V(2).Infof("Validation of AzureMachineClass failed %s", validationerr.ToAggregate().Error())
			return MachineClass, secretRef, nil
		}

		// Get secretRef
		secretRef, err = c.getSecret(AzureMachineClass.Spec.SecretRef, AzureMachineClass.Name)
		if err != nil || secretRef == nil {
			klog.V(2).Info("Secret reference not found")
			return MachineClass, secretRef, err

		}
	case "GCPMachineClass":
		GCPMachineClass, err := c.gcpMachineClassLister.GCPMachineClasses(c.namespace).Get(classSpec.Name)
		if err != nil {
			klog.V(2).Infof("GCPMachineClass %q not found. Skipping. %v", classSpec.Name, err)
			return MachineClass, secretRef, err
		}
		MachineClass = GCPMachineClass

		// Validate GCPMachineClass
		internalGCPMachineClass := &machineapi.GCPMachineClass{}
		err = c.internalExternalScheme.Convert(GCPMachineClass, internalGCPMachineClass, nil)
		if err != nil {
			klog.V(2).Info("Error in scheme conversion")
			return MachineClass, secretRef, err
		}

		validationerr := validation.ValidateGCPMachineClass(internalGCPMachineClass)
		if validationerr.ToAggregate() != nil && len(validationerr.ToAggregate().Errors()) > 0 {
			klog.V(2).Infof("Validation of GCPMachineClass failed %s", validationerr.ToAggregate().Error())
			return MachineClass, secretRef, nil
		}

		// Get secretRef
		secretRef, err = c.getSecret(GCPMachineClass.Spec.SecretRef, GCPMachineClass.Name)
		if err != nil || secretRef == nil {
			klog.V(2).Info("Secret reference not found")
			return MachineClass, secretRef, err
		}
	case "OpenStackMachineClass":
		OpenStackMachineClass, err := c.openStackMachineClassLister.OpenStackMachineClasses(c.namespace).Get(classSpec.Name)
		if err != nil {
			klog.V(2).Infof("OpenStackMachineClass %q not found. Skipping. %v", classSpec.Name, err)
			return MachineClass, secretRef, err
		}
		MachineClass = OpenStackMachineClass

		// Validate OpenStackMachineClass
		internalOpenStackMachineClass := &machineapi.OpenStackMachineClass{}
		err = c.internalExternalScheme.Convert(OpenStackMachineClass, internalOpenStackMachineClass, nil)
		if err != nil {
			klog.V(2).Info("Error in scheme conversion")
			return MachineClass, secretRef, err
		}

		validationerr := validation.ValidateOpenStackMachineClass(internalOpenStackMachineClass)
		if validationerr.ToAggregate() != nil && len(validationerr.ToAggregate().Errors()) > 0 {
			klog.V(2).Infof("Validation of OpenStackMachineClass failed %s", validationerr.ToAggregate().Error())
			return MachineClass, secretRef, nil
		}

		// Get secretRef
		secretRef, err = c.getSecret(OpenStackMachineClass.Spec.SecretRef, OpenStackMachineClass.Name)
		if err != nil || secretRef == nil {
			klog.V(2).Info("Secret reference not found")
			return MachineClass, secretRef, err
		}
	case "AlicloudMachineClass":
		AlicloudMachineClass, err := c.alicloudMachineClassLister.AlicloudMachineClasses(c.namespace).Get(classSpec.Name)
		if err != nil {
			klog.V(2).Infof("AlicloudMachineClass %q/%q not found. Skipping. %v", c.namespace, classSpec.Name, err)
			return MachineClass, secretRef, err
		}
		MachineClass = AlicloudMachineClass

		// Validate AlicloudMachineClass
		internalAlicloudMachineClass := &machineapi.AlicloudMachineClass{}
		err = c.internalExternalScheme.Convert(AlicloudMachineClass, internalAlicloudMachineClass, nil)
		if err != nil {
			klog.V(2).Info("Error in scheme conversion")
			return MachineClass, secretRef, err
		}

		validationerr := validation.ValidateAlicloudMachineClass(internalAlicloudMachineClass)
		if validationerr.ToAggregate() != nil && len(validationerr.ToAggregate().Errors()) > 0 {
			klog.V(2).Infof("Validation of AlicloudMachineClass failed %s", validationerr.ToAggregate().Error())
			return MachineClass, secretRef, nil
		}

		// Get secretRef
		secretRef, err = c.getSecret(AlicloudMachineClass.Spec.SecretRef, AlicloudMachineClass.Name)
		if err != nil || secretRef == nil {
			klog.V(2).Info("Secret reference not found")
			return MachineClass, secretRef, err
		}
	case "PacketMachineClass":
		PacketMachineClass, err := c.packetMachineClassLister.PacketMachineClasses(c.namespace).Get(classSpec.Name)
		if err != nil {
			klog.V(2).Infof("PacketMachineClass %q/%q not found. Skipping. %v", c.namespace, classSpec.Name, err)
			return MachineClass, secretRef, err
		}
		MachineClass = PacketMachineClass

		// Validate AlicloudMachineClass
		internalPacketMachineClass := &machineapi.PacketMachineClass{}
		err = c.internalExternalScheme.Convert(PacketMachineClass, internalPacketMachineClass, nil)
		if err != nil {
			klog.V(2).Info("Error in scheme conversion")
			return MachineClass, secretRef, err
		}

		validationerr := validation.ValidatePacketMachineClass(internalPacketMachineClass)
		if validationerr.ToAggregate() != nil && len(validationerr.ToAggregate().Errors()) > 0 {
			klog.V(2).Infof("Validation of PacketMachineClass failed %s", validationerr.ToAggregate().Error())
			return MachineClass, secretRef, nil
		}

		// Get secretRef
		secretRef, err = c.getSecret(PacketMachineClass.Spec.SecretRef, PacketMachineClass.Name)
		if err != nil || secretRef == nil {
			klog.V(2).Info("Secret reference not found")
			return MachineClass, secretRef, err
		}
	case "MetalMachineClass":
		MetalMachineClass, err := c.metalMachineClassLister.MetalMachineClasses(c.namespace).Get(classSpec.Name)
		if err != nil {
			klog.V(2).Infof("MetalMachineClass %q/%q not found. Skipping. %v", c.namespace, classSpec.Name, err)
			return MachineClass, secretRef, err
		}
		MachineClass = MetalMachineClass

		// Validate MetalMachineClass
		internalMetalMachineClass := &machineapi.MetalMachineClass{}
		err = c.internalExternalScheme.Convert(MetalMachineClass, internalMetalMachineClass, nil)
		if err != nil {
			klog.V(2).Info("Error in scheme conversion")
			return MachineClass, secretRef, err
		}

		validationerr := validation.ValidateMetalMachineClass(internalMetalMachineClass)
		if validationerr.ToAggregate() != nil && len(validationerr.ToAggregate().Errors()) > 0 {
			klog.V(2).Infof("Validation of MetalMachineClass failed %s", validationerr.ToAggregate().Error())
			return MachineClass, secretRef, nil
		}

		// Get secretRef
		secretRef, err = c.getSecret(MetalMachineClass.Spec.SecretRef, MetalMachineClass.Name)
		if err != nil || secretRef == nil {
			klog.V(2).Info("Secret reference not found")
			return MachineClass, secretRef, err
		}

	default:
		klog.V(2).Infof("ClassKind %q not found", classSpec.Kind)
	}

	return MachineClass, secretRef, nil
}

// nodeConditionsHaveChanged compares two node statuses to see if any of the statuses have changed
func nodeConditionsHaveChanged(machineConditions []v1.NodeCondition, nodeConditions []v1.NodeCondition) bool {

	if len(machineConditions) != len(nodeConditions) {
		return true
	}

	for i := range nodeConditions {
		if nodeConditions[i].Status != machineConditions[i].Status {
			return true
		}
	}

	return false
}

// syncMachineNodeTemplate syncs nodeTemplates between machine and corresponding node-object.
// It ensures, that any nodeTemplate element available on Machine should be available on node-object.
// Although there could be more elements already available on node-object which will not be touched.
func (c *controller) syncMachineNodeTemplates(machine *v1alpha1.Machine) error {
	var (
		initializedNodeAnnotation   = false
		lastAppliedALT              v1alpha1.NodeTemplateSpec
		currentlyAppliedALTJSONByte []byte
	)

	node, err := c.nodeLister.Get(machine.Status.Node)
	if err != nil || node == nil {
		klog.Errorf("Error: Could not get the node-object or node-object is missing - err: %q", err)
		// Dont return error so that other steps can be executed.
		return nil
	}
	nodeCopy := node.DeepCopy()

	// Initialize node annotations if empty
	if nodeCopy.Annotations == nil {
		nodeCopy.Annotations = make(map[string]string)
		initializedNodeAnnotation = true
	}

	// Extracts the last applied annotations to lastAppliedLabels
	lastAppliedALTJSONString, exists := node.Annotations[LastAppliedALTAnnotation]
	if exists {
		err = json.Unmarshal([]byte(lastAppliedALTJSONString), &lastAppliedALT)
		if err != nil {
			klog.Errorf("Error occurred while syncing node annotations, labels & taints: %s", err)
			return err
		}
	}

	annotationsChanged := SyncMachineAnnotations(machine, nodeCopy, lastAppliedALT.Annotations)
	labelsChanged := SyncMachineLabels(machine, nodeCopy, lastAppliedALT.Labels)
	taintsChanged := SyncMachineTaints(machine, nodeCopy, lastAppliedALT.Spec.Taints)

	// Update node-object with latest nodeTemplate elements if elements have changed.
	if initializedNodeAnnotation || labelsChanged || annotationsChanged || taintsChanged {

		klog.V(2).Infof(
			"Updating machine annotations:%v, labels:%v, taints:%v for machine: %q",
			annotationsChanged,
			labelsChanged,
			taintsChanged,
			machine.Name,
		)

		// Update the LastAppliedALTAnnotation
		lastAppliedALT = machine.Spec.NodeTemplateSpec
		currentlyAppliedALTJSONByte, err = json.Marshal(lastAppliedALT)
		if err != nil {
			klog.Errorf("Error occurred while syncing node annotations, labels & taints: %s", err)
			return err
		}
		nodeCopy.Annotations[LastAppliedALTAnnotation] = string(currentlyAppliedALTJSONByte)

		_, err := c.targetCoreClient.CoreV1().Nodes().Update(nodeCopy)
		if err != nil {
			return err
		}
	}
	return nil
}

// SyncMachineAnnotations syncs the annotations of the machine with node-objects.
// It returns true if update is needed else false.
func SyncMachineAnnotations(
	machine *v1alpha1.Machine,
	node *v1.Node,
	lastAppliedAnnotations map[string]string,
) bool {
	toBeUpdated := false
	mAnnotations, nAnnotations := machine.Spec.NodeTemplateSpec.Annotations, node.Annotations

	// Initialize node annotations if nil
	if nAnnotations == nil {
		nAnnotations = make(map[string]string)
		node.Annotations = nAnnotations
	}
	// Intialize machine annotations to empty map if nil
	if mAnnotations == nil {
		mAnnotations = emptyMap
	}

	// Delete any annotation that existed in the past but has been deleted now
	for lastAppliedAnnotationKey := range lastAppliedAnnotations {
		if _, exists := mAnnotations[lastAppliedAnnotationKey]; !exists {
			delete(nAnnotations, lastAppliedAnnotationKey)
			toBeUpdated = true
		}
	}

	// Add/Update any key that doesn't exist or whose value as changed
	for mKey, mValue := range mAnnotations {
		if nValue, exists := nAnnotations[mKey]; !exists || mValue != nValue {
			nAnnotations[mKey] = mValue
			toBeUpdated = true
		}
	}

	return toBeUpdated
}

// SyncMachineLabels syncs the labels of the machine with node-objects.
// It returns true if update is needed else false.
func SyncMachineLabels(
	machine *v1alpha1.Machine,
	node *v1.Node,
	lastAppliedLabels map[string]string,
) bool {
	toBeUpdated := false
	mLabels, nLabels := machine.Spec.NodeTemplateSpec.Labels, node.Labels

	// Initialize node labels if nil
	if nLabels == nil {
		nLabels = make(map[string]string)
		node.Labels = nLabels
	}
	// Intialize machine labels to empty map if nil
	if mLabels == nil {
		mLabels = emptyMap
	}

	// Delete any labels that existed in the past but has been deleted now
	for lastAppliedLabelKey := range lastAppliedLabels {
		if _, exists := mLabels[lastAppliedLabelKey]; !exists {
			delete(nLabels, lastAppliedLabelKey)
			toBeUpdated = true
		}
	}

	// Add/Update any key that doesn't exist or whose value as changed
	for mKey, mValue := range mLabels {
		if nValue, exists := nLabels[mKey]; !exists || mValue != nValue {
			nLabels[mKey] = mValue
			toBeUpdated = true
		}
	}

	return toBeUpdated
}

type taintKeyEffect struct {
	// Required. The taint key to be applied to a node.
	Key string
	// Valid effects are NoSchedule, PreferNoSchedule and NoExecute.
	Effect v1.TaintEffect
}

// SyncMachineTaints syncs the taints of the machine with node-objects.
// It returns true if update is needed else false.
func SyncMachineTaints(
	machine *v1alpha1.Machine,
	node *v1.Node,
	lastAppliedTaints []v1.Taint,
) bool {
	toBeUpdated := false
	mTaints, nTaints := machine.Spec.NodeTemplateSpec.Spec.Taints, node.Spec.Taints
	mTaintsMap := make(map[taintKeyEffect]*v1.Taint, 0)
	nTaintsMap := make(map[taintKeyEffect]*v1.Taint, 0)

	// Convert the slice of taints to map of taint [key, effect] = Taint
	// Helps with indexed searching
	for i := range mTaints {
		mTaint := &mTaints[i]
		taintKE := taintKeyEffect{
			Key:    mTaint.Key,
			Effect: mTaint.Effect,
		}
		mTaintsMap[taintKE] = mTaint
	}
	for i := range nTaints {
		nTaint := &nTaints[i]
		taintKE := taintKeyEffect{
			Key:    nTaint.Key,
			Effect: nTaint.Effect,
		}
		nTaintsMap[taintKE] = nTaint
	}

	// Delete taints that existed on the machine object in the last update but deleted now
	for _, lastAppliedTaint := range lastAppliedTaints {

		lastAppliedKE := taintKeyEffect{
			Key:    lastAppliedTaint.Key,
			Effect: lastAppliedTaint.Effect,
		}

		if _, exists := mTaintsMap[lastAppliedKE]; !exists {
			delete(nTaintsMap, lastAppliedKE)
			toBeUpdated = true
		}
	}

	// Add any taints that exists in the machine object but not on the node object
	for mKE, mV := range mTaintsMap {
		if nV, exists := nTaintsMap[mKE]; !exists || *nV != *mV {
			nTaintsMap[mKE] = mV
			toBeUpdated = true
		}
	}

	if toBeUpdated {
		// Convert the map of taints to slice of taints
		nTaints = make([]v1.Taint, len(nTaintsMap))
		i := 0
		for _, nV := range nTaintsMap {
			nTaints[i] = *nV
			i++
		}
		node.Spec.Taints = nTaints
	}

	return toBeUpdated
}
