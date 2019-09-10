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

package fakeclient

import (
	"fmt"
	"math"
	"sync"
	"time"

	"errors"

	fakeuntyped "github.com/gardener/machine-controller-manager/pkg/client/clientset/versioned/fake"
	apipolicyv1beta1 "k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	serializer "k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/discovery"
	fakediscovery "k8s.io/client-go/discovery/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	policyv1beta1 "k8s.io/client-go/kubernetes/typed/policy/v1beta1"
	fakepolicyv1beta1 "k8s.io/client-go/kubernetes/typed/policy/v1beta1/fake"
	k8stesting "k8s.io/client-go/testing"
)

// FakeObjectTracker implements both k8stesting.ObjectTracker as well as watch.Interface.
type FakeObjectTracker struct {
	*watch.FakeWatcher
	delegatee    k8stesting.ObjectTracker
	watchers     []*watcher
	trackerMutex sync.Mutex
	fakingOptions
}

// Add recieves an add event with the object
func (t *FakeObjectTracker) Add(obj runtime.Object) error {
	if t.fakingEnabled {
		err := t.RunFakeInvocations()
		if err != nil {
			return err
		}
	}

	return t.delegatee.Add(obj)
}

// Get recieves an get event with the object
func (t *FakeObjectTracker) Get(gvr schema.GroupVersionResource, ns, name string) (runtime.Object, error) {
	if t.fakingEnabled {
		defer t.DecrementCounter()

		err := t.RunFakeInvocations()
		if err != nil {
			return nil, err
		}

		if gvr.Resource == "machines" {
			if t.fakingOptions.failAt.Machine.Get != "" {
				return nil, errors.New(t.fakingOptions.failAt.Machine.Get)
			}
		}

	}

	return t.delegatee.Get(gvr, ns, name)
}

// Create recieves an create event with the object
func (t *FakeObjectTracker) Create(gvr schema.GroupVersionResource, obj runtime.Object, ns string) error {
	if t.fakingEnabled {
		defer t.DecrementCounter()

		err := t.RunFakeInvocations()
		if err != nil {
			return err
		}
	}

	err := t.delegatee.Create(gvr, obj, ns)
	if err != nil {
		return err
	}

	if t.FakeWatcher == nil {
		return errors.New("Error sending event on a tracker with no watch support")
	}

	if t.IsStopped() {
		return errors.New("Error sending event on a stopped tracker")
	}

	t.FakeWatcher.Add(obj)
	return nil
}

// Update recieves an update event with the object
func (t *FakeObjectTracker) Update(gvr schema.GroupVersionResource, obj runtime.Object, ns string) error {

	if t.fakingEnabled {
		defer t.DecrementCounter()

		err := t.RunFakeInvocations()
		if err != nil {
			return err
		}

		if gvr.Resource == "nodes" {
			if t.fakingOptions.failAt.Node.Update != "" {
				return errors.New(t.fakingOptions.failAt.Node.Update)
			}
		}
	}

	err := t.delegatee.Update(gvr, obj, ns)
	if err != nil {
		return err
	}

	if t.FakeWatcher == nil {
		return errors.New("Error sending event on a tracker with no watch support")
	}

	if t.IsStopped() {
		return errors.New("Error sending event on a stopped tracker")
	}

	t.FakeWatcher.Modify(obj)
	return nil
}

// List recieves an list event with the object
func (t *FakeObjectTracker) List(gvr schema.GroupVersionResource, gvk schema.GroupVersionKind, ns string) (runtime.Object, error) {
	if t.fakingEnabled {
		defer t.DecrementCounter()

		err := t.RunFakeInvocations()
		if err != nil {
			return nil, err
		}
	}
	return t.delegatee.List(gvr, gvk, ns)
}

// Delete recieves an delete event with the object
func (t *FakeObjectTracker) Delete(gvr schema.GroupVersionResource, ns, name string) error {
	if t.fakingEnabled {
		defer t.DecrementCounter()

		err := t.RunFakeInvocations()
		if err != nil {
			return err
		}
	}

	obj, errGet := t.delegatee.Get(gvr, ns, name)
	err := t.delegatee.Delete(gvr, ns, name)
	if err != nil {
		return err
	}

	if errGet != nil {
		return errGet
	}

	if t.FakeWatcher == nil {
		return errors.New("Error sending event on a tracker with no watch support")
	}

	if t.IsStopped() {
		return errors.New("Error sending event on a stopped tracker")
	}

	t.FakeWatcher.Delete(obj)
	return nil
}

