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
	corelisters "k8s.io/client-go/listers/core/v1"
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
	Out                io.Writer
	ErrOut             io.Writer
	Driver             driver.Driver
	pvcLister          corelisters.PersistentVolumeClaimLister
	pvLister           corelisters.PersistentVolumeLister
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

	// PodEvictionRetryInterval is the interval in which to retry eviction for pods
	PodEvictionRetryInterval = time.Second * 5
	// GetPvDetailsRetryInterval is the interval in which to retry getting PV details
	GetPvDetailsRetryInterval = time.Second * 5
	// GetPvDetailsMaxRetries is the number of max retries to get PV details
	GetPvDetailsMaxRetries = 3
	// VolumeDetachPollInterval is the interval in which to recheck if the volume is detached from the node
	VolumeDetachPollInterval = time.Second * 5

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
	gracePeriodSeconds int,
	force bool,
	ignoreDaemonsets bool,
	deleteLocalData bool,
	out io.Writer,
	errOut io.Writer,
	driver driver.Driver,
	pvcLister corelisters.PersistentVolumeClaimLister,
	pvLister corelisters.PersistentVolumeLister,
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
		Out:                out,
		ErrOut:             errOut,
		Driver:             driver,
		pvcLister:          pvcLister,
		pvLister:           pvLister,
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
	// Closing stop channel makes sure that all go routines started later are stopped.
	stopCh := make(chan struct{})
	defer close(stopCh)

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
		err := o.evictPods(pods, policyGroupVersion, getPodFn, stopCh)
		if err != nil {
			glog.Warningf("Pod eviction was timed out, Error: %v. \nHowever, drain will continue to forcefully delete the pods by setting graceful termination period to 0s", err)
		} else {
			return nil
		}
	}
	return o.deletePods(pods, getPodFn, stopCh)
}

func volIsPvc(vol *corev1.Volume) bool {
	return vol.PersistentVolumeClaim != nil
}

func filterPodsWithPv(pods []api.Pod) ([]*api.Pod, []*api.Pod) {
	podsWithPv, podsWithoutPv := []*api.Pod{}, []*api.Pod{}

	for i := range pods {
		hasPv := false
		pod := &pods[i]
		vols := pod.Spec.Volumes
		for k := range vols {
			vol := &vols[k]
			hasPv = volIsPvc(vol)
			if hasPv {
				podsWithPv = append(podsWithPv, pod)
				// No need to process rest of the volumes
				break
			}
		}
		if !hasPv {
			podsWithoutPv = append(podsWithoutPv, pod)
		}
	}
	return podsWithPv, podsWithoutPv
}

