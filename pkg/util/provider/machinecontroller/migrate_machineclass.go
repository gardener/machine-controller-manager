// Package controller is used to provide the core functionalities of machine-controller-manager
package controller

import (
	"context"
	"fmt"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/driver"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machinecodes/codes"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machinecodes/status"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machineutils"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog"
)

const (
	// AlicloudMachineClassKind is used to identify the machineClassKind as Alicloud
	AlicloudMachineClassKind = "AlicloudMachineClass"
	// AWSMachineClassKind is used to identify the machineClassKind as AWS
	AWSMachineClassKind = "AWSMachineClass"
	// AzureMachineClassKind is used to identify the machineClassKind as Azure
	AzureMachineClassKind = "AzureMachineClass"
	// GCPMachineClassKind is used to identify the machineClassKind as GCP
	GCPMachineClassKind = "GCPMachineClass"
	// OpenStackMachineClassKind is used to identify the machineClassKind as OpenStack
	OpenStackMachineClassKind = "OpenStackMachineClass"
	// PacketMachineClassKind is used to identify the machineClassKind as Packet
	PacketMachineClassKind = "PacketMachineClass"
	// MachineController is the string constant to identify the controller responsible for the migration
	MachineController = "machine-controller"
)

// getProviderSpecificMachineClass returns the providerSpecificMachineClass object for the provided machineClass
func (c *controller) getProviderSpecificMachineClass(classSpec *v1alpha1.ClassSpec) (interface{}, error) {
	var (
		machineClass interface{}
		err          error
	)

	switch classSpec.Kind {
	case AlicloudMachineClassKind:
		machineClass, err = c.controlMachineClient.AlicloudMachineClasses(c.namespace).Get(classSpec.Name, metav1.GetOptions{
			TypeMeta:        metav1.TypeMeta{},
			ResourceVersion: "",
		})
	case AWSMachineClassKind:
		machineClass, err = c.controlMachineClient.AWSMachineClasses(c.namespace).Get(classSpec.Name, metav1.GetOptions{
			TypeMeta:        metav1.TypeMeta{},
			ResourceVersion: "",
		})
	case AzureMachineClassKind:
		machineClass, err = c.controlMachineClient.AzureMachineClasses(c.namespace).Get(classSpec.Name, metav1.GetOptions{
			TypeMeta:        metav1.TypeMeta{},
			ResourceVersion: "",
		})
	case GCPMachineClassKind:
		machineClass, err = c.controlMachineClient.GCPMachineClasses(c.namespace).Get(classSpec.Name, metav1.GetOptions{
			TypeMeta:        metav1.TypeMeta{},
			ResourceVersion: "",
		})
	case OpenStackMachineClassKind:
		machineClass, err = c.controlMachineClient.OpenStackMachineClasses(c.namespace).Get(classSpec.Name, metav1.GetOptions{
			TypeMeta:        metav1.TypeMeta{},
			ResourceVersion: "",
		})
	case PacketMachineClassKind:
		machineClass, err = c.controlMachineClient.PacketMachineClasses(c.namespace).Get(classSpec.Name, metav1.GetOptions{
			TypeMeta:        metav1.TypeMeta{},
			ResourceVersion: "",
		})
	default:
		return nil, fmt.Errorf("Machine class kind not found")
	}

	if err != nil {
		return nil, err
	}

	return machineClass, nil
}

