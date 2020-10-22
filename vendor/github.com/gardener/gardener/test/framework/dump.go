// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package framework

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"sort"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"

	"github.com/hashicorp/go-multierror"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	healthy   = "healthy"
	unhealthy = "unhealthy"
)

// DumpDefaultResourcesInAllNamespaces dumps all default k8s resources of a namespace
func (f *CommonFramework) DumpDefaultResourcesInAllNamespaces(ctx context.Context, ctxIdentifier string, k8sClient kubernetes.Interface) error {
	namespaces := &corev1.NamespaceList{}
	if err := k8sClient.DirectClient().List(ctx, namespaces); err != nil {
		return err
	}

	var result error

	for _, ns := range namespaces.Items {
		if err := f.DumpDefaultResourcesInNamespace(ctx, ctxIdentifier, k8sClient, ns.Name); err != nil {
			result = multierror.Append(result, err)
		}
	}
	return result
}

// DumpDefaultResourcesInNamespace dumps all default K8s resources of a namespace.
func (f *CommonFramework) DumpDefaultResourcesInNamespace(ctx context.Context, ctxIdentifier string, k8sClient kubernetes.Interface, namespace string) error {
	var result error
	if err := f.dumpEventsInNamespace(ctx, ctxIdentifier, k8sClient, namespace); err != nil {
		result = multierror.Append(result, fmt.Errorf("unable to fetch Events from namespace %s: %s", namespace, err.Error()))
	}
	if err := f.dumpPodInfoForNamespace(ctx, ctxIdentifier, k8sClient, namespace); err != nil {
		result = multierror.Append(result, fmt.Errorf("unable to fetch information of Pods from namespace %s: %s", namespace, err.Error()))
	}
	if err := f.dumpDeploymentInfoForNamespace(ctx, ctxIdentifier, k8sClient, namespace); err != nil {
		result = multierror.Append(result, fmt.Errorf("unable to fetch information of Deployments from namespace %s: %s", namespace, err.Error()))
	}
	if err := f.dumpStatefulSetInfoForNamespace(ctx, ctxIdentifier, k8sClient, namespace); err != nil {
		result = multierror.Append(result, fmt.Errorf("unable to fetch information of StatefulSets from namespace %s: %s", namespace, err.Error()))
	}
	if err := f.dumpDaemonSetInfoForNamespace(ctx, ctxIdentifier, k8sClient, namespace); err != nil {
		result = multierror.Append(result, fmt.Errorf("unable to fetch information of DaemonSets from namespace %s: %s", namespace, err.Error()))
	}
	if err := f.dumpServiceInfoForNamespace(ctx, ctxIdentifier, k8sClient, namespace); err != nil {
		result = multierror.Append(result, fmt.Errorf("unable to fetch information of Services from namespace %s: %s", namespace, err.Error()))
	}
	if err := f.dumpVolumeInfoForNamespace(ctx, ctxIdentifier, k8sClient, namespace); err != nil {
		result = multierror.Append(result, fmt.Errorf("unable to fetch information of Volumes from namespace %s: %s", namespace, err.Error()))
	}
	return result
}

func (f *GardenerFramework) dumpControlplaneInSeed(ctx context.Context, seed *gardencorev1beta1.Seed, namespace string) error {
	ctxIdentifier := fmt.Sprintf("[SEED %s]", seed.GetName())
	f.Logger.Info(ctxIdentifier)

	_, seedClient, err := f.GetSeed(ctx, seed.GetName())
	if err != nil {
		return err
	}

	var result error
	if err := f.dumpGardenerExtensionsInNamespace(ctx, ctxIdentifier, seedClient, namespace); err != nil {
		result = multierror.Append(result, fmt.Errorf("unable to dump Extensions from namespace %s in seed %s: %s", namespace, seed.Name, err.Error()))
	}
	if err := f.dumpEventsInNamespace(ctx, ctxIdentifier, seedClient, namespace); err != nil {
		result = multierror.Append(result, fmt.Errorf("unable to dump Events from namespace %s in seed %s: %s", namespace, seed.Name, err.Error()))
	}
	if err := f.dumpPodInfoForNamespace(ctx, ctxIdentifier, seedClient, namespace); err != nil {
		result = multierror.Append(result, fmt.Errorf("unable to dump information of Pods from namespace %s in seed %s: %s", namespace, seed.Name, err.Error()))
	}
	if err := f.dumpDeploymentInfoForNamespace(ctx, ctxIdentifier, seedClient, namespace); err != nil {
		result = multierror.Append(result, fmt.Errorf("unable to dump information of Deployments from namespace %s in seed %s: %s", namespace, seed.Name, err.Error()))
	}
	if err := f.dumpStatefulSetInfoForNamespace(ctx, ctxIdentifier, seedClient, namespace); err != nil {
		result = multierror.Append(result, fmt.Errorf("unable to dump information of StatefulSets from namespace %s in seed %s: %s", namespace, seed.Name, err.Error()))
	}
	if err := f.dumpDaemonSetInfoForNamespace(ctx, ctxIdentifier, seedClient, namespace); err != nil {
		result = multierror.Append(result, fmt.Errorf("unable to dump information of DaemonSets from namespace %s in seed %s: %s", namespace, seed.Name, err.Error()))
	}
	if err := f.dumpServiceInfoForNamespace(ctx, ctxIdentifier, seedClient, namespace); err != nil {
		result = multierror.Append(result, fmt.Errorf("unable to dump information of Services from namespace %s in seed %s: %s", namespace, seed.Name, err.Error()))
	}
	if err := f.dumpVolumeInfoForNamespace(ctx, ctxIdentifier, seedClient, namespace); err != nil {
		result = multierror.Append(result, fmt.Errorf("unable to fetch information of Volumes from namespace %s: %s", namespace, err.Error()))
	}

	return result
}

