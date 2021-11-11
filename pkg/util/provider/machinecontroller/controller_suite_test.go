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
	"context"
	"fmt"
	"testing"
	"time"

	machine_internal "github.com/gardener/machine-controller-manager/pkg/apis/machine"
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	faketyped "github.com/gardener/machine-controller-manager/pkg/client/clientset/versioned/typed/machine/v1alpha1/fake"
	machineinformers "github.com/gardener/machine-controller-manager/pkg/client/informers/externalversions"
	customfake "github.com/gardener/machine-controller-manager/pkg/fakeclient"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/cache"

	"github.com/gardener/machine-controller-manager/pkg/util/provider/drain"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/driver"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/options"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	coreinformers "k8s.io/client-go/informers"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

func TestMachineControllerSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Machine Controller Suite")
}

var (
	MachineClass     = "MachineClass"
	TestMachineClass = "machineClass-0"
)

func newMachine(
	specTemplate *v1alpha1.MachineTemplateSpec,
	statusTemplate *v1alpha1.MachineStatus,
	owner *metav1.OwnerReference,
	annotations map[string]string,
	labels map[string]string,
	addFinalizer bool,
	creationTimestamp metav1.Time,
) *v1alpha1.Machine {
	return newMachines(1, specTemplate, statusTemplate, owner, annotations, labels, addFinalizer, creationTimestamp)[0]
}

func newMachines(
	machineCount int,
	specTemplate *v1alpha1.MachineTemplateSpec,
	statusTemplate *v1alpha1.MachineStatus,
	owner *metav1.OwnerReference,
	annotations map[string]string,
	labels map[string]string,
	addFinalizer bool,
	creationTimestamp metav1.Time,
) []*v1alpha1.Machine {
	machines := make([]*v1alpha1.Machine, machineCount)

	if annotations == nil {
		annotations = make(map[string]string)
	}
	if labels == nil {
		labels = make(map[string]string)
	}

	for i := range machines {
		m := &v1alpha1.Machine{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "machine.sapcloud.io",
				Kind:       "Machine",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:              fmt.Sprintf("machine-%d", i),
				Namespace:         testNamespace,
				Labels:            labels,
				Annotations:       annotations,
				CreationTimestamp: creationTimestamp,
				DeletionTimestamp: &creationTimestamp, //TODO: Add parametrize this
			},
			Spec: *newMachineSpec(&specTemplate.Spec, i),
		}
		finalizers := sets.NewString(m.Finalizers...)

		if addFinalizer {
			finalizers.Insert(MCMFinalizerName)
		}
		m.Finalizers = finalizers.List()

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

func createController(
	stop <-chan struct{},
	namespace string,
	controlMachineObjects, controlCoreObjects, targetCoreObjects []runtime.Object,
	fakedriver driver.Driver,
) (*controller, *customfake.FakeObjectTrackers) {

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
	pvcs := coreTargetSharedInformers.PersistentVolumeClaims()
	pvs := coreTargetSharedInformers.PersistentVolumes()

	coreControlInformerFactory := coreinformers.NewFilteredSharedInformerFactory(
		fakeControlCoreClient,
		100*time.Millisecond,
		namespace,
		nil,
	)
	defer coreControlInformerFactory.Start(stop)

	coreControlSharedInformers := coreControlInformerFactory.Core().V1()
	secrets := coreControlSharedInformers.Secrets()

	controlMachineInformerFactory := machineinformers.NewFilteredSharedInformerFactory(
		fakeControlMachineClient,
		100*time.Millisecond,
		namespace,
		nil,
	)
	defer controlMachineInformerFactory.Start(stop)

	machineSharedInformers := controlMachineInformerFactory.Machine().V1alpha1()
	machineClass := machineSharedInformers.MachineClasses()
	machines := machineSharedInformers.Machines()

	internalExternalScheme := runtime.NewScheme()
	Expect(machine_internal.AddToScheme(internalExternalScheme)).To(Succeed())
	Expect(v1alpha1.AddToScheme(internalExternalScheme)).To(Succeed())

	safetyOptions := options.SafetyOptions{
		MachineCreationTimeout:                   metav1.Duration{Duration: 20 * time.Minute},
		MachineHealthTimeout:                     metav1.Duration{Duration: 10 * time.Minute},
		MachineDrainTimeout:                      metav1.Duration{Duration: 5 * time.Minute},
		MachineSafetyOrphanVMsPeriod:             metav1.Duration{Duration: 30 * time.Minute},
		MachineSafetyAPIServerStatusCheckPeriod:  metav1.Duration{Duration: 1 * time.Minute},
		MachineSafetyAPIServerStatusCheckTimeout: metav1.Duration{Duration: 30 * time.Second},
		MaxEvictRetries:                          drain.DefaultMaxEvictRetries,
	}

	controller := &controller{
		namespace:                   namespace,
		driver:                      fakedriver,
		safetyOptions:               safetyOptions,
		machineClassLister:          machineClass.Lister(),
		machineClassSynced:          machineClass.Informer().HasSynced,
		targetCoreClient:            fakeTargetCoreClient,
		controlCoreClient:           fakeControlCoreClient,
		controlMachineClient:        fakeTypedMachineClient,
		internalExternalScheme:      internalExternalScheme,
		nodeLister:                  nodes.Lister(),
		pvcLister:                   pvcs.Lister(),
		secretLister:                secrets.Lister(),
		pvLister:                    pvs.Lister(),
		machineLister:               machines.Lister(),
		machineSynced:               machines.Informer().HasSynced,
		nodeSynced:                  nodes.Informer().HasSynced,
		secretSynced:                secrets.Informer().HasSynced,
		machineClassQueue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "machineclass"),
		secretQueue:                 workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "secret"),
		nodeQueue:                   workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "node"),
		machineQueue:                workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "machine"),
		machineSafetyOrphanVMsQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "machinesafetyorphanvms"),
		machineSafetyAPIServerQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "machinesafetyapiserver"),
		recorder:                    record.NewBroadcaster().NewRecorder(nil, corev1.EventSource{Component: ""}),
	}

	// controller.internalExternalScheme = runtime.NewScheme()

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: v1core.New(fakeControlCoreClient.CoreV1().RESTClient()).Events(namespace)})

	return controller, fakeObjectTrackers
}

func waitForCacheSync(stop <-chan struct{}, controller *controller) {
	Expect(cache.WaitForCacheSync(
		stop,
		controller.machineClassSynced,
		controller.machineSynced,
		controller.secretSynced,
		controller.nodeSynced,
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
		}, nil, nil, nil, nil, false, metav1.Now())

		stop := make(chan struct{})
		defer close(stop)

		c, trackers := createController(stop, objMeta.Namespace, nil, nil, nil, nil)
		defer trackers.Stop()

		waitForCacheSync(stop, c)

		Expect(c).NotTo(BeNil())

		allMachineWatch, err := c.controlMachineClient.Machines(objMeta.Namespace).Watch(context.TODO(), metav1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())
		defer allMachineWatch.Stop()

		machine0Watch, err := c.controlMachineClient.Machines(objMeta.Namespace).Watch(context.TODO(), metav1.ListOptions{
			FieldSelector: fmt.Sprintf("metadata.name=%s", machine0.Name),
		})
		Expect(err).NotTo(HaveOccurred())
		defer machine0Watch.Stop()

		go func() {
			_, err := c.controlMachineClient.Machines(objMeta.Namespace).Create(context.TODO(), machine0, metav1.CreateOptions{})
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
