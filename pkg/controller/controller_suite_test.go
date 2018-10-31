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

	"testing"

	machine_internal "github.com/gardener/machine-controller-manager/pkg/apis/machine"
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	faketyped "github.com/gardener/machine-controller-manager/pkg/client/clientset/versioned/typed/machine/v1alpha1/fake"
	machineinformers "github.com/gardener/machine-controller-manager/pkg/client/informers/externalversions"
	customfake "github.com/gardener/machine-controller-manager/pkg/fakeclient"
	"github.com/gardener/machine-controller-manager/pkg/options"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	coreinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

func TestMachineControllerManagerSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Machine Controller Manager Suite")
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
			TypeMeta: metav1.TypeMeta{
				APIVersion: "machine.sapcloud.io",
				Kind:       "MachineSet",
			},
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

		if owner != nil {
			ms.OwnerReferences = append(ms.OwnerReferences, *owner.DeepCopy())
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
			TypeMeta: metav1.TypeMeta{
				APIVersion: "machine.sapcloud.io",
				Kind:       "Machine",
			},
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

func createController(stop <-chan struct{}, namespace string, controlMachineObjects, controlCoreObjects, targetCoreObjects []runtime.Object) (*controller, *customfake.FakeObjectTrackers) {

	fakeControlMachineClient, controlMachineObjectTracker := customfake.NewMachineClientSet(controlMachineObjects...)
	fakeTypedMachineClient := &faketyped.FakeMachineV1alpha1{
		Fake: &fakeControlMachineClient.Fake,
	}
	fakeControlCoreClient, controlCoreObjectTracker := customfake.NewCoreClientSet(controlCoreObjects...)
	fakeTargetCoreClient, targetCoreObjectTracker := customfake.NewCoreClientSet(targetCoreObjects...)
	fakeObjectTrackers := customfake.NewFakeObjectTrackers(
		controlMachineObjectTracker,
		controlCoreObjectTracker,
		targetCoreObjectTracker,
	)
	fakeObjectTrackers.Start()

	coreTargetInformerFactory := coreinformers.NewFilteredSharedInformerFactory(
		fakeTargetCoreClient,
		100*time.Millisecond,
		namespace,
		nil,
	)
	defer coreTargetInformerFactory.Start(stop)
	coreTargetSharedInformers := coreTargetInformerFactory.Core().V1()
	nodes := coreTargetSharedInformers.Nodes()

	controlMachineInformerFactory := machineinformers.NewFilteredSharedInformerFactory(
		fakeControlMachineClient,
		100*time.Millisecond,
		namespace,
		nil,
	)
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

	safetyOptions := options.SafetyOptions{
		SafetyUp:                                 2,
		SafetyDown:                               1,
		MachineHealthTimeout:                     metav1.Duration{Duration: 10 * time.Minute},
		MachineSafetyOrphanVMsPeriod:             metav1.Duration{Duration: 30 * time.Minute},
		MachineSafetyOvershootingPeriod:          metav1.Duration{Duration: 1 * time.Minute},
		MachineSafetyAPIServerStatusCheckPeriod:  metav1.Duration{Duration: 1 * time.Minute},
		MachineSafetyAPIServerStatusCheckTimeout: metav1.Duration{Duration: 30 * time.Second},
	}

	return &controller{
		namespace:                      namespace,
		safetyOptions:                  safetyOptions,
		awsMachineClassLister:          aws.Lister(),
		awsMachineClassSynced:          aws.Informer().HasSynced,
		azureMachineClassLister:        azure.Lister(),
		gcpMachineClassLister:          gcp.Lister(),
		openStackMachineClassLister:    openstack.Lister(),
		targetCoreClient:               fakeTargetCoreClient,
		controlCoreClient:              fakeControlCoreClient,
		controlMachineClient:           fakeTypedMachineClient,
		internalExternalScheme:         internalExternalScheme,
		nodeLister:                     nodes.Lister(),
		machineLister:                  machines.Lister(),
		machineSetLister:               machineSets.Lister(),
		machineDeploymentLister:        machineDeployments.Lister(),
		machineSynced:                  machines.Informer().HasSynced,
		machineSetSynced:               machineSets.Informer().HasSynced,
		machineDeploymentSynced:        machineDeployments.Informer().HasSynced,
		secretQueue:                    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "secret"),
		nodeQueue:                      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "node"),
		openStackMachineClassQueue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "openstackmachineclass"),
		awsMachineClassQueue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "awsmachineclass"),
		azureMachineClassQueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "azuremachineclass"),
		gcpMachineClassQueue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "gcpmachineclass"),
		machineQueue:                   workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "machine"),
		machineSetQueue:                workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "machineset"),
		machineDeploymentQueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "machinedeployment"),
		machineSafetyOrphanVMsQueue:    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "machinesafetyorphanvms"),
		machineSafetyOvershootingQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "machinesafetyovershooting"),
		machineSafetyAPIServerQueue:    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "machinesafetyapiserver"),
	}, fakeObjectTrackers
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

var _ = Describe("#createController", func() {
	objMeta := &metav1.ObjectMeta{
		GenerateName: "machine",
		Namespace:    "test",
	}

	It("success", func() {
		machine0 := newMachine(&v1alpha1.MachineTemplateSpec{
			ObjectMeta: *objMeta,
		}, nil, nil)

		stop := make(chan struct{})
		defer close(stop)

		c, trackers := createController(stop, objMeta.Namespace, nil, nil, nil)
		defer trackers.Stop()

		waitForCacheSync(stop, c)

		Expect(c).NotTo(BeNil())

		allMachineWatch, err := c.controlMachineClient.Machines(objMeta.Namespace).Watch(metav1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())
		defer allMachineWatch.Stop()

		machine0Watch, err := c.controlMachineClient.Machines(objMeta.Namespace).Watch(metav1.ListOptions{
			FieldSelector: fmt.Sprintf("metadata.name=%s", machine0.Name),
		})
		Expect(err).NotTo(HaveOccurred())
		defer machine0Watch.Stop()

		go func() {
			_, err := c.controlMachineClient.Machines(objMeta.Namespace).Create(machine0)
			if err != nil {
				fmt.Printf("Error creating machine: %s", err)
			}
		}()

		var event watch.Event
		Eventually(allMachineWatch.ResultChan()).Should(Receive(&event))
		Expect(event.Type).To(Equal(watch.Added))
		Expect(event.Object).To(Equal(machine0))

		Eventually(machine0Watch.ResultChan()).Should(Receive(&event))
		Expect(event.Type).To(Equal(watch.Added))
		Expect(event.Object).To(Equal(machine0))
	})
})
