// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Package drain is used to drain nodes
package drain

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gcustom"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	coreinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"

	"github.com/gardener/machine-controller-manager/pkg/fakeclient"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/driver"
)

var _ = Describe("drain", func() {
	const (
		oldNodeName                    = "old-node"
		newNodeName                    = "new-node"
		terminationGracePeriodShort    = 5 * time.Second
		terminationGracePeriodShortBy4 = terminationGracePeriodShort / 4
		terminationGracePeriodShortBy8 = terminationGracePeriodShort / 8
		terminationGracePeriodMedium   = 10 * time.Second
		terminationGracePeriodDefault  = 20 * time.Second
		terminationGracePeriodLong     = 2 * time.Minute
		testNamespace                  = "test"
	)

	type stats struct {
		nPodsWithoutPV                int
		nPodsWithOnlyExclusivePV      int
		nPodsWithOnlySharedPV         int
		nPodsWithExclusiveAndSharedPV int
		nPVsPerPodWithExclusivePV     int
	}
	type setup struct {
		stats
		attemptEviction           bool
		volumeAttachmentSupported bool
		maxEvictRetries           int32
		terminationGracePeriod    time.Duration
		pvReattachTimeout         time.Duration
		force                     bool
		evictError                error
		deleteError               error
	}

	type expectation struct {
		stats
		timeout          time.Duration
		drainTimeout     bool
		drainError       error
		nEvictions       int
		minDrainDuration time.Duration
	}

	type podDrainHandler func(client kubernetes.Interface, pod *corev1.Pod, detachExclusiveVolumesCh chan<- *corev1.Pod) error

	run := func(setup *setup, podDrainHandlers []podDrainHandler, expected *expectation) {
		stop := make(chan struct{})
		defer close(stop)

		wg := sync.WaitGroup{}

		podsWithoutPV := getPodsWithoutPV(setup.nPodsWithoutPV, testNamespace, "nopv-", oldNodeName, setup.terminationGracePeriod, map[string]string{
			"volumes": "none",
		})
		podsWithOnlyExclusivePV := getPodsWithPV(setup.nPodsWithOnlyExclusivePV, setup.nPodsWithOnlyExclusivePV, 0, setup.nPVsPerPodWithExclusivePV, testNamespace, "expv-", "expv-", "", oldNodeName, setup.terminationGracePeriod, map[string]string{
			"volumes": "only-exclusive",
		})
		podsWithOnlySharedPV := getPodsWithPV(setup.nPodsWithOnlySharedPV, 0, setup.nPodsWithOnlySharedPV/2, setup.nPVsPerPodWithExclusivePV, testNamespace, "shpv-", "", "shpv-", oldNodeName, setup.terminationGracePeriod, map[string]string{
			"volumes": "only-shared",
		})
		nPodsWithExclusiveAndSharedPV := getPodsWithPV(setup.nPodsWithExclusiveAndSharedPV, setup.nPodsWithExclusiveAndSharedPV, setup.nPodsWithExclusiveAndSharedPV/2, setup.nPVsPerPodWithExclusivePV, testNamespace, "exshpv-", "exshexpv-", "exshshpv-", oldNodeName, setup.terminationGracePeriod, map[string]string{
			"volumes": "exclusive-and-shared",
		})

		var pods []*corev1.Pod
		pods = append(pods, podsWithoutPV...)
		pods = append(pods, podsWithOnlyExclusivePV...)
		pods = append(pods, podsWithOnlySharedPV...)
		pods = append(pods, nPodsWithExclusiveAndSharedPV...)

		pvcs := getPVCs(pods)
		pvs := getPVs(pvcs)
		nodes := []*corev1.Node{getNode(oldNodeName, pvs)}

		var targetCoreObjects []runtime.Object
		targetCoreObjects = appendPods(targetCoreObjects, pods)
		targetCoreObjects = appendPVCs(targetCoreObjects, pvcs)
		targetCoreObjects = appendPVs(targetCoreObjects, pvs)
		targetCoreObjects = appendNodes(targetCoreObjects, nodes)

		var volumeAttachmentHandler *VolumeAttachmentHandler
		// If volumeAttachmentSupported is enabled
		// setup volume attachments as well
		if setup.volumeAttachmentSupported {
			volumeAttachmentHandler = NewVolumeAttachmentHandler()
			volumeAttachments := getVolumeAttachments(pvs, oldNodeName)
			targetCoreObjects = appendVolumeAttachments(targetCoreObjects, volumeAttachments)
		}

		fakeTargetCoreClient, fakePVLister, fakePVCLister, fakeNodeLister, fakePodLister, pvcSynced, pvSynced, nodeSynced, podSynced, tracker := createFakeController(
			stop, testNamespace, targetCoreObjects,
		)
		defer tracker.Stop()

		// Waiting for cache sync
		Expect(cache.WaitForCacheSync(stop, pvcSynced, pvSynced, nodeSynced)).To(BeTrue())

		maxEvictRetries := setup.maxEvictRetries
		if maxEvictRetries <= 0 {
			maxEvictRetries = 3
		}

		pvReattachTimeout := setup.pvReattachTimeout
		if pvReattachTimeout == time.Duration(0) {
			// To mock quick reattachments by setting
			// reattachment time to 1 millisecond
			pvReattachTimeout = 1 * time.Millisecond
		}

		d := &Options{
			client:                       fakeTargetCoreClient,
			DeleteLocalData:              true,
			Driver:                       &drainDriver{},
			drainStartedOn:               time.Time{},
			drainEndedOn:                 time.Time{},
			ErrOut:                       GinkgoWriter,
			ForceDeletePods:              setup.force,
			GracePeriodSeconds:           30,
			IgnorePodsWithoutControllers: true,
			IgnoreDaemonsets:             true,
			MaxEvictRetries:              maxEvictRetries,
			PvDetachTimeout:              30 * time.Second,
			PvReattachTimeout:            pvReattachTimeout,
			nodeName:                     oldNodeName,
			Out:                          GinkgoWriter,
			pvcLister:                    fakePVCLister,
			pvLister:                     fakePVLister,
			pdbLister:                    nil,
			nodeLister:                   fakeNodeLister,
			podLister:                    fakePodLister,
			Timeout:                      2 * time.Minute,
			volumeAttachmentHandler:      volumeAttachmentHandler,
			podSynced:                    podSynced,
		}

		// Get the pod directly from the ObjectTracker to avoid locking issues in the Fake object.
		getPod := func(gvr schema.GroupVersionResource, ns, name string) (*corev1.Pod, error) {
			ro, err := tracker.Get(gvr, ns, name)
			if err != nil {
				return nil, err
			}

			return ro.(*corev1.Pod), nil
		}

		// Serialize volume detachment to avoid concurrency issues during node update.
		detachExclusiveVolumesCh := make(chan *corev1.Pod)
		defer close(detachExclusiveVolumesCh)

		runPodDrainHandlers := func(pod *corev1.Pod) {
			var err error
			for _, handler := range podDrainHandlers {
				err = handler(d.client, pod, detachExclusiveVolumesCh)
				if err != nil {
					break
				}
			}

			if err != nil {
				fmt.Fprintf(GinkgoWriter, "Error simulating eviction for the pod %s/%s: %s", pod.Namespace, pod.Name, err)
			}
		}

		// Serialize volume detachment to avoid concurrency issues during node update.
		go func() {
			for pod := range detachExclusiveVolumesCh {
				nodes := d.client.CoreV1().Nodes()
				node, err := nodes.Get(context.TODO(), pod.Spec.NodeName, metav1.GetOptions{})
				if err != nil {
					_, _ = fmt.Fprintln(GinkgoWriter, err)
					continue
				}

				node = node.DeepCopy()
				nodeUpdateRequired := false
				{
					remainingVolumesAttached := []corev1.AttachedVolume{}
					pvcs := getPVCs([]*corev1.Pod{pod})
					pvs := getPVs(pvcs)
					regexpObj, err := regexp.Compile("expv-")
					if err != nil {
						return
					}
					for i := range node.Status.VolumesAttached {
						va := &node.Status.VolumesAttached[i]

						if !regexpObj.Match([]byte(va.Name)) {
							// Detach only exclusive volumes
							remainingVolumesAttached = append(remainingVolumesAttached, *va)
							continue
						}

						found := false
						n := len(pvs)
						for i := range pvs {
							// Inverting reattachment logic to support to test out of order reattach
							j := n - i - 1
							if va.Name == corev1.UniqueVolumeName(getDrainTestVolumeName(&pvs[j].Spec)) {
								found = true
								if setup.volumeAttachmentSupported {
									// Serially reattach
									updateVolumeAttachments(d, pvs[j].Name, newNodeName)
								}
								break
							}
						}
						if !found {
							remainingVolumesAttached = append(remainingVolumesAttached, *va)
						}
					}
					if nodeUpdateRequired = len(remainingVolumesAttached) != len(node.Status.VolumesAttached); nodeUpdateRequired {
						node.Status.VolumesAttached = remainingVolumesAttached
					}
				}

				if !nodeUpdateRequired {
					continue
				}

				_, err = nodes.Update(context.TODO(), node, metav1.UpdateOptions{})
				_, _ = fmt.Fprintln(GinkgoWriter, err)

				_, err = nodes.UpdateStatus(context.TODO(), node, metav1.UpdateOptions{})
				_, _ = fmt.Fprintln(GinkgoWriter, err)
			}
		}()

		ctx, cancelCtx := context.WithTimeout(context.Background(), expected.timeout)
		defer cancelCtx()

		nEvictions := 0
		if setup.attemptEviction {
			fakeTargetCoreClient := fakeTargetCoreClient.(*fakeclient.Clientset)
			fakeTargetCoreClient.FakeDiscovery.Resources = []*metav1.APIResourceList{
				{
					GroupVersion: "policy/v1",
				},
				{
					GroupVersion: "v1",
					APIResources: []metav1.APIResource{
						{
							Name: EvictionSubresource,
							Kind: EvictionKind,
						},
					},
				},
			}

			// Fake eviction
			fakeTargetCoreClient.PrependReactor("post", "pods", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
				if setup.evictError != nil {
					return true, nil, setup.evictError
				}

				start := time.Now()
				switch ga := action.(type) {
				case k8stesting.GetAction:
					if ga.GetSubresource() != "eviction" {
						return
					}

					var pod *corev1.Pod
					pod, err = getPod(action.GetResource(), ga.GetNamespace(), ga.GetName())
					if err != nil {
						return
					}

					// Delete the pod asyncronously to work around the lock problems in testing.Fake
					wg.Add(1)
					go func() {
						defer wg.Done()
						runPodDrainHandlers(pod)
						fmt.Fprintf(GinkgoWriter, "Drained pod %s/%s in %s\n", pod.Namespace, pod.Name, time.Since(start).String())
					}()

					nEvictions++
					return
				default:
					err = fmt.Errorf("Expected type k8stesting.GetAction but got %T", action)
					return
				}
			})
		} else {
			// Work-around: Use a non-handling reactor in place of watch (because watch is not working).
			fakeTargetCoreClient := fakeTargetCoreClient.(*fakeclient.Clientset)
			fakeTargetCoreClient.PrependReactor("delete", "pods", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
				if setup.deleteError != nil {
					return true, nil, setup.deleteError
				}

				start := time.Now()
				switch ga := action.(type) {
				case k8stesting.DeleteAction:
					var pod *corev1.Pod
					pod, err = getPod(action.GetResource(), ga.GetNamespace(), ga.GetName())
					if err != nil {
						return
					}

					// Delete the pod asyncronously to work around the lock problems in testing.Fake
					wg.Add(1)
					go func() {
						defer wg.Done()
						runPodDrainHandlers(pod)
						fmt.Fprintf(GinkgoWriter, "Drained pod %s/%s in %s\n", pod.Namespace, pod.Name, time.Since(start).String())
					}()
				default:
					err = fmt.Errorf("Expected type k8stesting.GetAction but got %T", action)
				}

				return
			})
		}

		var drainErr error
		var drainStart, drainEnd *time.Time
		go func() {
			start := time.Now()
			drainStart = &start
			drainErr = d.RunDrain(context.TODO())
			end := time.Now()
			drainEnd = &end
			cancelCtx()
		}()

		// Wait for the context to complete or timeout.
		<-ctx.Done()

		if expected.drainTimeout {
			Expect(ctx.Err()).To(Equal(context.DeadlineExceeded))

			// TODO Find a way to validate rest of the details in case of an expected timeout.
			return
		}

		Expect(ctx.Err()).ToNot(Equal(context.DeadlineExceeded))

		if expected.drainError == nil {
			Expect(drainErr).ShouldNot(HaveOccurred())
		} else {
			Expect(drainErr).To(Equal(expected.drainError))
		}

		wg.Wait()

		Expect(nEvictions).To(Equal(expected.nEvictions))

		if expected.minDrainDuration > 0 {
			Expect(drainStart).ToNot(BeNil())
			Expect(drainEnd).ToNot(BeNil())
			Expect(drainEnd.Sub(*drainStart)).To(BeNumerically(">=", expected.minDrainDuration))
		}

		validatePodCount := func(labelSelector string, nExpected int) {
			podList, err := d.client.CoreV1().Pods(testNamespace).List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(podList).ToNot(BeNil())
			Expect(podList.Items).To(HaveLen(nExpected))
		}

		validatePodCount("volumes=none", expected.nPodsWithoutPV)
		validatePodCount("volumes=only-exclusive", expected.nPodsWithOnlyExclusivePV)
		validatePodCount("volumes=only-shared", expected.nPodsWithOnlySharedPV)
		validatePodCount("volumes=exclusive-and-shared", expected.nPodsWithExclusiveAndSharedPV)
	}

	sleepFor := func(d time.Duration) podDrainHandler {
		return func(_ kubernetes.Interface, _ *corev1.Pod, _ chan<- *corev1.Pod) error {
			time.Sleep(d)
			return nil
		}
	}

	deletePod := func(client kubernetes.Interface, pod *corev1.Pod, _ chan<- *corev1.Pod) error {
		return client.CoreV1().Pods(pod.Namespace).Delete(context.TODO(), pod.Name, metav1.DeleteOptions{})
	}

	detachExclusiveVolumes := func(_ kubernetes.Interface, pod *corev1.Pod, detachExclusiveVolumesCh chan<- *corev1.Pod) error {
		detachExclusiveVolumesCh <- pod
		return nil
	}

	DescribeTable("RunDrain", run,
		Entry("Successful drain without support for eviction pods without volume",
			&setup{
				stats: stats{
					nPodsWithoutPV:                10,
					nPodsWithOnlyExclusivePV:      0,
					nPodsWithOnlySharedPV:         0,
					nPodsWithExclusiveAndSharedPV: 0,
					nPVsPerPodWithExclusivePV:     1,
				},
				attemptEviction:        false,
				terminationGracePeriod: terminationGracePeriodShort,
			},
			nil,
			&expectation{
				stats: stats{
					nPodsWithoutPV:                0,
					nPodsWithOnlyExclusivePV:      0,
					nPodsWithOnlySharedPV:         0,
					nPodsWithExclusiveAndSharedPV: 0,
				},
				timeout:          terminationGracePeriodShort,
				drainTimeout:     false,
				drainError:       nil,
				nEvictions:       0,
				minDrainDuration: 0,
			}),
		Entry("Successful drain with support for eviction of pods without volume",
			&setup{
				stats: stats{
					nPodsWithoutPV:                10,
					nPodsWithOnlyExclusivePV:      0,
					nPodsWithOnlySharedPV:         0,
					nPodsWithExclusiveAndSharedPV: 0,
					nPVsPerPodWithExclusivePV:     1,
				},
				attemptEviction:        true,
				terminationGracePeriod: terminationGracePeriodShort,
			},
			[]podDrainHandler{deletePod},
			&expectation{
				stats: stats{
					nPodsWithoutPV:                0,
					nPodsWithOnlyExclusivePV:      0,
					nPodsWithOnlySharedPV:         0,
					nPodsWithExclusiveAndSharedPV: 0,
				},
				// Because waitForDelete polling Interval is equal to terminationGracePeriodShort
				timeout:      terminationGracePeriodMedium,
				drainTimeout: false,
				drainError:   nil,
				nEvictions:   10,
				// Because waitForDelete polling Interval is equal to terminationGracePeriodShort
				minDrainDuration: terminationGracePeriodShort,
			}),
		Entry("Successful drain without support for eviction of pods with exclusive volumes",
			&setup{
				stats: stats{
					nPodsWithoutPV:                0,
					nPodsWithOnlyExclusivePV:      2,
					nPodsWithOnlySharedPV:         0,
					nPodsWithExclusiveAndSharedPV: 0,
					nPVsPerPodWithExclusivePV:     1,
				},
				attemptEviction:        false,
				terminationGracePeriod: terminationGracePeriodShort,
			},
			[]podDrainHandler{sleepFor(terminationGracePeriodShortBy8), detachExclusiveVolumes},
			&expectation{
				stats: stats{
					nPodsWithoutPV:                0,
					nPodsWithOnlyExclusivePV:      0,
					nPodsWithOnlySharedPV:         0,
					nPodsWithExclusiveAndSharedPV: 0,
				},
				// Because waitForDetach polling Interval is equal to terminationGracePeriodShort
				timeout:      terminationGracePeriodDefault,
				drainTimeout: false,
				drainError:   nil,
				nEvictions:   0,
				// Because waitForDetach polling Interval is equal to terminationGracePeriodShort
				minDrainDuration: terminationGracePeriodMedium,
			}),
		Entry("Successful drain with support for eviction of pods with exclusive volumes",
			&setup{
				stats: stats{
					nPodsWithoutPV:                0,
					nPodsWithOnlyExclusivePV:      2,
					nPodsWithOnlySharedPV:         0,
					nPodsWithExclusiveAndSharedPV: 0,
					nPVsPerPodWithExclusivePV:     1,
				},
				attemptEviction:        true,
				terminationGracePeriod: terminationGracePeriodShort,
			},
			[]podDrainHandler{deletePod, sleepFor(terminationGracePeriodShortBy8), detachExclusiveVolumes},
			&expectation{
				stats: stats{
					nPodsWithoutPV:                0,
					nPodsWithOnlyExclusivePV:      0,
					nPodsWithOnlySharedPV:         0,
					nPodsWithExclusiveAndSharedPV: 0,
				},
				// Because waitForDetach polling Interval is equal to terminationGracePeriodShort
				timeout:      terminationGracePeriodDefault,
				drainTimeout: false,
				drainError:   nil,
				nEvictions:   2,
				// Because waitForDetach polling Interval is equal to terminationGracePeriodShort
				minDrainDuration: terminationGracePeriodMedium,
			}),
		Entry("Successful drain with support for eviction of pods with exclusive volumes with volume attachments",
			&setup{
				stats: stats{
					nPodsWithoutPV:                0,
					nPodsWithOnlyExclusivePV:      2,
					nPodsWithOnlySharedPV:         0,
					nPodsWithExclusiveAndSharedPV: 0,
					nPVsPerPodWithExclusivePV:     1,
				},
				attemptEviction:           true,
				volumeAttachmentSupported: true,
				pvReattachTimeout:         30 * time.Second,
				terminationGracePeriod:    terminationGracePeriodShort,
			},
			[]podDrainHandler{deletePod, sleepFor(terminationGracePeriodShortBy8), detachExclusiveVolumes},
			&expectation{
				stats: stats{
					nPodsWithoutPV:                0,
					nPodsWithOnlyExclusivePV:      0,
					nPodsWithOnlySharedPV:         0,
					nPodsWithExclusiveAndSharedPV: 0,
				},
				// Because waitForDetach polling Interval is equal to terminationGracePeriodDefault
				timeout:      terminationGracePeriodLong,
				drainTimeout: false,
				drainError:   nil,
				nEvictions:   2,
				// Because waitForDetach polling Interval is equal to terminationGracePeriodMedium
				minDrainDuration: terminationGracePeriodMedium,
			}),
		Entry("Successful drain with support for eviction of pods with 2 exclusive volumes with volume attachments",
			&setup{
				stats: stats{
					nPodsWithoutPV:                0,
					nPodsWithOnlyExclusivePV:      1,
					nPodsWithOnlySharedPV:         0,
					nPodsWithExclusiveAndSharedPV: 0,
					nPVsPerPodWithExclusivePV:     2,
				},
				attemptEviction:           true,
				volumeAttachmentSupported: true,
				pvReattachTimeout:         30 * time.Second,
				terminationGracePeriod:    terminationGracePeriodShort,
			},
			[]podDrainHandler{deletePod, sleepFor(terminationGracePeriodShortBy8), detachExclusiveVolumes},
			&expectation{
				stats: stats{
					nPodsWithoutPV:                0,
					nPodsWithOnlyExclusivePV:      0,
					nPodsWithOnlySharedPV:         0,
					nPodsWithExclusiveAndSharedPV: 0,
				},
				// Because waitForDetach polling Interval is equal to terminationGracePeriodDefault
				timeout:      terminationGracePeriodDefault,
				drainTimeout: false,
				drainError:   nil,
				nEvictions:   1,
				// Because waitForDetach polling Interval is equal to terminationGracePeriodMedium
				minDrainDuration: terminationGracePeriodMedium,
			}),
		Entry("Successful drain without support for eviction of pods with shared volumes",
			&setup{
				stats: stats{
					nPodsWithoutPV:                0,
					nPodsWithOnlyExclusivePV:      0,
					nPodsWithOnlySharedPV:         2,
					nPodsWithExclusiveAndSharedPV: 0,
					nPVsPerPodWithExclusivePV:     1,
				},
				attemptEviction:        false,
				terminationGracePeriod: terminationGracePeriodShort,
			},
			nil,
			&expectation{
				stats: stats{
					nPodsWithoutPV:                0,
					nPodsWithOnlyExclusivePV:      0,
					nPodsWithOnlySharedPV:         0,
					nPodsWithExclusiveAndSharedPV: 0,
				},
				timeout:          terminationGracePeriodShort,
				drainTimeout:     false,
				drainError:       nil,
				nEvictions:       0,
				minDrainDuration: 0,
			}),
		Entry("Successful drain with support for eviction of pods with shared volumes",
			&setup{
				stats: stats{
					nPodsWithoutPV:                0,
					nPodsWithOnlyExclusivePV:      0,
					nPodsWithOnlySharedPV:         2,
					nPodsWithExclusiveAndSharedPV: 0,
					nPVsPerPodWithExclusivePV:     1,
				},
				attemptEviction:        true,
				terminationGracePeriod: terminationGracePeriodShort,
			},
			[]podDrainHandler{sleepFor(terminationGracePeriodShortBy4), deletePod},
			&expectation{
				stats: stats{
					nPodsWithoutPV:                0,
					nPodsWithOnlyExclusivePV:      0,
					nPodsWithOnlySharedPV:         0,
					nPodsWithExclusiveAndSharedPV: 0,
				},
				timeout:          terminationGracePeriodShort,
				drainTimeout:     false,
				drainError:       nil,
				nEvictions:       2,
				minDrainDuration: 0,
			}),
		Entry("Successful drain without support for eviction of pods with exclusive and shared volumes",
			&setup{
				stats: stats{
					nPodsWithoutPV:                0,
					nPodsWithOnlyExclusivePV:      0,
					nPodsWithOnlySharedPV:         0,
					nPodsWithExclusiveAndSharedPV: 2,
					nPVsPerPodWithExclusivePV:     1,
				},
				attemptEviction:        false,
				terminationGracePeriod: terminationGracePeriodShort,
			},
			[]podDrainHandler{sleepFor(terminationGracePeriodShortBy8), detachExclusiveVolumes},
			&expectation{
				stats: stats{
					nPodsWithoutPV:                0,
					nPodsWithOnlyExclusivePV:      0,
					nPodsWithOnlySharedPV:         0,
					nPodsWithExclusiveAndSharedPV: 0,
				},
				// Because waitForDetach polling Interval is equal to terminationGracePeriodShort
				timeout:      terminationGracePeriodDefault,
				drainTimeout: false,
				drainError:   nil,
				nEvictions:   0,
				// Because waitForDetach polling Interval is equal to terminationGracePeriodShort
				minDrainDuration: terminationGracePeriodMedium,
			}),
		Entry("Successful drain with support for eviction of pods with exclusive and shared volumes",
			&setup{
				stats: stats{
					nPodsWithoutPV:                0,
					nPodsWithOnlyExclusivePV:      0,
					nPodsWithOnlySharedPV:         0,
					nPodsWithExclusiveAndSharedPV: 2,
					nPVsPerPodWithExclusivePV:     1,
				},
				attemptEviction:        true,
				terminationGracePeriod: terminationGracePeriodShort,
			},
			[]podDrainHandler{deletePod, sleepFor(terminationGracePeriodShortBy8), detachExclusiveVolumes},
			&expectation{
				stats: stats{
					nPodsWithoutPV:                0,
					nPodsWithOnlyExclusivePV:      0,
					nPodsWithOnlySharedPV:         0,
					nPodsWithExclusiveAndSharedPV: 0,
				},
				// Because waitForDetach polling Interval is equal to terminationGracePeriodShort
				timeout:      terminationGracePeriodDefault,
				drainTimeout: false,
				drainError:   nil,
				nEvictions:   2,
				// Because waitForDetach polling Interval is equal to terminationGracePeriodShort
				minDrainDuration: terminationGracePeriodMedium,
			}),
		Entry("Successful drain without support for eviction of pods with and without volume",
			&setup{
				stats: stats{
					nPodsWithoutPV:                10,
					nPodsWithOnlyExclusivePV:      2,
					nPodsWithOnlySharedPV:         0,
					nPodsWithExclusiveAndSharedPV: 0,
					nPVsPerPodWithExclusivePV:     1,
				},
				attemptEviction:        false,
				terminationGracePeriod: terminationGracePeriodShort,
			},
			[]podDrainHandler{sleepFor(terminationGracePeriodShortBy8), detachExclusiveVolumes},
			&expectation{
				stats: stats{
					nPodsWithoutPV:                0,
					nPodsWithOnlyExclusivePV:      0,
					nPodsWithOnlySharedPV:         0,
					nPodsWithExclusiveAndSharedPV: 0,
				},
				// Because waitForDetach polling Interval is equal to terminationGracePeriodShort
				timeout:      terminationGracePeriodDefault,
				drainTimeout: false,
				drainError:   nil,
				nEvictions:   0,
				// Because waitForDetach polling Interval is equal to terminationGracePeriodShort
				minDrainDuration: terminationGracePeriodMedium,
			}),
		Entry("Successful drain with support for eviction of pods with and without volume",
			&setup{
				stats: stats{
					nPodsWithoutPV:                10,
					nPodsWithOnlyExclusivePV:      2,
					nPodsWithOnlySharedPV:         0,
					nPodsWithExclusiveAndSharedPV: 0,
					nPVsPerPodWithExclusivePV:     1,
				},
				attemptEviction:        true,
				terminationGracePeriod: terminationGracePeriodShort,
			},
			[]podDrainHandler{deletePod, sleepFor(terminationGracePeriodShortBy8), detachExclusiveVolumes},
			&expectation{
				stats: stats{
					nPodsWithoutPV:                0,
					nPodsWithOnlyExclusivePV:      0,
					nPodsWithOnlySharedPV:         0,
					nPodsWithExclusiveAndSharedPV: 0,
				},
				// Because waitForDetach polling Interval is equal to terminationGracePeriodShort
				timeout:      terminationGracePeriodDefault,
				drainTimeout: false,
				drainError:   nil,
				nEvictions:   12,
				// Because waitForDetach polling Interval is equal to terminationGracePeriodShort
				minDrainDuration: terminationGracePeriodMedium,
			}),
		Entry("Successful forced drain without support for eviction of pods with and without volume",
			&setup{
				stats: stats{
					nPodsWithoutPV:                10,
					nPodsWithOnlyExclusivePV:      2,
					nPodsWithOnlySharedPV:         0,
					nPodsWithExclusiveAndSharedPV: 0,
					nPVsPerPodWithExclusivePV:     1,
				},
				attemptEviction:        false,
				terminationGracePeriod: terminationGracePeriodShort,
				force:                  true,
			},
			nil,
			&expectation{
				stats: stats{
					nPodsWithoutPV:                0,
					nPodsWithOnlyExclusivePV:      0,
					nPodsWithOnlySharedPV:         0,
					nPodsWithExclusiveAndSharedPV: 0,
				},
				timeout:          terminationGracePeriodShort,
				drainTimeout:     false,
				drainError:       nil,
				nEvictions:       0,
				minDrainDuration: 0,
			}),
		Entry("Successful forced drain with support for eviction of pods with and without volume",
			&setup{
				stats: stats{
					nPodsWithoutPV:                10,
					nPodsWithOnlyExclusivePV:      2,
					nPodsWithOnlySharedPV:         0,
					nPodsWithExclusiveAndSharedPV: 0,
					nPVsPerPodWithExclusivePV:     1,
				},
				attemptEviction:        true,
				terminationGracePeriod: terminationGracePeriodShort,
				force:                  true,
			},
			[]podDrainHandler{deletePod},
			&expectation{
				stats: stats{
					nPodsWithoutPV:                0,
					nPodsWithOnlyExclusivePV:      0,
					nPodsWithOnlySharedPV:         0,
					nPodsWithExclusiveAndSharedPV: 0,
				},
				timeout:          terminationGracePeriodShort,
				drainTimeout:     false,
				drainError:       nil,
				nEvictions:       0,
				minDrainDuration: 0,
			}),
		Entry("Successful forced drain with support for eviction of pods with and without volume when eviction fails",
			&setup{
				stats: stats{
					nPodsWithoutPV:                10,
					nPodsWithOnlyExclusivePV:      2,
					nPodsWithOnlySharedPV:         0,
					nPodsWithExclusiveAndSharedPV: 0,
					nPVsPerPodWithExclusivePV:     1,
				},
				maxEvictRetries:        1,
				attemptEviction:        true,
				terminationGracePeriod: terminationGracePeriodShort,
				force:                  true,
				evictError:             apierrors.NewTooManyRequestsError(""),
			},
			nil,
			&expectation{
				stats: stats{
					nPodsWithoutPV:                0,
					nPodsWithOnlyExclusivePV:      0,
					nPodsWithOnlySharedPV:         0,
					nPodsWithExclusiveAndSharedPV: 0,
				},
				timeout:          terminationGracePeriodMedium,
				drainTimeout:     false,
				drainError:       nil,
				nEvictions:       0,
				minDrainDuration: 0,
			}),
		Entry("Successful drain for pods with long termination grace period",
			&setup{
				stats: stats{
					nPodsWithoutPV:                10,
					nPodsWithOnlyExclusivePV:      2,
					nPodsWithOnlySharedPV:         0,
					nPodsWithExclusiveAndSharedPV: 0,
					nPVsPerPodWithExclusivePV:     1,
				},
				attemptEviction:        true,
				terminationGracePeriod: terminationGracePeriodLong,
			},
			[]podDrainHandler{deletePod, sleepFor(terminationGracePeriodShortBy8), detachExclusiveVolumes},
			&expectation{
				stats: stats{
					nPodsWithoutPV:                0,
					nPodsWithOnlyExclusivePV:      0,
					nPodsWithOnlySharedPV:         0,
					nPodsWithExclusiveAndSharedPV: 0,
				},
				// Because waitForDetach polling Interval is equal to terminationGracePeriodShort
				timeout:      terminationGracePeriodLong,
				drainTimeout: false,
				drainError:   nil,
				nEvictions:   12,
				// Because waitForDetach polling Interval is equal to terminationGracePeriodShort
				minDrainDuration: terminationGracePeriodMedium,
			}),
	)

	Describe("getPodVolumeInfos", func() {
		var (
			ctx context.Context

			drain *Options

			pvInformer, pvcInformer cache.SharedIndexInformer

			pods []*corev1.Pod
		)

		BeforeEach(func() {
			ctx = context.Background()

			kubeInformerFactory := coreinformers.NewSharedInformerFactory(nil, 0)

			pvInformer = kubeInformerFactory.Core().V1().PersistentVolumes().Informer()
			_ = pvInformer
			pvcInformer = kubeInformerFactory.Core().V1().PersistentVolumeClaims().Informer()
			_ = pvcInformer

			drain = &Options{
				ErrOut: GinkgoWriter,
				Out:    GinkgoWriter,

				Driver:    &drainDriver{},
				pvLister:  kubeInformerFactory.Core().V1().PersistentVolumes().Lister(),
				pvcLister: kubeInformerFactory.Core().V1().PersistentVolumeClaims().Lister(),
			}

			pods = []*corev1.Pod{}
		})

		It("should return empty map for empty pod list", func() {
			Expect(drain.getPodVolumeInfos(ctx, pods)).To(BeEmpty())
		})

		It("should return empty volume list for pods without volumes", func() {
			pods = append(pods, getPodsWithoutPV(2, "foo", "bar", "", terminationGracePeriodDefault, nil)...)

			podVolumeInfos := drain.getPodVolumeInfos(ctx, pods)
			Expect(podVolumeInfos).To(HaveLen(2))
			Expect(podVolumeInfos).To(And(
				HaveKeyWithValue("foo/bar0", matchPodPersistentVolumeNames(BeEmpty())),
				HaveKeyWithValue("foo/bar1", matchPodPersistentVolumeNames(BeEmpty())),
			))
		})

		It("should return list of exclusive volumes", func() {
			pods = append(pods, getPodWithPV("foo", "bar", "exclusive", "", "", terminationGracePeriodDefault, nil, 1))

			pvcs := getPVCs(pods)
			addAll(pvcInformer, pvcs...)
			addAll(pvInformer, appendSuffixToVolumeHandles(getPVs(pvcs), "-id")...)

			podVolumeInfos := drain.getPodVolumeInfos(ctx, pods)
			Expect(podVolumeInfos).To(HaveLen(1))
			Expect(podVolumeInfos).To(HaveKeyWithValue(
				"foo/bar", And(
					matchPodPersistentVolumeNames(ConsistOf("exclusive-0")),
					matchPodVolumeIDs(ConsistOf("exclusive-0-id")),
				),
			))
		})

		It("should filter out shared volumes", func() {
			pods = append(pods, getPodsWithPV(2, 2, 1, 1, "foo", "bar", "exclusive", "shared", "", terminationGracePeriodDefault, nil)...)

			pvcs := getPVCs(pods)
			addAll(pvcInformer, pvcs...)
			addAll(pvInformer, getPVs(pvcs)...)

			podVolumeInfos := drain.getPodVolumeInfos(ctx, pods)
			Expect(podVolumeInfos).To(HaveLen(2))
			Expect(podVolumeInfos).To(And(
				HaveKeyWithValue(
					"foo/bar0", matchPodPersistentVolumeNames(ConsistOf("exclusive0-0")),
				),
				HaveKeyWithValue(
					"foo/bar1", matchPodPersistentVolumeNames(ConsistOf("exclusive1-0")),
				),
			))
		})

		It("should filter out provider-unrelated volumes", func() {
			pods = append(pods, getPodWithPV("foo", "bar", "exclusive", "", "", terminationGracePeriodDefault, nil, 1))

			pvcs := getPVCs(pods)
			addAll(pvcInformer, pvcs...)

			pvs := getPVs(pvcs)
			pvs[0].Spec.CSI = nil
			pvs[0].Spec.NFS = &corev1.NFSVolumeSource{
				Server: "my-nfs-server.example.com",
				Path:   "/my-share",
			}
			addAll(pvInformer, pvs...)

			podVolumeInfos := drain.getPodVolumeInfos(ctx, pods)
			Expect(podVolumeInfos).To(HaveLen(1))
			Expect(podVolumeInfos).To(HaveKeyWithValue(
				"foo/bar", matchPodPersistentVolumeNames(BeEmpty()),
			))
		})

		It("should filter out non-existing PVCs", func() {
			pods = append(pods, getPodWithPV("foo", "bar", "exclusive", "", "", terminationGracePeriodDefault, nil, 1))

			podVolumeInfos := drain.getPodVolumeInfos(ctx, pods)
			Expect(podVolumeInfos).To(HaveLen(1))
			Expect(podVolumeInfos).To(HaveKeyWithValue(
				"foo/bar", matchPodPersistentVolumeNames(BeEmpty()),
			))
		})

		It("should filter out non-existing persistent volumes", func() {
			pods = append(pods, getPodWithPV("foo", "bar", "exclusive", "", "", terminationGracePeriodDefault, nil, 1))

			addAll(pvcInformer, getPVCs(pods)...)

			podVolumeInfos := drain.getPodVolumeInfos(ctx, pods)
			Expect(podVolumeInfos).To(HaveLen(1))
			Expect(podVolumeInfos).To(HaveKeyWithValue(
				"foo/bar", matchPodPersistentVolumeNames(BeEmpty()),
			))
		})
	})
})

