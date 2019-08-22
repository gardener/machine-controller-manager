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
	"context"
	"errors"
	"fmt"
	"io"
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
	client                       kubernetes.Interface
	ForceDeletePods              bool
	IgnorePodsWithoutControllers bool
	GracePeriodSeconds           int
	IgnoreDaemonsets             bool
	Timeout                      time.Duration
	MaxEvictRetries              int32
	PvDetachTimeout              time.Duration
	DeleteLocalData              bool
	nodeName                     string
	Out                          io.Writer
	ErrOut                       io.Writer
	Driver                       driver.Driver
	pvcLister                    corelisters.PersistentVolumeClaimLister
	pvLister                     corelisters.PersistentVolumeLister
	drainStartedOn               time.Time
	drainEndedOn                 time.Time
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

	// DefaultMachineDrainTimeout is the default value for MachineDrainTimeout
	DefaultMachineDrainTimeout = 12 * time.Hour

	// DefaultMaxEvictRetries is the default value for MaxEvictRetries
	DefaultMaxEvictRetries = int32(3)

	// PodsWithoutPVDrainGracePeriod defines the grace period to wait for the pods without PV during machine drain.
	// This is in addition to the maximum terminationGracePeriod amount the pods.
	PodsWithoutPVDrainGracePeriod = 3 * time.Minute

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
	maxEvictRetries int32,
	pvDetachTimeout time.Duration,
	nodeName string,
	gracePeriodSeconds int,
	forceDeletePods bool,
	ignorePodsWithoutControllers bool,
	ignoreDaemonsets bool,
	deleteLocalData bool,
	out io.Writer,
	errOut io.Writer,
	driver driver.Driver,
	pvcLister corelisters.PersistentVolumeClaimLister,
	pvLister corelisters.PersistentVolumeLister,
) *DrainOptions {

	return &DrainOptions{
		client:                       client,
		ForceDeletePods:              forceDeletePods,
		IgnorePodsWithoutControllers: ignorePodsWithoutControllers,
		GracePeriodSeconds:           gracePeriodSeconds,
		IgnoreDaemonsets:             ignoreDaemonsets,
		MaxEvictRetries:              maxEvictRetries,
		Timeout:                      timeout,
		PvDetachTimeout:              pvDetachTimeout,
		DeleteLocalData:              deleteLocalData,
		nodeName:                     nodeName,
		Out:                          out,
		ErrOut:                       errOut,
		Driver:                       driver,
		pvcLister:                    pvcLister,
		pvLister:                     pvLister,
	}

}