// dumpGardenerExtensionsInNamespace prints all gardener extension crds in the shoot namespace
func (f *GardenerFramework) dumpGardenerExtensionsInNamespace(ctx context.Context, ctxIdentifier string, k8sClient kubernetes.Interface, namespace string) error {
	var result *multierror.Error
	ctxIdentifier = fmt.Sprintf("%s [NAMESPACE %s]", ctxIdentifier, namespace)

	f.Logger.Infof("%s [EXTENSIONS] [INFRASTRUCTURE]", ctxIdentifier)
	infrastructures := &v1alpha1.InfrastructureList{}
	err := k8sClient.DirectClient().List(ctx, infrastructures, client.InNamespace(namespace))
	result = multierror.Append(result, err)
	if err != nil {
		for _, infra := range infrastructures.Items {
			f.dumpGardenerExtension(&infra)
		}
		f.Logger.Println()
	}

	f.Logger.Infof("%s [EXTENSIONS] [CONTROLPLANE]", ctxIdentifier)
	controlplanes := &v1alpha1.ControlPlaneList{}
	err = k8sClient.DirectClient().List(ctx, controlplanes, client.InNamespace(namespace))
	if err != nil {
		for _, cp := range controlplanes.Items {
			f.dumpGardenerExtension(&cp)
		}
		f.Logger.Println()
	}

	f.Logger.Infof("%s [EXTENSIONS] [OS]", ctxIdentifier)
	operatingSystems := &v1alpha1.OperatingSystemConfigList{}
	err = k8sClient.DirectClient().List(ctx, operatingSystems, client.InNamespace(namespace))
	result = multierror.Append(result, err)
	if err == nil {
		for _, os := range operatingSystems.Items {
			f.dumpGardenerExtension(&os)
		}
		f.Logger.Println()
	}

	f.Logger.Infof("%s [EXTENSIONS] [WORKER]", ctxIdentifier)
	workers := &v1alpha1.WorkerList{}
	err = k8sClient.DirectClient().List(ctx, workers, client.InNamespace(namespace))
	result = multierror.Append(result, err)
	if err == nil {
		for _, worker := range workers.Items {
			f.dumpGardenerExtension(&worker)
		}
		f.Logger.Println()
	}

	f.Logger.Infof("%s [EXTENSIONS] [BACKUPBUCKET]", ctxIdentifier)
	backupBuckets := &v1alpha1.BackupBucketList{}
	err = k8sClient.DirectClient().List(ctx, backupBuckets, client.InNamespace(namespace))
	result = multierror.Append(result, err)
	if err == nil {
		for _, bucket := range backupBuckets.Items {
			f.dumpGardenerExtension(&bucket)
		}
		f.Logger.Println()
	}

	f.Logger.Infof("%s [EXTENSIONS] [BACKUPENTRY]", ctxIdentifier)
	backupEntries := &v1alpha1.BackupEntryList{}
	err = k8sClient.DirectClient().List(ctx, backupEntries, client.InNamespace(namespace))
	result = multierror.Append(result, err)
	if err == nil {
		for _, entry := range backupEntries.Items {
			f.dumpGardenerExtension(&entry)
		}
		f.Logger.Println()
	}

	f.Logger.Infof("%s [EXTENSIONS] [NETWORK]", ctxIdentifier)
	networks := &v1alpha1.NetworkList{}
	err = k8sClient.DirectClient().List(ctx, networks, client.InNamespace(namespace))
	result = multierror.Append(result, err)
	if err == nil {
		for _, network := range networks.Items {
			f.dumpGardenerExtension(&network)
		}
		f.Logger.Println()
	}

	return result.ErrorOrNil()
}

