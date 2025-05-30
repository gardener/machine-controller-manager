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

Modifications Copyright SAP SE or an SAP affiliate company and Gardener contributors
*/

// Package drain is used to drain nodes
package drain

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	policyv1listers "k8s.io/client-go/listers/policy/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	"github.com/gardener/machine-controller-manager/pkg/util/provider/driver"
)

// Options are configurable options while draining a node before deletion
type Options struct {
	client                       kubernetes.Interface
	kubernetesVersion            *semver.Version
	DeleteLocalData              bool
	Driver                       driver.Driver
	drainStartedOn               time.Time
	drainEndedOn                 time.Time
	ErrOut                       io.Writer
	ForceDeletePods              bool
	GracePeriodSeconds           int
	IgnorePodsWithoutControllers bool
	IgnoreDaemonsets             bool
	MaxEvictRetries              int32
	PvDetachTimeout              time.Duration
	PvReattachTimeout            time.Duration
	nodeName                     string
	Out                          io.Writer
	pvcLister                    corelisters.PersistentVolumeClaimLister
	pvLister                     corelisters.PersistentVolumeLister
	pdbLister                    policyv1listers.PodDisruptionBudgetLister
	nodeLister                   corelisters.NodeLister
	podLister                    corelisters.PodLister
	volumeAttachmentHandler      *VolumeAttachmentHandler
	Timeout                      time.Duration
	podSynced                    cache.InformerSynced
}

// Takes a pod and returns a bool indicating whether or not to operate on the
// pod, an optional warning message, and an optional fatal error.
type podFilter func(corev1.Pod) (include bool, w *warning, f *fatal)

type warning struct {
	string
}

type fatal struct {
	string
}

// PodVolumeInfo holds a list of infos about PersistentVolumes referenced by a single pod.
type PodVolumeInfo struct {
	// volumes is the list of infos about all PersistentVolumes referenced by a pod via PersistentVolumeClaims.
	volumes []VolumeInfo
}

// PersistentVolumeNames returns the names of all PersistentVolumes used by the pod.
func (p PodVolumeInfo) PersistentVolumeNames() []string {
	out := make([]string, 0, len(p.volumes))
	for _, volume := range p.volumes {
		out = append(out, volume.persistentVolumeName)
	}
	return out
}

// VolumeIDs returns the volume IDs/handles of all PersistentVolumes used by the pod.
func (p PodVolumeInfo) VolumeIDs() []string {
	out := make([]string, 0, len(p.volumes))
	for _, volume := range p.volumes {
		out = append(out, volume.volumeID)
	}
	return out
}

// VolumeInfo holds relevant information about a PersistentVolume for tracking attachments.
type VolumeInfo struct {
	// The name of the PersistentVolume referenced by PersistentVolumeClaim used in a Pod.
	persistentVolumeName string
	// The volume ID/handle corresponding to the PersistentVolume name as return by Driver.GetVolumeIDs.
	volumeID string
}

const (
	// EvictionKind is the kind used for eviction
	EvictionKind = "Eviction"
	// EvictionSubresource is the kind used for evicting pods
	EvictionSubresource = "pods/eviction"

	// DefaultMachineDrainTimeout is the default value for MachineDrainTimeout
	DefaultMachineDrainTimeout = 2 * time.Hour

	// PodsWithoutPVDrainGracePeriod defines the grace period to wait for the pods without PV during machine drain.
	// This is in addition to the maximum terminationGracePeriod amount the pods.
	PodsWithoutPVDrainGracePeriod = 3 * time.Minute

	// Interval is the default Poll interval
	Interval = time.Second * 5

	// PodEvictionRetryInterval is the interval in which to retry eviction for pods
	PodEvictionRetryInterval = time.Second * 20

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
	reattachTimeoutErr  = "Timeout occurred while waiting for PV to reattach to a different node"
)

var (
	// DefaultMaxEvictRetries is the default value for MaxEvictRetries
	DefaultMaxEvictRetries = int32(DefaultMachineDrainTimeout.Seconds() / PodEvictionRetryInterval.Seconds())
)

