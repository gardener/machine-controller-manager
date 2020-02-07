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
	"time"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
)

// reconcileClusterSecretKey reconciles an secret due to controller resync
// or an event on the secret
func (c *controller) reconcileClusterSecretKey(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	} else if c.namespace != namespace {
		// Secret exists outside of controller namespace
		return nil
	}

	secret, err := c.secretLister.Secrets(c.namespace).Get(name)
	if errors.IsNotFound(err) {
		klog.V(4).Infof("%q: Not doing work because it has been deleted", key)
		return nil
	} else if err != nil {
		klog.V(4).Infof("%q: Unable to retrieve object from store: %v", key, err)
		return err
	}

	return c.reconcileClusterSecret(secret)
}

// reconcileClusterSecret manipulates finalizers based on
// machineClass references
func (c *controller) reconcileClusterSecret(secret *corev1.Secret) error {
	startTime := time.Now()

	klog.V(4).Infof("Start syncing %q", secret.Name)
	defer func() {
		c.enqueueSecretAfter(secret, 10*time.Minute)
		klog.V(4).Infof("Finished syncing %q (%v)", secret.Name, time.Since(startTime))
	}()

	// Check if machineClasses are referring to this secret
	exists, err := c.existsMachineClassForSecret(secret.Name)
	if err != nil {
		return err
	}

	if exists {
		// If one or more machineClasses refer this, add finalizer (if it doesn't exist)
		err = c.addSecretFinalizers(secret)
		if err != nil {
			return err
		}
	} else {
		if finalizers := sets.NewString(secret.Finalizers...); !finalizers.Has(DeleteFinalizerName) {
			// Finalizer doesn't exist, simply return nil
			return nil
		}
		err = c.deleteSecretFinalizers(secret)
		if err != nil {
			return err
		}
	}

	return nil
}

/*
	SECTION
	Manipulate Finalizers
*/

func (c *controller) addSecretFinalizers(secret *corev1.Secret) error {
	clone := secret.DeepCopy()

	if finalizers := sets.NewString(clone.Finalizers...); !finalizers.Has(DeleteFinalizerName) {
		finalizers.Insert(DeleteFinalizerName)
		return c.updateSecretFinalizers(clone, finalizers.List())
	}
	return nil
}

func (c *controller) deleteSecretFinalizers(secret *corev1.Secret) error {
	clone := secret.DeepCopy()

	if finalizers := sets.NewString(clone.Finalizers...); finalizers.Has(DeleteFinalizerName) {
		finalizers.Delete(DeleteFinalizerName)
		return c.updateSecretFinalizers(clone, finalizers.List())
	}
	return nil
}

func (c *controller) updateSecretFinalizers(secret *corev1.Secret, finalizers []string) error {
	// Get the latest version of the secret so that we can avoid conflicts
	secret, err := c.controlCoreClient.CoreV1().Secrets(secret.Namespace).Get(secret.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	clone := secret.DeepCopy()
	clone.Finalizers = finalizers
	_, err = c.controlCoreClient.CoreV1().Secrets(clone.Namespace).Update(clone)

	if err != nil {
		klog.Warning("Updating secret finalizers failed, retrying", secret.Name, err)
		return err
	}
	klog.V(3).Infof("Successfully added/removed finalizer on the secret %q", secret.Name)
	return err
}

/*
	SECTION
	Event handlers
*/

func (c *controller) secretAdd(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.secretQueue.Add(key)
}

func (c *controller) secretDelete(obj interface{}) {
	c.secretAdd(obj)
}

func (c *controller) enqueueSecretAfter(obj interface{}, after time.Duration) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	c.secretQueue.AddAfter(key, after)
}

func (c *controller) openStackMachineClassToSecretAdd(obj interface{}) {
	machineClass, ok := obj.(*v1alpha1.OpenStackMachineClass)
	if machineClass == nil || !ok {
		return
	}
	c.secretQueue.Add(machineClass.Spec.SecretRef.Namespace + "/" + machineClass.Spec.SecretRef.Name)
}

func (c *controller) openStackMachineClassToSecretUpdate(oldObj interface{}, newObj interface{}) {
	oldMachineClass, ok := oldObj.(*v1alpha1.OpenStackMachineClass)
	if oldMachineClass == nil || !ok {
		return
	}
	newMachineClass, ok := newObj.(*v1alpha1.OpenStackMachineClass)
	if newMachineClass == nil || !ok {
		return
	}

	if oldMachineClass.Spec.SecretRef.Name != newMachineClass.Spec.SecretRef.Name ||
		oldMachineClass.Spec.SecretRef.Namespace != newMachineClass.Spec.SecretRef.Namespace {
		c.secretQueue.Add(oldMachineClass.Spec.SecretRef.Namespace + "/" + oldMachineClass.Spec.SecretRef.Name)
		c.secretQueue.Add(newMachineClass.Spec.SecretRef.Namespace + "/" + newMachineClass.Spec.SecretRef.Name)
	}
}

