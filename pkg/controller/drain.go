/*
Copyright 2015 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

This file was copied and modified from the kubernetes/kubernetes project
https://github.com/kubernetes/kubernetes/blob/release-1.8/pkg/kubectl/cmd/drain.go

Modifications Copyright (c) 2017 SAP SE or an SAP affiliate company. All rights reserved.
*/

// Package controller is used to provide the core functionalities of machine-controller-manager
package controller

import (
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gardener/machine-controller-manager/pkg/driver"
	"github.com/golang/glog"
	api "k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	policy "k8s.io/api/policy/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

// DrainOptions are configurable options while draining a node before deletion
type DrainOptions struct {
	client             kubernetes.Interface
	Force              bool
	GracePeriodSeconds int
	IgnoreDaemonsets   bool
	Timeout            time.Duration
	PvDetachTimeout    time.Duration
	DeleteLocalData    bool
	nodeName           string
	machineName        string
	Out                io.Writer
	ErrOut             io.Writer
	Driver             driver.Driver
}

// Takes a pod and returns a bool indicating whether or not to operate on the
// pod, an optional warning message, and an optional fatal error.
type podFilter func(api.Pod) (include bool, w *warning, f *fatal)
type warning struct {
	string
}
type fatal struct {
	string
}

const (
	// EvictionKind is the kind used for eviction
	EvictionKind = "Eviction"
	// EvictionSubresource is the kind used for evicting pods
	EvictionSubresource = "pods/eviction"

	// Interval is the default Poll interval
	Interval = time.Second * 5

	daemonsetFatal      = "DaemonSet-managed pods (use --ignore-daemonsets to ignore)"
	daemonsetWarning    = "Ignoring DaemonSet-managed pods"
	localStorageFatal   = "pods with local storage (use --delete-local-data to override)"
	localStorageWarning = "Deleting pods with local storage"
	unmanagedFatal      = "pods not managed by ReplicationController, ReplicaSet, Job, DaemonSet or StatefulSet (use --force to override)"
	unmanagedWarning    = "Deleting pods not managed by ReplicationController, ReplicaSet, Job, DaemonSet or StatefulSet"
)

// NewDrainOptions creates a new DrainOptions struct and returns a pointer to it
func NewDrainOptions(
	client kubernetes.Interface,
	timeout time.Duration,
	pvDetachTimeout time.Duration,
	nodeName string,
	machineName string,
	gracePeriodSeconds int,
	force bool,
	ignoreDaemonsets bool,
	deleteLocalData bool,
	out io.Writer,
	errOut io.Writer,
	driver driver.Driver,
) *DrainOptions {

	return &DrainOptions{
		client:             client,
		Force:              force,
		GracePeriodSeconds: gracePeriodSeconds,
		IgnoreDaemonsets:   ignoreDaemonsets,
		Timeout:            timeout,
		PvDetachTimeout:    pvDetachTimeout,
		DeleteLocalData:    deleteLocalData,
		nodeName:           nodeName,
		machineName:        machineName,
		Out:                out,
		ErrOut:             errOut,
		Driver:             driver,
	}

}

// RunDrain runs the 'drain' command
func (o *DrainOptions) RunDrain() error {
	if err := o.RunCordonOrUncordon(true); err != nil {
		return err
	}

	err := o.deleteOrEvictPodsSimple()
	return err
}

func (o *DrainOptions) deleteOrEvictPodsSimple() error {
	pods, err := o.getPodsForDeletion()
	if err != nil {
		return err
	}

	err = o.deleteOrEvictPods(pods)
	if err != nil {
		pendingPods, newErr := o.getPodsForDeletion()
		if newErr != nil {
			return newErr
		}
		fmt.Fprintf(o.ErrOut, "There are pending pods when an error occurred: %v\n", err)
		for _, pendingPod := range pendingPods {
			fmt.Fprintf(o.ErrOut, "%s/%s\n", "pod", pendingPod.Name)
		}
	}
	return err
}

func (o *DrainOptions) getPodController(pod api.Pod) *metav1.OwnerReference {
	return metav1.GetControllerOf(&pod)
}

func (o *DrainOptions) unreplicatedFilter(pod api.Pod) (bool, *warning, *fatal) {
	// any finished pod can be removed
	if pod.Status.Phase == api.PodSucceeded || pod.Status.Phase == api.PodFailed {
		return true, nil, nil
	}

	controllerRef := o.getPodController(pod)
	if controllerRef != nil {
		return true, nil, nil
	}
	if !o.Force {
		return false, nil, &fatal{unmanagedFatal}
	}
	return true, &warning{unmanagedWarning}, nil
}

func (o *DrainOptions) daemonsetFilter(pod api.Pod) (bool, *warning, *fatal) {
	// Note that we return false in cases where the pod is DaemonSet managed,
	// regardless of flags.  We never delete them, the only question is whether
	// their presence constitutes an error.
	//
	// The exception is for pods that are orphaned (the referencing
	// management resource - including DaemonSet - is not found).
	// Such pods will be deleted if --force is used.
	controllerRef := o.getPodController(pod)
	if controllerRef == nil || controllerRef.Kind != "DaemonSet" {
		return true, nil, nil
	}
	if _, err := o.client.Extensions().DaemonSets(pod.Namespace).Get(controllerRef.Name, metav1.GetOptions{}); err != nil {
		return false, nil, &fatal{err.Error()}
	}
	if !o.IgnoreDaemonsets {
		return false, nil, &fatal{daemonsetFatal}
	}
	return false, &warning{daemonsetWarning}, nil
}

func mirrorPodFilter(pod api.Pod) (bool, *warning, *fatal) {
	if _, found := pod.ObjectMeta.Annotations[corev1.MirrorPodAnnotationKey]; found {
		return false, nil, nil
	}
	return true, nil, nil
}

func hasLocalStorage(pod api.Pod) bool {
	for _, volume := range pod.Spec.Volumes {
		if volume.EmptyDir != nil {
			return true
		}
	}

	return false
}

func (o *DrainOptions) localStorageFilter(pod api.Pod) (bool, *warning, *fatal) {
	if !hasLocalStorage(pod) {
		return true, nil, nil
	}
	if !o.DeleteLocalData {
		return false, nil, &fatal{localStorageFatal}
	}
	return true, &warning{localStorageWarning}, nil
}

// Map of status message to a list of pod names having that status.
type podStatuses map[string][]string

func (ps podStatuses) Message() string {
	msgs := []string{}

	for key, pods := range ps {
		msgs = append(msgs, fmt.Sprintf("%s: %s", key, strings.Join(pods, ", ")))
	}
	return strings.Join(msgs, "; ")
}

// getPodsForDeletion returns all the pods we're going to delete.  If there are
// any pods preventing us from deleting, we return that list in an error.
func (o *DrainOptions) getPodsForDeletion() (pods []api.Pod, err error) {
	podList, err := o.client.Core().Pods(metav1.NamespaceAll).List(metav1.ListOptions{
		FieldSelector: fields.SelectorFromSet(fields.Set{"spec.nodeName": o.nodeName}).String()})
	if err != nil {
		return pods, err
	}

	ws := podStatuses{}
	fs := podStatuses{}

	for _, pod := range podList.Items {
		podOk := true
		for _, filt := range []podFilter{mirrorPodFilter, o.localStorageFilter, o.unreplicatedFilter, o.daemonsetFilter} {
			filterOk, w, f := filt(pod)

			podOk = podOk && filterOk
			if w != nil {
				ws[w.string] = append(ws[w.string], pod.Name)
			}
			if f != nil {
				fs[f.string] = append(fs[f.string], pod.Name)
			}
		}
		if podOk {
			pods = append(pods, pod)
		}
	}

	if len(fs) > 0 {
		return []api.Pod{}, errors.New(fs.Message())
	}
	if len(ws) > 0 {
		fmt.Fprintf(o.ErrOut, "WARNING: %s\n", ws.Message())
	}
	return pods, nil
}

func (o *DrainOptions) deletePod(pod api.Pod) error {
	deleteOptions := &metav1.DeleteOptions{}
	gracePeriodSeconds := int64(0)
	deleteOptions.GracePeriodSeconds = &gracePeriodSeconds

	return o.client.Core().Pods(pod.Namespace).Delete(pod.Name, deleteOptions)
}

func (o *DrainOptions) evictPod(pod api.Pod, policyGroupVersion string) error {
	deleteOptions := &metav1.DeleteOptions{}
	if o.GracePeriodSeconds >= 0 {
		gracePeriodSeconds := int64(o.GracePeriodSeconds)
		deleteOptions.GracePeriodSeconds = &gracePeriodSeconds
	}
	eviction := &policy.Eviction{
		TypeMeta: metav1.TypeMeta{
			APIVersion: policyGroupVersion,
			Kind:       EvictionKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod.Name,
			Namespace: pod.Namespace,
		},
		DeleteOptions: deleteOptions,
	}
	// Remember to change change the URL manipulation func when Evction's version change
	return o.client.Policy().Evictions(eviction.Namespace).Evict(eviction)
}

// deleteOrEvictPods deletes or evicts the pods on the api server
func (o *DrainOptions) deleteOrEvictPods(pods []api.Pod) error {
	if len(pods) == 0 {
		return nil
	}

	policyGroupVersion, err := SupportEviction(o.client)
	if err != nil {
		return err
	}

	getPodFn := func(namespace, name string) (*api.Pod, error) {
		return o.client.Core().Pods(namespace).Get(name, metav1.GetOptions{})
	}

	if len(policyGroupVersion) > 0 {
		err := o.evictPods(pods, policyGroupVersion, getPodFn)
		if err != nil {
			glog.Warningf("Pod eviction was timed out, Error: %v. \nHowever, drain will continue to forcefully delete the pods by setting graceful termination period to 0s", err)
		} else {
			return nil
		}
	}
	return o.deletePods(pods, getPodFn)
}

func volIsPvc(vol corev1.Volume) bool {
	if vol.PersistentVolumeClaim != nil {
		return true
	}
	return false
}

func filterPodsWithPv(pods []api.Pod) ([]api.Pod, []api.Pod) {
	podsWithPv, podsWithoutPv := []api.Pod{}, []api.Pod{}

	for i, pod := range pods {
		hasPv := false

		vols := pod.Spec.Volumes
		for _, vol := range vols {
			hasPv = volIsPvc(vol)
			if hasPv {
				podsWithPv = append(podsWithPv, pods[i])
				// No need to process rest of the volumes
				break
			}
		}
		if !hasPv {
			podsWithoutPv = append(podsWithoutPv, pods[i])
		}
	}
	return podsWithPv, podsWithoutPv
}

func (o *DrainOptions) evictPods(pods []api.Pod, policyGroupVersion string, getPodFn func(namespace, name string) (*api.Pod, error)) error {
	returnCh := make(chan error, 1)

	podsWithPv, podsWithoutPv := filterPodsWithPv(pods)

	glog.V(3).Info("Evicting pods on the node: ", o.nodeName)

	go o.evictPodsWithPv(podsWithPv, policyGroupVersion, getPodFn, returnCh)
	go o.evictPodsWithoutPv(podsWithoutPv, policyGroupVersion, getPodFn, returnCh)

	doneCount := 0
	var errors []error

	// 0 timeout means infinite, we use MaxInt64 to represent it.
	var globalTimeout time.Duration
	if o.Timeout == 0 {
		globalTimeout = time.Duration(math.MaxInt64)
	} else {
		globalTimeout = o.Timeout
	}
	globalTimeoutCh := time.After(globalTimeout)
	numPods := len(pods)
	for doneCount < numPods {
		select {
		case err := <-returnCh:
			doneCount++
			if err != nil {
				errors = append(errors, err)
			}
		case <-globalTimeoutCh:
			return fmt.Errorf("Drain did not complete within %v", globalTimeout)
		}
	}
	return utilerrors.NewAggregate(errors)
}

func (o *DrainOptions) evictPodsWithoutPv(pods []corev1.Pod, policyGroupVersion string, getPodFn func(namespace, name string) (*api.Pod, error), returnCh chan error) {
	for _, pod := range pods {
		go o.evictPodInternal(pod, policyGroupVersion, getPodFn, returnCh, false)
	}
	return
}

func sortPodsByPriority(pods []corev1.Pod) {
	sort.Slice(pods, func(i, j int) bool {
		return *pods[i].Spec.Priority > *pods[j].Spec.Priority
	})
}

// doAccountingOfPvs returns the map with keys as pod names and values as array of attached volumes
func (o *DrainOptions) doAccountingOfPvs(pods []corev1.Pod) map[string][]string {
	volMap := make(map[string][]string)
	for _, pod := range pods {
		podPVs := o.getPvs(pod)
		volMap[pod.Namespace+"/"+pod.Name] = podPVs
	}
	glog.V(4).Info("Volume map: ", volMap)
	return volMap
}

func filterSharedPVs(volMap map[string][]string) {
	sharedVol := make(map[string]bool)
	sharedVolumesFound := false

	// Create hash map of volumes:
	// Key: volume name
	// Value: 'true' if any other pod shares this volume, else 'false'
	for _, vols := range volMap {
		for _, vol := range vols {
			if _, ok := sharedVol[vol]; !ok {
				sharedVol[vol] = false
			} else {
				sharedVol[vol] = true
				sharedVolumesFound = true
			}
		}
	}

	if !sharedVolumesFound {
		glog.V(4).Info("No shared volumes found.")
		return
	}

	// Recreate the values of volMap. Append volume if it is not shared
	for pod, vols := range volMap {
		volList := []string{}
		for _, vol := range vols {
			if sharedVol[vol] == false {
				volList = append(volList, vol)
			}
		}
		volMap[pod] = volList
	}
	glog.V(4).Info("Removed shared volumes. Filtered list of pods with volumes: ", volMap)
}

func (o *DrainOptions) evictPodsWithPv(pods []corev1.Pod, policyGroupVersion string, getPodFn func(namespace, name string) (*api.Pod, error), returnCh chan error) {
	returnChInt := make(chan error, 1)
	detachCh := make(chan bool, 1)

	sortPodsByPriority(pods)

	volMap := o.doAccountingOfPvs(pods)
	filterSharedPVs(volMap)

	for {
		retryPods := []corev1.Pod{}
		for _, pod := range pods {
			pvs := volMap[pod.Namespace+"/"+pod.Name]
			go o.evictPodInternal(pod, policyGroupVersion, getPodFn, returnChInt, true)
			startTime := time.Now()
			select {
			case err := <-returnChInt:
				if apierrors.IsTooManyRequests(err) {
					glog.V(3).Info("Pod ", pod.Namespace, "/", pod.Name, " couldn't be evicted because of PDB violation. Will be retried.")
					// Pod eviction failed because of PDB violation, retry
					retryPods = append(retryPods, pod)
					continue
				} else if err != nil {
					returnCh <- fmt.Errorf("Error when evicting pod: %v/%v. Err: %v", pod.Namespace, pod.Name, err)
					continue
				}

				go o.waitForDetach(pvs, o.nodeName, detachCh)

				glog.V(3).Info("waiting pv to detach for pod: ", pod.Name)
				select {
				case <-detachCh:
					glog.V(3).Info("Detached pv for pod: ", pod.Name)
					returnCh <- nil
				case <-time.After(o.PvDetachTimeout - time.Now().Sub(startTime)):
					glog.Warningf("Timout occured when detaching pv for pod: %v", pod.Name)
					returnCh <- nil
				}
			case <-time.After(o.PvDetachTimeout):
				returnCh <- fmt.Errorf("Timout when evicting pod: %v", pod)
			}
		}
		if len(retryPods) == 0 {
			break
		}
		glog.V(4).Info("Eviction for some pods will be retried after 5 seconds.")
		pods = retryPods
		time.Sleep(5 * time.Second)
	}
	return
}

func (o *DrainOptions) getPvs(pod corev1.Pod) []string {
	pvs := []string{}
	for _, vol := range pod.Spec.Volumes {
		if vol.PersistentVolumeClaim != nil {
			pvc, err := o.client.CoreV1().PersistentVolumeClaims(pod.Namespace).Get(vol.PersistentVolumeClaim.ClaimName, metav1.GetOptions{})
			if err != nil {
				// If error is not nil, then skip this volume
				continue
			}
			pvs = append(pvs, pvc.Spec.VolumeName)
		}
	}
	return pvs
}

func (o *DrainOptions) waitForDetach(pvNames []string, nodeName string, detachCh chan bool) {
	if pvNames == nil || len(pvNames) == 0 || nodeName == "" {
		// If pv or node name is not available, nothing to do. Just log this as warning
		glog.Warningf("Node name: %v, list of PVs: %v", nodeName, pvNames)
		detachCh <- false
		return
	}
	volumeIDs, err := o.getVolIDsFromDriver(pvNames)
	if err != nil {
		glog.Errorf("Error when getting volume IDs from the driver. Err: %v", err)
		detachCh <- false
		return
	}
	glog.V(4).Info("Volume names from driver: ", volumeIDs)

	found := true
	for found {
		found = false
		node, err := o.client.Core().Nodes().Get(nodeName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			glog.V(4).Info("Node not found: ", nodeName)
			detachCh <- true
			return
		} else if err != nil {
			glog.Errorf("Error getting details for node: %v. Err: %v", nodeName, err)
			detachCh <- false
			return
		}

		glog.V(4).Info("Attached volumes: ", node.Status.VolumesAttached)
		attachedVols := node.Status.VolumesAttached
		if len(attachedVols) == 0 {
			glog.V(4).Info("No volumes attached to the node")
			break
		}

		for _, volumeID := range volumeIDs {
			for _, attachedVol := range attachedVols {
				found, _ = regexp.MatchString(volumeID, string(attachedVol.Name))
				if found {
					glog.V(4).Info("Found volume: ", volumeID, " attached to the node")
					break
				}
			}
			if found {
				glog.V(4).Info("Will recheck in 5 seconds")
				time.Sleep(5 * time.Second)
				break
			}
		}
	}
	glog.V(4).Info("Detached PVs: ", pvNames)
	detachCh <- true
}

func (o *DrainOptions) getVolIDsFromDriver(pvNames []string) ([]string, error) {
	pvSpecs := []corev1.PersistentVolumeSpec{}
	for _, pvName := range pvNames {
		pv, err := o.client.CoreV1().PersistentVolumes().Get(pvName, metav1.GetOptions{})
		if err != nil {
			glog.Warningf("Error getting PV: %v", pvName)
			continue
		}
		pvSpecs = append(pvSpecs, pv.Spec)
	}

	return o.Driver.GetVolNames(pvSpecs)
}

func (o *DrainOptions) evictPodInternal(pod corev1.Pod, policyGroupVersion string, getPodFn func(namespace, name string) (*api.Pod, error), returnCh chan error, podWithPv bool) {
	var err error
	glog.V(3).Info("Evicting ", pod.Name)
	for {
		err = o.evictPod(pod, policyGroupVersion)
		if err == nil {
			break
		} else if apierrors.IsNotFound(err) {
			glog.V(3).Info("\t", pod.Name, " evicted")
			returnCh <- nil
			return
		} else if apierrors.IsTooManyRequests(err) {
			// Pod couldn't be evicted because of PDB violation
			if podWithPv {
				// If it is a pod with PV, leave it to the caller function to handle this situation
				returnCh <- err
				return
			}
			// Else retry eviction after 5 seconds
			time.Sleep(5 * time.Second)
		} else {
			returnCh <- fmt.Errorf("error when evicting pod %q: %v", pod.Name, err)
			return
		}
	}
	podArray := []api.Pod{pod}
	_, err = o.waitForDelete(podArray, Interval, time.Duration(math.MaxInt64), true, getPodFn)
	if err == nil {
		returnCh <- nil
	} else {
		returnCh <- fmt.Errorf("error when waiting for pod %q terminating: %v", pod.Name, err)
	}
}

func (o *DrainOptions) deletePods(pods []api.Pod, getPodFn func(namespace, name string) (*api.Pod, error)) error {
	// 0 timeout means infinite, we use MaxInt64 to represent it.
	var globalTimeout time.Duration
	if o.Timeout == 0 {
		globalTimeout = time.Duration(math.MaxInt64)
	} else {
		globalTimeout = o.Timeout
	}
	for _, pod := range pods {
		err := o.deletePod(pod)
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	_, err := o.waitForDelete(pods, Interval, globalTimeout, false, getPodFn)
	return err
}

func (o *DrainOptions) waitForDelete(pods []api.Pod, interval, timeout time.Duration, usingEviction bool, getPodFn func(string, string) (*api.Pod, error)) ([]api.Pod, error) {
	err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		pendingPods := []api.Pod{}
		for i, pod := range pods {
			p, err := getPodFn(pod.Namespace, pod.Name)
			if apierrors.IsNotFound(err) || (p != nil && p.ObjectMeta.UID != pod.ObjectMeta.UID) {
				//cmdutil.PrintSuccess(o.mapper, false, o.Out, "pod", pod.Name, false, verbStr)
				//glog.Info("pod deleted successfully found")
				continue
			} else if err != nil {
				return false, err
			} else {
				pendingPods = append(pendingPods, pods[i])
			}
		}
		pods = pendingPods
		if len(pendingPods) > 0 {
			return false, nil
		}
		return true, nil
	})
	return pods, err
}

