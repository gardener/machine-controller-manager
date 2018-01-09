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
	
	"github.com/gardener/node-controller-manager/pkg/driver"
	machineapi "github.com/gardener/node-controller-manager/pkg/apis/machine"
	"github.com/gardener/node-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/node-controller-manager/pkg/apis/machine/validation"
)

/* 
	SECTION
	Machine controller - Machine add, update, delete watches
*/
func (c *controller) machineAdd(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		glog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.machineQueue.Add(key)
}

func (c *controller) machineUpdate(oldObj, newObj interface{}) {
	c.machineAdd(newObj)
}

func (c *controller) machineDelete(obj interface{}) {
	c.machineAdd(obj)
}

func (c *controller) enqueueMachineAfter(obj interface{}, after time.Duration) {
	key, err := KeyFunc(obj)
	if err != nil {
		return
	}
	c.machineQueue.AddAfter(key, after)
}

func (c *controller) reconcileClusterMachineKey(key string) error {
	machine, err := c.machineLister.Get(key)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		glog.Error("ClusterMachine %q: Unable to retrieve object from store: %v", key, err)
		return err
	}

	return c.reconcileClusterMachine(machine)
}

func (c *controller) reconcileClusterMachine(machine *v1alpha1.Machine) error {
	
	/*	
	glog.V(2).Info("Start Reconciling machine: ", machine.Name)
	defer func() {
		glog.V(2).Info("Stop Reconciling machine: ", machine.Name)
	}()*/

	if !shouldReconcileMachine(machine, time.Now()) {
		return nil
	}

	// Validate Machine
	internalMachine := &machineapi.Machine{}
	err := api.Scheme.Convert(machine, internalMachine, nil)
	if err != nil {
		return err
	}
	validationerr := validation.ValidateMachine(internalMachine)
	if validationerr.ToAggregate() != nil && len(validationerr.ToAggregate().Errors()) > 0 {
		glog.V(2).Infof("Validation of Machine failled %s", validationerr.ToAggregate().Error())
		return nil
	}

	AWSMachineClass, err := c.awsMachineClassLister.Get(machine.Spec.Class.Name)
	if err != nil {
		glog.V(2).Infof("AWSMachineClass for Machine %q not found %q. Skipping. %v", machine.Name, machine.Spec.Class.Name, err)
		return nil
	}
	// Do not modify the original objects in any way!
	AWSMachineClassCopy := AWSMachineClass.DeepCopy()
	//glog.Info(AWSMachineClassCopy)

	// Validate AWSMachineClass
	internalAWSMachineClass := &machineapi.AWSMachineClass{}
	err = api.Scheme.Convert(AWSMachineClass, internalAWSMachineClass, nil)
	if err != nil {
		return err
	}
	validationerr = validation.ValidateAWSMachineClass(internalAWSMachineClass)
	if validationerr.ToAggregate() != nil && len(validationerr.ToAggregate().Errors()) > 0 {
		glog.V(2).Infof("Validation of AWSMachineClass failled %s", validationerr.ToAggregate().Error())
		return nil
	}

	// Get secretRef
	secretRef, err := c.getSecret(AWSMachineClass.Spec.SecretRef, AWSMachineClass)
	if err != nil || secretRef == nil {
		return err
	}

	splitProviderId := strings.Split(machine.Spec.ProviderID, "/")
	machineID := splitProviderId[len(splitProviderId) - 1]
	driver := driver.NewDriver(machineID, AWSMachineClassCopy, secretRef, machine.Spec.Class.Kind)
	actualID, err := driver.GetExisting()
	if err != nil {
		return err
	} else if actualID == "fake" {
		glog.Info("Fake")
		return nil
	}

	//glog.Info(actualID, machineID)

	// Get the latest version of the machine so that we can avoid conflicts
	machine, err = c.nodeClient.Machines().Get(machine.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if machine.DeletionTimestamp != nil {
		// Deleting machine
		err := c.deleteMachine(machine, driver)
		if err != nil {
			return err
		}

	} else if machine.Status.CurrentStatus.TimeoutActive {
		// Processing machine	
		c.checkMachineTimeout(machine)

	} else {
		// Processing of create or update event
		//glog.V(2).Info("Processing Create/Update")
		c.addMachineFinalizers(machine)
		
		if actualID == "" {
			// Creating machine
			err := c.createMachine(machine, AWSMachineClass.Spec.AvailabilityZone, driver)
			if err != nil {
				return err
			}

		} else if actualID != machineID {
			// Updating machine
			err := c.updateMachine(machine, AWSMachineClass.Spec.AvailabilityZone, actualID)
			if err != nil {
				return err
			}
		}

	} 

	return nil
}

/*
	SECTION
	Machine controller - nodeToMachine
*/
func (c *controller) nodeToMachineAdd(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		glog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.nodeToMachineQueue.Add(key)
}

func (c *controller) nodeToMachineUpdate(oldObj, newObj interface{}) {
	c.nodeToMachineAdd(newObj)
}

func (c *controller) nodeToMachineDelete(obj interface{}) {
	c.nodeToMachineAdd(obj)
}

func (c *controller) reconcileClusterNodeToMachineKey(key string) error {
	node, err := c.nodeLister.Get(key)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		glog.Error("ClusterNode %q: Unable to retrieve object from store: %v", key, err)
		return err
	}

	return c.reconcileClusterNodeToMachine(node)
}