func getPodWithoutPV(ns, name, nodeName string, terminationGracePeriod time.Duration, labels map[string]string) *corev1.Pod {
	controller := true
	priority := int32(0)
	tgps := int64(terminationGracePeriod / time.Second)
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				{Controller: &controller},
			},
		},
		Spec: corev1.PodSpec{
			NodeName:                      nodeName,
			TerminationGracePeriodSeconds: &tgps,
			Priority:                      &priority,
		},
	}
}

func getPodsWithoutPV(n int, ns, podPrefix, nodeName string, terminationGracePeriod time.Duration, labels map[string]string) []*corev1.Pod {
	pods := make([]*corev1.Pod, n)
	for i := range pods {
		pods[i] = getPodWithoutPV(ns, fmt.Sprintf("%s%d", podPrefix, i), nodeName, terminationGracePeriod, labels)
	}
	return pods
}

func getPodWithPV(ns, name, exclusivePV, sharedPV, nodeName string, terminationGracePeriod time.Duration, labels map[string]string, numberOfExclusivePVs int) *corev1.Pod {
	pod := getPodWithoutPV(ns, name, nodeName, terminationGracePeriod, labels)

	appendVolume := func(pod *corev1.Pod, vol string) {
		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: vol,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: vol,
				},
			},
		})
	}

	if exclusivePV != "" {
		for i := 0; i < numberOfExclusivePVs; i++ {
			appendVolume(pod, exclusivePV+"-"+strconv.Itoa(i))
		}
	}
	if sharedPV != "" {
		appendVolume(pod, sharedPV)
	}
	return pod
}

