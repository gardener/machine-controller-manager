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

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/client-go/tools/cache"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/golang/glog"

	"code.sapcloud.io/kubernetes/node-controller-manager/pkg/apis/node/validation"
	"code.sapcloud.io/kubernetes/node-controller-manager/pkg/apis/node/v1alpha1"
	"code.sapcloud.io/kubernetes/node-controller-manager/pkg/apis/node"
)

func (c *controller) awsInstanceClassAdd(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		glog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.awsInstanceClassQueue.Add(key)
}

func (c *controller) awsInstanceClassUpdate(oldObj, newObj interface{}) {
	old, ok := oldObj.(*v1alpha1.AWSInstanceClass)
	if old == nil || !ok {
		return
	}
	new, ok := oldObj.(*v1alpha1.AWSInstanceClass)
	if new == nil || !ok {
		return
	}
	if reflect.DeepEqual(old.Spec, new.Spec) {
		return
	}

	c.awsInstanceClassAdd(newObj)
}

func (c *controller) awsInstanceClassDelete(obj interface{}) {
	awsInstanceClass, ok := obj.(*v1alpha1.AWSInstanceClass)
	if awsInstanceClass == nil || !ok {
		return
	}

	glog.V(2).Infof("Received delete event for awsInstanceClass %v; no further processing will occur", awsInstanceClass.Name)
}

// reconcileawsInstanceClassKey reconciles a awsInstanceClass due to controller resync
// or an event on the awsInstanceClass.
func (c *controller) reconcileClusterawsInstanceClassKey(key string) error {
	plan, err := c.awsInstanceClassLister.Get(key)
	if errors.IsNotFound(err) {
		glog.Infof("ClusterawsInstanceClass %q: Not doing work because it has been deleted", key)
		return nil
	}
	if err != nil {
		glog.Infof("ClusterawsInstanceClass %q: Unable to retrieve object from store: %v", key, err)
		return err
	}

	return c.reconcileClusterawsInstanceClass(plan)
}

func (c *controller) reconcileClusterawsInstanceClass(awsInstanceClass *v1alpha1.AWSInstanceClass) error {
	
	internalAWSInstanceClass := &node.AWSInstanceClass{}
	err := api.Scheme.Convert(awsInstanceClass, internalAWSInstanceClass, nil)
	if err != nil {
		return err
	}
	// TODO this should be put in own API server
	validationerr := validation.ValidateAWSInstanceClass(internalAWSInstanceClass)
	if validationerr.ToAggregate() != nil && len(validationerr.ToAggregate().Errors()) > 0 {
		glog.V(2).Infof("Validation of awsInstanceClass failled %s", validationerr.ToAggregate().Error())
		return nil
	}

	instances, err := c.resolveInstances(awsInstanceClass)
	if err != nil {
		return err
	}

	for _, instance := range instances {
		c.instanceQueue.Add(instance.Name)
	}
	return nil
}

func (c *controller) resolveInstances(awsInstanceClass *v1alpha1.AWSInstanceClass) ([]*v1alpha1.Instance, error) {
	instances, err := c.instanceLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	var filtered []*v1alpha1.Instance
	for _, instance := range instances {
		if instance.Spec.Class.Name == awsInstanceClass.Name {
			filtered = append(filtered, instance)
		}
	}
	return filtered, nil
}
