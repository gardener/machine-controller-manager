// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

/**
	Overview
		- Tests Shoot cluster autoscaling

	AfterSuite
		- Cleanup Workload in Shoot

	Test:
		1. Create a Deployment with affinity that does not allow Pods to be co-located in the same Node
		2. Scale up the Deployment and see one Node added (because of the Pod affinity)
		3. Scale down the Deployment and see one Node removed (after spec.kubernetes.clusterAutoscaler.scaleDownUnneededTime|scaleDownDelayAfterAdd)
	Expected Output
		- Scale-up/down should work properly
 **/

package operations

import (
	"context"
	"time"

	corev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/test/framework"
	"github.com/gardener/gardener/test/framework/resources/templates"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	podAntiAffinityDeploymentName      = "pod-anti-affinity"
	podAntiAffinityDeploymentNamespace = metav1.NamespaceDefault

	scaleDownDelayAfterAdd = 1 * time.Minute
	scaleDownUnneededTime  = 1 * time.Minute
	testTimeout            = 60 * time.Minute
	scaleUpTimeout         = 20 * time.Minute
	scaleDownTimeout       = 20 * time.Minute
	cleanupTimeout         = 20 * time.Minute
)

var _ = ginkgo.Describe("Shoot clusterautoscaler testing", func() {

	var (
		f = framework.NewShootFramework(nil)

		testWorkerPoolName          = "ca-test"
		origClusterAutoscalerConfig *corev1beta1.ClusterAutoscaler
		origWorkers                 []corev1beta1.Worker
		origMinWorkers              int32
		origMaxWorkers              int32
	)

	f.Beta().Serial().CIt("should autoscale a single worker group", func(ctx context.Context) {
		var (
			shoot = f.Shoot

			workerPoolName = shoot.Spec.Provider.Workers[0].Name
		)

		origClusterAutoscalerConfig = shoot.Spec.Kubernetes.ClusterAutoscaler.DeepCopy()
		origMinWorkers = shoot.Spec.Provider.Workers[0].Minimum
		origMaxWorkers = shoot.Spec.Provider.Workers[0].Maximum

		ginkgo.By("updating shoot spec for test")
		// set clusterautoscaler params to lower values so we don't have to wait too long
		// and ensure the worker pool has maximum > minimum
		err := f.UpdateShoot(ctx, func(s *corev1beta1.Shoot) error {
			if s.Spec.Kubernetes.ClusterAutoscaler == nil {
				s.Spec.Kubernetes.ClusterAutoscaler = &corev1beta1.ClusterAutoscaler{}
			}
			s.Spec.Kubernetes.ClusterAutoscaler.ScaleDownDelayAfterAdd = &metav1.Duration{Duration: scaleDownDelayAfterAdd}
			s.Spec.Kubernetes.ClusterAutoscaler.ScaleDownUnneededTime = &metav1.Duration{Duration: scaleDownUnneededTime}

			if origMaxWorkers != origMinWorkers+1 {
				s.Spec.Provider.Workers[0].Maximum = origMinWorkers + 1
			}

			return nil
		})
		framework.ExpectNoError(err)

		ginkgo.By("creating pod-anti-affinity deployment")
		values := podAntiAffinityValues{
			Name:       podAntiAffinityDeploymentName,
			Namespace:  podAntiAffinityDeploymentNamespace,
			Replicas:   origMinWorkers,
			WorkerPool: workerPoolName,
		}
		err = f.RenderAndDeployTemplate(ctx, f.ShootClient, templates.PodAntiAffinityDeploymentName, values)
		framework.ExpectNoError(err)

		err = f.WaitUntilDeploymentIsReady(ctx, values.Name, values.Namespace, f.ShootClient)
		framework.ExpectNoError(err)

		ginkgo.By("scaling up pod-anti-affinity deployment")
		err = kubernetes.ScaleDeployment(ctx, f.ShootClient.DirectClient(), client.ObjectKey{Namespace: values.Namespace, Name: values.Name}, origMinWorkers+1)
		framework.ExpectNoError(err)

		ginkgo.By("one node should be added to the worker pool")
		err = framework.WaitForNNodesToBeHealthyInWorkerPool(ctx, f.ShootClient, int(origMinWorkers+1), &workerPoolName, scaleUpTimeout)
		framework.ExpectNoError(err)

		ginkgo.By("pod-anti-affinity deployment should get healthy again")
		err = f.WaitUntilDeploymentIsReady(ctx, values.Name, values.Namespace, f.ShootClient)
		framework.ExpectNoError(err)

		ginkgo.By("scaling down pod-anti-affinity deployment")
		err = kubernetes.ScaleDeployment(ctx, f.ShootClient.DirectClient(), client.ObjectKey{Namespace: values.Namespace, Name: values.Name}, origMinWorkers)
		framework.ExpectNoError(err)

		ginkgo.By("one node should be removed from the worker pool")
		err = framework.WaitForNNodesToBeHealthyInWorkerPool(ctx, f.ShootClient, int(origMinWorkers), &workerPoolName, scaleDownTimeout)
		framework.ExpectNoError(err)
	}, testTimeout, framework.WithCAfterTest(func(ctx context.Context) {

		ginkgo.By("reverting shoot spec changes by test")
		err := f.UpdateShoot(ctx, func(s *corev1beta1.Shoot) error {
			s.Spec.Kubernetes.ClusterAutoscaler = origClusterAutoscalerConfig
			s.Spec.Provider.Workers[0].Maximum = origMaxWorkers

			return nil
		})
		framework.ExpectNoError(err)

		ginkgo.By("deleting pod-anti-affinity deployment")
		err = kutil.DeleteObject(ctx, f.ShootClient.Client(), &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podAntiAffinityDeploymentName,
				Namespace: podAntiAffinityDeploymentNamespace,
			},
		})
		framework.ExpectNoError(err)
	}, cleanupTimeout))

	f.Beta().Serial().CIt("should autoscale a single worker group to/from zero", func(ctx context.Context) {
		var shoot = f.Shoot

		origClusterAutoscalerConfig = shoot.Spec.Kubernetes.ClusterAutoscaler.DeepCopy()
		origWorkers = shoot.Spec.Provider.Workers

		if shoot.Spec.Provider.Type != "aws" && shoot.Spec.Provider.Type != "azure" {
			ginkgo.Skip("not applicable")
		}

		// Create a dedicated worker-pool for cluster autoscaler.
		testWorkerPool := origWorkers[0]
		testWorkerPool.Name = testWorkerPoolName
		testWorkerPool.Minimum = 0
		testWorkerPool.Maximum = 2
		testWorkerPool.Taints = []corev1.Taint{
			{
				Key:    testWorkerPoolName,
				Effect: corev1.TaintEffectNoSchedule,
				Value:  testWorkerPoolName,
			},
		}

		ginkgo.By("updating shoot spec for test")
		err := f.UpdateShoot(ctx, func(s *corev1beta1.Shoot) error {
			s.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, testWorkerPool)

			if s.Spec.Kubernetes.ClusterAutoscaler == nil {
				s.Spec.Kubernetes.ClusterAutoscaler = &corev1beta1.ClusterAutoscaler{}
			}
			s.Spec.Kubernetes.ClusterAutoscaler.ScaleDownDelayAfterAdd = &metav1.Duration{Duration: scaleDownDelayAfterAdd}
			s.Spec.Kubernetes.ClusterAutoscaler.ScaleDownUnneededTime = &metav1.Duration{Duration: scaleDownUnneededTime}

			return nil
		})
		framework.ExpectNoError(err)

		nodeList, err := framework.GetAllNodesInWorkerPool(ctx, f.ShootClient, &testWorkerPoolName)
		framework.ExpectNoError(err)

		nodeCount := len(nodeList.Items)
		gomega.Expect(nodeCount).To(gomega.BeEquivalentTo(testWorkerPool.Minimum), "shoot should have minimum node count before the test")

		ginkgo.By("creating pod-anti-affinity deployment")
		values := podAntiAffinityValues{
			Name:          podAntiAffinityDeploymentName,
			Namespace:     podAntiAffinityDeploymentNamespace,
			Replicas:      0, // This is to test the scale-from-zero.
			WorkerPool:    testWorkerPoolName,
			TolerationKey: testWorkerPoolName,
		}
		err = f.RenderAndDeployTemplate(ctx, f.ShootClient, templates.PodAntiAffinityDeploymentName, values)
		framework.ExpectNoError(err)

		err = f.WaitUntilDeploymentIsReady(ctx, values.Name, values.Namespace, f.ShootClient)
		framework.ExpectNoError(err)

		ginkgo.By("scaling up pod-anti-affinity deployment")
		err = kubernetes.ScaleDeployment(ctx, f.ShootClient.DirectClient(), client.ObjectKey{Namespace: values.Namespace, Name: values.Name}, 1)
		framework.ExpectNoError(err)

		ginkgo.By("one node should be added to the worker pool")
		err = framework.WaitForNNodesToBeHealthyInWorkerPool(ctx, f.ShootClient, 1, &testWorkerPoolName, scaleUpTimeout)
		framework.ExpectNoError(err)

		ginkgo.By("pod-anti-affinity deployment should get healthy again")
		err = f.WaitUntilDeploymentIsReady(ctx, values.Name, values.Namespace, f.ShootClient)
		framework.ExpectNoError(err)

		ginkgo.By("scaling down pod-anti-affinity deployment")
		err = kubernetes.ScaleDeployment(ctx, f.ShootClient.DirectClient(), client.ObjectKey{Namespace: values.Namespace, Name: values.Name}, 0)
		framework.ExpectNoError(err)

		ginkgo.By("worker pool should be scaled-down to 0")
		err = framework.WaitForNNodesToBeHealthyInWorkerPool(ctx, f.ShootClient, 0, &testWorkerPoolName, scaleDownTimeout)
		framework.ExpectNoError(err)
	}, testTimeout, framework.WithCAfterTest(func(ctx context.Context) {
		ginkgo.By("reverting shoot spec changes by test")
		err := f.UpdateShoot(ctx, func(s *corev1beta1.Shoot) error {
			s.Spec.Kubernetes.ClusterAutoscaler = origClusterAutoscalerConfig

			for i, worker := range s.Spec.Provider.Workers {
				if worker.Name == testWorkerPoolName {
					// Remove the dedicated ca-test workerpool
					s.Spec.Provider.Workers[i] = s.Spec.Provider.Workers[len(s.Spec.Provider.Workers)-1]
					s.Spec.Provider.Workers = s.Spec.Provider.Workers[:len(s.Spec.Provider.Workers)-1]
					break
				}
			}

			return nil
		})
		framework.ExpectNoError(err)

		ginkgo.By("deleting pod-anti-affinity deployment")
		err = kutil.DeleteObject(ctx, f.ShootClient.Client(), &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podAntiAffinityDeploymentName,
				Namespace: podAntiAffinityDeploymentNamespace,
			},
		})
		framework.ExpectNoError(err)
	}, cleanupTimeout))

})

type podAntiAffinityValues struct {
	Name          string
	Namespace     string
	Replicas      int32
	WorkerPool    string
	TolerationKey string
}
