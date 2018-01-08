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
	"errors"
	"time"
	"strings"
	
	"github.com/golang/glog"

	"k8s.io/api/core/v1"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	
	"github.com/gardener/node-controller-manager/pkg/apis/node/v1alpha1"
	"github.com/gardener/node-controller-manager/pkg/driver"
	"github.com/gardener/node-controller-manager/pkg/apis/node/validation"
	"github.com/gardener/node-controller-manager/pkg/apis/node"
)

/* 
	SECTION
	Instance controller - Instance add, update, delete watches
*/
func (c *controller) instanceAdd(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		glog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.instanceQueue.Add(key)
}

func (c *controller) instanceUpdate(oldObj, newObj interface{}) {
	c.instanceAdd(newObj)
}

func (c *controller) instanceDelete(obj interface{}) {
	c.instanceAdd(obj)
}

func (c *controller) enqueueInstanceAfter(obj interface{}, after time.Duration) {
	key, err := KeyFunc(obj)
	if err != nil {
		return
	}
	c.instanceQueue.AddAfter(key, after)
}

func (c *controller) reconcileClusterInstanceKey(key string) error {
	instance, err := c.instanceLister.Get(key)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		glog.Error("ClusterInstance %q: Unable to retrieve object from store: %v", key, err)
		return err
	}

	return c.reconcileClusterInstance(instance)
}

func (c *controller) reconcileClusterInstance(instance *v1alpha1.Instance) error {
	
	/*	
	glog.V(2).Info("Start Reconciling instance: ", instance.Name)
	defer func() {
		glog.V(2).Info("Stop Reconciling instance: ", instance.Name)
	}()*/

	if !shouldReconcileInstance(instance, time.Now()) {
		return nil
	}

	// Validate Instance
	internalInstance := &node.Instance{}
	err := api.Scheme.Convert(instance, internalInstance, nil)
	if err != nil {
		return err
	}
	validationerr := validation.ValidateInstance(internalInstance)
	if validationerr.ToAggregate() != nil && len(validationerr.ToAggregate().Errors()) > 0 {
		glog.V(2).Infof("Validation of Instance failled %s", validationerr.ToAggregate().Error())
		return nil
	}

	AWSInstanceClass, err := c.awsInstanceClassLister.Get(instance.Spec.Class.Name)
	if err != nil {
		glog.V(2).Infof("AWSInstanceClass for Instance %q not found %q. Skipping. %v", instance.Name, instance.Spec.Class.Name, err)
		return nil
	}
	// Do not modify the original objects in any way!
	AWSInstanceClassCopy := AWSInstanceClass.DeepCopy()
	//glog.Info(AWSInstanceClassCopy)

	// Validate AWSInstanceClass
	internalAWSInstanceClass := &node.AWSInstanceClass{}
	err = api.Scheme.Convert(AWSInstanceClass, internalAWSInstanceClass, nil)
	if err != nil {
		return err
	}
	validationerr = validation.ValidateAWSInstanceClass(internalAWSInstanceClass)
	if validationerr.ToAggregate() != nil && len(validationerr.ToAggregate().Errors()) > 0 {
		glog.V(2).Infof("Validation of AWSInstanceClass failled %s", validationerr.ToAggregate().Error())
		return nil
	}

	// Get secretRef
	secretRef, err := c.getSecret(AWSInstanceClass.Spec.SecretRef, AWSInstanceClass)
	if err != nil || secretRef == nil {
		return err
	}

	splitProviderId := strings.Split(instance.Spec.ProviderID, "/")
	instanceID := splitProviderId[len(splitProviderId) - 1]
	driver := driver.NewDriver(instanceID, AWSInstanceClassCopy, secretRef, instance.Spec.Class.Kind)
	actualID, err := driver.GetExisting()
	if err != nil {
		return err
	} else if actualID == "fake" {
		glog.Info("Fake")
		return nil
	}

	//glog.Info(actualID, instanceID)

	// Get the latest version of the instance so that we can avoid conflicts
	instance, err = c.nodeClient.Instances().Get(instance.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if instance.DeletionTimestamp != nil {
		// Deleting instance
		err := c.deleteInstance(instance, driver)
		if err != nil {
			return err
		}

	} else if instance.Status.CurrentStatus.TimeoutActive {
		// Processing instance	
		c.checkInstanceTimeout(instance)

	} else {
		// Processing of create or update event
		//glog.V(2).Info("Processing Create/Update")
		c.addInstanceFinalizers(instance)
		
		if actualID == "" {
			// Creating instance
			err := c.createInstance(instance, AWSInstanceClass.Spec.AvailabilityZone, driver)
			if err != nil {
				return err
			}

		} else if actualID != instanceID {
			// Updating instance
			err := c.updateInstance(instance, AWSInstanceClass.Spec.AvailabilityZone, actualID)
			if err != nil {
				return err
			}
		}

	} 

	return nil
}

/*
	SECTION
	Instance controller - nodeToInstance
*/
func (c *controller) nodeToInstanceAdd(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		glog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.nodeToInstanceQueue.Add(key)
}

func (c *controller) nodeToInstanceUpdate(oldObj, newObj interface{}) {
	c.nodeToInstanceAdd(newObj)
}

func (c *controller) nodeToInstanceDelete(obj interface{}) {
	c.nodeToInstanceAdd(obj)
}

func (c *controller) reconcileClusterNodeToInstanceKey(key string) error {
	node, err := c.nodeLister.Get(key)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		glog.Error("ClusterNode %q: Unable to retrieve object from store: %v", key, err)
		return err
	}

	return c.reconcileClusterNodeToInstance(node)
}

