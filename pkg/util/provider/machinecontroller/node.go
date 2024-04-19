// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Package controller is used to provide the core functionalities of machine-controller-manager
package controller

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

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
