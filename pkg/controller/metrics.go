/*
Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved.

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

// Package controller is used to provide the core functionalities of machine-controller-manager
package controller

import (
	"strconv"

	v1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// Describe is method required to implement the prometheus.Collect interface.
func (c *controller) Describe(ch chan<- *prometheus.Desc) {
	ch <- metrics.MachineCountDesc
	ch <- metrics.MachineSetCountDesc
	ch <- metrics.MachineDeploymentCountDesc
}

// CollectMachineDeploymentMetrics is method to collect machineSet related metrics.
func (c *controller) CollectMachineDeploymentMetrics(ch chan<- prometheus.Metric) {
	machineDeploymentList, err := c.machineDeploymentLister.MachineDeployments(c.namespace).List(labels.Everything())
	if err != nil {
		metrics.ScrapeFailedCounter.With(prometheus.Labels{"kind": "Machinedeployment-count"}).Inc()
		return
	}
	metric, err := prometheus.NewConstMetric(metrics.MachineDeploymentCountDesc, prometheus.GaugeValue, float64(len(machineDeploymentList)))
	if err != nil {
		metrics.ScrapeFailedCounter.With(prometheus.Labels{"kind": "Machinedeployment-count"}).Inc()
		return
	}
	ch <- metric

	for _, machineDeployment := range machineDeploymentList {

		mdMeta := machineDeployment.ObjectMeta
		mdSpec := machineDeployment.Spec

		metrics.MachineDeploymentInfo.With(prometheus.Labels{
			"name":               mdMeta.Name,
			"namespace":          mdMeta.Namespace,
			"createdAt":          strconv.FormatInt(mdMeta.GetCreationTimestamp().Time.Unix(), 10),
			"spec_strategy_type": string(mdSpec.Strategy.Type),
		}).Set(float64(1))

		var paused float64
		if mdSpec.Paused {
			paused = 1
		}
		metrics.MachineDeploymentInfoSpecPaused.With(prometheus.Labels{
			"name":      mdMeta.Name,
			"namespace": mdMeta.Namespace}).Set(paused)

		metrics.MachineDeploymentInfoSpecReplicas.With(prometheus.Labels{
			"name":      mdMeta.Name,
			"namespace": mdMeta.Namespace}).Set(float64(mdSpec.Replicas))

		metrics.MachineDeploymentInfoSpecMinReadySeconds.With(prometheus.Labels{
			"name":      mdMeta.Name,
			"namespace": mdMeta.Namespace}).Set(float64(mdSpec.MinReadySeconds))

		if mdSpec.Strategy.Type == v1alpha1.RollingUpdateMachineDeploymentStrategyType {
			metrics.MachineDeploymentInfoSpecRollingUpdateMaxSurge.With(prometheus.Labels{
				"name":      mdMeta.Name,
				"namespace": mdMeta.Namespace}).Set(float64(mdSpec.Strategy.RollingUpdate.MaxSurge.IntValue()))
			metrics.MachineDeploymentInfoSpecRollingUpdateMaxUnavailable.With(prometheus.Labels{
				"name":      mdMeta.Name,
				"namespace": mdMeta.Namespace}).Set(float64(mdSpec.Strategy.RollingUpdate.MaxUnavailable.IntValue()))
		}
		if mdSpec.RevisionHistoryLimit != nil {
			metrics.MachineDeploymentInfoSpecRevisionHistoryLimit.With(prometheus.Labels{
				"name":      mdMeta.Name,
				"namespace": mdMeta.Namespace}).Set(float64(int64(*mdSpec.RevisionHistoryLimit)))
		}
		if mdSpec.ProgressDeadlineSeconds != nil {
			metrics.MachineDeploymentInfoSpecProgressDeadlineSeconds.With(prometheus.Labels{
				"name":      mdMeta.Name,
				"namespace": mdMeta.Namespace}).Set(float64(int64(*mdSpec.ProgressDeadlineSeconds)))
		}
		if mdSpec.RollbackTo != nil {
			metrics.MachineDeploymentInfoSpecRollbackToRevision.With(prometheus.Labels{
				"name":      mdMeta.Name,
				"namespace": mdMeta.Namespace}).Set(float64(mdSpec.RollbackTo.Revision))
		}

		for _, condition := range machineDeployment.Status.Conditions {
			var status float64
			switch condition.Status {
			case v1alpha1.ConditionTrue:
				status = 1
			case v1alpha1.ConditionFalse:
				status = 0
			case v1alpha1.ConditionUnknown:
				status = 2
			}

			metrics.MachineDeploymentStatusCondition.With(prometheus.Labels{
				"name":      mdMeta.Name,
				"namespace": mdMeta.Namespace,
				"condition": string(condition.Type),
			}).Set(status)
		}

		statusLabels := prometheus.Labels{
			"name":      mdMeta.Name,
			"namespace": mdMeta.Namespace,
		}
		metrics.MachineDeploymentStatusAvailableReplicas.With(statusLabels).Set(float64(machineDeployment.Status.AvailableReplicas))
		metrics.MachineDeploymentStatusUnavailableReplicas.With(statusLabels).Set(float64(machineDeployment.Status.UnavailableReplicas))
		metrics.MachineDeploymentStatusReadyReplicas.With(statusLabels).Set(float64(machineDeployment.Status.ReadyReplicas))
		metrics.MachineDeploymentStatusUpdatedReplicas.With(statusLabels).Set(float64(machineDeployment.Status.UpdatedReplicas))
		metrics.MachineDeploymentStatusReplicas.With(statusLabels).Set(float64(machineDeployment.Status.Replicas))

		if machineDeployment.Status.CollisionCount != nil {
			metrics.MachineDeploymentStatusCollisionCount.With(statusLabels).Set(float64(*machineDeployment.Status.CollisionCount))
		}

		if machineDeployment.Status.FailedMachines != nil {
			for _, failedMachine := range machineDeployment.Status.FailedMachines {
				metrics.MachineDeploymentStatusFailedMachines.With(prometheus.Labels{
					"name":                                                 mdMeta.Name,
					"namespace":                                            mdMeta.Namespace,
					"failed_machine_name":                                  failedMachine.Name,
					"failed_machine_provider_id":                           failedMachine.ProviderID,
					"failed_machine_last_operation_state":                  string(failedMachine.LastOperation.State),
					"failed_machine_last_operation_machine_operation_type": string(failedMachine.LastOperation.Type),
					"failed_machine_owner_ref":                             failedMachine.OwnerRef}).Set(float64(1))

			}
		}

	}
}

// CollectMachineSetMetrics is method to collect machineSet related metrics.
func (c *controller) CollectMachineSetMetrics(ch chan<- prometheus.Metric) {
	machineSetList, err := c.machineSetLister.MachineSets(c.namespace).List(labels.Everything())
	if err != nil {
		metrics.ScrapeFailedCounter.With(prometheus.Labels{"kind": "Machineset-count"}).Inc()
		return
	}
	metric, err := prometheus.NewConstMetric(metrics.MachineSetCountDesc, prometheus.GaugeValue, float64(len(machineSetList)))
	if err != nil {
		metrics.ScrapeFailedCounter.With(prometheus.Labels{"kind": "Machineset-count"}).Inc()
		return
	}
	ch <- metric

	for _, machineSet := range machineSetList {

		msMeta := machineSet.ObjectMeta
		msSpec := machineSet.Spec

		metrics.MachineSetInfo.With(prometheus.Labels{
			"name":                         msMeta.Name,
			"namespace":                    msMeta.Namespace,
			"createdAt":                    strconv.FormatInt(msMeta.GetCreationTimestamp().Time.Unix(), 10),
			"spec_machine_class_api_group": msSpec.MachineClass.APIGroup,
			"spec_machine_class_kind":      msSpec.MachineClass.Kind,
			"spec_machine_class_name":      msSpec.MachineClass.Name}).Set(float64(1))

		metrics.MachineSetInfoSpecReplicas.With(prometheus.Labels{
			"name":      msMeta.Name,
			"namespace": msMeta.Namespace}).Set(float64(msSpec.Replicas))
		metrics.MachineSetInfoSpecMinReadySeconds.With(prometheus.Labels{
			"name":      msMeta.Name,
			"namespace": msMeta.Namespace}).Set(float64(msSpec.MinReadySeconds))

		for _, condition := range machineSet.Status.Conditions {
			var status float64
			switch condition.Status {
			case v1alpha1.ConditionTrue:
				status = 1
			case v1alpha1.ConditionFalse:
				status = 0
			case v1alpha1.ConditionUnknown:
				status = 2
			}

			metrics.MachineSetStatusCondition.With(prometheus.Labels{
				"name":      msMeta.Name,
				"namespace": msMeta.Namespace,
				"condition": string(condition.Type),
			}).Set(status)
		}

		metrics.MachineSetStatusAvailableReplicas.With(prometheus.Labels{
			"name":      msMeta.Name,
			"namespace": msMeta.Namespace,
		}).Set(float64(machineSet.Status.AvailableReplicas))

		metrics.MachineSetStatusFullyLabelledReplicas.With(prometheus.Labels{
			"name":      msMeta.Name,
			"namespace": msMeta.Namespace,
		}).Set(float64(machineSet.Status.FullyLabeledReplicas))

		metrics.MachineSetStatusReadyReplicas.With(prometheus.Labels{
			"name":      msMeta.Name,
			"namespace": msMeta.Namespace,
		}).Set(float64(machineSet.Status.ReadyReplicas))

		metrics.MachineSetStatusReplicas.With(prometheus.Labels{
			"name":      msMeta.Name,
			"namespace": msMeta.Namespace,
		}).Set(float64(machineSet.Status.ReadyReplicas))

		if machineSet.Status.FailedMachines != nil {

			for _, failedMachine := range *machineSet.Status.FailedMachines {
				metrics.MachineSetStatusFailedMachines.With(prometheus.Labels{
					"name":                                                 msMeta.Name,
					"namespace":                                            msMeta.Namespace,
					"failed_machine_name":                                  failedMachine.Name,
					"failed_machine_provider_id":                           failedMachine.ProviderID,
					"failed_machine_last_operation_state":                  string(failedMachine.LastOperation.State),
					"failed_machine_last_operation_machine_operation_type": string(failedMachine.LastOperation.Type),
					"failed_machine_owner_ref":                             failedMachine.OwnerRef}).Set(float64(1))
			}
		}
	}
}

// CollectMachines is method to collect Machine related metrics.
func (c *controller) CollectMachineMetrics(ch chan<- prometheus.Metric) {
	// Collect the count of machines managed by the mcm.
	machineList, err := c.machineLister.Machines(c.namespace).List(labels.Everything())
	if err != nil {
		metrics.ScrapeFailedCounter.With(prometheus.Labels{"kind": "Machine-count"}).Inc()
		return
	}

	for _, machine := range machineList {
		mMeta := machine.ObjectMeta
		mSpec := machine.Spec

		metrics.MachineInfo.With(prometheus.Labels{
			"name":                 mMeta.Name,
			"namespace":            mMeta.Namespace,
			"createdAt":            strconv.FormatInt(mMeta.GetCreationTimestamp().Time.Unix(), 10),
			"spec_provider_id":     mSpec.ProviderID,
			"spec_class_api_group": mSpec.Class.APIGroup,
			"spec_class_kind":      mSpec.Class.Kind,
			"spec_class_name":      mSpec.Class.Name}).Set(float64(1))

		for _, condition := range machine.Status.Conditions {
			var status float64
			switch condition.Status {
			case v1.ConditionTrue:
				status = 1
			case v1.ConditionFalse:
				status = 0
			case v1.ConditionUnknown:
				status = 2
			}

			metrics.MachineStatusCondition.With(prometheus.Labels{
				"name":      mMeta.Name,
				"namespace": mMeta.Namespace,
				"condition": string(condition.Type),
			}).Set(status)
		}

		var phase float64
		switch machine.Status.CurrentStatus.Phase {
		case v1alpha1.MachinePending:
			phase = -2
		case v1alpha1.MachineAvailable:
			phase = -1
		case v1alpha1.MachineRunning:
			phase = 0
		case v1alpha1.MachineTerminating:
			phase = 1
		case v1alpha1.MachineUnknown:
			phase = 2
		case v1alpha1.MachineFailed:
			phase = 3
		}
		metrics.MachineCSPhase.With(prometheus.Labels{
			"name":      mMeta.Name,
			"namespace": mMeta.Namespace,
		}).Set(phase)

	}

	metric, err := prometheus.NewConstMetric(metrics.MachineCountDesc, prometheus.GaugeValue, float64(len(machineList)))
	if err != nil {
		metrics.ScrapeFailedCounter.With(prometheus.Labels{"kind": "Machine-count"}).Inc()
		return
	}
	ch <- metric

}

// CollectMachines is method to collect Machine related metrics.
func (c *controller) CollectMachineControllerFrozenStatus(ch chan<- prometheus.Metric) {
	var frozenStatus float64
	if c.safetyOptions.MachineControllerFrozen {
		frozenStatus = 1
	}
	metric, err := prometheus.NewConstMetric(metrics.MachineControllerFrozenDesc, prometheus.GaugeValue, frozenStatus)
	if err != nil {
		metrics.ScrapeFailedCounter.With(prometheus.Labels{"kind": "Machine-count"}).Inc()
		return
	}
	ch <- metric
}

// Collect is method required to implement the prometheus.Collect interface.
func (c *controller) Collect(ch chan<- prometheus.Metric) {
	c.CollectMachineMetrics(ch)
	c.CollectMachineSetMetrics(ch)
	c.CollectMachineDeploymentMetrics(ch)
	c.CollectMachineControllerFrozenStatus(ch)
}