func (c *controller) reconcileClusterNodeToInstance(node *v1.Node) error {
    instance, err := c.getInstanceFromNode(node.Name)

	if err != nil {
		glog.Error("Couldn't fetch instance: ", err)
		return err
	} else if instance == nil {
		return nil
	}
	
	err = c.updateInstanceState(instance, node)
	if err != nil {
		return err
	}
	
	return nil
}

/*
	SECTION
	NodeToInstance operations 
*/

func (c *controller) getInstanceFromNode(nodeName string) (*v1alpha1.Instance, error) {
	
	var list []string
    list = append(list, nodeName)
	
	selector := labels.NewSelector()
    req, _ := labels.NewRequirement("node", selection.Equals, list)
    selector = selector.Add(*req)
    instances, _ := c.instanceLister.List(selector)

    if len(instances) > 1 {
    	err := errors.New("Multiple instances matching node")
        return nil, err
    } else if len(instances) < 1 {
    	return nil, nil
    }

    return instances[0], nil
}

func (c *controller) updateInstanceState (instance *v1alpha1.Instance, node *v1.Node) error {
	
	instance = c.updateInstanceConditions(instance, node.Status.Conditions)

	if instance.Status.LastOperation.State != "Successful" {
		
		if instance.Status.LastOperation.Type == "Create" {
			
			if instance.Status.LastOperation.Description == "Creating instance on cloud provider" {
				// Instance is ready but yet to join the cluster
				lastOperation := v1alpha1.LastOperation {
					Description: 	"Waiting for instance to join the cluster (Not Ready)",
					State: 			"Processing",
					Type:			"Create",
					LastUpdateTime: metav1.Now(),
				}
				currentStatus := v1alpha1.CurrentStatus {
					Phase:			v1alpha1.InstanceAvailable,
					TimeoutActive:	true,
					LastUpdateTime: instance.Status.CurrentStatus.LastUpdateTime,
				}
				c.updateInstanceStatus(instance, lastOperation, currentStatus)	

			} else if instance.Status.LastOperation.Description == "Waiting for instance to join the cluster (Not Ready)" && instance.Status.Conditions[3].Status == "True" {
				// Instance is ready and has joined the cluster
				lastOperation := v1alpha1.LastOperation {
					Description: 	"Instance is now ready",
					State: 			"Successful",
					Type:			"Create",
					LastUpdateTime: metav1.Now(),
				}
				c.updateInstanceStatus(instance, lastOperation, instance.Status.CurrentStatus)

			}

		}
	}
	return nil
}