// dumpGardenerExtensions prints all gardener extension crds in the shoot namespace
func (f *GardenerFramework) dumpGardenerExtension(extension v1alpha1.Object) {
	if err := health.CheckExtensionObject(extension); err != nil {
		f.Logger.Printf("%s of type %s is %s - Error: %s", extension.GetName(), extension.GetExtensionSpec().GetExtensionType(), unhealthy, err.Error())
	} else {
		f.Logger.Printf("%s of type %s is %s", extension.GetName(), extension.GetExtensionSpec().GetExtensionType(), healthy)
	}
	f.Logger.Printf("At %v - last operation %s %s: %s", extension.GetExtensionStatus().GetLastOperation().LastUpdateTime, extension.GetExtensionStatus().GetLastOperation().Type, extension.GetExtensionStatus().GetLastOperation().State, extension.GetExtensionStatus().GetLastOperation().Description)
	if extension.GetExtensionStatus().GetLastError() != nil {
		f.Logger.Printf("At %v - last error: %s", extension.GetExtensionStatus().GetLastError().LastUpdateTime, extension.GetExtensionStatus().GetLastError().Description)
	}
}

func (f *CommonFramework) DumpLogsForPodsWithLabelsInNamespace(ctx context.Context, ctxIdentifier string, k8sClient kubernetes.Interface, namespace string, opts ...client.ListOption) error {
	pods := &corev1.PodList{}
	opts = append(opts, client.InNamespace(namespace))
	if err := k8sClient.DirectClient().List(ctx, pods, opts...); err != nil {
		return err
	}

	var result error
	for _, pod := range pods.Items {
		if err := f.DumpLogsForPodInNamespace(ctx, ctxIdentifier, k8sClient, namespace, pod.Name); err != nil {
			result = multierror.Append(result, err)
		}
	}
	return result
}

func (f *CommonFramework) DumpLogsForPodInNamespace(ctx context.Context, ctxIdentifier string, k8sClient kubernetes.Interface, podNamespace, podName string) error {
	f.Logger.Infof("%s [NAMESPACE %s] [POD %s] [LOGS]", ctxIdentifier, podNamespace, podName)
	podIf := k8sClient.Kubernetes().CoreV1().Pods(podNamespace)
	logs, err := kubernetes.GetPodLogs(ctx, podIf, podName, &corev1.PodLogOptions{})
	if err != nil {
		return err
	}
	scanner := bufio.NewScanner(bytes.NewReader(logs))
	for scanner.Scan() {
		f.Logger.Println(scanner.Text())
	}

	return nil
}

// dumpDeploymentInfoForNamespace prints information about all Deployments of a namespace
func (f *CommonFramework) dumpDeploymentInfoForNamespace(ctx context.Context, ctxIdentifier string, k8sClient kubernetes.Interface, namespace string) error {
	f.Logger.Infof("%s [NAMESPACE %s] [DEPLOYMENTS]", ctxIdentifier, namespace)
	deployments := &appsv1.DeploymentList{}
	if err := k8sClient.DirectClient().List(ctx, deployments, client.InNamespace(namespace)); err != nil {
		return err
	}
	for _, deployment := range deployments.Items {
		if err := health.CheckDeployment(&deployment); err != nil {
			f.Logger.Printf("Deployment %s is %s with %d/%d replicas - Error: %s - Conditions %v", deployment.Name, unhealthy, deployment.Status.AvailableReplicas, deployment.Status.Replicas, err.Error(), deployment.Status.Conditions)
			continue
		}
		f.Logger.Printf("Deployment %s is %s with %d/%d replicas", deployment.Name, healthy, deployment.Status.AvailableReplicas, deployment.Status.Replicas)
	}
	f.Logger.Println()
	return nil
}