func getPodsWithPV(nPod, nExclusivePV, nSharedPV, numberOfExclusivePVs int, ns, podPrefix, exclusivePVPrefix, sharedPVPrefix, nodeName string, terminationGracePeriod time.Duration, labels map[string]string) []*corev1.Pod {
	pods := make([]*corev1.Pod, nPod)
	for i := range pods {
		var (
			podName     = fmt.Sprintf("%s%d", podPrefix, i)
			exclusivePV string
			sharedPV    string
		)
		if nExclusivePV > 0 {
			exclusivePV = fmt.Sprintf("%s%d", exclusivePVPrefix, i%nExclusivePV)
		}
		if nSharedPV > 0 {
			sharedPV = fmt.Sprintf("%s%d", sharedPVPrefix, i%nSharedPV)
		}
		pods[i] = getPodWithPV(ns, podName, exclusivePV, sharedPV, nodeName, terminationGracePeriod, labels, numberOfExclusivePVs)
	}
	return pods
}

func getPVCs(pods []*corev1.Pod) []*corev1.PersistentVolumeClaim {
	m := make(map[string]*corev1.PersistentVolumeClaim)
	for _, pod := range pods {
		for i := range pod.Spec.Volumes {
			vol := &pod.Spec.Volumes[i]
			if vol.PersistentVolumeClaim != nil {
				pvc := vol.PersistentVolumeClaim

				if _, ok := m[pvc.ClaimName]; ok {
					continue
				}

				m[pvc.ClaimName] = &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      pvc.ClaimName,
						Namespace: pod.Namespace,
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						VolumeName: pvc.ClaimName,
					},
				}
			}
		}
	}

	pvcs := make([]*corev1.PersistentVolumeClaim, len(m))
	var i = 0
	for _, pvc := range m {
		pvcs[i] = pvc
		i++
	}
	return pvcs
}