/*
	SECTION
	Instance operations - Create, Update, Delete
*/

func (c *controller) createInstance (instance *v1alpha1.Instance, availabilityZone string, driver driver.Driver) error {
	//glog.V(2).Infof("Creating instance %s", instance.Name)
	
	actualID, private_dns, err := driver.Create()
	if err != nil {

		lastOperation := v1alpha1.LastOperation {
			Description: 	"Cloud provider message - " + err.Error(),
			State: 			"Failed",
			Type:			"Create",
			LastUpdateTime: metav1.Now(),
		}
		currentStatus := v1alpha1.CurrentStatus {
			Phase:			v1alpha1.InstanceFailed,
			TimeoutActive:	false,
			LastUpdateTime: metav1.Now(),
		}
		c.updateInstanceStatus(instance, lastOperation, currentStatus)

		return err
	}
	glog.V(2).Infof("Created instance: %s, AWS Instance-id: %s", instance.Name, actualID)

	for {
		// Get the latest version of the instance so that we can avoid conflicts
		instance, err := c.nodeClient.Instances().Get(instance.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		lastOperation := v1alpha1.LastOperation {
			Description: 	"Creating instance on cloud provider",
			State: 			"Processing",
			Type:			"Create",
			LastUpdateTime: metav1.Now(),
		}
		currentStatus := v1alpha1.CurrentStatus {
			Phase:			v1alpha1.InstancePending,
			TimeoutActive:	true,
			LastUpdateTime: metav1.Now(),
		}

		clone := instance.DeepCopy()
		clone.Spec.ProviderID = "aws:///" + availabilityZone + "/" + actualID
		if clone.Labels == nil {
			clone.Labels = make(map[string]string)
		}
		clone.Labels["node"] = private_dns
		clone.Status.Node = private_dns
		clone.Status.LastOperation = lastOperation
		clone.Status.CurrentStatus = currentStatus
		
		_, err = c.nodeClient.Instances().Update(clone)
		if err == nil {
			break
		}
		glog.Warning("Updated failed, retrying")
	}

	return nil
}


func (c *controller) updateInstance (instance *v1alpha1.Instance, availabilityZone string, actualID string) error {
	glog.V(2).Infof("Setting InstanceId of %s to %s", instance.Name, actualID)

	for {
		instance, err := c.nodeClient.Instances().Get(instance.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		clone := instance.DeepCopy()
		clone.Spec.ProviderID = "aws:///" + availabilityZone + "/" + actualID
		lastOperation := v1alpha1.LastOperation {
			Description: 	"Updated provider ID",
			State: 			"Successful",
			Type:			"Update",
			LastUpdateTime: metav1.Now(),
		}
		clone.Status.LastOperation = lastOperation
		
		_, err = c.nodeClient.Instances().Update(clone)
		if err == nil {
			break
		}
		glog.Warning("Updated failed, retrying")
	}

	return nil
}

func (c *controller) deleteInstance (instance *v1alpha1.Instance, driver driver.Driver) error {
	
	if finalizers := sets.NewString(instance.Finalizers...); finalizers.Has(DeleteFinalizerName) {
		
		var err error
		glog.V(2).Infof("Deleting Instance %s", instance.Name)
		
		// If instance was created on the cloud provider
		instanceID, _ := driver.GetExisting()

		if instanceID == "" {
			err = errors.New("No provider-ID found on instance")
		} else {
			err = driver.Delete()
		}

		if err != nil {
			// When instance deletion fails
			glog.V(2).Infof("Deletion failed: %s", err)
			
			lastOperation := v1alpha1.LastOperation {
				Description: 	"Cloud provider message - " + err.Error(),
				State: 			"Failed",
				Type:			"Delete",
				LastUpdateTime: metav1.Now(),
			}
			currentStatus := v1alpha1.CurrentStatus {
				Phase:			v1alpha1.InstanceFailed,
				TimeoutActive:	false,
				LastUpdateTime: metav1.Now(),
			}
			c.updateInstanceStatus(instance, lastOperation, currentStatus)

			return err
		}	
		c.deleteInstanceFinalizers(instance)
		c.nodeClient.Instances().Delete(instance.Name, &metav1.DeleteOptions{})
	}
	return nil
}

/*
	SECTION
	Update instance object
*/

func (c *controller) updateInstanceStatus (
	instance *v1alpha1.Instance, 
	lastOperation v1alpha1.LastOperation, 
	currentStatus v1alpha1.CurrentStatus,
){
	// Get the latest version of the instance so that we can avoid conflicts
	instance, err := c.nodeClient.Instances().Get(instance.Name, metav1.GetOptions{})
	if err != nil {
		return
	}

	clone := instance.DeepCopy()
	clone.Status.LastOperation = lastOperation
	clone.Status.CurrentStatus = currentStatus

	_, err = c.nodeClient.Instances().Update(clone)
	if err != nil {
		// Keep retrying until update goes through
		glog.V(4).Info("Warning: Updated failed, retrying")
		c.updateInstanceStatus(instance, lastOperation, currentStatus)
	} 
}

func (c *controller) updateInstanceConditions (instance *v1alpha1.Instance, conditions []v1.NodeCondition) *v1alpha1.Instance {
	// Get the latest version of the instance so that we can avoid conflicts
	instance, err := c.nodeClient.Instances().Get(instance.Name, metav1.GetOptions{})
	if err != nil {
		return instance
	}

	clone := instance.DeepCopy()
	clone.Status.Conditions = conditions

	if clone.Status.CurrentStatus.Phase == v1alpha1.InstanceFailed ||
		clone.Status.CurrentStatus.Phase == v1alpha1.InstanceTerminating {
		// If instance is already in failed state, don't update
		clone.Status.CurrentStatus = clone.Status.CurrentStatus

	} else if !c.isHealthy(clone) && clone.Status.CurrentStatus.Phase == v1alpha1.InstanceRunning {
		currentStatus := v1alpha1.CurrentStatus {
			Phase:			v1alpha1.InstanceUnknown,
			TimeoutActive:	true,
			LastUpdateTime: metav1.Now(),
		}
		clone.Status.CurrentStatus = currentStatus
	} else if c.isHealthy(clone) && clone.Status.CurrentStatus.Phase != v1alpha1.InstanceRunning {
		currentStatus := v1alpha1.CurrentStatus {
			Phase:			v1alpha1.InstanceRunning,
			TimeoutActive:	false,
			LastUpdateTime: metav1.Now(),
		}
		clone.Status.CurrentStatus = currentStatus
	}

	clone, err = c.nodeClient.Instances().Update(clone)
	if err != nil {
		// Keep retrying until update goes through
		glog.V(4).Info("Warning: Updated failed, retrying")
		c.updateInstanceConditions(instance, conditions)
		return instance
	} 

	return clone
}

func (c *controller) updateInstanceFinalizers(instance *v1alpha1.Instance, finalizers []string) {
	// Get the latest version of the instance so that we can avoid conflicts
	instance, err := c.nodeClient.Instances().Get(instance.Name, metav1.GetOptions{})
	if err != nil {
		return
	}

	clone := instance.DeepCopy()
	clone.Finalizers = finalizers
	_, err = c.nodeClient.Instances().Update(clone)
	if err != nil {
		// Keep retrying until update goes through
		glog.V(4).Info("Warning: Updated failed, retrying")
		c.updateInstanceFinalizers(instance, finalizers)
	} 
}

/*
	SECTION
	Manipulate Finalizers
*/

func (c *controller) addInstanceFinalizers (instance *v1alpha1.Instance) {
	clone := instance.DeepCopy()

	if finalizers := sets.NewString(clone.Finalizers...); !finalizers.Has(DeleteFinalizerName) {
		finalizers.Insert(DeleteFinalizerName)
		c.updateInstanceFinalizers(clone, finalizers.List())
	}
}

func (c *controller) deleteInstanceFinalizers (instance *v1alpha1.Instance) {
	clone := instance.DeepCopy()

	if finalizers := sets.NewString(clone.Finalizers...); finalizers.Has(DeleteFinalizerName) {
		finalizers.Delete(DeleteFinalizerName)
		c.updateInstanceFinalizers(clone, finalizers.List())
	}
}

/*
	SECTION
	Helper Functions
*/
func (c *controller) isHealthy (instance *v1alpha1.Instance) bool {

	//TODO: Change index numbers to status names
	if instance.Status.Conditions[0].Status == "True" {
		return false
	} else if instance.Status.Conditions[1].Status == "True" {
		return false
	} else if instance.Status.Conditions[2].Status == "True" {
		return false
	} else if instance.Status.Conditions[3].Status != "True" {
		return false
	}
	return true
}

func (c *controller) getSecret(ref *v1.SecretReference, AWSInstanceClass *v1alpha1.AWSInstanceClass) (*v1.Secret, error) {
	secretRef, err := c.secretLister.Secrets(ref.Namespace).Get(ref.Name)
	if apierrors.IsNotFound(err) {
		glog.V(2).Infof("No secret %q: found for AWSInstanceClass %q", ref, AWSInstanceClass.Name)
		return nil, nil
	}
	if err != nil {
		glog.Errorf("Unable get secret %q for AWSInstanceClass %q: %v", AWSInstanceClass.Name, ref, err)
		return nil, err
	}
	return secretRef, err
}

func (c *controller) checkInstanceTimeout (instance *v1alpha1.Instance) {

	if instance.Status.CurrentStatus.Phase != v1alpha1.InstanceRunning {

		timeOutDuration := 5 * time.Minute
		sleepTime := 1 * time.Minute

		// Timeout value obtained by subtracting last operation with expected time out period
		timeOut := metav1.Now().Add(-timeOutDuration).Sub(instance.Status.CurrentStatus.LastUpdateTime.Time)
		
		//glog.V(2).Info("TIMEOUT: ", instance.Name, " ", timeOut)

		if timeOut > 0 {
			
			currentStatus := v1alpha1.CurrentStatus {
				Phase:			v1alpha1.InstanceFailed,
				TimeoutActive:	false,
				LastUpdateTime: metav1.Now(),
			}

			if instance.Status.CurrentStatus.Phase == v1alpha1.InstancePending {  
				lastOperation := v1alpha1.LastOperation {
					Description: 	instance.Status.LastOperation.Description,
					State: 			"Failed",
					Type:			instance.Status.LastOperation.Type,
					LastUpdateTime: metav1.Now(),
				}
				c.updateInstanceStatus(instance, lastOperation, currentStatus)

			} else {
				c.updateInstanceStatus(instance, instance.Status.LastOperation, currentStatus)

			}
		} else {
			currentStatus := v1alpha1.CurrentStatus {
				Phase:			instance.Status.CurrentStatus.Phase,
				TimeoutActive:	true,
				LastUpdateTime: instance.Status.CurrentStatus.LastUpdateTime,
			}
			c.updateInstanceStatus(instance, instance.Status.LastOperation, currentStatus)

			/*
			time.Sleep(sleepTime)
			instance, err := c.nodeClient.Instances().Get(instance.Name, metav1.GetOptions{})
			if err != nil {
				return
			}
			c.reconcileClusterInstance(instance)*/

			c.enqueueInstanceAfter(instance, sleepTime)
		}
	}
}

func shouldReconcileInstance(instance *v1alpha1.Instance, now time.Time) bool {
	if instance.DeletionTimestamp != nil {
		return true
	}
	if instance.Spec.ProviderID == "" {
		return true
	}
	// TODO add more cases where this will be false

	return true
}