// dumpStatefulSetInfoForNamespace prints information about all StatefulSets of a namespace
func (f *CommonFramework) dumpStatefulSetInfoForNamespace(ctx context.Context, ctxIdentifier string, k8sClient kubernetes.Interface, namespace string) error {
	f.Logger.Infof("%s [NAMESPACE %s] [STATEFULSETS]", ctxIdentifier, namespace)
	statefulSets := &appsv1.StatefulSetList{}
	if err := k8sClient.DirectClient().List(ctx, statefulSets, client.InNamespace(namespace)); err != nil {
		return err
	}
	for _, statefulSet := range statefulSets.Items {
		if err := health.CheckStatefulSet(&statefulSet); err != nil {
			f.Logger.Printf("StatefulSet %s is %s with %d/%d replicas - Error: %s - Conditions %v", statefulSet.Name, unhealthy, statefulSet.Status.ReadyReplicas, statefulSet.Status.Replicas, err.Error(), statefulSet.Status.Conditions)
			continue
		}
		f.Logger.Printf("StatefulSet %s is %s with %d/%d replicas", statefulSet.Name, healthy, statefulSet.Status.ReadyReplicas, statefulSet.Status.Replicas)
	}
	f.Logger.Println()
	return nil
}

// dumpDaemonSetInfoForNamespace prints information about all DaemonSets of a namespace
func (f *CommonFramework) dumpDaemonSetInfoForNamespace(ctx context.Context, ctxIdentifier string, k8sClient kubernetes.Interface, namespace string) error {
	f.Logger.Infof("%s [NAMESPACE %s] [DAEMONSETS]", ctxIdentifier, namespace)
	daemonSets := &appsv1.DaemonSetList{}
	if err := k8sClient.DirectClient().List(ctx, daemonSets, client.InNamespace(namespace)); err != nil {
		return err
	}
	for _, ds := range daemonSets.Items {
		if err := health.CheckDaemonSet(&ds); err != nil {
			f.Logger.Printf("DaemonSet %s is %s with %d/%d replicas - Error: %s - Conditions %v", ds.Name, unhealthy, ds.Status.CurrentNumberScheduled, ds.Status.DesiredNumberScheduled, err.Error(), ds.Status.Conditions)
			continue
		}
		f.Logger.Printf("DaemonSet %s is %s with %d/%d replicas", ds.Name, healthy, ds.Status.CurrentNumberScheduled, ds.Status.DesiredNumberScheduled)
	}
	f.Logger.Println()
	return nil
}

// dumpServiceInfoForNamespace prints information about all Services of a namespace
func (f *CommonFramework) dumpServiceInfoForNamespace(ctx context.Context, ctxIdentifier string, k8sClient kubernetes.Interface, namespace string) error {
	f.Logger.Infof("%s [NAMESPACE %s] [SERVICES]", ctxIdentifier, namespace)
	services := &corev1.ServiceList{}
	if err := k8sClient.DirectClient().List(ctx, services, client.InNamespace(namespace)); err != nil {
		return err
	}
	for _, service := range services.Items {
		f.Logger.Printf("Service %s - Spec %v - Status %v", service.Name, service.Spec, service.Status)
	}
	f.Logger.Println()
	return nil
}

// dumpVolumeInfoForNamespace prints information about all PVs and PVCs of a namespace
func (f *CommonFramework) dumpVolumeInfoForNamespace(ctx context.Context, ctxIdentifier string, k8sClient kubernetes.Interface, namespace string) error {
	f.Logger.Infof("%s [NAMESPACE %s] [PVC]", ctxIdentifier, namespace)
	pvcs := &corev1.PersistentVolumeClaimList{}
	if err := k8sClient.Client().List(ctx, pvcs, client.InNamespace(namespace)); err != nil {
		return err
	}
	for _, pvc := range pvcs.Items {
		f.Logger.Printf("PVC %s - Spec %v - Status %v", pvc.Name, pvc.Spec, pvc.Status)
	}
	f.Logger.Println()

	f.Logger.Infof("%s [NAMESPACE %s] [PV]", ctxIdentifier, namespace)
	pvs := &corev1.PersistentVolumeList{}
	if err := k8sClient.Client().List(ctx, pvs, client.InNamespace(namespace)); err != nil {
		return err
	}
	for _, pv := range pvs.Items {
		f.Logger.Printf("PV %s - Spec %v - Status %v", pv.Name, pv.Spec, pv.Status)
	}
	f.Logger.Println()
	return nil
}

