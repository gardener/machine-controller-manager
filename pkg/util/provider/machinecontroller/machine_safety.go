/*
Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved.

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
	"strings"
	"time"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/cache"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/driver"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machinecodes/codes"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machineutils"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
)

// reconcileClusterMachineSafetyOrphanVMs checks for any orphan VMs and deletes them
func (c *controller) reconcileClusterMachineSafetyOrphanVMs(key string) error {
	ctx := context.Background()
	reSyncAfter := c.safetyOptions.MachineSafetyOrphanVMsPeriod.Duration
	defer c.machineSafetyOrphanVMsQueue.AddAfter("", reSyncAfter)

	klog.V(3).Infof("reconcileClusterMachineSafetyOrphanVMs: Start")
	defer klog.V(3).Infof("reconcileClusterMachineSafetyOrphanVMs: End, reSync-Period: %v", reSyncAfter)

	retryPeriod, err := c.checkMachineClasses(ctx)
	if err != nil {
		klog.Errorf("reconcileClusterMachineSafetyOrphanVMs: Error occurred while checking for orphan VMs: %s", err)
		c.machineSafetyOrphanVMsQueue.AddAfter("", time.Duration(retryPeriod))
	}

	retryPeriod, err = c.AnnotateNodesUnmanagedByMCM(ctx)
	if err != nil {
		klog.Errorf("reconcileClusterMachineSafetyOrphanVMs: Error occurred while checking for nodes not handled by MCM: %s", err)
		c.machineSafetyOrphanVMsQueue.AddAfter("", time.Duration(retryPeriod))
	}

	return nil
}

// reconcileClusterMachineSafetyAPIServer checks control and target clusters
// and checks if their APIServer's are reachable
// If they are not reachable, they set a machineControllerFreeze flag
func (c *controller) reconcileClusterMachineSafetyAPIServer(key string) error {
	ctx := context.Background()
	statusCheckTimeout := c.safetyOptions.MachineSafetyAPIServerStatusCheckTimeout.Duration
	statusCheckPeriod := c.safetyOptions.MachineSafetyAPIServerStatusCheckPeriod.Duration

	klog.V(4).Infof("reconcileClusterMachineSafetyAPIServer: Start")
	defer klog.V(4).Infof("reconcileClusterMachineSafetyAPIServer: Stop")

	if c.safetyOptions.MachineControllerFrozen {
		// MachineController is frozen
		if c.isAPIServerUp(ctx) {
			// APIServer is up now, hence we need reset all machine health checks (to avoid unwanted freezes) and unfreeze
			machines, err := c.machineLister.List(labels.Everything())
			if err != nil {
				klog.Error("SafetyController: Unable to LIST machines. Error:", err)
				return err
			}
			for _, machine := range machines {
				if machine.Status.CurrentStatus.Phase == v1alpha1.MachineUnknown {
					machine, err := c.controlMachineClient.Machines(c.namespace).Get(ctx, machine.Name, metav1.GetOptions{})
					if err != nil {
						klog.Error("SafetyController: Unable to GET machines. Error:", err)
						return err
					}

					machine.Status.CurrentStatus = v1alpha1.CurrentStatus{
						Phase:          v1alpha1.MachineRunning,
						TimeoutActive:  false,
						LastUpdateTime: metav1.Now(),
					}
					machine.Status.LastOperation = v1alpha1.LastOperation{
						Description:    "Machine Health Timeout was reset due to APIServer being unreachable",
						LastUpdateTime: metav1.Now(),
						State:          v1alpha1.MachineStateSuccessful,
						Type:           v1alpha1.MachineOperationHealthCheck,
					}
					_, err = c.controlMachineClient.Machines(c.namespace).UpdateStatus(ctx, machine, metav1.UpdateOptions{})
					if err != nil {
						klog.Error("SafetyController: Unable to UPDATE machine/status. Error:", err)
						return err
					}

					klog.V(2).Infof("SafetyController: Reinitializing machine health check for machine: %q with backing node: %q and providerID: %q", machine.Name, getNodeName(machine), getProviderID(machine))
				}

				// En-queue after 30 seconds, to ensure all machine phases are reconciled to their actual value
				// as they are currently reset to `Running`
				c.enqueueMachineAfter(machine, 30*time.Second, "kube-api-servers are up again, so reconcile of machine phase is needed")
			}

			c.safetyOptions.MachineControllerFrozen = false
			c.safetyOptions.APIserverInactiveStartTime = time.Time{}
			klog.V(2).Infof("SafetyController: UnFreezing Machine Controller")
		}
	} else {
		// MachineController is not frozen
		if !c.isAPIServerUp(ctx) {
			// If APIServer is not up
			if c.safetyOptions.APIserverInactiveStartTime.Equal(time.Time{}) {
				// If timeout has not started
				c.safetyOptions.APIserverInactiveStartTime = time.Now()
			}
			if time.Since(c.safetyOptions.APIserverInactiveStartTime) > statusCheckTimeout {
				// If APIServer has been down for more than statusCheckTimeout
				c.safetyOptions.MachineControllerFrozen = true
				klog.V(2).Infof("SafetyController: Freezing Machine Controller")
			}

			// Re-enqueue the safety check more often if APIServer is not active and is not frozen yet
			defer c.machineSafetyAPIServerQueue.AddAfter("", statusCheckTimeout/5)
			return nil
		}
	}

	defer c.machineSafetyAPIServerQueue.AddAfter("", statusCheckPeriod)
	return nil
}

// isAPIServerUp returns true if APIServers are up
// Both control and target APIServers
func (c *controller) isAPIServerUp(ctx context.Context) bool {
	// Dummy get call to check if control APIServer is reachable
	_, err := c.controlMachineClient.Machines(c.namespace).Get(ctx, "dummy_name", metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		// Get returns an error other than object not found = Assume APIServer is not reachable
		klog.Error("SafetyController: Unable to GET on machine objects ", err)
		return false
	}

	// Dummy get call to check if target APIServer is reachable
	_, err = c.targetCoreClient.CoreV1().Nodes().Get(ctx, "dummy_name", metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		// Get returns an error other than object not found = Assume APIServer is not reachable
		klog.Error("SafetyController: Unable to GET on node objects ", err)
		return false
	}

	return true
}

// AnnotateNodesUnmanagedByMCM checks for nodes which are not handled by MCM and annotes them
func (c *controller) AnnotateNodesUnmanagedByMCM(ctx context.Context) (machineutils.RetryPeriod, error) {
	// list all the nodes on target cluster
	nodes, err := c.nodeLister.List(labels.Everything())
	if err != nil {
		klog.Errorf("Safety-Net: Error getting nodes")
		return machineutils.LongRetry, err
	}
	for _, node := range nodes {
		machine, err := c.getMachineFromNode(node.Name)
		if err != nil {
			if err == errMultipleMachineMatch {
				klog.Errorf("Couldn't fetch machine, Error: %s", err)
			} else if err == errNoMachineMatch {

				if !node.CreationTimestamp.Time.Before(time.Now().Add(c.safetyOptions.MachineCreationTimeout.Duration * -1)) {
					// node creationTimestamp is NOT before now() - machineCreationTime
					// meaning creationTimeout has not occurred since node creation
					// hence don't tag such nodes
					klog.V(3).Infof("Node %q is still too young to be tagged with NotManagedByMCM", node.Name)
					continue
				} else if _, annotationPresent := node.ObjectMeta.Annotations[machineutils.NotManagedByMCM]; annotationPresent {
					// annotation already exists, ignore this node
					continue
				}

				// if no backing machine object for a node, annotate it
				nodeCopy := node.DeepCopy()
				annotations := map[string]string{
					machineutils.NotManagedByMCM: "1",
				}

				klog.V(3).Infof("Adding NotManagedByMCM annotation to Node %q", node.Name)
				// err is returned only when node update fails
				if err := c.updateNodeWithAnnotations(ctx, nodeCopy, annotations); err != nil {
					return machineutils.MediumRetry, err
				}
			}
		} else {
			_, hasAnnot := node.Annotations[machineutils.NotManagedByMCM]
			if !hasAnnot {
				continue
			}
			klog.V(3).Infof("Removing NotManagedByMCM annotation from Node %q associated with Machine %q", node.Name, machine.Name)
			nodeCopy := node.DeepCopy()
			delete(nodeCopy.Annotations, machineutils.NotManagedByMCM)
			if err := c.updateNodeWithAnnotations(ctx, nodeCopy, nil); err != nil {
				return machineutils.MediumRetry, err
			}
		}
	}

	return machineutils.LongRetry, nil
}

// checkCommonMachineClass checks for orphan VMs in MachinesClasses
func (c *controller) checkMachineClasses(ctx context.Context) (machineutils.RetryPeriod, error) {
	MachineClasses, err := c.machineClassLister.List(labels.Everything())
	if err != nil {
		klog.Error("Safety-Net: Error getting machineClasses")
		return machineutils.LongRetry, err
	}

	for _, machineClass := range MachineClasses {
		retry, err := c.checkMachineClass(ctx, machineClass)
		if err != nil {
			return retry, err
		}
	}

	return machineutils.LongRetry, nil
}

// checkMachineClass checks a particular machineClass for orphan instances
func (c *controller) checkMachineClass(ctx context.Context, machineClass *v1alpha1.MachineClass) (machineutils.RetryPeriod, error) {

	// Get secret data
	secretData, err := c.getSecretData(machineClass.Name, machineClass.SecretRef, machineClass.CredentialsSecretRef)
	if err != nil {
		klog.Errorf("SafetyController: Secret Data could not be computed for MachineClass: %q", machineClass.Name)
		return machineutils.LongRetry, err
	}

	listMachineResponse, err := c.driver.ListMachines(ctx, &driver.ListMachinesRequest{
		MachineClass: machineClass,
		Secret:       &corev1.Secret{Data: secretData},
	})
	if err != nil {
		klog.Errorf("SafetyController: Failed to LIST VMs at provider. Error: %s", err)
		return machineutils.LongRetry, err
	}

	// making sure cache is updated .This is for cases where a new machine object is at etcd, but cache is unaware
	// and its correspinding VM is in the list.
	if len(listMachineResponse.MachineList) > 0 {
		stopCh := make(chan struct{})
		defer close(stopCh)

		if !cache.WaitForCacheSync(stopCh, c.machineSynced) {
			klog.Errorf("SafetyController: Timed out waiting for caches to sync. Error: %s", err)
			return machineutils.ShortRetry, err
		}
	}

	for machineID, machineName := range listMachineResponse.MachineList {
		machine, err := c.machineLister.Machines(c.namespace).Get(machineName)

		if apierrors.IsNotFound(err) || err == nil {
			if err == nil {
				if machine.Spec.ProviderID == machineID {
					continue
				}
				// machine obj is still being processed by the machine controller
				if machine.Status.CurrentStatus.Phase == "" || machine.Status.CurrentStatus.Phase == v1alpha1.MachineCrashLoopBackOff {
					klog.V(3).Infof("SafetyController: Machine object %q with backing nodeName %q , providerID %q is being processed by machine controller, hence skipping", machine.Name, getNodeName(machine), getProviderID(machine))
					continue
				}
			}

			// Creating a dummy machine object to create deleteMachineRequest
			machine = &v1alpha1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name: machineName,
				},
				Spec: v1alpha1.MachineSpec{
					ProviderID: machineID,
				},
			}

			_, err := c.driver.DeleteMachine(ctx, &driver.DeleteMachineRequest{
				Machine:      machine,
				MachineClass: machineClass,
				Secret:       &corev1.Secret{Data: secretData},
			})
			if err != nil {
				klog.Errorf("SafetyController: Error while trying to DELETE VM on CP - %s. Shall retry in next safety controller sync.", err)
			} else {
				klog.V(2).Infof("SafetyController: Orphan VM found and terminated VM: %s, %s", machineName, machineID)
			}
		} else {
			// errors other than NotFound error
			klog.Errorf("SafetyController: Error while trying to GET machine %s. Error: %s", machineName, err)
		}
	}

	return machineutils.LongRetry, nil
}

// updateMachineToSafety enqueues into machineSafetyQueue when a machine is updated to particular status
func (c *controller) updateMachineToSafety(oldObj, newObj interface{}) {
	oldMachine := oldObj.(*v1alpha1.Machine)
	newMachine := newObj.(*v1alpha1.Machine)

	if oldMachine == nil || newMachine == nil {
		klog.Errorf("Couldn't convert to machine resource from object")
		return
	}

	if !strings.Contains(oldMachine.Status.LastOperation.Description, codes.OutOfRange.String()) && strings.Contains(newMachine.Status.LastOperation.Description, codes.OutOfRange.String()) {
		klog.Warningf("Multiple VMs backing machine obj %q found, triggering orphan collection.", newMachine.Name)
		c.enqueueMachineSafetyOrphanVMsKey(newMachine)
	}
}

// deleteMachineToSafety enqueues into machineSafetyQueue when a new machine is deleted
func (c *controller) deleteMachineToSafety(obj interface{}) {
	machine := obj.(*v1alpha1.Machine)
	if machine == nil {
		klog.Errorf("Couldn't convert to machine resource from object")
		return
	}
	c.enqueueMachineSafetyOrphanVMsKey(machine)
}

// enqueueMachineSafetyOrphanVMsKey enqueues into machineSafetyOrphanVMsQueue
func (c *controller) enqueueMachineSafetyOrphanVMsKey(obj interface{}) {
	c.machineSafetyOrphanVMsQueue.Add("")
}