func getPVs(pvcs []*corev1.PersistentVolumeClaim) []*corev1.PersistentVolume {
	m := make(map[string]*corev1.PersistentVolume)
	for _, pvc := range pvcs {
		if _, ok := m[pvc.Spec.VolumeName]; ok {
			continue
		}

		m[pvc.Spec.VolumeName] = &corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: pvc.Spec.VolumeName,
			},
			Spec: corev1.PersistentVolumeSpec{
				PersistentVolumeSource: corev1.PersistentVolumeSource{
					CSI: &corev1.CSIPersistentVolumeSource{
						VolumeHandle: pvc.Spec.VolumeName,
					},
				},
			},
		}
	}

	pvs := make([]*corev1.PersistentVolume, len(m))
	var i = 0
	for _, pv := range m {
		pvs[i] = pv
		i++
	}
	return pvs
}

func getVolumeAttachments(pvs []*corev1.PersistentVolume, nodeName string) []*storagev1.VolumeAttachment {
	volumeAttachments := make([]*storagev1.VolumeAttachment, 0)

	for _, pv := range pvs {
		pvName := pv.Name

		volumeAttachment := &storagev1.VolumeAttachment{
			ObjectMeta: metav1.ObjectMeta{
				// TODO: Get random value
				Name: "csi-old-" + pv.Name,
			},
			Spec: storagev1.VolumeAttachmentSpec{
				Attacher: "disk.csi.azure.com",
				Source: storagev1.VolumeAttachmentSource{
					PersistentVolumeName: &pvName,
				},
				NodeName: nodeName,
			},
			Status: storagev1.VolumeAttachmentStatus{
				Attached: true,
			},
		}

		volumeAttachments = append(volumeAttachments, volumeAttachment)
	}

	return volumeAttachments
}

