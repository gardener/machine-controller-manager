package helpers

import (
	"context"
	"os"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// CreateMachine creates a test-machine using machineclass "test-mc"
func (c *Cluster) CreateMachine(namespace string) error {
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
				},
			},
			metav1.CreateOptions{},
		)
	return err
}

// CreateMachineDeployment creates a test-machine-deployment with 3 replicas and returns error if it occurs
func (c *Cluster) CreateMachineDeployment(namespace string) error {
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
					Replicas:        3,
					MinReadySeconds: 500,
					Strategy: v1alpha1.MachineDeploymentStrategy{
						Type: v1alpha1.RollingUpdateMachineDeploymentStrategyType,
						RollingUpdate: &v1alpha1.RollingUpdateMachineDeployment{
							MaxSurge:       &intstr.IntOrString{IntVal: 2},
							MaxUnavailable: &intstr.IntOrString{IntVal: 1},
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