func (c *controller) openStackMachineClassToSecretDelete(obj interface{}) {
	c.openStackMachineClassToSecretAdd(obj)
}

func (c *controller) gcpMachineClassToSecretAdd(obj interface{}) {
	machineClass, ok := obj.(*v1alpha1.GCPMachineClass)
	if machineClass == nil || !ok {
		return
	}
	c.secretQueue.Add(machineClass.Spec.SecretRef.Namespace + "/" + machineClass.Spec.SecretRef.Name)
}

func (c *controller) gcpMachineClassToSecretUpdate(oldObj interface{}, newObj interface{}) {
	oldMachineClass, ok := oldObj.(*v1alpha1.GCPMachineClass)
	if oldMachineClass == nil || !ok {
		return
	}
	newMachineClass, ok := newObj.(*v1alpha1.GCPMachineClass)
	if newMachineClass == nil || !ok {
		return
	}

	if oldMachineClass.Spec.SecretRef.Name != newMachineClass.Spec.SecretRef.Name ||
		oldMachineClass.Spec.SecretRef.Namespace != newMachineClass.Spec.SecretRef.Namespace {
		c.secretQueue.Add(oldMachineClass.Spec.SecretRef.Namespace + "/" + oldMachineClass.Spec.SecretRef.Name)
		c.secretQueue.Add(newMachineClass.Spec.SecretRef.Namespace + "/" + newMachineClass.Spec.SecretRef.Name)
	}
}

func (c *controller) gcpMachineClassToSecretDelete(obj interface{}) {
	c.gcpMachineClassToSecretAdd(obj)
}

func (c *controller) azureMachineClassToSecretAdd(obj interface{}) {
	machineClass, ok := obj.(*v1alpha1.AzureMachineClass)
	if machineClass == nil || !ok {
		return
	}
	c.secretQueue.Add(machineClass.Spec.SecretRef.Namespace + "/" + machineClass.Spec.SecretRef.Name)
}

func (c *controller) azureMachineClassToSecretUpdate(oldObj interface{}, newObj interface{}) {
	oldMachineClass, ok := oldObj.(*v1alpha1.AzureMachineClass)
	if oldMachineClass == nil || !ok {
		return
	}
	newMachineClass, ok := newObj.(*v1alpha1.AzureMachineClass)
	if newMachineClass == nil || !ok {
		return
	}

	if oldMachineClass.Spec.SecretRef.Name != newMachineClass.Spec.SecretRef.Name ||
		oldMachineClass.Spec.SecretRef.Namespace != newMachineClass.Spec.SecretRef.Namespace {
		c.secretQueue.Add(oldMachineClass.Spec.SecretRef.Namespace + "/" + oldMachineClass.Spec.SecretRef.Name)
		c.secretQueue.Add(newMachineClass.Spec.SecretRef.Namespace + "/" + newMachineClass.Spec.SecretRef.Name)
	}
}

func (c *controller) azureMachineClassToSecretDelete(obj interface{}) {
	c.azureMachineClassToSecretAdd(obj)
}

func (c *controller) alicloudMachineClassToSecretAdd(obj interface{}) {
	machineClass, ok := obj.(*v1alpha1.AlicloudMachineClass)
	if machineClass == nil || !ok {
		return
	}
	c.secretQueue.Add(machineClass.Spec.SecretRef.Namespace + "/" + machineClass.Spec.SecretRef.Name)
}

func (c *controller) alicloudMachineClassToSecretUpdate(oldObj interface{}, newObj interface{}) {
	oldMachineClass, ok := oldObj.(*v1alpha1.AlicloudMachineClass)
	if oldMachineClass == nil || !ok {
		return
	}
	newMachineClass, ok := newObj.(*v1alpha1.AlicloudMachineClass)
	if newMachineClass == nil || !ok {
		return
	}

	if oldMachineClass.Spec.SecretRef.Name != newMachineClass.Spec.SecretRef.Name ||
		oldMachineClass.Spec.SecretRef.Namespace != newMachineClass.Spec.SecretRef.Namespace {
		c.secretQueue.Add(oldMachineClass.Spec.SecretRef.Namespace + "/" + oldMachineClass.Spec.SecretRef.Name)
		c.secretQueue.Add(newMachineClass.Spec.SecretRef.Namespace + "/" + newMachineClass.Spec.SecretRef.Name)
	}
}