func getNode(name string, pvs []*corev1.PersistentVolume) *corev1.Node {
	n := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	vols := make([]corev1.AttachedVolume, len(pvs))
	for i, pv := range pvs {
		vols[i] = corev1.AttachedVolume{
			Name: corev1.UniqueVolumeName(getDrainTestVolumeName(&pv.Spec)),
		}
	}

	n.Status.VolumesAttached = vols

	return n
}

func getDrainTestVolumeName(pvSpec *corev1.PersistentVolumeSpec) string {
	if pvSpec.CSI == nil {
		return ""
	}
	return pvSpec.CSI.VolumeHandle
}

type drainDriver struct {
	*driver.FakeDriver
}

func (d *drainDriver) GetVolumeIDs(_ context.Context, req *driver.GetVolumeIDsRequest) (*driver.GetVolumeIDsResponse, error) {
	volNames := make([]string, 0, len(req.PVSpecs))

	for _, spec := range req.PVSpecs {
		// real drivers filter volumes in GetVolumeIDs and only return IDs of provider-related volumes
		if volumeName := getDrainTestVolumeName(spec); volumeName != "" {
			volNames = append(volNames, volumeName)
		}
	}

	return &driver.GetVolumeIDsResponse{
		VolumeIDs: volNames,
	}, nil
}

