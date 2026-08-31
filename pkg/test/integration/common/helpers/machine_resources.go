// SPDX-FileCopyrightText: Contributors to the Gardener project
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
	"k8s.io/client-go/util/retry"
)

const (
	gnaSecretNameLabelKey = "worker.gardener.cloud/gardener-node-agent-secret-name"
	// McdName is the name of the test machine deployment
	McdName = "test-machine-deployment"
	// InPlaceMcdNameHappyPath is the name of the MCD used for the happy path inplace update test
	InPlaceMcdNameHappyPath = "test-mcd-inplace-happy"
	// InPlaceMcdNameFailure is the name of the MCD used for the inplace update failure test
	InPlaceMcdNameFailure = "test-mcd-inplace-failure"
	// InPlaceMcdNameTimeout is the name of the MCD used for the inplace update timeout test
	InPlaceMcdNameTimeout = "test-mcd-inplace-timeout"
	// InPlaceMcdNameManual is the name of the MCD used for the manual orchestration inplace update test
	InPlaceMcdNameManual = "test-mcd-inplace-manual"
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
	labels := map[string]string{"test-label": "test-label"}
	_, err := c.McmClient.
		MachineV1alpha1().
		MachineDeployments(namespace).
		Create(
			context.Background(),
			&v1alpha1.MachineDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-machine-deployment",
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
						MatchLabels: labels,
					},
					Template: v1alpha1.MachineTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: labels,
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

// IsMachineDeploymentDeleted returns boolean value indicating whether the specified machine deployment is deleted or not
func (c *Cluster) IsMachineDeploymentDeleted(ctx context.Context, machineDeploymentName string, namespace string) bool {
	_, err := c.McmClient.
		MachineV1alpha1().
		MachineDeployments(namespace).
		Get(ctx, machineDeploymentName, metav1.GetOptions{})

	return errors.IsNotFound(err)
}

// NewMachineDeployment returns a MachineDeployment object with the specified namespace, gnaSecretName, replicas and machineLabels
func NewMachineDeployment(namespace string, gnaSecretName string, replicas int32) v1alpha1.MachineDeployment {
	mcd := v1alpha1.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      McdName,
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

// NewMachineDeploymentWithName returns a MachineDeployment with a custom name and a unique label derived from that name,
// so that machines belonging to different MCDs can be listed independently.
func NewMachineDeploymentWithName(name string, namespace string, gnaSecretName string, replicas int32) v1alpha1.MachineDeployment {
	mcdLabels := map[string]string{"test-label": name}
	mcd := NewMachineDeployment(namespace, gnaSecretName, replicas)
	mcd.Name = name
	mcd.Spec.Selector = &metav1.LabelSelector{MatchLabels: mcdLabels}
	mcd.Spec.Template.Labels = mcdLabels
	return mcd
}

// GetRunningMachineListByLabel lists all running machines with the given label selector
func (c *Cluster) GetRunningMachineListByLabel(ctx context.Context, namespace string, machineLabels map[string]string) ([]v1alpha1.Machine, error) {
	selector := labels.SelectorFromSet(machineLabels)
	machineList, err := c.McmClient.MachineV1alpha1().Machines(namespace).List(
		ctx,
		metav1.ListOptions{LabelSelector: selector.String()},
	)
	if err != nil {
		log.Printf("error listing machines: %v\n", err)
		return nil, err
	}
	var runningMachines []v1alpha1.Machine
	for _, mc := range machineList.Items {
		if mc.Status.CurrentStatus.Phase == v1alpha1.MachineRunning {
			runningMachines = append(runningMachines, mc)
		}
	}
	return runningMachines, nil
}

// InPlaceMcdLabels returns the unique label map used for a given inplace test MCD name
func InPlaceMcdLabels(mcdName string) map[string]string {
	return map[string]string{"test-label": mcdName}
}

// CreateOrUpdateMcd creates or updates the given MachineDeployment in the specified namespace
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

// AreMachinesInPhase returns true if all specified machines are in the given phase
func (c *Cluster) AreMachinesInPhase(ctx context.Context, machineNames []string, namespace string, phase v1alpha1.MachinePhase) bool {
	for _, mcName := range machineNames {
		mc, err := c.McmClient.
			MachineV1alpha1().
			Machines(namespace).
			Get(ctx, mcName, metav1.GetOptions{})

		if err != nil {
			log.Println("error fetching machine: ", err)
			return false
		}

		if mc.Status.CurrentStatus.Phase != phase {
			return false
		}
	}

	return true
}
