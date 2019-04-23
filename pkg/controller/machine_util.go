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
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/validation"
	"github.com/golang/glog"

	machineapi "github.com/gardener/machine-controller-manager/pkg/apis/machine"
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	v1alpha1client "github.com/gardener/machine-controller-manager/pkg/client/clientset/versioned/typed/machine/v1alpha1"
	v1alpha1listers "github.com/gardener/machine-controller-manager/pkg/client/listers/machine/v1alpha1"
	"k8s.io/api/core/v1"
	errorsutil "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/util/retry"
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
		glog.V(4).Infof("Machine %s precondition doesn't hold, skip updating it.", name)
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
			glog.V(2).Infof("AWSMachineClass %q/%q not found. Skipping. %v", c.namespace, classSpec.Name, err)
			return MachineClass, secretRef, err
		}
		MachineClass = AWSMachineClass

		// Validate AWSMachineClass
		internalAWSMachineClass := &machineapi.AWSMachineClass{}
		err = c.internalExternalScheme.Convert(AWSMachineClass, internalAWSMachineClass, nil)
		if err != nil {
			glog.V(2).Info("Error in scheme conversion")
			return MachineClass, secretRef, err
		}

		validationerr := validation.ValidateAWSMachineClass(internalAWSMachineClass)
		if validationerr.ToAggregate() != nil && len(validationerr.ToAggregate().Errors()) > 0 {
			glog.V(2).Infof("Validation of AWSMachineClass failed %s", validationerr.ToAggregate().Error())
			return MachineClass, secretRef, nil
		}

		// Get secretRef
		secretRef, err = c.getSecret(AWSMachineClass.Spec.SecretRef, AWSMachineClass.Name)
		if err != nil || secretRef == nil {
			glog.V(2).Info("Secret reference not found")
			return MachineClass, secretRef, err
		}
	case "AzureMachineClass":
		AzureMachineClass, err := c.azureMachineClassLister.AzureMachineClasses(c.namespace).Get(classSpec.Name)
		if err != nil {
			glog.V(2).Infof("AzureMachineClass %q not found. Skipping. %v", classSpec.Name, err)
			return MachineClass, secretRef, err
		}
		MachineClass = AzureMachineClass

		// Validate AzureMachineClass
		internalAzureMachineClass := &machineapi.AzureMachineClass{}
		err = c.internalExternalScheme.Convert(AzureMachineClass, internalAzureMachineClass, nil)
		if err != nil {
			glog.V(2).Info("Error in scheme conversion")
			return MachineClass, secretRef, err
		}

		validationerr := validation.ValidateAzureMachineClass(internalAzureMachineClass)
		if validationerr.ToAggregate() != nil && len(validationerr.ToAggregate().Errors()) > 0 {
			glog.V(2).Infof("Validation of AzureMachineClass failed %s", validationerr.ToAggregate().Error())
			return MachineClass, secretRef, nil
		}

		// Get secretRef
		secretRef, err = c.getSecret(AzureMachineClass.Spec.SecretRef, AzureMachineClass.Name)
		if err != nil || secretRef == nil {
			glog.V(2).Info("Secret reference not found")
			return MachineClass, secretRef, err

		}
	case "GCPMachineClass":
		GCPMachineClass, err := c.gcpMachineClassLister.GCPMachineClasses(c.namespace).Get(classSpec.Name)
		if err != nil {
			glog.V(2).Infof("GCPMachineClass %q not found. Skipping. %v", classSpec.Name, err)
			return MachineClass, secretRef, err
		}
		MachineClass = GCPMachineClass

		// Validate GCPMachineClass
		internalGCPMachineClass := &machineapi.GCPMachineClass{}
		err = c.internalExternalScheme.Convert(GCPMachineClass, internalGCPMachineClass, nil)
		if err != nil {
			glog.V(2).Info("Error in scheme conversion")
			return MachineClass, secretRef, err
		}

		validationerr := validation.ValidateGCPMachineClass(internalGCPMachineClass)
		if validationerr.ToAggregate() != nil && len(validationerr.ToAggregate().Errors()) > 0 {
			glog.V(2).Infof("Validation of GCPMachineClass failed %s", validationerr.ToAggregate().Error())
			return MachineClass, secretRef, nil
		}

		// Get secretRef
		secretRef, err = c.getSecret(GCPMachineClass.Spec.SecretRef, GCPMachineClass.Name)
		if err != nil || secretRef == nil {
			glog.V(2).Info("Secret reference not found")
			return MachineClass, secretRef, err
		}
	case "OpenStackMachineClass":
		OpenStackMachineClass, err := c.openStackMachineClassLister.OpenStackMachineClasses(c.namespace).Get(classSpec.Name)
		if err != nil {
			glog.V(2).Infof("OpenStackMachineClass %q not found. Skipping. %v", classSpec.Name, err)
			return MachineClass, secretRef, err
		}
		MachineClass = OpenStackMachineClass

		// Validate OpenStackMachineClass
		internalOpenStackMachineClass := &machineapi.OpenStackMachineClass{}
		err = c.internalExternalScheme.Convert(OpenStackMachineClass, internalOpenStackMachineClass, nil)
		if err != nil {
			glog.V(2).Info("Error in scheme conversion")
			return MachineClass, secretRef, err
		}

		validationerr := validation.ValidateOpenStackMachineClass(internalOpenStackMachineClass)
		if validationerr.ToAggregate() != nil && len(validationerr.ToAggregate().Errors()) > 0 {
			glog.V(2).Infof("Validation of OpenStackMachineClass failed %s", validationerr.ToAggregate().Error())
			return MachineClass, secretRef, nil
		}

		// Get secretRef
		secretRef, err = c.getSecret(OpenStackMachineClass.Spec.SecretRef, OpenStackMachineClass.Name)
		if err != nil || secretRef == nil {
			glog.V(2).Info("Secret reference not found")
			return MachineClass, secretRef, err
		}
	case "AlicloudMachineClass":
		AlicloudMachineClass, err := c.alicloudMachineClassLister.AlicloudMachineClasses(c.namespace).Get(classSpec.Name)
		if err != nil {
			glog.V(2).Infof("AlicloudMachineClass %q/%q not found. Skipping. %v", c.namespace, classSpec.Name, err)
			return MachineClass, secretRef, err
		}
		MachineClass = AlicloudMachineClass

		// Validate AlicloudMachineClass
		internalAlicloudMachineClass := &machineapi.AlicloudMachineClass{}
		err = c.internalExternalScheme.Convert(AlicloudMachineClass, internalAlicloudMachineClass, nil)
		if err != nil {
			glog.V(2).Info("Error in scheme convertion")
			return MachineClass, secretRef, err
		}

		validationerr := validation.ValidateAlicloudMachineClass(internalAlicloudMachineClass)
		if validationerr.ToAggregate() != nil && len(validationerr.ToAggregate().Errors()) > 0 {
			glog.V(2).Infof("Validation of AlicloudMachineClass failed %s", validationerr.ToAggregate().Error())
			return MachineClass, secretRef, nil
		}

		// Get secretRef
		secretRef, err = c.getSecret(AlicloudMachineClass.Spec.SecretRef, AlicloudMachineClass.Name)
		if err != nil || secretRef == nil {
			glog.V(2).Info("Secret reference not found")
			return MachineClass, secretRef, err
		}
	case "PacketMachineClass":
		PacketMachineClass, err := c.packetMachineClassLister.PacketMachineClasses(c.namespace).Get(classSpec.Name)
		if err != nil {
			glog.V(2).Infof("PacketMachineClass %q/%q not found. Skipping. %v", c.namespace, classSpec.Name, err)
			return MachineClass, secretRef, err
		}
		MachineClass = PacketMachineClass

		// Validate AlicloudMachineClass
		internalPacketMachineClass := &machineapi.PacketMachineClass{}
		err = c.internalExternalScheme.Convert(PacketMachineClass, internalPacketMachineClass, nil)
		if err != nil {
			glog.V(2).Info("Error in scheme conversion")
			return MachineClass, secretRef, err
		}

		validationerr := validation.ValidatePacketMachineClass(internalPacketMachineClass)
		if validationerr.ToAggregate() != nil && len(validationerr.ToAggregate().Errors()) > 0 {
			glog.V(2).Infof("Validation of PacketMachineClass failed %s", validationerr.ToAggregate().Error())
			return MachineClass, secretRef, nil
		}

		// Get secretRef
		secretRef, err = c.getSecret(PacketMachineClass.Spec.SecretRef, PacketMachineClass.Name)
		if err != nil || secretRef == nil {
			glog.V(2).Info("Secret reference not found")
			return MachineClass, secretRef, err
		}
	default:
		glog.V(2).Infof("ClassKind %q not found", classSpec.Kind)
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
	if machine.Status.Node == "" {
		glog.Warning("Warning: Node field is empty on Machine-object, %q", machine.Name)
	}
	node, err := c.nodeLister.Get(machine.Status.Node)
	if err != nil || node == nil {
		glog.Errorf("Error: Could not get the node-object or node-object is missing - err: %q", err)
		return err
	}
	nodeCopy := node.DeepCopy()

	// Sync Labels
	labelsChanged := SyncMachineLabels(machine, nodeCopy)

	// Sync Annotations
	annotationsChanged := SyncMachineAnnotations(machine, nodeCopy)

	// Sync Taints
	taintsChanged := SyncMachineTaints(machine, nodeCopy)

	// Update node-object with latest nodeTemplate elements if elements have changed.
	if labelsChanged || annotationsChanged || taintsChanged {
		_, err := c.targetCoreClient.Core().Nodes().Update(nodeCopy)
		if err != nil {
			return err
		}
	}
	return nil
}