func (c *controller) alicloudMachineClassToSecretDelete(obj interface{}) {
	c.alicloudMachineClassToSecretAdd(obj)
}

func (c *controller) awsMachineClassToSecretAdd(obj interface{}) {
	machineClass, ok := obj.(*v1alpha1.AWSMachineClass)
	if machineClass == nil || !ok {
		return
	}
	c.secretQueue.Add(machineClass.Spec.SecretRef.Namespace + "/" + machineClass.Spec.SecretRef.Name)
}

func (c *controller) awsMachineClassToSecretUpdate(oldObj interface{}, newObj interface{}) {
	oldMachineClass, ok := oldObj.(*v1alpha1.AWSMachineClass)
	if oldMachineClass == nil || !ok {
		return
	}
	newMachineClass, ok := newObj.(*v1alpha1.AWSMachineClass)
	if newMachineClass == nil || !ok {
		return
	}

	if oldMachineClass.Spec.SecretRef.Name != newMachineClass.Spec.SecretRef.Name ||
		oldMachineClass.Spec.SecretRef.Namespace != newMachineClass.Spec.SecretRef.Namespace {
		c.secretQueue.Add(oldMachineClass.Spec.SecretRef.Namespace + "/" + oldMachineClass.Spec.SecretRef.Name)
		c.secretQueue.Add(newMachineClass.Spec.SecretRef.Namespace + "/" + newMachineClass.Spec.SecretRef.Name)
	}
}

func (c *controller) awsMachineClassToSecretDelete(obj interface{}) {
	c.awsMachineClassToSecretAdd(obj)
}

func (c *controller) packetMachineClassToSecretAdd(obj interface{}) {
	machineClass, ok := obj.(*v1alpha1.PacketMachineClass)
	if machineClass == nil || !ok {
		return
	}
	c.secretQueue.Add(machineClass.Spec.SecretRef.Namespace + "/" + machineClass.Spec.SecretRef.Name)
}

func (c *controller) packetMachineClassToSecretUpdate(oldObj interface{}, newObj interface{}) {
	oldMachineClass, ok := oldObj.(*v1alpha1.PacketMachineClass)
	if oldMachineClass == nil || !ok {
		return
	}
	newMachineClass, ok := newObj.(*v1alpha1.PacketMachineClass)
	if newMachineClass == nil || !ok {
		return
	}

	if oldMachineClass.Spec.SecretRef.Name != newMachineClass.Spec.SecretRef.Name ||
		oldMachineClass.Spec.SecretRef.Namespace != newMachineClass.Spec.SecretRef.Namespace {
		c.secretQueue.Add(oldMachineClass.Spec.SecretRef.Namespace + "/" + oldMachineClass.Spec.SecretRef.Name)
		c.secretQueue.Add(newMachineClass.Spec.SecretRef.Namespace + "/" + newMachineClass.Spec.SecretRef.Name)
	}
}

func (c *controller) packetMachineClassToSecretDelete(obj interface{}) {
	c.packetMachineClassToSecretAdd(obj)
}

func (c *controller) metalMachineClassToSecretAdd(obj interface{}) {
	machineClass, ok := obj.(*v1alpha1.MetalMachineClass)
	if machineClass == nil || !ok {
		return
	}
	c.secretQueue.Add(machineClass.Spec.SecretRef.Namespace + "/" + machineClass.Spec.SecretRef.Name)
}

func (c *controller) metalMachineClassToSecretUpdate(oldObj interface{}, newObj interface{}) {
	oldMachineClass, ok := oldObj.(*v1alpha1.MetalMachineClass)
	if oldMachineClass == nil || !ok {
		return
	}
	newMachineClass, ok := newObj.(*v1alpha1.MetalMachineClass)
	if newMachineClass == nil || !ok {
		return
	}

	if oldMachineClass.Spec.SecretRef.Name != newMachineClass.Spec.SecretRef.Name ||
		oldMachineClass.Spec.SecretRef.Namespace != newMachineClass.Spec.SecretRef.Namespace {
		c.secretQueue.Add(oldMachineClass.Spec.SecretRef.Namespace + "/" + oldMachineClass.Spec.SecretRef.Name)
		c.secretQueue.Add(newMachineClass.Spec.SecretRef.Namespace + "/" + newMachineClass.Spec.SecretRef.Name)
	}
}

func (c *controller) metalMachineClassToSecretDelete(obj interface{}) {
	c.metalMachineClassToSecretAdd(obj)
}
