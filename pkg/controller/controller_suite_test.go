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

package controller

import (
	"fmt"
	"time"

	machine_internal "github.com/gardener/machine-controller-manager/pkg/apis/machine"
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	fakeuntyped "github.com/gardener/machine-controller-manager/pkg/client/clientset/versioned/fake"
	faketyped "github.com/gardener/machine-controller-manager/pkg/client/clientset/versioned/typed/machine/v1alpha1/fake"
	machineinformers "github.com/gardener/machine-controller-manager/pkg/client/informers/externalversions"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"

	"testing"
)

func TestMachineControllerManagerSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Machine Controller Manager suite")
}

var (
	controllerKindMachine = v1alpha1.SchemeGroupVersion.WithKind("Machine")
)

func newMachineSetFromMachineDeployment(machineSetCount int, machineDeployment *v1alpha1.MachineDeployment, replicas int32, statusTemplate *v1alpha1.MachineSetStatus) *v1alpha1.MachineSet {
	return newMachineSetsFromMachineDeployment(1, machineDeployment, replicas, statusTemplate)[0]
}

func newMachineSetsFromMachineDeployment(machineSetCount int, machineDeployment *v1alpha1.MachineDeployment, replicas int32, statusTemplate *v1alpha1.MachineSetStatus) []*v1alpha1.MachineSet {
	t := &machineDeployment.TypeMeta
	return newMachineSets(machineSetCount, &machineDeployment.Spec.Template, replicas, machineDeployment.Spec.MinReadySeconds, statusTemplate, &metav1.OwnerReference{
		APIVersion:         t.APIVersion,
		Kind:               t.Kind,
		Name:               machineDeployment.Name,
		UID:                machineDeployment.UID,
		BlockOwnerDeletion: boolPtr(true),
		Controller:         boolPtr(true),
	})
}

func newMachineSet(specTemplate *v1alpha1.MachineTemplateSpec, replicas, minReadySeconds int32, statusTemplate *v1alpha1.MachineSetStatus, owner *metav1.OwnerReference) *v1alpha1.MachineSet {
	return newMachineSets(1, specTemplate, replicas, minReadySeconds, statusTemplate, owner)[0]
}

func newMachineSets(machineSetCount int, specTemplate *v1alpha1.MachineTemplateSpec, replicas, minReadySeconds int32, statusTemplate *v1alpha1.MachineSetStatus, owner *metav1.OwnerReference) []*v1alpha1.MachineSet {
	machineSets := make([]*v1alpha1.MachineSet, replicas)
	for i := range machineSets {
		ms := &v1alpha1.MachineSet{
			ObjectMeta: *newObjectMeta(&specTemplate.ObjectMeta, i),
			Spec: v1alpha1.MachineSetSpec{
				MachineClass:    *specTemplate.Spec.Class.DeepCopy(),
				MinReadySeconds: minReadySeconds,
				Replicas:        replicas,
				Selector: &metav1.LabelSelector{
					MatchLabels: deepCopy(specTemplate.ObjectMeta.Labels),
				},
				Template: *specTemplate.DeepCopy(),
			},
		}
		if statusTemplate != nil {
			ms.Status = *statusTemplate.DeepCopy()
		}
		machineSets[i] = ms
	}
	return machineSets
}

func deepCopy(m map[string]string) map[string]string {
	r := make(map[string]string, len(m))
	for k := range m {
		r[k] = m[k]
	}
	return r
}

func newMachineFromMachineSet(machineSet *v1alpha1.MachineSet, statusTemplate *v1alpha1.MachineStatus) *v1alpha1.Machine {
	return newMachinesFromMachineSet(1, machineSet, statusTemplate)[0]
}

func newMachinesFromMachineSet(machineCount int, machineSet *v1alpha1.MachineSet, statusTemplate *v1alpha1.MachineStatus) []*v1alpha1.Machine {
	t := machineSet.TypeMeta
	return newMachines(machineCount, &machineSet.Spec.Template, statusTemplate, &metav1.OwnerReference{
		APIVersion:         t.APIVersion,
		Kind:               t.Kind,
		Name:               machineSet.Name,
		UID:                machineSet.UID,
		BlockOwnerDeletion: boolPtr(true),
		Controller:         boolPtr(true),
	})
}

func newMachine(specTemplate *v1alpha1.MachineTemplateSpec, statusTemplate *v1alpha1.MachineStatus, owner *metav1.OwnerReference) *v1alpha1.Machine {
	return newMachines(1, specTemplate, statusTemplate, owner)[0]
}