// dumpNodes prints information about all nodes
func (f *CommonFramework) dumpNodes(ctx context.Context, ctxIdentifier string, k8sClient kubernetes.Interface) error {
	f.Logger.Infof("%s [NODES]", ctxIdentifier)
	nodes := &corev1.NodeList{}
	if err := k8sClient.DirectClient().List(ctx, nodes); err != nil {
		return err
	}
	for _, node := range nodes.Items {
		if err := health.CheckNode(&node); err != nil {
			f.Logger.Printf("Node %s is %s with phase %s - Error: %s - Conditions %v", node.Name, unhealthy, node.Status.Phase, err.Error(), node.Status.Conditions)
		} else {
			f.Logger.Printf("Node %s is %s with phase %s", node.Name, healthy, node.Status.Phase)
		}
		f.Logger.Printf("Node %s has a capacity of %s cpu, %s memory", node.Name, node.Status.Capacity.Cpu().String(), node.Status.Capacity.Memory().String())

		nodeMetric := &metricsv1beta1.NodeMetrics{}
		if err := k8sClient.DirectClient().Get(ctx, client.ObjectKey{Name: node.Name}, nodeMetric); err != nil {
			f.Logger.Errorf("unable to receive metrics for node %s: %s", node.Name, err.Error())
			continue
		}
		f.Logger.Printf("Node %s currently uses %s cpu, %s memory", node.Name, nodeMetric.Usage.Cpu().String(), nodeMetric.Usage.Memory().String())
	}
	f.Logger.Println()
	return nil
}

// dumpPodInfoForNamespace prints node information of all pods in a namespace
func (f *CommonFramework) dumpPodInfoForNamespace(ctx context.Context, ctxIdentifier string, k8sClient kubernetes.Interface, namespace string) error {
	f.Logger.Infof("%s [NAMESPACE %s] [PODS]", ctxIdentifier, namespace)
	pods := &corev1.PodList{}
	if err := k8sClient.DirectClient().List(ctx, pods, client.InNamespace(namespace)); err != nil {
		return err
	}
	for _, pod := range pods.Items {
		f.Logger.Infof("Pod %s is %s on Node %s", pod.Name, pod.Status.Phase, pod.Spec.NodeName)
	}
	f.Logger.Println()
	return nil
}

// dumpEventsInNamespace prints all events of a namespace
func (f *CommonFramework) dumpEventsInAllNamespace(ctx context.Context, ctxIdentifier string, k8sClient kubernetes.Interface, filters ...EventFilterFunc) error {
	namespaces := &corev1.NamespaceList{}
	if err := k8sClient.DirectClient().List(ctx, namespaces); err != nil {
		return err
	}

	var result error

	for _, ns := range namespaces.Items {
		if err := f.dumpEventsInNamespace(ctx, ctxIdentifier, k8sClient, ns.Name); err != nil {
			result = multierror.Append(result, err)
		}
	}
	return result
}

// dumpEventsInNamespace prints all events of a namespace
func (f *CommonFramework) dumpEventsInNamespace(ctx context.Context, ctxIdentifier string, k8sClient kubernetes.Interface, namespace string, filters ...EventFilterFunc) error {
	f.Logger.Infof("%s [NAMESPACE %s] [EVENTS]", ctxIdentifier, namespace)
	events := &corev1.EventList{}
	if err := k8sClient.DirectClient().List(ctx, events, client.InNamespace(namespace)); err != nil {
		return err
	}

	if len(events.Items) > 1 {
		sort.Sort(eventByFirstTimestamp(events.Items))
	}
	for _, event := range events.Items {
		if ApplyFilters(event, filters...) {
			f.Logger.Printf("At %v - event for %s: %v %v: %s", event.FirstTimestamp, event.InvolvedObject.Name, event.Source, event.Reason, event.Message)
		}
	}
	f.Logger.Println()
	return nil
}

// EventFilterFunc is a function to filter events
type EventFilterFunc func(event corev1.Event) bool

// ApplyFilters checks if one of the EventFilters filters the current event
func ApplyFilters(event corev1.Event, filters ...EventFilterFunc) bool {
	for _, filter := range filters {
		if !filter(event) {
			return false
		}
	}
	return true
}

// eventByFirstTimestamp sorts a slice of events by first timestamp, using their involvedObject's name as a tie breaker.
type eventByFirstTimestamp []corev1.Event

func (o eventByFirstTimestamp) Len() int      { return len(o) }
func (o eventByFirstTimestamp) Swap(i, j int) { o[i], o[j] = o[j], o[i] }

func (o eventByFirstTimestamp) Less(i, j int) bool {
	if o[i].FirstTimestamp.Equal(&o[j].FirstTimestamp) {
		return o[i].InvolvedObject.Name < o[j].InvolvedObject.Name
	}
	return o[i].FirstTimestamp.Before(&o[j].FirstTimestamp)
}