// createMachineClass creates the generic machineClass corresponding to the providerMachineClass
func (c *controller) createMachineClass(providerSpecificMachineClass interface{}, classSpec *v1alpha1.ClassSpec) (machineutils.Retry, error) {

	machineClass, err := c.machineClassLister.MachineClasses(c.namespace).Get(classSpec.Name)
	if err != nil && apierrors.IsNotFound(err) {
		// MachineClass doesn't exist, initialize a new one
		machineClass = &v1alpha1.MachineClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: classSpec.Name,
			},
		}
		machineClass, err = c.controlMachineClient.MachineClasses(c.namespace).Create(machineClass)
		if err != nil {
			return machineutils.RetryOp, err
		}

	} else if err != nil {
		// Anyother kind of error while fetching the machineClass object
		return machineutils.RetryOp, err
	}

	cloneMachineClass := machineClass.DeepCopy()

	// Generate the generic machineClass object from driver
	_, err = c.driver.GenerateMachineClassForMigration(
		context.TODO(),
		&driver.GenerateMachineClassForMigrationRequest{
			ProviderSpecificMachineClass: providerSpecificMachineClass,
			MachineClass:                 cloneMachineClass,
			ClassSpec:                    classSpec,
		},
	)
	if err != nil {
		retry := machineutils.DoNotRetryOp

		if machineErr, ok := status.FromError(err); !ok {
			// Do nothing
		} else {
			// Decoding machineErr error code
			switch machineErr.Code() {
			case codes.Unimplemented:
				retry = machineutils.DoNotRetryOp
			default:
				retry = machineutils.RetryOp
			}
			err = machineErr
		}
		klog.V(3).Infof("Couldn't fill up the machine class %s/%s. Error: %s", classSpec.Kind, classSpec.Name, err)
		return retry, err
	}

	klog.V(1).Infof("Generated generic machineClass for class %s/%s", classSpec.Kind, classSpec.Name)

	// MachineClass exists, and needs to be updated
	_, err = c.controlMachineClient.MachineClasses(c.namespace).Update(cloneMachineClass)
	if err != nil {
		return machineutils.RetryOp, err
	}

	klog.V(2).Infof("Create/Apply successful for MachineClass %s", machineClass.Name)

	return machineutils.RetryOp, err
}