func (o *DrainOptions) evictPods(pods []api.Pod, policyGroupVersion string, getPodFn func(namespace, name string) (*api.Pod, error), stopCh <-chan struct{}) error {
	returnCh := make(chan error, len(pods))

	podsWithPv, podsWithoutPv := filterPodsWithPv(pods)

	glog.V(3).Info("Evicting pods on the node: ", o.nodeName)

	go o.evictPodsWithPv(podsWithPv, policyGroupVersion, getPodFn, returnCh, stopCh)
	go o.evictPodsWithoutPv(podsWithoutPv, policyGroupVersion, getPodFn, returnCh, stopCh)

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

func (o *DrainOptions) evictPodsWithoutPv(pods []*corev1.Pod,
	policyGroupVersion string,
	getPodFn func(namespace, name string) (*api.Pod, error),
	returnCh chan error,
	stopCh <-chan struct{},
) {
	for _, pod := range pods {
		go o.evictPodInternal(pod, policyGroupVersion, getPodFn, returnCh, stopCh)
	}
	return
}

func sortPodsByPriority(pods []*corev1.Pod) {
	sort.Slice(pods, func(i, j int) bool {
		return *pods[i].Spec.Priority > *pods[j].Spec.Priority
	})
}

// doAccountingOfPvs returns the map with keys as pod names and values as array of attached volumes' IDs
func (o *DrainOptions) doAccountingOfPvs(pods []*corev1.Pod) map[string][]string {
	volMap := make(map[string][]string)
	pvMap := make(map[string][]string)

	for _, pod := range pods {
		podPVs, _ := o.getPvs(pod)
		pvMap[pod.Namespace+"/"+pod.Name] = podPVs
	}
	glog.V(4).Info("PV map: ", pvMap)

	filterSharedPVs(pvMap)

	for i := range pvMap {
		pvList := pvMap[i]
		vols, err := o.getVolIDsFromDriver(pvList)
		if err != nil {
			// In case of error, log and skip this set of volumes
			glog.Errorf("Error getting volume ID from cloud provider. Skipping volumes for pod: %v. Err: %v", i, err)
			continue
		}
		volMap[i] = vols
	}
	glog.V(4).Info("Volume map: ", volMap)
	return volMap
}

// filterSharedPVs filters out the PVs that are shared among pods.
func filterSharedPVs(pvMap map[string][]string) {
	sharedVol := make(map[string]bool)
	sharedVolumesFound := false

	// Create hash map of volumes:
	// Key: volume name
	// Value: 'true' if any other pod shares this volume, else 'false'
	for _, vols := range pvMap {
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

	// Recreate the values of pvMap. Append volume if it is not shared
	for pod, vols := range pvMap {
		volList := []string{}
		for _, vol := range vols {
			if sharedVol[vol] == false {
				volList = append(volList, vol)
			}
		}
		pvMap[pod] = volList
	}
	glog.V(4).Info("Removed shared volumes. Filtered list of pods with volumes: ", pvMap)
}

func (o *DrainOptions) evictPodsWithPv(pods []*corev1.Pod,
	policyGroupVersion string,
	getPodFn func(namespace, name string) (*api.Pod, error),
	returnCh chan error,
	stopCh <-chan struct{},
) {
	remainingPods := 0

	sortPodsByPriority(pods)

	volMap := o.doAccountingOfPvs(pods)

EvictingPods:
	for {
		retryPods := []*corev1.Pod{}

		for i, pod := range pods {
			select {
			case <-stopCh:
				glog.Warningf("The caller function returned. No need to try evictions now")
				return
			default:
			}

			err := o.evictPod(*pod, policyGroupVersion)

			if apierrors.IsTooManyRequests(err) {
				// Pod eviction failed because of PDB violation, we will retry one we are done with this list.
				glog.V(3).Info("Pod ", pod.Namespace, "/", pod.Name, " couldn't be evicted because of PDB violation. Will be retried.")
				retryPods = append(retryPods, pod)
				continue
			} else if apierrors.IsNotFound(err) {
				glog.V(3).Info("\t", pod.Name, " is already gone")
				returnCh <- nil
				continue
			} else if err != nil {
				returnCh <- fmt.Errorf("Error when evicting pod: %v/%v. Will not be retried. Err: %v", pod.Namespace, pod.Name, err)
				continue
			}

			glog.V(3).Info("waiting for PVs to detach from node for pod: ", pod.Name)
			// Eviction was successful. Wait for pvs for this pod to detach
			pvs := volMap[pod.Namespace+"/"+pod.Name]
			err = o.waitForDetach(pvs, o.nodeName, stopCh)

			if apierrors.IsNotFound(err) {
				glog.V(3).Info("Node not found anymore")
				returnCh <- nil
				remainingPods = len(pods) - (i + 1) + len(retryPods)
				// The node itself is gone. Stop evicting pods.
				// We should anyway return successful eviction for the remaining pods
				break EvictingPods
			} else if err != nil {
				glog.Errorf("Error when waiting for volume to detach from node. Err: %v", err)
				returnCh <- err
				continue
			}
			glog.V(3).Info("Detached pv for pod: ", pod.Name)
			returnCh <- nil
		}

		if len(retryPods) == 0 {
			// There are no pods to retry
			break
		}

		glog.V(4).Info("Eviction for some pods will be retried after ", PodEvictionRetryInterval)
		pods = retryPods
		time.Sleep(PodEvictionRetryInterval)
	}

	for j := 0; j < remainingPods; j++ {
		glog.V(4).Info("Returning success for remaining pods")
		// This is executed when node is not found anymore.
		// Return success to caller for all non-processed pods so that the caller function can move on.
		returnCh <- nil
	}

	return
}

func (o *DrainOptions) getPvs(pod *corev1.Pod) ([]string, error) {
	pvs := []string{}
	for i := range pod.Spec.Volumes {
		vol := &pod.Spec.Volumes[i]

		if vol.PersistentVolumeClaim != nil {
			try := 0

			for {
				pvc, err := o.pvcLister.PersistentVolumeClaims(pod.Namespace).Get(vol.PersistentVolumeClaim.ClaimName)

				if apierrors.IsNotFound(err) {
					// If this PVC is not found, move on to the next PVC
					break
				} else if err != nil {
					try++

					if try == GetPvDetailsMaxRetries {
						// Log warning, and skip trying this volume anymore
						glog.Errorf("Error getting PVC. Err: %v", err)
						break
					}
					// In case of error, try again after few seconds
					time.Sleep(GetPvDetailsRetryInterval)
					continue
				}

				// Found PVC; append and exit
				pvs = append(pvs, pvc.Spec.VolumeName)
				break
			}
		}
	}
	return pvs, nil
}

func (o *DrainOptions) waitForDetach(volumeIDs []string, nodeName string, stopCh <-chan struct{}) error {
	if volumeIDs == nil || len(volumeIDs) == 0 || nodeName == "" {
		// If volume or node name is not available, nothing to do. Just log this as warning
		glog.Warningf("Node name: %v, list of pod PVs to wait for detach: %v", nodeName, volumeIDs)
		return nil
	}

	glog.V(4).Info("waiting for following volumes to detach: ", volumeIDs)

	timeout := time.After(o.PvDetachTimeout)
	found := true

	for found {
		select {
		case <-stopCh:
			glog.Warningf("The caller function returned, and is not waiting for PV to detach now")
			return fmt.Errorf("The caller function returned, and is not waiting for PV to detach now")

		case <-timeout:
			glog.Warningf("Timeout while waiting for PVs to detach from node")
			return fmt.Errorf("Timeout while waiting for PVs to detach from node")

		default:
		}

		found = false

		node, err := o.client.Core().Nodes().Get(nodeName, metav1.GetOptions{})

		if apierrors.IsNotFound(err) {
			glog.V(4).Info("Node not found: ", nodeName)
			return err
		} else if err != nil {
			glog.Errorf("Error getting details for node: %v. Err: %v", nodeName, err)
			return err
		}

		glog.V(4).Info("Attached volumes: ", node.Status.VolumesAttached)
		attachedVols := node.Status.VolumesAttached
		if len(attachedVols) == 0 {
			glog.V(4).Info("No volumes attached to the node")
			return nil
		}

	LookUpVolume:
		for i := range volumeIDs {
			volumeID := &volumeIDs[i]

			for j := range attachedVols {
				attachedVol := &attachedVols[j]

				found, _ = regexp.MatchString(*volumeID, string(attachedVol.Name))

				if found {
					glog.V(4).Info("Found volume: ", *volumeID, " still attached to the node. Will recheck in ", VolumeDetachPollInterval)
					time.Sleep(VolumeDetachPollInterval)
					break LookUpVolume
				}
			}
		}
	}

	glog.V(4).Info("Detached volumes: ", volumeIDs)
	return nil
}

func (o *DrainOptions) getVolIDsFromDriver(pvNames []string) ([]string, error) {
	pvSpecs := []corev1.PersistentVolumeSpec{}

	for _, pvName := range pvNames {
		try := 0

		for {
			pv, err := o.pvLister.Get(pvName)

			if apierrors.IsNotFound(err) {
				break
			} else if err != nil {
				try++
				if try == GetPvDetailsMaxRetries {
					break
				}
				// In case of error, try again after few seconds
				time.Sleep(GetPvDetailsRetryInterval)
				continue
			}

			// Found PV; append and exit
			pvSpecs = append(pvSpecs, pv.Spec)
			break
		}
	}

	return o.Driver.GetVolNames(pvSpecs)
}

func (o *DrainOptions) evictPodInternal(pod *corev1.Pod, policyGroupVersion string, getPodFn func(namespace, name string) (*api.Pod, error), returnCh chan error, stopCh <-chan struct{}) {
	var err error
	glog.V(3).Info("Evicting ", pod.Name)

	for {
		select {
		case <-stopCh:
			glog.Warningf("The caller function returned. No need to try evicting pods now")
			return
		default:
		}
		err = o.evictPod(*pod, policyGroupVersion)

		if err == nil {
			break
		} else if apierrors.IsNotFound(err) {
			glog.V(3).Info("\t", pod.Name, " evicted")
			returnCh <- nil
			return
		} else if apierrors.IsTooManyRequests(err) {
			// Pod couldn't be evicted because of PDB violation
			time.Sleep(PodEvictionRetryInterval)
		} else {
			returnCh <- fmt.Errorf("error when evicting pod %q: %v", pod.Name, err)
			return
		}
	}

	podArray := []api.Pod{*pod}

	_, err = o.waitForDelete(podArray, Interval, time.Duration(math.MaxInt64), true, getPodFn, stopCh)
	if err == nil {
		returnCh <- nil
	} else {
		returnCh <- fmt.Errorf("error when waiting for pod %q terminating: %v", pod.Name, err)
	}
}

func (o *DrainOptions) deletePods(pods []api.Pod, getPodFn func(namespace, name string) (*api.Pod, error), stopCh <-chan struct{}) error {
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
	_, err := o.waitForDelete(pods, Interval, globalTimeout, false, getPodFn, stopCh)
	return err
}

func (o *DrainOptions) waitForDelete(pods []api.Pod, interval, timeout time.Duration, usingEviction bool, getPodFn func(string, string) (*api.Pod, error), stopCh <-chan struct{}) ([]api.Pod, error) {
	err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		pendingPods := []api.Pod{}
		for i, pod := range pods {
			select {
			case <-stopCh:
				return false, fmt.Errorf("The caller function returned. No need to wait for pods to delete now")
			default:
			}

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
