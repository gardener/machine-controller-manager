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
	"context"
	"time"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

// reconcileClusterSecretKey reconciles an secret due to controller resync
// or an event on the secret
func (c *controller) reconcileClusterSecretKey(key string) error {
	ctx := context.Background()
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

	return c.reconcileClusterSecret(ctx, secret)
}

// reconcileClusterSecret manipulates finalizers based on
// machineClass references
func (c *controller) reconcileClusterSecret(ctx context.Context, secret *corev1.Secret) error {
	startTime := time.Now()

	klog.V(5).Infof("Start syncing %q", secret.Name)
	defer func() {
		c.enqueueSecretAfter(secret, 10*time.Minute)
		klog.V(5).Infof("Finished syncing %q (%v)", secret.Name, time.Since(startTime))
	}()

	// Check if machineClasses are referring to this secret
	exists, err := c.existsMachineClassForSecret(secret.Name)
	if err != nil {
		return err
	}

	if exists {
		// If one or more machineClasses refer this, add finalizer (if it doesn't exist)
		err = c.addSecretFinalizers(ctx, secret)
		if err != nil {
			return err
		}
	} else {
		if finalizers := sets.NewString(secret.Finalizers...); !finalizers.Has(MCFinalizerName) {
			// Finalizer doesn't exist, simply return nil
			return nil
		}
		err = c.deleteSecretFinalizers(ctx, secret)
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

func (c *controller) addSecretFinalizers(ctx context.Context, secret *corev1.Secret) error {
	clone := secret.DeepCopy()

	if finalizers := sets.NewString(clone.Finalizers...); !finalizers.Has(MCFinalizerName) {
		finalizers.Insert(MCFinalizerName)
		return c.updateSecretFinalizers(ctx, clone, finalizers.List())
	}
	return nil
}

func (c *controller) deleteSecretFinalizers(ctx context.Context, secret *corev1.Secret) error {
	clone := secret.DeepCopy()

	if finalizers := sets.NewString(clone.Finalizers...); finalizers.Has(MCFinalizerName) {
		finalizers.Delete(MCFinalizerName)
		return c.updateSecretFinalizers(ctx, clone, finalizers.List())
	}
	return nil
}

func (c *controller) updateSecretFinalizers(ctx context.Context, secret *corev1.Secret, finalizers []string) error {
	// Get the latest version of the secret so that we can avoid conflicts
	secret, err := c.controlCoreClient.CoreV1().Secrets(secret.Namespace).Get(ctx, secret.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	clone := secret.DeepCopy()
	clone.Finalizers = finalizers
	_, err = c.controlCoreClient.CoreV1().Secrets(clone.Namespace).Update(ctx, clone, metav1.UpdateOptions{})

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

func enqueueSecretForReferences(queue workqueue.RateLimitingInterface, secretRefs ...*corev1.SecretReference) {
	for _, secretRef := range secretRefs {
		if secretRef != nil {
			queue.Add(secretRef.Namespace + "/" + secretRef.Name)
		}
	}
}

func enqueueSecretForReferenceIfChanged(queue workqueue.RateLimitingInterface, oldSecretRef, newSecretRef *corev1.SecretReference) {
	if !apiequality.Semantic.DeepEqual(oldSecretRef, newSecretRef) {
		if oldSecretRef != nil {
			queue.Add(oldSecretRef.Namespace + "/" + oldSecretRef.Name)
		}
		if newSecretRef != nil {
			queue.Add(newSecretRef.Namespace + "/" + newSecretRef.Name)
		}
	}
}

func (c *controller) machineClassToSecretAdd(obj interface{}) {
	machineClass, ok := obj.(*v1alpha1.MachineClass)
	if !ok || machineClass == nil || machineClass.SecretRef == nil {
		return
	}

	enqueueSecretForReferences(c.secretQueue, machineClass.SecretRef, machineClass.CredentialsSecretRef)
}

func (c *controller) machineClassToSecretUpdate(oldObj interface{}, newObj interface{}) {
	oldMachineClass, ok := oldObj.(*v1alpha1.MachineClass)
	if !ok || oldMachineClass == nil || oldMachineClass.SecretRef == nil {
		return
	}
	newMachineClass, ok := newObj.(*v1alpha1.MachineClass)
	if !ok || newMachineClass == nil || newMachineClass.SecretRef == nil {
		return
	}

	enqueueSecretForReferenceIfChanged(c.secretQueue, oldMachineClass.SecretRef, newMachineClass.SecretRef)
	enqueueSecretForReferenceIfChanged(c.secretQueue, oldMachineClass.CredentialsSecretRef, newMachineClass.CredentialsSecretRef)
}

func (c *controller) machineClassToSecretDelete(obj interface{}) {
	c.machineClassToSecretAdd(obj)
}