func appendPods(objects []runtime.Object, pods []*corev1.Pod) []runtime.Object {
	for _, pod := range pods {
		objects = append(objects, pod)
	}
	return objects
}

func appendPVCs(objects []runtime.Object, pvcs []*corev1.PersistentVolumeClaim) []runtime.Object {
	for _, pvc := range pvcs {
		objects = append(objects, pvc)
	}
	return objects
}

func appendPVs(objects []runtime.Object, pvs []*corev1.PersistentVolume) []runtime.Object {
	for _, pv := range pvs {
		objects = append(objects, pv)
	}
	return objects
}

func appendNodes(objects []runtime.Object, nodes []*corev1.Node) []runtime.Object {
	for _, n := range nodes {
		objects = append(objects, n)
	}
	return objects
}

func appendVolumeAttachments(objects []runtime.Object, volumeAttachments []*storagev1.VolumeAttachment) []runtime.Object {
	for _, va := range volumeAttachments {
		objects = append(objects, va)
	}
	return objects
}

func updateVolumeAttachments(drainOptions *Options, pvName string, nodeName string) {
	var (
		found            bool
		volumeAttachment storagev1.VolumeAttachment
	)
	const reattachmentDelay = 5 * time.Second
	time.Sleep(reattachmentDelay)

	// Delete existing volume attachment
	volumeAttachments, err := drainOptions.client.StorageV1().VolumeAttachments().List(context.TODO(), metav1.ListOptions{})
	Expect(err).To(BeNil())

	for _, volumeAttachment = range volumeAttachments.Items {
		if *volumeAttachment.Spec.Source.PersistentVolumeName == pvName {
			found = true
			break
		}
	}

	Expect(found).To(BeTrue())
	err = drainOptions.client.StorageV1().VolumeAttachments().Delete(context.TODO(), volumeAttachment.Name, metav1.DeleteOptions{})
	Expect(err).To(BeNil())

	// Create new volumeAttachment object
	newVolumeAttachment := &storagev1.VolumeAttachment{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: "csi-new-" + pvName,
		},
		Spec: storagev1.VolumeAttachmentSpec{
			Attacher: "disk.csi.azure.com",
			Source: storagev1.VolumeAttachmentSource{
				PersistentVolumeName: &pvName,
			},
			NodeName: nodeName,
		},
		Status: storagev1.VolumeAttachmentStatus{
			Attached: true,
		},
	}

	newVolumeAttachment, err = drainOptions.client.StorageV1().VolumeAttachments().Create(context.TODO(), newVolumeAttachment, metav1.CreateOptions{})
	Expect(err).To(BeNil())

	newVolumeAttachment, err = drainOptions.client.StorageV1().VolumeAttachments().UpdateStatus(context.TODO(), newVolumeAttachment, metav1.UpdateOptions{})
	Expect(err).To(BeNil())

	drainOptions.volumeAttachmentHandler.AddVolumeAttachment(newVolumeAttachment)
}