// NewDrainOptions creates a new DrainOptions struct and returns a pointer to it
func NewDrainOptions(
	client kubernetes.Interface,
	kubernetesVersion *semver.Version,
	timeout time.Duration,
	maxEvictRetries int32,
	pvDetachTimeout time.Duration,
	pvReattachTimeout time.Duration,
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
	pdbLister policyv1listers.PodDisruptionBudgetLister,
	nodeLister corelisters.NodeLister,
	podLister corelisters.PodLister,
	volumeAttachmentHandler *VolumeAttachmentHandler,
	podSynced cache.InformerSynced,
) *Options {
	return &Options{
		client:                       client,
		kubernetesVersion:            kubernetesVersion,
		ForceDeletePods:              forceDeletePods,
		IgnorePodsWithoutControllers: ignorePodsWithoutControllers,
		GracePeriodSeconds:           gracePeriodSeconds,
		IgnoreDaemonsets:             ignoreDaemonsets,
		MaxEvictRetries:              maxEvictRetries,
		Timeout:                      timeout,
		PvDetachTimeout:              pvDetachTimeout,
		PvReattachTimeout:            pvReattachTimeout,
		DeleteLocalData:              deleteLocalData,
		nodeName:                     nodeName,
		Out:                          out,
		ErrOut:                       errOut,
		Driver:                       driver,
		pvcLister:                    pvcLister,
		pvLister:                     pvLister,
		pdbLister:                    pdbLister,
		nodeLister:                   nodeLister,
		podLister:                    podLister,
		volumeAttachmentHandler:      volumeAttachmentHandler,
		podSynced:                    podSynced,
	}
}

// RunDrain runs the 'drain' command
func (o *Options) RunDrain(ctx context.Context) error {
	o.drainStartedOn = time.Now()
	drainContext, cancelFn := context.WithDeadline(ctx, o.drainStartedOn.Add(o.Timeout))
	klog.V(4).Infof(
		"Machine drain started on %s for %q",
		o.drainStartedOn,
		o.nodeName,
	)

	defer func() {
		o.drainEndedOn = time.Now()
		cancelFn()
		klog.Infof(
			"Machine drain ended on %s and took %s for %q",
			o.drainEndedOn,
			o.drainEndedOn.Sub(o.drainStartedOn),
			o.nodeName,
		)
	}()

	if err := o.RunCordonOrUncordon(drainContext, true); err != nil {
		klog.Errorf("Drain Error: Cordoning of node failed with error: %v", err)
		return err
	}
	if !cache.WaitForCacheSync(drainContext.Done(), o.podSynced) {
		err := fmt.Errorf("timed out waiting for pod cache to sync")
		return err
	}

	err := o.deleteOrEvictPodsSimple(drainContext)
	return err
}

