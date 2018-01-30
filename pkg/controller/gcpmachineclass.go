/*
Copyright 2017 The Gardener Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package controller

import (
	"reflect"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/pkg/api"

	"github.com/golang/glog"

	"github.com/gardener/node-controller-manager/pkg/apis/machine"
	"github.com/gardener/node-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/node-controller-manager/pkg/apis/machine/validation"
)

func (c *controller) gcpMachineClassAdd(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		glog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.gcpMachineClassQueue.Add(key)
}

func (c *controller) gcpMachineClassUpdate(oldObj, newObj interface{}) {
	old, ok := oldObj.(*v1alpha1.GCPMachineClass)
	if old == nil || !ok {
		return
	}
	new, ok := oldObj.(*v1alpha1.GCPMachineClass)
	if new == nil || !ok {
		return
	}
	if reflect.DeepEqual(old.Spec, new.Spec) {
		return
	}

	c.gcpMachineClassAdd(newObj)
}

func (c *controller) gcpMachineClassDelete(obj interface{}) {
	gcpMachineClass, ok := obj.(*v1alpha1.GCPMachineClass)
	if gcpMachineClass == nil || !ok {
		return
	}
}

// reconcilegcpMachineClassKey reconciles a gcpMachineClass due to controller resync
// or an event on the gcpMachineClass.
func (c *controller) reconcileClusterGCPMachineClassKey(key string) error {
	plan, err := c.gcpMachineClassLister.Get(key)
	if errors.IsNotFound(err) {
		glog.Infof("ClustergcpMachineClass %q: Not doing work because it has been deleted", key)
		return nil
	}
	if err != nil {
		glog.Infof("ClustergcpMachineClass %q: Unable to retrieve object from store: %v", key, err)
		return err
	}

	return c.reconcileClusterGCPMachineClass(plan)
}

func (c *controller) reconcileClusterGCPMachineClass(gcpMachineClass *v1alpha1.GCPMachineClass) error {

	internalGCPMachineClass := &machine.GCPMachineClass{}
	err := api.Scheme.Convert(gcpMachineClass, internalGCPMachineClass, nil)
	if err != nil {
		return err
	}
	// TODO this should be put in own API server
	validationerr := validation.ValidateGCPMachineClass(internalGCPMachineClass)
	if validationerr.ToAggregate() != nil && len(validationerr.ToAggregate().Errors()) > 0 {
		glog.V(2).Infof("Validation of gcpMachineClass failed %s", validationerr.ToAggregate().Error())
		return nil
	}

	machines, err := c.resolveGCPMachines(gcpMachineClass)
	if err != nil {
		return err
	}

	for _, machine := range machines {
		c.machineQueue.Add(machine.Name)
	}
	return nil
}

func (c *controller) resolveGCPMachines(gcpMachineClass *v1alpha1.GCPMachineClass) ([]*v1alpha1.Machine, error) {
	machines, err := c.machineLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	var filtered []*v1alpha1.Machine
	for _, machine := range machines {
		if machine.Spec.Class.Kind == "GCPMachineClass" && machine.Spec.Class.Name == gcpMachineClass.Name {
			filtered = append(filtered, machine)
		}
	}
	return filtered, nil
}