// matchPodPersistentVolumeNames applies the given matcher to the result of PodVolumeInfo.PersistentVolumeNames().
func matchPodPersistentVolumeNames(matcher gomegatypes.GomegaMatcher) gomegatypes.GomegaMatcher {
	return gcustom.MakeMatcher(func(actual PodVolumeInfo) (bool, error) {
		return matcher.Match(actual.PersistentVolumeNames())
	})
}

// matchPodVolumeIDs applies the given matcher to the result of PodVolumeInfo.VolumeIDs().
func matchPodVolumeIDs(matcher gomegatypes.GomegaMatcher) gomegatypes.GomegaMatcher {
	return gcustom.MakeMatcher(func(actual PodVolumeInfo) (bool, error) {
		return matcher.Match(actual.VolumeIDs())
	})
}

func addAll[T runtime.Object](informer cache.SharedIndexInformer, objects ...T) {
	GinkgoHelper()

	for _, object := range objects {
		Expect(informer.GetStore().Add(object)).NotTo(HaveOccurred())
	}
}

func appendSuffixToVolumeHandles(pvs []*corev1.PersistentVolume, suffix string) []*corev1.PersistentVolume {
	for _, pv := range pvs {
		pv.Spec.CSI.VolumeHandle += suffix
	}
	return pvs
}