// SupportEviction uses Discovery API to find out if the server support eviction subresource
// If support, it will return its groupVersion; Otherwise, it will return ""
func SupportEviction(clientset kubernetes.Interface) (string, error) {
	discoveryClient := clientset.Discovery()
	groupList, err := discoveryClient.ServerGroups()
	if err != nil {
		return "", err
	}
	foundPolicyGroup := false
	var policyGroupVersion string
	for _, group := range groupList.Groups {
		if group.Name == "policy" {
			foundPolicyGroup = true
			policyGroupVersion = group.PreferredVersion.GroupVersion
			break
		}
	}
	if !foundPolicyGroup {
		return "", nil
	}
	resourceList, err := discoveryClient.ServerResourcesForGroupVersion("v1")
	if err != nil {
		return "", err
	}
	for _, resource := range resourceList.APIResources {
		if resource.Name == EvictionSubresource && resource.Kind == EvictionKind {
			return policyGroupVersion, nil
		}
	}
	return "", nil
}

// RunCordonOrUncordon runs either Cordon or Uncordon.  The desired value for
// "Unschedulable" is passed as the first arg.
func (o *DrainOptions) RunCordonOrUncordon(desired bool) error {
	node, err := o.client.CoreV1().Nodes().Get(o.nodeName, metav1.GetOptions{})
	if err != nil {
		// Deletion could be triggered when machine is just being created, no node present then
		return nil
	}
	unsched := node.Spec.Unschedulable
	if unsched == desired {
		glog.V(3).Info("Already desired")
	} else {
		clone := node.DeepCopy()
		clone.Spec.Unschedulable = desired

		_, err = o.client.CoreV1().Nodes().Update(clone)
		if err != nil {
			return err
		}
	}
	return nil
}