// RunDrain runs the 'drain' command
func (o *DrainOptions) RunDrain() error {
	o.drainStartedOn = time.Now()
	glog.V(4).Infof(
		"Machine drain started on %s for %q",
		o.drainStartedOn,
		o.nodeName,
	)

	defer func() {
		o.drainEndedOn = time.Now()
		glog.Infof(
			"Machine drain ended on %s and took %s for %q",
			o.drainEndedOn,
			o.drainEndedOn.Sub(o.drainStartedOn),
			o.nodeName,
		)
	}()

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
			fmt.Fprintf(o.ErrOut, "%s/%s\n", pendingPod.Namespace, pendingPod.Name)
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
	if !o.IgnorePodsWithoutControllers {
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

func (o *DrainOptions) deletePod(pod *api.Pod) error {
	deleteOptions := &metav1.DeleteOptions{}
	gracePeriodSeconds := int64(0)
	deleteOptions.GracePeriodSeconds = &gracePeriodSeconds

	return o.client.Core().Pods(pod.Namespace).Delete(pod.Name, deleteOptions)
}

func (o *DrainOptions) evictPod(pod *api.Pod, policyGroupVersion string) error {
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

	return o.evictPods(len(policyGroupVersion) > 0, pods, policyGroupVersion, getPodFn)
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

func (o *DrainOptions) getTerminationGracePeriod(pod *api.Pod) time.Duration {
	if pod == nil || pod.Spec.TerminationGracePeriodSeconds == nil {
		return time.Duration(0)
	}

	return time.Duration(*pod.Spec.TerminationGracePeriodSeconds) * time.Second
}

func (o *DrainOptions) getGlobalTimeoutForPodsWithoutPV(pods []*api.Pod) time.Duration {
	var tgpsMax time.Duration
	for _, pod := range pods {
		tgps := o.getTerminationGracePeriod(pod)
		if tgps > tgpsMax {
			tgpsMax = tgps
		}
	}

	return tgpsMax + PodsWithoutPVDrainGracePeriod
}

func (o *DrainOptions) evictPods(attemptEvict bool, pods []api.Pod, policyGroupVersion string, getPodFn func(namespace, name string) (*api.Pod, error)) error {
	returnCh := make(chan error, len(pods))

	if o.ForceDeletePods {
		podsToDrain := make([]*api.Pod, len(pods))
		for i := range pods {
			podsToDrain[i] = &pods[i]
		}

		glog.V(3).Infof("Forceful eviction of pods on the node: %q", o.nodeName)

		// evict all pods in parallel without waiting for pods or volume detachment
		go o.evictPodsWithoutPv(attemptEvict, podsToDrain, policyGroupVersion, getPodFn, returnCh)
	} else {
		podsWithPv, podsWithoutPv := filterPodsWithPv(pods)

		glog.V(3).Infof("Normal eviction of pods on the node: %q", o.nodeName)

		// evcit all pods without PV in parallel and with PV in serial (waiting for vol detachment)
		go o.evictPodsWithPv(attemptEvict, podsWithPv, policyGroupVersion, getPodFn, returnCh)
		go o.evictPodsWithoutPv(attemptEvict, podsWithoutPv, policyGroupVersion, getPodFn, returnCh)
	}

	doneCount := 0
	var errors []error

	numPods := len(pods)
	for doneCount < numPods {
		err := <-returnCh
		doneCount++
		if err != nil {
			errors = append(errors, err)
		}
	}
	return utilerrors.NewAggregate(errors)
}

func (o *DrainOptions) evictPodsWithoutPv(attemptEvict bool, pods []*corev1.Pod,
	policyGroupVersion string,
	getPodFn func(namespace, name string) (*api.Pod, error),
	returnCh chan error,
) {
	for _, pod := range pods {
		go o.evictPodWithoutPVInternal(attemptEvict, pod, policyGroupVersion, getPodFn, returnCh)
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
	glog.V(3).Info("Removed shared volumes. Filtered list of pods with volumes: ", pvMap)
}

func (o *DrainOptions) evictPodsWithPv(attemptEvict bool, pods []*corev1.Pod,
	policyGroupVersion string,
	getPodFn func(namespace, name string) (*api.Pod, error),
	returnCh chan error,
) {
	sortPodsByPriority(pods)

	volMap := o.doAccountingOfPvs(pods)

	var (
		remainingPods []*api.Pod
		fastTrack     bool
		nretries      = int(o.MaxEvictRetries)
	)

	if attemptEvict {
		for i := 0; i < nretries; i++ {
			remainingPods, fastTrack = o.evictPodsWithPVInternal(attemptEvict, pods, volMap, policyGroupVersion, getPodFn, returnCh)
			if fastTrack || len(remainingPods) == 0 {
				//Either all pods got evicted or we need to fast track the return (node deletion detected)
				break
			}

			glog.V(4).Infof(
				"Eviction/deletion for some pods will be retried after %s for node %q",
				PodEvictionRetryInterval,
				o.nodeName,
			)
			pods = remainingPods
			time.Sleep(PodEvictionRetryInterval)
		}

		if !fastTrack && len(remainingPods) > 0 {
			// Force delete the pods remaining after evict retries.
			pods = remainingPods
			remainingPods, _ = o.evictPodsWithPVInternal(false, pods, volMap, policyGroupVersion, getPodFn, returnCh)
		}
	} else {
		remainingPods, _ = o.evictPodsWithPVInternal(false, pods, volMap, policyGroupVersion, getPodFn, returnCh)
	}

	// Placate the caller by returning the nil status for the remaining pods.
	for _, pod := range remainingPods {
		glog.V(4).Infof("Returning success for remaining pods for node %q", o.nodeName)
		if fastTrack {
			// This is executed when node is not found anymore.
			// Return success to caller for all non-processed pods so that the caller function can move on.
			returnCh <- nil
		} else if attemptEvict {
			returnCh <- fmt.Errorf("Error evicting pod %s/%s from node %q", pod.Namespace, pod.Name, pod.Spec.NodeName)
		} else {
			returnCh <- fmt.Errorf("Error deleting pod %s/%s from node %q", pod.Namespace, pod.Name, pod.Spec.NodeName)
		}
	}

	return
}

func (o *DrainOptions) evictPodsWithPVInternal(attemptEvict bool, pods []*corev1.Pod, volMap map[string][]string,
	policyGroupVersion string,
	getPodFn func(namespace, name string) (*api.Pod, error),
	returnCh chan error) (remainingPods []*api.Pod, fastTrack bool) {
	var (
		mainContext       context.Context
		cancelMainContext context.CancelFunc
		retryPods         []*api.Pod
	)
	mainContext, cancelMainContext = context.WithDeadline(context.Background(), o.drainStartedOn.Add(o.Timeout))
	defer cancelMainContext()

	for i, pod := range pods {
		select {
		case <-mainContext.Done():
			// Timeout occurred. Abort and report the remaining pods.
			returnCh <- nil
			return append(retryPods, pods[i+1:len(pods)]...), true
		default:
		}

		var (
			err                  error
			podEvictionStartTime time.Time
		)

		podEvictionStartTime = time.Now()

		if attemptEvict {
			err = o.evictPod(pod, policyGroupVersion)
		} else {
			err = o.deletePod(pod)
		}

		if attemptEvict && apierrors.IsTooManyRequests(err) {
			// Pod eviction failed because of PDB violation, we will retry one we are done with this list.
			glog.V(3).Info("Pod ", pod.Namespace, "/", pod.Name, " from node ", pod.Spec.NodeName, " couldn't be evicted. This may also occur due to PDB violation. Will be retried. Error:", err)
			retryPods = append(retryPods, pod)
			continue
		} else if apierrors.IsNotFound(err) {
			glog.V(3).Info("\t", pod.Name, " from node ", pod.Spec.NodeName, " is already gone")
			returnCh <- nil
			continue
		} else if err != nil {
			glog.V(4).Infof("Error when evicting pod: %v/%v from node %v. Will be retried. Err: %v", pod.Namespace, pod.Name, pod.Spec.NodeName, err)
			retryPods = append(retryPods, pod)
			continue
		}

		// Eviction was successful. Wait for pvs for this pod to detach
		glog.V(3).Infof(
			"Pod eviction/deletion for Pod %s/%s in Node %q and took %v. Now waiting for volume detachment.",
			pod.Namespace,
			pod.Name,
			pod.Spec.NodeName,
			time.Since(podEvictionStartTime),
		)

		pvs := volMap[pod.Namespace+"/"+pod.Name]
		ctx, cancelFn := context.WithTimeout(mainContext, o.getTerminationGracePeriod(pod)+o.PvDetachTimeout)
		err = o.waitForDetach(ctx, pvs, o.nodeName)
		cancelFn()

		if apierrors.IsNotFound(err) {
			glog.V(3).Info("Node not found anymore")
			returnCh <- nil
			return append(retryPods, pods[i+1:len(pods)]...), true
		} else if err != nil {
			glog.Errorf("Error when waiting for volume to detach from node. Err: %v", err)
			returnCh <- err
			continue
		}
		glog.V(3).Infof(
			"Volume detached for Pod %s/%s in Node %q and took %v (including pod eviction/deletion time).",
			pod.Namespace,
			pod.Name,
			pod.Spec.NodeName,
			time.Since(podEvictionStartTime),
		)
		returnCh <- nil
	}

	return retryPods, false
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

func (o *DrainOptions) waitForDetach(ctx context.Context, volumeIDs []string, nodeName string) error {
	if volumeIDs == nil || len(volumeIDs) == 0 || nodeName == "" {
		// If volume or node name is not available, nothing to do. Just log this as warning
		glog.Warningf("Node name: %q, list of pod PVs to wait for detach: %v", nodeName, volumeIDs)
		return nil
	}

	glog.V(4).Info("Waiting for following volumes to detach: ", volumeIDs)

	found := true

	for found {
		select {
		case <-ctx.Done():
			glog.Warningf("Timeout occurred while waiting for PVs to detach from node %q", nodeName)
			return fmt.Errorf("Timeout while waiting for PVs to detach from node")
		default:
		}

		found = false

		node, err := o.client.Core().Nodes().Get(nodeName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			glog.V(4).Info("Node not found: ", nodeName)
			return err
		} else if err != nil {
			glog.Errorf("Error getting details for node: %q. Err: %v", nodeName, err)
			return err
		}

		glog.V(4).Infof("No of attached volumes for node %q is %s", nodeName, node.Status.VolumesAttached)
		attachedVols := node.Status.VolumesAttached
		if len(attachedVols) == 0 {
			glog.V(4).Infof("No volumes attached to the node %q", nodeName)
			return nil
		}

	LookUpVolume:
		for i := range volumeIDs {
			volumeID := &volumeIDs[i]

			for j := range attachedVols {
				attachedVol := &attachedVols[j]

				found, _ = regexp.MatchString(*volumeID, string(attachedVol.Name))

				if found {
					glog.V(4).Infof(
						"Found volume:%s still attached to node %q. Will re-check in %s",
						*volumeID,
						nodeName,
						VolumeDetachPollInterval,
					)
					time.Sleep(VolumeDetachPollInterval)
					break LookUpVolume
				}
			}
		}
	}

	glog.V(4).Infof("Detached volumes:%s from node %q", volumeIDs, nodeName)
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

func (o *DrainOptions) evictPodWithoutPVInternal(attemptEvict bool, pod *corev1.Pod, policyGroupVersion string, getPodFn func(namespace, name string) (*api.Pod, error), returnCh chan error) {
	var err error
	glog.V(3).Infof(
		"Evicting pod %s/%s from node %q ",
		pod.Namespace,
		pod.Name,
		pod.Spec.NodeName,
	)

	nretries := int(o.MaxEvictRetries)
	for i := 0; ; i++ {
		if i >= nretries {
			attemptEvict = false
		}

		if attemptEvict {
			err = o.evictPod(pod, policyGroupVersion)
		} else {
			err = o.deletePod(pod)
		}

		if err == nil {
			break
		} else if apierrors.IsNotFound(err) {
			glog.V(3).Info("\t", pod.Name, " evicted from node ", pod.Spec.NodeName)
			returnCh <- nil
			return
		} else if attemptEvict && apierrors.IsTooManyRequests(err) {
			// Pod couldn't be evicted because of PDB violation
			time.Sleep(PodEvictionRetryInterval)
		} else {
			returnCh <- fmt.Errorf("error when evicting pod %q: %v scheduled on node %v", pod.Name, err, pod.Spec.NodeName)
			return
		}
	}

	if o.ForceDeletePods {
		// Skip waiting for pod termination in case of forced drain
		if err == nil {
			returnCh <- nil
		} else {
			returnCh <- err
		}
		return
	}

	podArray := []*api.Pod{pod}

	timeout := o.getTerminationGracePeriod(pod)
	if timeout > o.Timeout {
		glog.V(3).Infof("Overriding large termination grace period (%s) for the pod %s/%s and setting it to %s", timeout.String(), pod.Namespace, pod.Name, o.Timeout)
		timeout = o.Timeout
	}

	bufferPeriod := 30 * time.Second
	podArray, err = o.waitForDelete(podArray, Interval, timeout+bufferPeriod, true, getPodFn)
	if err == nil {
		if len(podArray) > 0 {
			returnCh <- fmt.Errorf("timeout expired while waiting for pod %q terminating scheduled on node %v", pod.Name, pod.Spec.NodeName)
		} else {
			returnCh <- nil
		}
	} else {
		returnCh <- fmt.Errorf("error when waiting for pod %q/%v terminating: %v", pod.Name, pod.Spec.NodeName, err)
	}
}

func (o *DrainOptions) waitForDelete(pods []*api.Pod, interval, timeout time.Duration, usingEviction bool, getPodFn func(string, string) (*api.Pod, error)) ([]*api.Pod, error) {
	err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		pendingPods := []*api.Pod{}
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