// Watch recieves an watch event with the object
func (t *FakeObjectTracker) Watch(gvr schema.GroupVersionResource, name string) (watch.Interface, error) {
	if t.fakingEnabled {
		defer t.DecrementCounter()

		err := t.RunFakeInvocations()
		if err != nil {
			return nil, err
		}
	}

	return t.delegatee.Watch(gvr, name)
}

func (t *FakeObjectTracker) watchReactionfunc(action k8stesting.Action) (bool, watch.Interface, error) {
	if t.FakeWatcher == nil {
		return false, nil, errors.New("Cannot watch on a tracker with no watch support")
	}

	switch a := action.(type) {
	case k8stesting.WatchAction:
		w := &watcher{
			FakeWatcher: watch.NewFake(),
			action:      a,
		}
		go w.dispatchInitialObjects(a, t)
		t.trackerMutex.Lock()
		defer t.trackerMutex.Unlock()
		t.watchers = append(t.watchers, w)
		return true, w, nil
	default:
		return false, nil, fmt.Errorf("Expected WatchAction but got %v", action)
	}
}

// Start begins tracking of an FakeObjectTracker
func (t *FakeObjectTracker) Start() error {
	if t.FakeWatcher == nil {
		return errors.New("Tracker has no watch support")
	}

	for event := range t.ResultChan() {
		t.dispatch(&event)
	}

	return nil
}

func (t *FakeObjectTracker) dispatch(event *watch.Event) {
	for _, w := range t.watchers {
		go w.dispatch(event)
	}
}

// Stop terminates tracking of an FakeObjectTracker
func (t *FakeObjectTracker) Stop() {
	if t.FakeWatcher == nil {
		panic(errors.New("Tracker has no watch support"))
	}

	t.trackerMutex.Lock()
	defer t.trackerMutex.Unlock()

	t.FakeWatcher.Stop()
	for _, w := range t.watchers {
		w.Stop()
	}
	t.watchers = []*watcher{}
}

type watcher struct {
	*watch.FakeWatcher
	action      k8stesting.WatchAction
	updateMutex sync.Mutex
}

func (w *watcher) Stop() {
	w.updateMutex.Lock()
	defer w.updateMutex.Unlock()

	w.FakeWatcher.Stop()
}

func (w *watcher) handles(event *watch.Event) bool {
	if w.IsStopped() {
		return false
	}

	t, err := meta.TypeAccessor(event.Object)
	if err != nil {
		return false
	}

	gvr, _ := meta.UnsafeGuessKindToResource(schema.FromAPIVersionAndKind(t.GetAPIVersion(), t.GetKind()))
	if !(&k8stesting.SimpleWatchReactor{Resource: gvr.Resource}).Handles(w.action) {
		return false
	}

	o, err := meta.Accessor(event.Object)
	if err != nil {
		return false
	}

	info := w.action.GetWatchRestrictions()
	rv, fs, ls := info.ResourceVersion, info.Fields, info.Labels
	if rv != "" && o.GetResourceVersion() != rv {
		return false
	}

	if fs != nil && !fs.Matches(fields.Set{
		"metadata.name":      o.GetName(),
		"metadata.namespace": o.GetNamespace(),
	}) {
		return false
	}

	if ls != nil && !ls.Matches(labels.Set(o.GetLabels())) {
		return false
	}

	return true
}

func (w *watcher) dispatch(event *watch.Event) {
	w.updateMutex.Lock()
	defer w.updateMutex.Unlock()

	if !w.handles(event) {
		return
	}
	w.Action(event.Type, event.Object)
}

func (w *watcher) dispatchInitialObjects(action k8stesting.WatchAction, t k8stesting.ObjectTracker) error {
	listObj, err := t.List(action.GetResource(), action.GetResource().GroupVersion().WithKind(action.GetResource().Resource), action.GetNamespace())
	if err != nil {
		return err
	}

	itemsPtr, err := meta.GetItemsPtr(listObj)
	if err != nil {
		return err
	}

	items := itemsPtr.([]runtime.Object)
	for _, o := range items {
		w.dispatch(&watch.Event{
			Type:   watch.Added,
			Object: o,
		})
	}

	return nil
}