func newMachines(machineCount int, specTemplate *v1alpha1.MachineTemplateSpec, statusTemplate *v1alpha1.MachineStatus, owner *metav1.OwnerReference) []*v1alpha1.Machine {
	machines := make([]*v1alpha1.Machine, machineCount)
	for i := range machines {
		m := &v1alpha1.Machine{
			ObjectMeta: *newObjectMeta(&specTemplate.ObjectMeta, i),
			Spec:       *newMachineSpec(&specTemplate.Spec, i),
		}

		if statusTemplate != nil {
			m.Status = *newMachineStatus(statusTemplate, i)
		}

		if owner != nil {
			m.OwnerReferences = append(m.OwnerReferences, *owner.DeepCopy())
		}

		machines[i] = m
	}
	return machines
}

func boolPtr(b bool) *bool {
	return &b
}

func newObjectMeta(meta *metav1.ObjectMeta, index int) *metav1.ObjectMeta {
	r := meta.DeepCopy()

	if r.Name != "" {
		return r
	}

	if r.GenerateName != "" {
		r.Name = fmt.Sprintf("%s-%d", r.GenerateName, index)
		return r
	}

	r.Name = fmt.Sprintf("machine-%d", index)
	return r
}

func newMachineSpec(specTemplate *v1alpha1.MachineSpec, index int) *v1alpha1.MachineSpec {
	r := specTemplate.DeepCopy()
	if r.ProviderID == "" {
		return r
	}

	r.ProviderID = fmt.Sprintf("%s-%d", r.ProviderID, index)
	return r
}

func newMachineStatus(statusTemplate *v1alpha1.MachineStatus, index int) *v1alpha1.MachineStatus {
	if statusTemplate == nil {
		return &v1alpha1.MachineStatus{}
	}

	r := statusTemplate.DeepCopy()
	if r.Node == "" {
		return r
	}

	r.Node = fmt.Sprintf("%s-%d", r.Node, index)
	return r
}

func newSecretReference(meta *metav1.ObjectMeta, index int) *corev1.SecretReference {
	r := &corev1.SecretReference{
		Namespace: meta.Namespace,
	}

	if meta.Name != "" {
		r.Name = meta.Name
		return r
	}

	if meta.GenerateName != "" {
		r.Name = fmt.Sprintf("%s-%d", meta.GenerateName, index)
		return r
	}

	r.Name = fmt.Sprintf("machine-%d", index)
	return r
}

func createController(stop <-chan struct{}, namespace string, machineObjects, coreObjects []runtime.Object) *controller {
	fakeCoreClient := k8sfake.NewSimpleClientset(coreObjects...)

	fakeMachineClient := fakeuntyped.NewSimpleClientset(machineObjects...)
	fakeTypedMachineClient := &faketyped.FakeMachineV1alpha1{
		Fake: &fakeMachineClient.Fake,
	}
	//TODO controlCoreClient
	controlMachineInformerFactory := machineinformers.NewSharedInformerFactory(fakeMachineClient, 100*time.Millisecond)
	defer controlMachineInformerFactory.Start(stop)

	machineSharedInformers := controlMachineInformerFactory.Machine().V1alpha1()
	aws := machineSharedInformers.AWSMachineClasses()
	azure := machineSharedInformers.AzureMachineClasses()
	gcp := machineSharedInformers.GCPMachineClasses()
	openstack := machineSharedInformers.OpenStackMachineClasses()
	machines := machineSharedInformers.Machines()
	machineSets := machineSharedInformers.MachineSets()
	machineDeployments := machineSharedInformers.MachineDeployments()

	internalExternalScheme := runtime.NewScheme()
	Expect(machine_internal.AddToScheme(internalExternalScheme)).To(Succeed())
	Expect(v1alpha1.AddToScheme(internalExternalScheme)).To(Succeed())

	return &controller{
		namespace:                   namespace,
		awsMachineClassLister:       aws.Lister(),
		awsMachineClassSynced:       aws.Informer().HasSynced,
		azureMachineClassLister:     azure.Lister(),
		gcpMachineClassLister:       gcp.Lister(),
		openStackMachineClassLister: openstack.Lister(),
		controlCoreClient:           fakeCoreClient,
		controlMachineClient:        fakeTypedMachineClient,
		internalExternalScheme:      internalExternalScheme,
		machineLister:               machines.Lister(),
		machineSetLister:            machineSets.Lister(),
		machineDeploymentLister:     machineDeployments.Lister(),
		machineSynced:               machines.Informer().HasSynced,
		machineSetSynced:            machineSets.Informer().HasSynced,
		machineDeploymentSynced:     machineDeployments.Informer().HasSynced,
	}
}

func waitForCacheSync(stop <-chan struct{}, controller *controller) {
	Expect(cache.WaitForCacheSync(
		stop,
		controller.awsMachineClassSynced,
		controller.machineSynced,
		controller.machineSetSynced,
		controller.machineDeploymentSynced,
	)).To(BeTrue())
}