func (c *controller) reconcileClusterNodeToMachine(node *v1.Node) error {
    machine, err := c.getMachineFromNode(node.Name)

	if err != nil {
		glog.Error("Couldn't fetch machine: ", err)
		return err
	} else if machine == nil {
		return nil
	}
	
	err = c.updateMachineState(machine, node)
	if err != nil {
		return err
	}
	
	return nil
}

/*
	SECTION
	NodeToMachine operations 
*/

func (c *controller) getMachineFromNode(nodeName string) (*v1alpha1.Machine, error) {
	
	var list []string
    list = append(list, nodeName)
	
	selector := labels.NewSelector()
    req, _ := labels.NewRequirement("node", selection.Equals, list)
    selector = selector.Add(*req)
    machines, _ := c.machineLister.List(selector)

    if len(machines) > 1 {
    	err := errors.New("Multiple machines matching node")
        return nil, err
    } else if len(machines) < 1 {
    	return nil, nil
    }

    return machines[0], nil
}

func (c *controller) updateMachineState (machine *v1alpha1.Machine, node *v1.Node) error {
	
	machine = c.updateMachineConditions(machine, node.Status.Conditions)

	if machine.Status.LastOperation.State != "Successful" {
		
		if machine.Status.LastOperation.Type == "Create" {
			
			if machine.Status.LastOperation.Description == "Creating machine on cloud provider" {
				// Machine is ready but yet to join the cluster
				lastOperation := v1alpha1.LastOperation {
					Description: 	"Waiting for machine to join the cluster (Not Ready)",
					State: 			"Processing",
					Type:			"Create",
					LastUpdateTime: metav1.Now(),
				}
				currentStatus := v1alpha1.CurrentStatus {
					Phase:			v1alpha1.MachineAvailable,
					TimeoutActive:	true,
					LastUpdateTime: machine.Status.CurrentStatus.LastUpdateTime,
				}
				c.updateMachineStatus(machine, lastOperation, currentStatus)	

			} else if machine.Status.LastOperation.Description == "Waiting for machine to join the cluster (Not Ready)" && len(machine.Status.Conditions)==4 && machine.Status.Conditions[3].Status == "True" {
				// Machine is ready and has joined the cluster
				lastOperation := v1alpha1.LastOperation {
					Description: 	"Machine is now ready",
					State: 			"Successful",
					Type:			"Create",
					LastUpdateTime: metav1.Now(),
				}
				c.updateMachineStatus(machine, lastOperation, machine.Status.CurrentStatus)

			}

		}
	}
	return nil
}