// ResourceActions contains of Kubernetes/Machine resources whose response can be faked
type ResourceActions struct {
	Node    Actions
	Machine Actions
}

// Actions contains the actions whose response can be faked
type Actions struct {
	Create string
	Get    string
	Delete string
	Update string
}

// fakingOptions are options that can be set while trying to fake object tracker returns
type fakingOptions struct {
	// To check if faking is enabled
	fakingEnabled bool
	// Number of times faking is to occur
	counter int
	// Error message to be displayed
	errorMessage string
	// Delay in providing response
	delay time.Duration
	// Fail at different resource action
	failAt *ResourceActions
}

// SetDelay sets delay while invoking any interface exposed by standard ObjectTrackers
func (o *fakingOptions) SetDelay(delay time.Duration) error {
	o.fakingEnabled = true
	o.delay = delay
	o.counter = math.MaxInt32
	return nil
}

// SetError sets up the errorMessage to be returned on further function calls
func (o *fakingOptions) SetError(message string) error {
	o.fakingEnabled = true
	o.errorMessage = message
	o.counter = math.MaxInt32
	return nil
}

// SetFakeResourceActions sets up the errorMessage to be returned on specific calls
func (o *fakingOptions) SetFakeResourceActions(resourceActions *ResourceActions, counter int) error {
	o.fakingEnabled = true
	o.failAt = resourceActions
	o.counter = counter
	return nil
}

// ClearOptions clears any faking options that have been sets
func (o *fakingOptions) ClearOptions() error {
	o.fakingEnabled = false
	o.errorMessage = ""
	o.delay = 0
	o.failAt = nil
	o.counter = 0
	return nil
}

func (o *fakingOptions) DecrementCounter() error {
	o.counter--
	if o.counter == 0 {
		o.ClearOptions()
	}
	return nil
}

// RunFakeInvocations runs any custom fake configurations/methods before invoking standard ObjectTrackers
func (o *fakingOptions) RunFakeInvocations() error {
	// Delay while returning call
	if o.delay != 0 {
		time.Sleep(o.delay)
	}

	// If error message has been set
	if o.errorMessage != "" {
		return errors.New(o.errorMessage)
	}

	return nil
}

// NewMachineClientSet returns a clientset that will respond with the provided objects.
// It's backed by a very simple object tracker that processes creates, updates and deletions as-is,
// without applying any validations and/or defaults. It shouldn't be considered a replacement
// for a real clientset and is mostly useful in simple unit tests.
func NewMachineClientSet(objects ...runtime.Object) (*fakeuntyped.Clientset, *FakeObjectTracker) {
	var scheme = runtime.NewScheme()
	var codecs = serializer.NewCodecFactory(scheme)

	metav1.AddToGroupVersion(scheme, schema.GroupVersion{Version: "v1"})
	fakeuntyped.AddToScheme(scheme)

	o := &FakeObjectTracker{
		FakeWatcher: watch.NewFake(),
		delegatee:   k8stesting.NewObjectTracker(scheme, codecs.UniversalDecoder()),
	}

	for _, obj := range objects {
		if err := o.Add(obj); err != nil {
			panic(err)
		}
	}

	cs := &fakeuntyped.Clientset{}
	cs.Fake.AddReactor("*", "*", k8stesting.ObjectReaction(o))
	cs.Fake.AddWatchReactor("*", o.watchReactionfunc)

	return cs, o
}

// FakeObjectTrackers is a struct containing all the controller fake object trackers
type FakeObjectTrackers struct {
	ControlMachine, ControlCore, TargetCore *FakeObjectTracker
}

// NewFakeObjectTrackers initialize's fakeObjectTrackers initializes the fake object trackers
func NewFakeObjectTrackers(controlMachine, controlCore, targetCore *FakeObjectTracker) *FakeObjectTrackers {

	fakeObjectTrackers := &FakeObjectTrackers{
		ControlMachine: controlMachine,
		ControlCore:    controlCore,
		TargetCore:     targetCore,
	}

	return fakeObjectTrackers
}

