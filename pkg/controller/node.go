/*
Copyright (c) 2017 SAP SE or an SAP affiliate company. All rights reserved.

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

// Package controller is used to provide the core functionalities of machine-controller-manager
package controller

import (
	"k8s.io/klog"

	"k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

func (c *controller) nodeAdd(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.nodeQueue.Add(key)
}

func (c *controller) nodeUpdate(oldObj, newObj interface{}) {
	c.nodeAdd(newObj)
}

func (c *controller) nodeDelete(obj interface{}) {
	node, ok := obj.(*v1.Node)
	if node == nil || !ok {
		return
	}

}

// Not being used at the moment, saving it for a future use case.
func (c *controller) reconcileClusterNodeKey(key string) error {
	node, err := c.nodeLister.Get(key)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		klog.Errorf("ClusterNode %q: Unable to retrieve object from store: %v", key, err)
		return err
	}

	return c.reconcileClusterNode(node)
}

func (c *controller) reconcileClusterNode(node *v1.Node) error {
	return nil
}