func (o *Options) deleteOrEvictPodsSimple(ctx context.Context) error {
	pods, err := o.getPodsForDeletion()
	if err != nil {
		return err
	}

	err = o.deleteOrEvictPods(ctx, pods)
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

func (o *Options) getPodController(pod corev1.Pod) *metav1.OwnerReference {
	return metav1.GetControllerOf(&pod)
}

func (o *Options) unreplicatedFilter(pod corev1.Pod) (bool, *warning, *fatal) {
	// any finished pod can be removed
	if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
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

func (o *Options) daemonsetFilter(pod corev1.Pod) (bool, *warning, *fatal) {
	// Note that we return false in cases where the pod is DaemonSet managed,
	// regardless of flags.  We never delete them, the only question is whether
	// their presence constitutes an error.
	//
	// TODO: Might need to revisit this. This feature is ignored for now
	// The exception is for pods that are orphaned (the referencing
	// management resource - including DaemonSet - is not found).
	// Such pods will be deleted if --force is used.
	controllerRef := o.getPodController(pod)
	if controllerRef == nil || controllerRef.Kind != "DaemonSet" {
		return true, nil, nil
	}
	if !o.IgnoreDaemonsets {
		return false, nil, &fatal{daemonsetFatal}
	}
	return false, &warning{daemonsetWarning}, nil
}

func mirrorPodFilter(pod corev1.Pod) (bool, *warning, *fatal) {
	if _, found := pod.ObjectMeta.Annotations[corev1.MirrorPodAnnotationKey]; found {
		return false, nil, nil
	}
	return true, nil, nil
}

func hasLocalStorage(pod corev1.Pod) bool {
	for _, volume := range pod.Spec.Volumes {
		if volume.EmptyDir != nil {
			return true
		}
	}

	return false
}

func (o *Options) localStorageFilter(pod corev1.Pod) (bool, *warning, *fatal) {
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
func (o *Options) getPodsForDeletion() (pods []corev1.Pod, err error) {
	podList, err := o.podLister.List(labels.Everything())
	if err != nil {
		return
	}
	if len(podList) == 0 {
		klog.Infof("no pods found in store")
		return
	}
	ws := podStatuses{}
	fs := podStatuses{}

	for _, pod := range podList {
		if pod.Spec.NodeName != o.nodeName {
			continue
		}
		podOk := true
		for _, filt := range []podFilter{mirrorPodFilter, o.localStorageFilter, o.unreplicatedFilter, o.daemonsetFilter} {
			filterOk, w, f := filt(*pod)
			podOk = podOk && filterOk
			if w != nil {
				ws[w.string] = append(ws[w.string], pod.Name)
			}
			if f != nil {
				fs[f.string] = append(fs[f.string], pod.Name)
			}
		}
		if podOk {
			pods = append(pods, *pod)
		}
	}

	if len(fs) > 0 {
		return []corev1.Pod{}, errors.New(fs.Message())
	}
	if len(ws) > 0 {
		fmt.Fprintf(o.ErrOut, "WARNING: %s\n", ws.Message())
	}
	return pods, nil
}

func (o *Options) deletePod(ctx context.Context, pod *corev1.Pod) error {
	deleteOptions := metav1.DeleteOptions{}
	gracePeriodSeconds := int64(0)
	deleteOptions.GracePeriodSeconds = &gracePeriodSeconds

	klog.V(3).Infof("Attempting to force-delete the pod:%q from node %q", pod.Name, o.nodeName)
	return o.client.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, deleteOptions)
}

func (o *Options) evictPod(ctx context.Context, pod *corev1.Pod, policyGroupVersion string) error {
	deleteOptions := &metav1.DeleteOptions{}
	if o.GracePeriodSeconds >= 0 {
		gracePeriodSeconds := int64(o.GracePeriodSeconds)
		deleteOptions.GracePeriodSeconds = &gracePeriodSeconds
	}
	eviction := &policyv1beta1.Eviction{
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
	klog.V(3).Infof("Attempting to evict the pod:%q from node %q", pod.Name, o.nodeName)
	// TODO: Remember to change the URL manipulation func when Eviction's version change
	return o.client.PolicyV1beta1().Evictions(eviction.Namespace).Evict(ctx, eviction)
}

// deleteOrEvictPods deletes or evicts the pods on the api server
func (o *Options) deleteOrEvictPods(ctx context.Context, pods []corev1.Pod) error {
	if len(pods) == 0 {
		return nil
	}

	policyGroupVersion, err := SupportEviction(o.client)
	if err != nil {
		return err
	}

	getPodFn := func(namespace, name string) (*corev1.Pod, error) {
		return o.client.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	}

	attemptEvict := !o.ForceDeletePods && len(policyGroupVersion) > 0

	return o.evictPods(ctx, attemptEvict, pods, policyGroupVersion, getPodFn)
}

func volIsPvc(vol *corev1.Volume) bool {
	return vol.PersistentVolumeClaim != nil
}

func filterPodsWithPv(pods []corev1.Pod) ([]*corev1.Pod, []*corev1.Pod) {
	podsWithPv, podsWithoutPv := []*corev1.Pod{}, []*corev1.Pod{}

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

func (o *Options) getTerminationGracePeriod(pod *corev1.Pod) time.Duration {
	if pod == nil || pod.Spec.TerminationGracePeriodSeconds == nil {
		return time.Duration(0)
	}

	return time.Duration(*pod.Spec.TerminationGracePeriodSeconds) * time.Second
}

func (o *Options) getGlobalTimeoutForPodsWithoutPV(pods []*corev1.Pod) time.Duration {
	var tgpsMax time.Duration
	for _, pod := range pods {
		tgps := o.getTerminationGracePeriod(pod)
		if tgps > tgpsMax {
			tgpsMax = tgps
		}
	}

	return tgpsMax + PodsWithoutPVDrainGracePeriod
}

func (o *Options) evictPods(ctx context.Context, attemptEvict bool, pods []corev1.Pod, policyGroupVersion string, getPodFn func(namespace, name string) (*corev1.Pod, error)) error {
	returnCh := make(chan error, len(pods))
	defer close(returnCh)

	if o.ForceDeletePods {
		podsToDrain := make([]*corev1.Pod, len(pods))
		for i := range pods {
			podsToDrain[i] = &pods[i]
		}

		klog.V(3).Infof("Forceful eviction of pods on the node: %q", o.nodeName)

		// evict all pods in parallel without waiting for pods or volume detachment
		go o.evictPodsWithoutPv(ctx, attemptEvict, podsToDrain, policyGroupVersion, getPodFn, returnCh)
	} else {
		podsWithPv, podsWithoutPv := filterPodsWithPv(pods)

		klog.V(3).Infof("Normal eviction of pods on the node: %q", o.nodeName)

		// evcit all pods without PV in parallel and with PV in serial (waiting for vol detachment)
		go o.evictPodsWithPv(ctx, attemptEvict, podsWithPv, policyGroupVersion, getPodFn, returnCh)
		go o.evictPodsWithoutPv(ctx, attemptEvict, podsWithoutPv, policyGroupVersion, getPodFn, returnCh)
	}

	doneCount := 0
	var evictErrors []error

	numPods := len(pods)
	for doneCount < numPods {
		err := <-returnCh
		doneCount++
		if err != nil {
			evictErrors = append(evictErrors, err)
		}
	}
	return utilerrors.NewAggregate(evictErrors)
}

func (o *Options) evictPodsWithoutPv(ctx context.Context, attemptEvict bool, pods []*corev1.Pod,
	policyGroupVersion string,
	getPodFn func(namespace, name string) (*corev1.Pod, error),
	returnCh chan error,
) {
	for _, pod := range pods {
		go o.evictPodWithoutPVInternal(ctx, attemptEvict, pod, policyGroupVersion, getPodFn, returnCh)
	}
}

func sortPodsByPriority(pods []*corev1.Pod) {
	sort.Slice(pods, func(i, j int) bool {
		return *pods[i].Spec.Priority > *pods[j].Spec.Priority
	})
}

// getPodVolumeInfos returns information about all PersistentVolumes of which the machine controller needs to track
// attachments when draining a node.
// It filters out shared PVs (used by multiple pods).
// Also, when mapping PersistentVolume names to volume IDs, the driver filters out volumes of types that don't belong
// to the respective cloud provider. E.g., this filters out CSI volumes of unrelated drivers and NFS volumes, etc.
func (o *Options) getPodVolumeInfos(ctx context.Context, pods []*corev1.Pod) map[string]PodVolumeInfo {
	var (
		persistentVolumeNamesByPod = make(map[string][]string)
		podVolumeInfos             = make(map[string]PodVolumeInfo)
	)

	for _, pod := range pods {
		persistentVolumeNamesByPod[getPodKey(pod)] = o.getPersistentVolumeNamesForPod(pod)
	}

	// Filter the list of shared PVs
	filterSharedPVs(persistentVolumeNamesByPod)

	for podKey, persistentVolumeNames := range persistentVolumeNamesByPod {
		podVolumeInfo := PodVolumeInfo{}

		for _, persistentVolumeName := range persistentVolumeNames {
			volumeID, err := o.getVolumeIDFromDriver(ctx, persistentVolumeName)
			if err != nil {
				// In case of error, log and skip this set of volumes
				klog.Errorf("error getting volume ID from cloud provider. Skipping volume %s for pod: %v. Err: %v", persistentVolumeName, podKey, err)
				continue
			}

			// Only if the driver returns a volume ID for this PV, we want to track its attachment during drain operations.
			if volumeID != "" {
				podVolumeInfo.volumes = append(podVolumeInfo.volumes, VolumeInfo{
					persistentVolumeName: persistentVolumeName,
					volumeID:             volumeID,
				})
			} else {
				klog.V(4).Infof("Driver did not return a volume ID for volume %s. Skipping provider-unrelated volume for pod %s", persistentVolumeName, podKey)
			}
		}

		podVolumeInfos[podKey] = podVolumeInfo
	}
	klog.V(4).Infof("PodVolumeInfos: %v", podVolumeInfos)

	return podVolumeInfos
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
		klog.V(4).Info("No shared volumes found.")
		return
	}

	// Recreate the values of pvMap. Append volume if it is not shared
	for pod, vols := range pvMap {
		volList := []string{}
		for _, vol := range vols {
			if !sharedVol[vol] {
				volList = append(volList, vol)
			}
		}
		pvMap[pod] = volList
	}
	klog.V(3).Info("Removed shared volumes. Filtered list of pods with volumes: ", pvMap)
}

func (o *Options) evictPodsWithPv(ctx context.Context, attemptEvict bool, pods []*corev1.Pod,
	policyGroupVersion string,
	getPodFn func(namespace, name string) (*corev1.Pod, error),
	returnCh chan error,
) {
	sortPodsByPriority(pods)

	podVolumeInfoMap := o.getPodVolumeInfos(ctx, pods)

	var (
		remainingPods []*corev1.Pod
		fastTrack     bool
		nretries      = int(o.MaxEvictRetries)
	)

	if attemptEvict {
		for i := 0; i < nretries; i++ {
			remainingPods, fastTrack = o.evictPodsWithPVInternal(ctx, attemptEvict, pods, podVolumeInfoMap, policyGroupVersion, getPodFn, returnCh)
			if fastTrack || len(remainingPods) == 0 {
				// Either all pods got evicted or we need to fast track the return (node deletion detected)
				break
			}

			klog.V(4).Infof(
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
			remainingPods, _ = o.evictPodsWithPVInternal(ctx, false, pods, podVolumeInfoMap, policyGroupVersion, getPodFn, returnCh)
		}
	} else {
		remainingPods, _ = o.evictPodsWithPVInternal(ctx, false, pods, podVolumeInfoMap, policyGroupVersion, getPodFn, returnCh)
	}

	// Placate the caller by returning the nil status for the remaining pods.
	for _, pod := range remainingPods {
		klog.V(4).Infof("Returning success for remaining pods for node %q", o.nodeName)
		if fastTrack {
			// This is executed when node is not found anymore.
			// Return success to caller for all non-processed pods so that the caller function can move on.
			returnCh <- nil
		} else if attemptEvict {
			returnCh <- fmt.Errorf("error evicting pod %s/%s from node %q", pod.Namespace, pod.Name, pod.Spec.NodeName)
		} else {
			returnCh <- fmt.Errorf("error deleting pod %s/%s from node %q", pod.Namespace, pod.Name, pod.Spec.NodeName)
		}
	}
}

// checkAndDeleteWorker is a helper method that check if volumeAttachmentHandler
// is supported and deletes the worker from the list of event handlers
func (o *Options) checkAndDeleteWorker(volumeAttachmentEventCh chan *storagev1.VolumeAttachment) {
	if o.volumeAttachmentHandler != nil {
		o.volumeAttachmentHandler.DeleteWorker(volumeAttachmentEventCh)
	}
}

func (o *Options) evictPodsWithPVInternal(
	ctx context.Context,
	attemptEvict bool,
	pods []*corev1.Pod,
	podVolumeInfoMap map[string]PodVolumeInfo,
	policyGroupVersion string,
	getPodFn func(namespace, name string) (*corev1.Pod, error),
	returnCh chan error,
) (remainingPods []*corev1.Pod, fastTrack bool) {
	var (
		retryPods, pendingPods []*corev1.Pod
	)

	for i, pod := range pods {
		select {
		case <-ctx.Done():
			// Timeout occurred. Abort and report the remaining pods.
			returnCh <- nil
			return append(retryPods, pods[i+1:]...), true
		default:
		}

		var (
			err                     error
			volumeAttachmentEventCh chan *storagev1.VolumeAttachment
			podEvictionStartTime    = time.Now()
		)

		if o.volumeAttachmentHandler != nil {
			// Initialize event handler before triggerring pod delete/evict to avoid missing of events
			volumeAttachmentEventCh = o.volumeAttachmentHandler.AddWorker()
		}

		if attemptEvict {
			err = o.evictPod(ctx, pod, policyGroupVersion)
		} else {
			err = o.deletePod(ctx, pod)
		}

		if attemptEvict && apierrors.IsTooManyRequests(err) {
			// Pod eviction failed because of PDB violation, we will retry one we are done with this list.
			klog.V(3).Infof("Pod %s/%s couldn't be evicted from node %s. This may also occur due to PDB violation. Will be retried. Error: %v", pod.Namespace, pod.Name, pod.Spec.NodeName, err)

			pdb := getPdbForPod(o.pdbLister, pod)
			if pdb != nil {
				if isMisconfiguredPdb(pdb) {
					pdbErr := fmt.Errorf("error while evicting pod %q: pod disruption budget %s/%s is misconfigured and requires zero voluntary evictions",
						pod.Name, pdb.Namespace, pdb.Name)
					returnCh <- pdbErr
					o.checkAndDeleteWorker(volumeAttachmentEventCh)
					continue
				}
			}

			retryPods = append(retryPods, pod)
			o.checkAndDeleteWorker(volumeAttachmentEventCh)
			continue
		} else if apierrors.IsNotFound(err) {
			klog.V(3).Info("\t", pod.Name, " from node ", pod.Spec.NodeName, " is already gone")
			returnCh <- nil
			o.checkAndDeleteWorker(volumeAttachmentEventCh)
			continue
		} else if err != nil {
			klog.Errorf("error when evicting pod: %v/%v from node %v. Will be retried. Err: %v", pod.Namespace, pod.Name, pod.Spec.NodeName, err)
			retryPods = append(retryPods, pod)
			o.checkAndDeleteWorker(volumeAttachmentEventCh)
			continue
		}

		// Eviction was successful. Wait for pvs for this pod to detach
		klog.V(3).Infof(
			"Pod eviction/deletion for Pod %s/%s in Node %q and took %v. Now waiting for volume detachment.",
			pod.Namespace,
			pod.Name,
			pod.Spec.NodeName,
			time.Since(podEvictionStartTime),
		)

		podVolumeInfo := podVolumeInfoMap[getPodKey(pod)]
		volDetachCtx, cancelFn := context.WithTimeout(ctx, o.getTerminationGracePeriod(pod)+o.PvDetachTimeout)
		err = o.waitForDetach(volDetachCtx, podVolumeInfo, o.nodeName)
		cancelFn()

		if apierrors.IsNotFound(err) {
			klog.V(3).Info("Node not found anymore")
			returnCh <- nil
			o.checkAndDeleteWorker(volumeAttachmentEventCh)
			return append(retryPods, pods[i+1:]...), true
		} else if err != nil {
			klog.Errorf("error when waiting for volume to detach from node. Err: %v", err)
			returnCh <- err
			o.checkAndDeleteWorker(volumeAttachmentEventCh)
			continue
		}
		klog.V(4).Infof(
			"Pod + volume detachment from Node %s for Pod %s/%s and took %v",
			pod.Namespace,
			pod.Name,
			pod.Spec.NodeName,
			time.Since(podEvictionStartTime),
		)

		volReattachCtx, cancelFn := context.WithTimeout(ctx, o.PvReattachTimeout)
		err = o.waitForReattach(volReattachCtx, podVolumeInfo, o.nodeName, volumeAttachmentEventCh)
		cancelFn()

		if err != nil {
			if err.Error() != reattachTimeoutErr {
				klog.Errorf("error when waiting for volume reattachment. Err: %v", err)
				returnCh <- err
				o.checkAndDeleteWorker(volumeAttachmentEventCh)
				continue
			}
			klog.Warningf("Timeout occurred for following volumes to reattach: %v", podVolumeInfo.volumes)
		}

		o.checkAndDeleteWorker(volumeAttachmentEventCh)
		klog.V(3).Infof(
			"Pod + volume detachment from node %s + volume reattachment to another node for Pod %s/%s took %v",
			o.nodeName,
			pod.Namespace,
			pod.Name,
			time.Since(podEvictionStartTime),
		)

		returnCh <- nil
	}

	for _, pod := range retryPods {
		p, err := getPodFn(pod.Namespace, pod.Name)
		if apierrors.IsNotFound(err) || (p != nil && p.ObjectMeta.UID != pod.ObjectMeta.UID) {
			returnCh <- nil
			continue
		} else if err != nil {
			klog.Errorf("error fetching pod %s status. Retrying. Err: %v", pod.Name, err)
		}
		pendingPods = append(pendingPods, pod)
	}

	return pendingPods, false
}

func (o *Options) getPersistentVolumeNamesForPod(pod *corev1.Pod) []string {
	var pvs []string

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
						klog.Errorf("error getting PVC. Err: %v", err)
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

	return pvs
}

func (o *Options) waitForDetach(ctx context.Context, podVolumeInfo PodVolumeInfo, nodeName string) error {
	if len(podVolumeInfo.volumes) == 0 || nodeName == "" {
		// If volume or node name is not available, nothing to do. Just log this as warning
		klog.Warningf("Node name: %q, list of pod PVs to wait for detach: %v", nodeName, podVolumeInfo.volumes)
		return nil
	}

	klog.V(3).Infof("Waiting for following volumes to detach: %v", podVolumeInfo.volumes)

	found := true

	for found {
		select {
		case <-ctx.Done():
			klog.Warningf("Timeout occurred while waiting for PVs to detach from node %q", nodeName)
			return fmt.Errorf("timeout while waiting for PVs to detach from node")
		default:
		}

		found = false

		node, err := o.client.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			klog.V(4).Info("Node not found: ", nodeName)
			return err
		} else if err != nil {
			klog.Errorf("error getting details for node: %q. Err: %v", nodeName, err)
			return err
		}

		klog.V(4).Infof("Volumes attached to node %q: %s", nodeName, node.Status.VolumesAttached)
		attachedVols := node.Status.VolumesAttached
		if len(attachedVols) == 0 {
			klog.V(4).Infof("No volumes attached to the node %q", nodeName)
			return nil
		}

	LookUpVolume:
		for _, volumeID := range podVolumeInfo.VolumeIDs() {

			for j := range attachedVols {
				attachedVol := &attachedVols[j]

				found, _ = regexp.MatchString(volumeID, string(attachedVol.Name))

				if found {
					klog.V(4).Infof(
						"Found volume %q still attached to node %q. Will re-check in %s",
						volumeID,
						nodeName,
						VolumeDetachPollInterval,
					)
					time.Sleep(VolumeDetachPollInterval)
					break LookUpVolume
				}
			}
		}
	}

	klog.V(3).Infof("Detached volumes %v from node %q", podVolumeInfo.volumes, nodeName)
	return nil
}

func isDesiredReattachment(volumeAttachment *storagev1.VolumeAttachment, previousNodeName string) bool {
	// klog.Errorf("\nPV: %s = %s, \nAttached: %b, \nNode: %s = %s", *volumeAttachment.Spec.Source.PersistentVolumeName, persistentVolumeName, volumeAttachment.Status.Attached, volumeAttachment.Spec.NodeName, previousNodeName)
	if volumeAttachment.Status.Attached && volumeAttachment.Spec.NodeName != previousNodeName {
		klog.V(4).Infof("ReattachmentSuccessful for PV: %q", *volumeAttachment.Spec.Source.PersistentVolumeName)
		return true
	}

	return false
}

// waitForReattach to consider 2 cases
// 1. If CSI is enabled use determine reattach
// 2. If all else fails, fallback to static timeout
func (o *Options) waitForReattach(ctx context.Context, podVolumeInfo PodVolumeInfo, previousNodeName string, volumeAttachmentEventCh chan *storagev1.VolumeAttachment) error {
	if len(podVolumeInfo.volumes) == 0 || previousNodeName == "" {
		// If volume or node name is not available, nothing to do. Just log this as warning
		klog.Warningf("List of pod PVs waiting for reattachment is 0: %v", podVolumeInfo.volumes)
		return nil
	}

	klog.V(3).Infof("Waiting for following volumes to reattach: %v", podVolumeInfo.volumes)

	var pvsWaitingForReattachments map[string]bool
	if volumeAttachmentEventCh != nil {
		pvsWaitingForReattachments = make(map[string]bool)
		for _, persistentVolumeName := range podVolumeInfo.PersistentVolumeNames() {
			pvsWaitingForReattachments[persistentVolumeName] = true
		}
	}

	// This loop exits in either of the following cases
	// 1. Context timeout occurs - PV rettachment timeout or Drain Timeout
	// 2. All PVs for given pod are reattached successfully
	for {
		select {

		case <-ctx.Done():
			// Timeout occurred waiting for reattachment, exit function with error
			klog.Warningf("Timeout occurred while waiting for PVs %v to reattach to a different node", podVolumeInfo.volumes)
			return fmt.Errorf("%s", reattachTimeoutErr)

		case incomingEvent := <-volumeAttachmentEventCh:
			persistentVolumeName := *incomingEvent.Spec.Source.PersistentVolumeName
			klog.V(4).Infof("VolumeAttachment event received for PV: %s", persistentVolumeName)

			// Checking if event for an PV that is being waited on
			if _, present := pvsWaitingForReattachments[persistentVolumeName]; present {
				// Check if reattachment was successful
				if reattachmentSuccess := isDesiredReattachment(incomingEvent, previousNodeName); reattachmentSuccess {
					delete(pvsWaitingForReattachments, persistentVolumeName)
				}
			}
		}

		if len(pvsWaitingForReattachments) == 0 {
			// If all PVs have been reattached, break out of for loop
			break
		}
	}

	klog.V(3).Infof("Successfully reattached volumes: %s", podVolumeInfo.volumes)
	return nil
}

func (o *Options) getVolumeIDFromDriver(ctx context.Context, pvName string) (string, error) {
	var pvSpec *corev1.PersistentVolumeSpec

	try := 0

	for {
		pv, err := o.pvLister.Get(pvName)

		if apierrors.IsNotFound(err) {
			return "", nil
		} else if err != nil {
			try++
			if try == GetPvDetailsMaxRetries {
				return "", err
			}
			// In case of error, try again after few seconds
			time.Sleep(GetPvDetailsRetryInterval)
			continue
		}

		// found PV
		pvSpec = &pv.Spec
		break
	}

	response, err := o.Driver.GetVolumeIDs(ctx, &driver.GetVolumeIDsRequest{PVSpecs: []*corev1.PersistentVolumeSpec{pvSpec}})
	if err != nil {
		return "", err
	}

	if len(response.VolumeIDs) > 0 {
		return response.VolumeIDs[0], nil
	}

	return "", nil
}

func (o *Options) evictPodWithoutPVInternal(ctx context.Context, attemptEvict bool, pod *corev1.Pod, policyGroupVersion string, getPodFn func(namespace, name string) (*corev1.Pod, error), returnCh chan error) {
	var err error
	klog.V(3).Infof(
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
			err = o.evictPod(ctx, pod, policyGroupVersion)
		} else {
			err = o.deletePod(ctx, pod)
		}

		if err == nil {
			break
		} else if apierrors.IsNotFound(err) {
			klog.V(3).Info("\t", pod.Name, " evicted from node ", pod.Spec.NodeName)
			returnCh <- nil
			return
		} else if !attemptEvict || !apierrors.IsTooManyRequests(err) {
			returnCh <- fmt.Errorf("error when evicting pod %q: %v scheduled on node %v", pod.Name, err, pod.Spec.NodeName)
			return
		}
		// Pod couldn't be evicted because of PDB violation
		klog.V(3).Infof("Pod %s/%s couldn't be evicted from node %s. This may also occur due to PDB violation. Will be retried. Error: %v", pod.Namespace, pod.Name, pod.Spec.NodeName, err)

		pdb := getPdbForPod(o.pdbLister, pod)
		if pdb != nil {
			if isMisconfiguredPdb(pdb) {
				pdbErr := fmt.Errorf("error while evicting pod %q: pod disruption budget %s/%s is misconfigured and requires zero voluntary evictions",
					pod.Name, pdb.Namespace, pdb.Name)
				returnCh <- pdbErr
				return
			}
		}

		time.Sleep(PodEvictionRetryInterval)
	}

	if o.ForceDeletePods {
		// Skip waiting for pod termination in case of forced drain
		returnCh <- nil
		return
	}

	podArray := []*corev1.Pod{pod}

	timeout := o.getTerminationGracePeriod(pod)
	if timeout > o.Timeout {
		klog.V(3).Infof("Overriding large termination grace period (%s) for the pod %s/%s and setting it to %s", timeout.String(), pod.Namespace, pod.Name, o.Timeout)
		timeout = o.Timeout
	}

	bufferPeriod := 30 * time.Second
	podArray, err = o.waitForDelete(podArray, Interval, timeout+bufferPeriod, getPodFn)
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

func (o *Options) waitForDelete(pods []*corev1.Pod, interval, timeout time.Duration, getPodFn func(string, string) (*corev1.Pod, error)) ([]*corev1.Pod, error) {
	err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		pendingPods := []*corev1.Pod{}
		for i, pod := range pods {
			p, err := getPodFn(pod.Namespace, pod.Name)
			if apierrors.IsNotFound(err) || (p != nil && p.ObjectMeta.UID != pod.ObjectMeta.UID) {
				// cmdutil.PrintSuccess(o.mapper, false, o.Out, "pod", pod.Name, false, verbStr)
				// klog.Info("pod deleted successfully found")
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
func (o *Options) RunCordonOrUncordon(ctx context.Context, desired bool) error {
	node, err := o.client.CoreV1().Nodes().Get(ctx, o.nodeName, metav1.GetOptions{})
	if err != nil {
		// Deletion could be triggered when machine is just being created, no node present then
		return nil
	}
	unsched := node.Spec.Unschedulable
	if unsched == desired {
		klog.V(3).Infof("Scheduling state for node %q is already in desired state", node.Name)
	} else {
		clone := node.DeepCopy()
		clone.Spec.Unschedulable = desired

		_, err = o.client.CoreV1().Nodes().Update(ctx, clone, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func getPdbForPod(pdbLister policyv1listers.PodDisruptionBudgetLister, pod *corev1.Pod) *policyv1.PodDisruptionBudget {
	// GetPodPodDisruptionBudgets returns an error only if no PodDisruptionBudgets are found.
	// We don't return that as an error to the caller.
	pdbs, err := pdbLister.GetPodPodDisruptionBudgets(pod)
	if err != nil {
		klog.V(4).Infof("No PodDisruptionBudgets found for pod %s/%s.", pod.Namespace, pod.Name)
		return nil
	}

	if len(pdbs) > 1 {
		klog.Warningf("Pod %s/%s matches multiple PodDisruptionBudgets. Chose %q arbitrarily.", pod.Namespace, pod.Name, pdbs[0].Name)
	}

	return pdbs[0]
}

func isMisconfiguredPdb(pdb *policyv1.PodDisruptionBudget) bool {
	if pdb.ObjectMeta.Generation != pdb.Status.ObservedGeneration {
		return false
	}

	return pdb.Status.ExpectedPods > 0 && pdb.Status.CurrentHealthy >= pdb.Status.ExpectedPods && pdb.Status.DisruptionsAllowed == 0
}