// updateClassReferences updates all machine objects to refer to the new MachineClass.
func (c *controller) updateClassReferences(classSpec *v1alpha1.ClassSpec) error {

	// Update Machines
	machineList, err := c.machineLister.Machines(c.namespace).List(labels.Everything())
	if err != nil {
		return err
	}
	for _, machine := range machineList {
		if machine.Spec.Class.Name == classSpec.Name && machine.Spec.Class.Kind == classSpec.Kind {
			clone := machine.DeepCopy()
			clone.Spec.Class.Kind = machineutils.MachineClassKind

			_, err := c.controlMachineClient.Machines(c.namespace).Update(clone)
			if err != nil {
				return err
			}
			klog.V(1).Infof("Updated class reference for machine %s/%s", c.namespace, machine.Name)
		}
	}

	// Update MachineSets
	machinesetList, err := c.controlMachineClient.MachineSets(c.namespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, machineset := range machinesetList.Items {
		if machineset.Spec.Template.Spec.Class.Name == classSpec.Name &&
			machineset.Spec.Template.Spec.Class.Kind == classSpec.Kind {
			clone := machineset.DeepCopy()
			clone.Spec.Template.Spec.Class.Kind = machineutils.MachineClassKind

			_, err := c.controlMachineClient.MachineSets(c.namespace).Update(clone)
			if err != nil {
				return err
			}
			klog.V(1).Infof("Updated class reference for machineset %s/%s", c.namespace, machineset.Name)
		}
	}

	// Update MachineSets
	machinedeploymentList, err := c.controlMachineClient.MachineDeployments(c.namespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, machinedeployment := range machinedeploymentList.Items {
		if machinedeployment.Spec.Template.Spec.Class.Name == classSpec.Name &&
			machinedeployment.Spec.Template.Spec.Class.Kind == classSpec.Kind {
			clone := machinedeployment.DeepCopy()
			clone.Spec.Template.Spec.Class.Kind = machineutils.MachineClassKind

			_, err := c.controlMachineClient.MachineDeployments(c.namespace).Update(clone)
			if err != nil {
				return err
			}
			klog.V(1).Infof("Updated class reference for machinedeployment %s/%s", c.namespace, machinedeployment.Name)
		}
	}

	return nil
}

// addMigratedAnnotationForProviderMachineClass adds ignore provider MachineClass annotation
func (c *controller) addMigratedAnnotationForProviderMachineClass(classSpec *v1alpha1.ClassSpec) error {

	switch classSpec.Kind {

	case AlicloudMachineClassKind:
		providerSpecificMachineClass, err := c.controlMachineClient.AlicloudMachineClasses(c.namespace).Get(classSpec.Name, metav1.GetOptions{
			TypeMeta:        metav1.TypeMeta{},
			ResourceVersion: "",
		})
		if err != nil {
			return err
		}

		clone := providerSpecificMachineClass.DeepCopy()
		if clone.Annotations == nil {
			clone.Annotations = make(map[string]string)
		}
		clone.Annotations[machineutils.MigratedMachineClass] = MachineController + "-" + classSpec.Kind

		// Remove finalizers from old provider specific machineClass
		if finalizers := sets.NewString(clone.Finalizers...); finalizers.Has(MCMFinalizerName) {
			finalizers.Delete(MCMFinalizerName)
			clone.Finalizers = finalizers.List()
		}

		_, err = c.controlMachineClient.AlicloudMachineClasses(c.namespace).Update(clone)
		if err != nil {
			return err
		}

	case AWSMachineClassKind:
		providerSpecificMachineClass, err := c.controlMachineClient.AWSMachineClasses(c.namespace).Get(classSpec.Name, metav1.GetOptions{
			TypeMeta:        metav1.TypeMeta{},
			ResourceVersion: "",
		})
		if err != nil {
			return err
		}

		clone := providerSpecificMachineClass.DeepCopy()
		if clone.Annotations == nil {
			clone.Annotations = make(map[string]string)
		}
		clone.Annotations[machineutils.MigratedMachineClass] = MachineController + "-" + classSpec.Kind

		// Remove finalizers from old provider specific machineClass
		if finalizers := sets.NewString(clone.Finalizers...); finalizers.Has(MCMFinalizerName) {
			finalizers.Delete(MCMFinalizerName)
			clone.Finalizers = finalizers.List()
		}

		_, err = c.controlMachineClient.AWSMachineClasses(c.namespace).Update(clone)
		if err != nil {
			return err
		}

	case AzureMachineClassKind:
		providerSpecificMachineClass, err := c.controlMachineClient.AzureMachineClasses(c.namespace).Get(classSpec.Name, metav1.GetOptions{
			TypeMeta:        metav1.TypeMeta{},
			ResourceVersion: "",
		})
		if err != nil {
			return err
		}

		clone := providerSpecificMachineClass.DeepCopy()
		if clone.Annotations == nil {
			clone.Annotations = make(map[string]string)
		}
		clone.Annotations[machineutils.MigratedMachineClass] = MachineController + "-" + classSpec.Kind

		// Remove finalizers from old provider specific machineClass
		if finalizers := sets.NewString(clone.Finalizers...); finalizers.Has(MCMFinalizerName) {
			finalizers.Delete(MCMFinalizerName)
			clone.Finalizers = finalizers.List()
		}

		_, err = c.controlMachineClient.AzureMachineClasses(c.namespace).Update(clone)
		if err != nil {
			return err
		}

	case GCPMachineClassKind:
		providerSpecificMachineClass, err := c.controlMachineClient.GCPMachineClasses(c.namespace).Get(classSpec.Name, metav1.GetOptions{
			TypeMeta:        metav1.TypeMeta{},
			ResourceVersion: "",
		})
		if err != nil {
			return err
		}

		clone := providerSpecificMachineClass.DeepCopy()
		if clone.Annotations == nil {
			clone.Annotations = make(map[string]string)
		}
		clone.Annotations[machineutils.MigratedMachineClass] = MachineController + "-" + classSpec.Kind

		// Remove finalizers from old provider specific machineClass
		if finalizers := sets.NewString(clone.Finalizers...); finalizers.Has(MCMFinalizerName) {
			finalizers.Delete(MCMFinalizerName)
			clone.Finalizers = finalizers.List()
		}

		_, err = c.controlMachineClient.GCPMachineClasses(c.namespace).Update(clone)
		if err != nil {
			return err
		}

	case OpenStackMachineClassKind:
		providerSpecificMachineClass, err := c.controlMachineClient.OpenStackMachineClasses(c.namespace).Get(classSpec.Name, metav1.GetOptions{
			TypeMeta:        metav1.TypeMeta{},
			ResourceVersion: "",
		})
		if err != nil {
			return err
		}

		clone := providerSpecificMachineClass.DeepCopy()
		if clone.Annotations == nil {
			clone.Annotations = make(map[string]string)
		}
		clone.Annotations[machineutils.MigratedMachineClass] = MachineController + "-" + classSpec.Kind

		// Remove finalizers from old provider specific machineClass
		if finalizers := sets.NewString(clone.Finalizers...); finalizers.Has(MCMFinalizerName) {
			finalizers.Delete(MCMFinalizerName)
			clone.Finalizers = finalizers.List()
		}

		_, err = c.controlMachineClient.OpenStackMachineClasses(c.namespace).Update(clone)
		if err != nil {
			return err
		}

	case PacketMachineClassKind:
		providerSpecificMachineClass, err := c.controlMachineClient.PacketMachineClasses(c.namespace).Get(classSpec.Name, metav1.GetOptions{
			TypeMeta:        metav1.TypeMeta{},
			ResourceVersion: "",
		})
		if err != nil {
			return err
		}

		clone := providerSpecificMachineClass.DeepCopy()
		if clone.Annotations == nil {
			clone.Annotations = make(map[string]string)
		}
		clone.Annotations[machineutils.MigratedMachineClass] = MachineController + "-" + classSpec.Kind

		// Remove finalizers from old provider specific machineClass
		if finalizers := sets.NewString(clone.Finalizers...); finalizers.Has(MCMFinalizerName) {
			finalizers.Delete(MCMFinalizerName)
			clone.Finalizers = finalizers.List()
		}

		_, err = c.controlMachineClient.PacketMachineClasses(c.namespace).Update(clone)
		if err != nil {
			return err
		}
	}

	klog.V(1).Infof("Set migrated annotation for ProviderSpecificMachineClass %s/%s", classSpec.Kind, classSpec.Name)

	return nil
}

// TryMachineClassMigration tries to migrate the provider-specific machine class to the generic machine-class.
func (c *controller) TryMachineClassMigration(classSpec *v1alpha1.ClassSpec) (*v1alpha1.MachineClass, *v1.Secret, machineutils.Retry, error) {
	var (
		err                          error
		providerSpecificMachineClass interface{}
	)

	// Get the provider specific (e.g. AWSMachineClass) from the classSpec
	if providerSpecificMachineClass, err = c.getProviderSpecificMachineClass(classSpec); err != nil {
		return nil, nil, machineutils.RetryOp, err
	}

	// Create/Apply the new MachineClass CR by copying/migrating over all the fields.
	if retry, err := c.createMachineClass(providerSpecificMachineClass, classSpec); err != nil {
		return nil, nil, retry, err
	}

	// Update any references to the old {Provider}MachineClass CR.
	if err = c.updateClassReferences(classSpec); err != nil {
		return nil, nil, machineutils.RetryOp, err
	}

	// Annotate the old {Provider}MachineClass CR with an migrated annotation.
	if err = c.addMigratedAnnotationForProviderMachineClass(classSpec); err != nil {
		return nil, nil, machineutils.RetryOp, err
	}

	klog.V(1).Infof("Migration successful for class %s/%s", classSpec.Kind, classSpec.Name)

	return nil, nil, machineutils.RetryOp, nil
}