// SyncMachineLabels syncs the labels of the machine with node-objects.
// It returns true if update is needed else false.
func SyncMachineLabels(machine *v1alpha1.Machine, node *v1.Node) bool {
	mCopy, nCopy := machine.Spec.NodeTemplateSpec.Labels, node.Labels
	updateNeeded := false

	if mCopy == nil {
		return false
	}
	if nCopy == nil {
		nCopy = make(map[string]string)
	}

	for mkey, mvalue := range mCopy {
		// if key doesnt exists or value does not match
		if _, ok := nCopy[mkey]; !ok || mvalue != nCopy[mkey] {
			updateNeeded = true
		}
		nCopy[mkey] = mvalue
	}

	return updateNeeded
}

// SyncMachineAnnotations syncs the annotations of the machine with node-objects.
// It returns true if update is needed else false.
func SyncMachineAnnotations(machine *v1alpha1.Machine, node *v1.Node) bool {
	mCopy, nCopy := machine.Spec.NodeTemplateSpec.Annotations, node.Annotations
	updateNeeded := false

	if mCopy == nil {
		return false
	}
	if nCopy == nil {
		nCopy = make(map[string]string)
	}

	for mkey, mvalue := range mCopy {
		// if key doesnt exists or value does not match
		if _, ok := nCopy[mkey]; !ok || mvalue != nCopy[mkey] {
			updateNeeded = true
		}
		nCopy[mkey] = mvalue
	}

	return updateNeeded
}

// SyncMachineTaints syncs the annotations of the machine with node-objects.
// It returns true if update is needed else false.
func SyncMachineTaints(machine *v1alpha1.Machine, node *v1.Node) bool {
	mTaintsCopy, nTaintsCopy := machine.Spec.NodeTemplateSpec.Spec.Taints, node.Spec.Taints
	updateNeeded := false

	for i := range mTaintsCopy {
		elementFound := false
		for j := range nTaintsCopy {
			if mTaintsCopy[i] == nTaintsCopy[j] {
				elementFound = true
				break
			} else if mTaintsCopy[i].Key == nTaintsCopy[j].Key {
				elementFound, updateNeeded = true, true
				nTaintsCopy[j] = mTaintsCopy[i]
				break
			}
		}
		if elementFound == false {
			nTaintsCopy = append(nTaintsCopy, mTaintsCopy[i])
			updateNeeded = true
		}
	}

	return updateNeeded
}
