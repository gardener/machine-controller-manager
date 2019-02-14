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

	if classSpec.Kind == "MachineClass" {

		CommonMachineClass, err := c.machineClassLister.MachineClasses(c.namespace).Get(classSpec.Name)
		if err != nil {
			glog.V(2).Infof("MachineClass %q/%q not found. Skipping. %v", c.namespace, classSpec.Name, err)
			return MachineClass, secretRef, err
		}

		MachineClass = CommonMachineClass

		internalMachineClass := &machineapi.MachineClass{}
		err = c.internalExternalScheme.Convert(MachineClass, internalMachineClass, nil)
		if err != nil {
			glog.V(2).Info("Error in scheme conversion")
			return MachineClass, secretRef, err
		}

		validationerr := validation.ValidateMachineClass(internalMachineClass)
		if validationerr.ToAggregate() != nil && len(validationerr.ToAggregate().Errors()) > 0 {
			glog.V(2).Infof("Validation of MachineClass failed %s", validationerr.ToAggregate().Error())
			return MachineClass, secretRef, nil
		}

		// Get secretRef
		secretRef, err = c.getSecret(CommonMachineClass.SecretRef, CommonMachineClass.Name)
		if err != nil || secretRef == nil {
			glog.V(2).Info("Secret reference not found")
			return MachineClass, secretRef, err
		}
	} else {
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
