// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helpers

import (
	"context"
	"log"
	"os"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	gnaSecretNameLabelKey = "worker.gardener.cloud/gardener-node-agent-secret-name"
	mcdName               = "test-machine-deployment"
)

var (
	testLabels = map[string]string{"test-label": "test-label"}
)

// CreateMachine creates a test-machine using machineclass "test-mc"
func (c *Cluster) CreateMachine(namespace string, gnaSecretName string) error {
	_, err := c.McmClient.
		MachineV1alpha1().
		Machines(namespace).
		Create(
			context.Background(),
			&v1alpha1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-machine",
					Namespace: namespace,
				},
				Spec: v1alpha1.MachineSpec{
					Class: v1alpha1.ClassSpec{
						Kind: "MachineClass",
						Name: "test-mc-v1",
					},
					NodeTemplateSpec: v1alpha1.NodeTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								gnaSecretNameLabelKey: gnaSecretName,
							},
						},
					},
				},
			},
			metav1.CreateOptions{},
		)
	return err
}

// CreateMachineDeployment creates a test-machine-deployment with 3 replicas and returns error if it occurs
func (c *Cluster) CreateMachineDeployment(namespace string, gnaSecretName string, replicas int32) error {
	_, err := c.McmClient.
		MachineV1alpha1().
		MachineDeployments(namespace).
		Create(
			context.Background(),
			&v1alpha1.MachineDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      mcdName,
					Namespace: namespace,
				},
				Spec: v1alpha1.MachineDeploymentSpec{
					Replicas:        replicas,
					MinReadySeconds: 500,
					Strategy: v1alpha1.MachineDeploymentStrategy{
						Type: v1alpha1.RollingUpdateMachineDeploymentStrategyType,
						RollingUpdate: &v1alpha1.RollingUpdateMachineDeployment{
							UpdateConfiguration: v1alpha1.UpdateConfiguration{
								MaxSurge:       &intstr.IntOrString{IntVal: 2},
								MaxUnavailable: &intstr.IntOrString{IntVal: 1},
							},
						},
					},
					Selector: &metav1.LabelSelector{
						MatchLabels: testLabels,
					},
					Template: v1alpha1.MachineTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: testLabels,
						},
						Spec: v1alpha1.MachineSpec{
							Class: v1alpha1.ClassSpec{
								Kind: "MachineClass",
								Name: "test-mc-v1",
							},
							NodeTemplateSpec: v1alpha1.NodeTemplateSpec{
								ObjectMeta: metav1.ObjectMeta{
									Labels: map[string]string{
										gnaSecretNameLabelKey: gnaSecretName,
									},
								},
							},
						},
					},
				},
			},
			metav1.CreateOptions{},
		)
	return err
}

// IsTestMachineDeleted returns boolean value of presence of 'test-machine' object
func (c *Cluster) IsTestMachineDeleted() bool {
	controlClusterNamespace := os.Getenv("CONTROL_CLUSTER_NAMESPACE")
	_, err := c.McmClient.
		MachineV1alpha1().
		Machines(controlClusterNamespace).
		Get(context.Background(), "test-machine", metav1.GetOptions{})

	return errors.IsNotFound(err)
}

// GetMachineList lists all machines that contain the given machine labels and returns the list of machines
func (c *Cluster) GetMachineList(ctx context.Context) *v1alpha1.MachineList {
	selector := labels.SelectorFromSet(testLabels)
	controlClusterNamespace := os.Getenv("CONTROL_CLUSTER_NAMESPACE")

	machineList, err := c.McmClient.MachineV1alpha1().Machines(controlClusterNamespace).List(
		ctx,
		metav1.ListOptions{
			LabelSelector: selector.String(),
		},
	)
	if err != nil {
		log.Printf("error listing machines: %v", err)
		return nil
	}

	return machineList
}

// IsMachineDeleted returns boolean value indicating whether the specified machine is deleted or not
func (c *Cluster) IsMachineDeleted(ctx context.Context, machineName string) bool {
	controlClusterNamespace := os.Getenv("CONTROL_CLUSTER_NAMESPACE")
	_, err := c.McmClient.
		MachineV1alpha1().
		Machines(controlClusterNamespace).
		Get(ctx, machineName, metav1.GetOptions{})

	return errors.IsNotFound(err)
}

// IsMachineRunning returns boolean value indicating whether the specified machine is in the running state or not
func (c *Cluster) IsMachineRunning(ctx context.Context, machineNames []string) bool {
	controlClusterNamespace := os.Getenv("CONTROL_CLUSTER_NAMESPACE")
	for _, mcName := range machineNames {
		mc, err := c.McmClient.
			MachineV1alpha1().
			Machines(controlClusterNamespace).
			Get(ctx, mcName, metav1.GetOptions{})

		if err != nil {
			log.Println("error fetching machine: ", err)
			return false
		}

		if mc.Status.CurrentStatus.Phase != v1alpha1.MachineRunning {
			return false
		}
	}

	return true
}

// IsFailingMachinePreserved returns boolean value indicating whether the specified machine is in the failed state and preserved or not
func (c *Cluster) IsFailingMachinePreserved(ctx context.Context, namespace string, machineNames []string) bool {
	for _, mcName := range machineNames {
		mc, err := c.McmClient.
			MachineV1alpha1().
			Machines(namespace).
			Get(ctx, mcName, metav1.GetOptions{})
		if err != nil {
			log.Printf("error listing machines: %v", err)
			return false
		}

		if mc.Status.CurrentStatus.PreserveExpiryTime == nil || mc.Status.CurrentStatus.Phase != v1alpha1.MachineFailed {
			return false
		}
	}

	return true
}

// GetMCD returns a MachineDeployment object with the specified namespace, gnaSecretName, replicas and machineLabels
func GetMCD(namespace string, gnaSecretName string, replicas int32) v1alpha1.MachineDeployment {
	mcd := v1alpha1.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mcdName,
			Namespace: namespace,
		},
		Spec: v1alpha1.MachineDeploymentSpec{
			Replicas:        replicas,
			MinReadySeconds: 500,
			Strategy: v1alpha1.MachineDeploymentStrategy{
				Type: v1alpha1.RollingUpdateMachineDeploymentStrategyType,
				RollingUpdate: &v1alpha1.RollingUpdateMachineDeployment{
					UpdateConfiguration: v1alpha1.UpdateConfiguration{
						MaxSurge:       &intstr.IntOrString{IntVal: 2},
						MaxUnavailable: &intstr.IntOrString{IntVal: 1},
					},
				},
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: testLabels,
			},
			Template: v1alpha1.MachineTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: testLabels,
				},
				Spec: v1alpha1.MachineSpec{
					Class: v1alpha1.ClassSpec{
						Kind: "MachineClass",
						Name: "test-mc-v1",
					},
					NodeTemplateSpec: v1alpha1.NodeTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								gnaSecretNameLabelKey: gnaSecretName,
							},
						},
					},
				},
			},
		},
	}

	return mcd
}