// Start starts all object trackers as go routines
func (o *FakeObjectTrackers) Start() error {
	go o.ControlMachine.Start()
	go o.ControlCore.Start()
	go o.TargetCore.Start()

	return nil
}

// Stop stops all object trackers
func (o *FakeObjectTrackers) Stop() error {
	o.ControlMachine.Stop()
	o.ControlCore.Stop()
	o.TargetCore.Stop()

	return nil
}

// NewCoreClientSet returns a clientset that will respond with the provided objects.
// It's backed by a very simple object tracker that processes creates, updates and deletions as-is,
// without applying any validations and/or defaults. It shouldn't be considered a replacement
// for a real clientset and is mostly useful in simple unit tests.
func NewCoreClientSet(objects ...runtime.Object) (*Clientset, *FakeObjectTracker) {

	var scheme = runtime.NewScheme()
	var codecs = serializer.NewCodecFactory(scheme)

	metav1.AddToGroupVersion(scheme, schema.GroupVersion{Version: "v1"})
	k8sfake.AddToScheme(scheme)

	o := &FakeObjectTracker{
		FakeWatcher: watch.NewFake(),
		delegatee:   k8stesting.NewObjectTracker(scheme, codecs.UniversalDecoder()),
	}

	for _, obj := range objects {
		if err := o.Add(obj); err != nil {
			panic(err)
		}
	}

	cs := &Clientset{Clientset: &k8sfake.Clientset{}}
	cs.FakeDiscovery = &fakediscovery.FakeDiscovery{Fake: &cs.Fake}
	cs.Fake.AddReactor("*", "*", k8stesting.ObjectReaction(o))
	cs.Fake.AddWatchReactor("*", o.watchReactionfunc)

	return cs, o
}

// Clientset extends k8sfake.Clientset to override the Policy implementation.
// This is because the default Policy fake implementation does not propagate the
// eviction name.
type Clientset struct {
	*k8sfake.Clientset
	FakeDiscovery *fakediscovery.FakeDiscovery
}

// Discovery returns the fake discovery implementation.
func (c *Clientset) Discovery() discovery.DiscoveryInterface {
	return c.FakeDiscovery
}

// PolicyV1beta1 retrieves the PolicyV1beta1Client
func (c *Clientset) PolicyV1beta1() policyv1beta1.PolicyV1beta1Interface {
	return &FakePolicyV1beta1{
		FakePolicyV1beta1: &fakepolicyv1beta1.FakePolicyV1beta1{
			Fake: &c.Fake,
		},
	}
}

// Policy retrieves the PolicyV1beta1Client
func (c *Clientset) Policy() policyv1beta1.PolicyV1beta1Interface {
	return c.PolicyV1beta1()
}

// FakePolicyV1beta1 extends fakepolicyv1beta1.FakePolicyV1beta1 to override the
// Policy implementation. This is because the default Policy fake implementation
// does not propagate the eviction name.
type FakePolicyV1beta1 struct {
	*fakepolicyv1beta1.FakePolicyV1beta1
}

// Evictions extends fakepolicyv1beta1.FakeEvictions to override the
// Policy implementation. This is because the default Policy fake implementation
// does not propagate the eviction name.
func (c *FakePolicyV1beta1) Evictions(namespace string) policyv1beta1.EvictionInterface {
	return &FakeEvictions{
		FakePolicyV1beta1: c.FakePolicyV1beta1,
		ns:                namespace,
	}
}

// FakeEvictions extends fakepolicyv1beta1.FakeEvictions to override the
// Policy implementation. This is because the default Policy fake implementation
// does not propagate the eviction name.
type FakeEvictions struct {
	*fakepolicyv1beta1.FakePolicyV1beta1
	ns string
}

// Evict overrides the fakepolicyv1beta1.FakeEvictions to override the
// Policy implementation. This is because the default Policy fake implementation
// does not propagate the eviction name.
func (c *FakeEvictions) Evict(eviction *apipolicyv1beta1.Eviction) error {
	action := k8stesting.GetActionImpl{}
	action.Name = eviction.Name
	action.Verb = "post"
	action.Namespace = c.ns
	action.Resource = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	action.Subresource = "eviction"
	_, err := c.Fake.Invokes(action, eviction)
	return err
}