/*
	SECTION
	Machine operations - Create, Update, Delete
*/

func (c *controller) createMachine (machine *v1alpha1.Machine, availabilityZone string, driver driver.Driver) error {
	//glog.V(2).Infof("Creating machine %s", machine.Name)
	
	actualID, private_dns, err := driver.Create()
	if err != nil {

		lastOperation := v1alpha1.LastOperation {
			Description: 	"Cloud provider message - " + err.Error(),
			State: 			"Failed",
			Type:			"Create",
			LastUpdateTime: metav1.Now(),
		}
		currentStatus := v1alpha1.CurrentStatus {
			Phase:			v1alpha1.MachineFailed,
			TimeoutActive:	false,
			LastUpdateTime: metav1.Now(),
		}
		c.updateMachineStatus(machine, lastOperation, currentStatus)

		return err
	}
	glog.V(2).Infof("Created machine: %s, AWS Machine-id: %s", machine.Name, actualID)

	for {
		// Get the latest version of the machine so that we can avoid conflicts
		machine, err := c.nodeClient.Machines().Get(machine.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		lastOperation := v1alpha1.LastOperation {
			Description: 	"Creating machine on cloud provider",
			State: 			"Processing",
			Type:			"Create",
			LastUpdateTime: metav1.Now(),
		}
		currentStatus := v1alpha1.CurrentStatus {
			Phase:			v1alpha1.MachinePending,
			TimeoutActive:	true,
			LastUpdateTime: metav1.Now(),
		}

		clone := machine.DeepCopy()
		clone.Spec.ProviderID = "aws:///" + availabilityZone + "/" + actualID
		if clone.Labels == nil {
			clone.Labels = make(map[string]string)
		}
		clone.Labels["node"] = private_dns
		clone.Status.Node = private_dns
		clone.Status.LastOperation = lastOperation
		clone.Status.CurrentStatus = currentStatus
		
		_, err = c.nodeClient.Machines().Update(clone)
		if err == nil {
			break
		}
		glog.Warning("Updated failed, retrying")
	}

	return nil
}


func (c *controller) updateMachine (machine *v1alpha1.Machine, availabilityZone string, actualID string) error {
	glog.V(2).Infof("Setting MachineId of %s to %s", machine.Name, actualID)

	for {
		machine, err := c.nodeClient.Machines().Get(machine.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		clone := machine.DeepCopy()
		clone.Spec.ProviderID = "aws:///" + availabilityZone + "/" + actualID
		lastOperation := v1alpha1.LastOperation {
			Description: 	"Updated provider ID",
			State: 			"Successful",
			Type:			"Update",
			LastUpdateTime: metav1.Now(),
		}
		clone.Status.LastOperation = lastOperation
		
		_, err = c.nodeClient.Machines().Update(clone)
		if err == nil {
			break
		}
		glog.Warning("Updated failed, retrying")
	}

	return nil
}

func (c *controller) deleteMachine (machine *v1alpha1.Machine, driver driver.Driver) error {
	
	if finalizers := sets.NewString(machine.Finalizers...); finalizers.Has(DeleteFinalizerName) {
		
		var err error
		glog.V(2).Infof("Deleting Machine %s", machine.Name)
		
		// If machine was created on the cloud provider
		machineID, _ := driver.GetExisting()

		if machineID == "" {
			err = errors.New("No provider-ID found on machine")
		} else {
			err = driver.Delete()
		}

		if err != nil {
			// When machine deletion fails
			glog.V(2).Infof("Deletion failed: %s", err)
			
			lastOperation := v1alpha1.LastOperation {
				Description: 	"Cloud provider message - " + err.Error(),
				State: 			"Failed",
				Type:			"Delete",
				LastUpdateTime: metav1.Now(),
			}
			currentStatus := v1alpha1.CurrentStatus {
				Phase:			v1alpha1.MachineFailed,
				TimeoutActive:	false,
				LastUpdateTime: metav1.Now(),
			}
			c.updateMachineStatus(machine, lastOperation, currentStatus)

			return err
		}	
		c.deleteMachineFinalizers(machine)
		c.nodeClient.Machines().Delete(machine.Name, &metav1.DeleteOptions{})
	}
	return nil
}

/*
	SECTION
	Update machine object
*/

func (c *controller) updateMachineStatus (
	machine *v1alpha1.Machine, 
	lastOperation v1alpha1.LastOperation, 
	currentStatus v1alpha1.CurrentStatus,
){
	// Get the latest version of the machine so that we can avoid conflicts
	machine, err := c.nodeClient.Machines().Get(machine.Name, metav1.GetOptions{})
	if err != nil {
		return
	}

	clone := machine.DeepCopy()
	clone.Status.LastOperation = lastOperation
	clone.Status.CurrentStatus = currentStatus

	_, err = c.nodeClient.Machines().Update(clone)
	if err != nil {
		// Keep retrying until update goes through
		glog.V(4).Info("Warning: Updated failed, retrying")
		c.updateMachineStatus(machine, lastOperation, currentStatus)
	} 
}

func (c *controller) updateMachineConditions (machine *v1alpha1.Machine, conditions []v1.NodeCondition) *v1alpha1.Machine {
	// Get the latest version of the machine so that we can avoid conflicts
	machine, err := c.nodeClient.Machines().Get(machine.Name, metav1.GetOptions{})
	if err != nil {
		return machine
	}

	clone := machine.DeepCopy()
	clone.Status.Conditions = conditions

	if clone.Status.CurrentStatus.Phase == v1alpha1.MachineFailed ||
		clone.Status.CurrentStatus.Phase == v1alpha1.MachineTerminating {
		// If machine is already in failed state, don't update
		clone.Status.CurrentStatus = clone.Status.CurrentStatus

	} else if !c.isHealthy(clone) && clone.Status.CurrentStatus.Phase == v1alpha1.MachineRunning {
		currentStatus := v1alpha1.CurrentStatus {
			Phase:			v1alpha1.MachineUnknown,
			TimeoutActive:	true,
			LastUpdateTime: metav1.Now(),
		}
		clone.Status.CurrentStatus = currentStatus
	} else if c.isHealthy(clone) && clone.Status.CurrentStatus.Phase != v1alpha1.MachineRunning {
		currentStatus := v1alpha1.CurrentStatus {
			Phase:			v1alpha1.MachineRunning,
			TimeoutActive:	false,
			LastUpdateTime: metav1.Now(),
		}
		clone.Status.CurrentStatus = currentStatus
	}

	clone, err = c.nodeClient.Machines().Update(clone)
	if err != nil {
		// Keep retrying until update goes through
		glog.V(2).Info("Warning: Updated failed, retrying")
		c.updateMachineConditions(machine, conditions)
		return machine
	} 

	return clone
}

func (c *controller) updateMachineFinalizers(machine *v1alpha1.Machine, finalizers []string) {
	// Get the latest version of the machine so that we can avoid conflicts
	machine, err := c.nodeClient.Machines().Get(machine.Name, metav1.GetOptions{})
	if err != nil {
		return
	}

	clone := machine.DeepCopy()
	clone.Finalizers = finalizers
	_, err = c.nodeClient.Machines().Update(clone)
	if err != nil {
		// Keep retrying until update goes through
		glog.V(4).Info("Warning: Updated failed, retrying")
		c.updateMachineFinalizers(machine, finalizers)
	} 
}

/*
	SECTION
	Manipulate Finalizers
*/

func (c *controller) addMachineFinalizers (machine *v1alpha1.Machine) {
	clone := machine.DeepCopy()

	if finalizers := sets.NewString(clone.Finalizers...); !finalizers.Has(DeleteFinalizerName) {
		finalizers.Insert(DeleteFinalizerName)
		c.updateMachineFinalizers(clone, finalizers.List())
	}
}

func (c *controller) deleteMachineFinalizers (machine *v1alpha1.Machine) {
	clone := machine.DeepCopy()

	if finalizers := sets.NewString(clone.Finalizers...); finalizers.Has(DeleteFinalizerName) {
		finalizers.Delete(DeleteFinalizerName)
		c.updateMachineFinalizers(clone, finalizers.List())
	}
}

/*
	SECTION
	Helper Functions
*/
func (c *controller) isHealthy (machine *v1alpha1.Machine) bool {

	//TODO: Change index numbers to status names
	if len(machine.Status.Conditions) == 0 {
		return false
	} else if machine.Status.Conditions[0].Status == "True" {
		return false
	} else if machine.Status.Conditions[1].Status == "True" {
		return false
	} else if machine.Status.Conditions[2].Status == "True" {
		return false
	} else if machine.Status.Conditions[3].Status != "True" {
		return false
	}
	return true
}

func (c *controller) getSecret(ref *v1.SecretReference, AWSMachineClass *v1alpha1.AWSMachineClass) (*v1.Secret, error) {
	secretRef, err := c.secretLister.Secrets(ref.Namespace).Get(ref.Name)
	if apierrors.IsNotFound(err) {
		glog.V(2).Infof("No secret %q: found for AWSMachineClass %q", ref, AWSMachineClass.Name)
		return nil, nil
	}
	if err != nil {
		glog.Errorf("Unable get secret %q for AWSMachineClass %q: %v", AWSMachineClass.Name, ref, err)
		return nil, err
	}
	return secretRef, err
}

func (c *controller) checkMachineTimeout (machine *v1alpha1.Machine) {

	if machine.Status.CurrentStatus.Phase != v1alpha1.MachineRunning {

		timeOutDuration := 5 * time.Minute
		sleepTime := 1 * time.Minute

		// Timeout value obtained by subtracting last operation with expected time out period
		timeOut := metav1.Now().Add(-timeOutDuration).Sub(machine.Status.CurrentStatus.LastUpdateTime.Time)
		
		//glog.V(2).Info("TIMEOUT: ", machine.Name, " ", timeOut)

		if timeOut > 0 {
			
			currentStatus := v1alpha1.CurrentStatus {
				Phase:			v1alpha1.MachineFailed,
				TimeoutActive:	false,
				LastUpdateTime: metav1.Now(),
			}

			if machine.Status.CurrentStatus.Phase == v1alpha1.MachinePending {  
				lastOperation := v1alpha1.LastOperation {
					Description: 	machine.Status.LastOperation.Description,
					State: 			"Failed",
					Type:			machine.Status.LastOperation.Type,
					LastUpdateTime: metav1.Now(),
				}
				c.updateMachineStatus(machine, lastOperation, currentStatus)

			} else {
				c.updateMachineStatus(machine, machine.Status.LastOperation, currentStatus)

			}
		} else {
			currentStatus := v1alpha1.CurrentStatus {
				Phase:			machine.Status.CurrentStatus.Phase,
				TimeoutActive:	true,
				LastUpdateTime: machine.Status.CurrentStatus.LastUpdateTime,
			}
			c.updateMachineStatus(machine, machine.Status.LastOperation, currentStatus)

			/*
			time.Sleep(sleepTime)
			machine, err := c.nodeClient.Machines().Get(machine.Name, metav1.GetOptions{})
			if err != nil {
				return
			}
			c.reconcileClusterMachine(machine)*/

			c.enqueueMachineAfter(machine, sleepTime)
		}
	}
}

func shouldReconcileMachine(machine *v1alpha1.Machine, now time.Time) bool {
	if machine.DeletionTimestamp != nil {
		return true
	}
	if machine.Spec.ProviderID == "" {
		return true
	}
	// TODO add more cases where this will be false

	return true
}
