// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helpers

import (
	"context"
	"log"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"

	"k8s.io/client-go/util/retry"
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

// CreateMachineDeployment creates a test-machine-deployment with a specified number of replicas and returns error if it occurs
func (c *Cluster) CreateMachineDeployment(namespace string, gnaSecretName string, replicas int32) error {
	mcd := NewMachineDeployment(namespace, gnaSecretName, replicas)
	_, err := c.McmClient.
		MachineV1alpha1().
		MachineDeployments(namespace).
		Create(context.Background(), &mcd, metav1.CreateOptions{})
	return err
}

// IsTestMachineDeleted returns boolean value of presence of 'test-machine' object
func (c *Cluster) IsTestMachineDeleted(ctx context.Context, namespace string) bool {
	_, err := c.McmClient.
		MachineV1alpha1().
		Machines(namespace).
		Get(ctx, "test-machine", metav1.GetOptions{})

	return errors.IsNotFound(err)
}

// GetRunningMachineList lists all running machines that contain the given machine labels and returns the list of such machines
func (c *Cluster) GetRunningMachineList(ctx context.Context, namespace string) ([]v1alpha1.Machine, error) {
	selector := labels.SelectorFromSet(testLabels)

	var runningMachines []v1alpha1.Machine

	machineList, err := c.McmClient.MachineV1alpha1().Machines(namespace).List(
		ctx,
		metav1.ListOptions{
			LabelSelector: selector.String(),
		},
	)
	if err != nil {
		log.Printf("error listing machines: %v\n", err)
		return nil, err
	}

	for _, mc := range machineList.Items {
		if mc.Status.CurrentStatus.Phase == v1alpha1.MachineRunning {
			runningMachines = append(runningMachines, mc)
		}
	}

	return runningMachines, nil
}

// IsMachineDeleted returns boolean value indicating whether the specified machine is deleted or not
func (c *Cluster) IsMachineDeleted(ctx context.Context, machineName string, namespace string) bool {
	_, err := c.McmClient.
		MachineV1alpha1().
		Machines(namespace).
		Get(ctx, machineName, metav1.GetOptions{})

	return errors.IsNotFound(err)
}

// AreMachinesRunning returns boolean value indicating whether all the machines names passed to it are in the running state or not
func (c *Cluster) AreMachinesRunning(ctx context.Context, machineNames []string, namespace string) bool {
	for _, mcName := range machineNames {
		mc, err := c.McmClient.
			MachineV1alpha1().
			Machines(namespace).
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

// AreFailingMachinesPreserved checks if all the specified machines are in the Failed phase and are preserved
func (c *Cluster) AreFailingMachinesPreserved(ctx context.Context, namespace string, machineNames []string) bool {
	for _, mcName := range machineNames {
		mc, err := c.McmClient.
			MachineV1alpha1().
			Machines(namespace).
			Get(ctx, mcName, metav1.GetOptions{})
		if err != nil {
			log.Printf("error listing machine %s: %v\n", mcName, err)
			return false
		}

		if mc.Status.CurrentStatus.PreserveExpiryTime == nil || mc.Status.CurrentStatus.Phase != v1alpha1.MachineFailed {
			return false
		}
	}

	return true
}

// NewMachineDeployment returns a MachineDeployment object with the specified namespace, gnaSecretName, replicas and machineLabels
func NewMachineDeployment(namespace string, gnaSecretName string, replicas int32) v1alpha1.MachineDeployment {
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

// CreateOrUpdateMcd creates or updates a MachineDeployment with the specified namespace, gnaSecretName, replicas and machineLabels
func (c *Cluster) CreateOrUpdateMcd(ctx context.Context, mcd v1alpha1.MachineDeployment, namespace string) error {
	_, err := c.McmClient.MachineV1alpha1().MachineDeployments(namespace).Create(ctx, &mcd, metav1.CreateOptions{})
	if errors.IsAlreadyExists(err) {
		retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			existingMCD, err := c.McmClient.MachineV1alpha1().MachineDeployments(namespace).Get(ctx, mcd.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			mcd.ResourceVersion = existingMCD.ResourceVersion
			_, updateErr := c.McmClient.MachineV1alpha1().MachineDeployments(namespace).Update(ctx, &mcd, metav1.UpdateOptions{})
			return updateErr
		})
		return retryErr
	}
	return err
}
