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
	v1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"strconv"
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
		mdType := machineDeployment.TypeMeta
		mdSpec := machineDeployment.Spec

		metrics.MachineDeploymentCreated.With(prometheus.Labels{
			"name":      mdMeta.Name,
			"uid":       string(mdMeta.UID),
			"namespace": mdMeta.Namespace}).Set(
			float64(mdMeta.GetCreationTimestamp().Time.Unix()))

		infoLabels := prometheus.Labels{
			"name":                                   mdMeta.Labels["name"],
			"namespace":                              mdMeta.Namespace,
			"uid":                                    string(mdMeta.UID),
			"generation":                             strconv.FormatInt(mdMeta.Generation, 10),
			"kind":                                   mdType.Kind,
			"api_version":                            mdType.APIVersion,
			"spec_replicas":                          strconv.FormatInt(int64(mdSpec.Replicas), 10),
			"spec_strategy_type":                     string(mdSpec.Strategy.Type),
			"spec_paused":                            strconv.FormatBool(mdSpec.Paused),
			"spec_min_ready_seconds":                 strconv.FormatInt(int64(mdSpec.MinReadySeconds), 10),
			"spec_strategy_rolling_update_max_surge": "",
			"spec_strategy_rolling_update_max_unavailable": "",
			"spec_revision_history_limit":                  "",
			"spec_progress_deadline_seconds":               "",
			"spec_rollbackto_revision":                     ""}
		if mdSpec.Strategy.Type == "" {
			infoLabels["spec_strategy_rolling_update_max_surge"] = mdSpec.Strategy.RollingUpdate.MaxSurge.String()
			infoLabels["spec_strategy_rolling_update_max_unavailable"] = mdSpec.Strategy.RollingUpdate.MaxUnavailable.String()
		}
		if mdSpec.RevisionHistoryLimit != nil {
			infoLabels["spec_revision_history_limit"] = strconv.FormatInt(int64(*mdSpec.RevisionHistoryLimit), 10)
		}
		if mdSpec.ProgressDeadlineSeconds != nil {
			infoLabels["spec_progress_deadline_seconds"] = strconv.FormatInt(int64(*mdSpec.ProgressDeadlineSeconds), 10)
		}
		if mdSpec.RollbackTo != nil {
			infoLabels["spec_rollbackto_revision"] = strconv.FormatInt(int64(mdSpec.RollbackTo.Revision), 10)
		}
		metrics.MachineDeploymentInfo.With(infoLabels).Set(float64(1))

		for _, condition := range machineDeployment.Status.Conditions {
			status := 0
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
				"uid":       string(mdMeta.UID),
				"condition": string(condition.Type),
				"status":    string(condition.Status)}).Set(float64(status))

		}

		statusLabels := prometheus.Labels{
			"name":                 mdMeta.Name,
			"namespace":            mdMeta.Namespace,
			"uid":                  string(mdMeta.UID),
			"available_replicas":   strconv.FormatInt(int64(machineDeployment.Status.AvailableReplicas), 10),
			"unavailable_replicas": strconv.FormatInt(int64(machineDeployment.Status.UnavailableReplicas), 10),
			"ready_replicas":       strconv.FormatInt(int64(machineDeployment.Status.ReadyReplicas), 10),
			"updated_replicas":     strconv.FormatInt(int64(machineDeployment.Status.UpdatedReplicas), 10),
			"collision_count":      "",
			"replicas":             strconv.FormatInt(int64(machineDeployment.Status.Replicas), 10)}

		if machineDeployment.Status.CollisionCount != nil {
			strconv.FormatInt(int64(*machineDeployment.Status.CollisionCount), 10)
		}
		metrics.MachineDeploymentStatus.With(statusLabels).Set(float64(machineDeployment.Status.ReadyReplicas - machineDeployment.Status.Replicas))

		if machineDeployment.Status.FailedMachines != nil {
			for _, failed_machine := range machineDeployment.Status.FailedMachines {
				metrics.MachineDeploymentStatusFailedMachines.With(prometheus.Labels{
					"name":                       mdMeta.Labels["name"],
					"namespace":                  mdMeta.Namespace,
					"uid":                        string(mdMeta.UID),
					"failed_machine_name":        failed_machine.Name,
					"failed_machine_provider_id": failed_machine.ProviderID,
					"failed_machine_last_operation_description":            failed_machine.LastOperation.Description,
					"failed_machine_last_operation_last_update_time":       strconv.FormatInt(failed_machine.LastOperation.LastUpdateTime.Unix(), 10),
					"failed_machine_last_operation_state":                  string(failed_machine.LastOperation.State),
					"failed_machine_last_operation_machine_operation_type": string(failed_machine.LastOperation.Type),
					"failed_machine_owner_ref":                             failed_machine.OwnerRef}).Set(float64(1))

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
		msType := machineSet.TypeMeta
		msSpec := machineSet.Spec

		metrics.MachineSetCreated.With(prometheus.Labels{
			"name":      msMeta.Name,
			"uid":       string(msMeta.UID),
			"namespace": msMeta.Namespace}).Set(
			float64(msMeta.GetCreationTimestamp().Time.Unix()))

		metrics.MachineSetInfo.With(prometheus.Labels{
			"name":                         msMeta.Labels["name"],
			"namespace":                    msMeta.Namespace,
			"uid":                          string(msMeta.UID),
			"generation":                   strconv.FormatInt(msMeta.Generation, 10),
			"kind":                         msType.Kind,
			"api_version":                  msType.APIVersion,
			"spec_replicas":                strconv.FormatInt(int64(msSpec.Replicas), 10),
			"spec_min_ready_reconds":       strconv.FormatInt(int64(msSpec.MinReadySeconds), 10),
			"spec_machine_class_api_group": msSpec.MachineClass.APIGroup,
			"spec_machine_class_kind":      msSpec.MachineClass.Kind,
			"spec_machine_class_name":      msSpec.MachineClass.Name}).Set(float64(1))

		for _, condition := range machineSet.Status.Conditions {
			status := 0
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
				"uid":       string(msMeta.UID),
				"condition": string(condition.Type),
				"status":    string(condition.Status)}).Set(float64(status))
		}

		metrics.MachineSetStatus.With(prometheus.Labels{
			"name":                   msMeta.Name,
			"namespace":              msMeta.Namespace,
			"uid":                    string(msMeta.UID),
			"available_replicas":     string(machineSet.Status.AvailableReplicas),
			"fully_labeled_replicas": string(machineSet.Status.FullyLabeledReplicas),
			"ready_replicas":         string(machineSet.Status.ReadyReplicas),
			"replicas":               string(machineSet.Status.Replicas)}).Set(float64(machineSet.Status.ReadyReplicas - machineSet.Status.Replicas))

		if machineSet.Status.FailedMachines != nil {

			for _, failed_machine := range *machineSet.Status.FailedMachines {
				metrics.MachineSetStatusFailedMachines.With(prometheus.Labels{
					"name":                       msMeta.Labels["name"],
					"namespace":                  msMeta.Namespace,
					"uid":                        string(msMeta.UID),
					"failed_machine_name":        failed_machine.Name,
					"failed_machine_provider_id": failed_machine.ProviderID,
					"failed_machine_last_operation_description":            failed_machine.LastOperation.Description,
					"failed_machine_last_operation_last_update_time":       strconv.FormatInt(failed_machine.LastOperation.LastUpdateTime.Unix(), 10),
					"failed_machine_last_operation_state":                  string(failed_machine.LastOperation.State),
					"failed_machine_last_operation_machine_operation_type": string(failed_machine.LastOperation.Type),
					"failed_machine_owner_ref":                             failed_machine.OwnerRef}).Set(float64(1))

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
		mType := machine.TypeMeta
		mSpec := machine.Spec

		metrics.MachineCreated.With(prometheus.Labels{
			"name":      mMeta.Name,
			"uid":       string(mMeta.UID),
			"namespace": mMeta.Namespace}).Set(
			float64(mMeta.GetCreationTimestamp().Time.Unix()))

		metrics.MachineInfo.With(prometheus.Labels{
			"name":                 mMeta.Name,
			"namespace":            mMeta.Namespace,
			"uid":                  string(mMeta.UID),
			"generation":           strconv.FormatInt(mMeta.Generation, 10),
			"kind":                 mType.Kind,
			"api_version":          mType.APIVersion,
			"spec_provider_id":     mSpec.ProviderID,
			"spec_class_api_group": mSpec.Class.APIGroup,
			"spec_class_kind":      mSpec.Class.Kind,
			"spec_class_name":      mSpec.Class.Name}).Set(float64(1))

		for _, condition := range machine.Status.Conditions {
			status := 0
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
				"uid":       string(mMeta.UID),
				"condition": string(condition.Type),
				"status":    string(condition.Status)}).Set(float64(status))

			phase := 0
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
				"uid":       string(mMeta.UID),
				"phase":     string(machine.Status.CurrentStatus.Phase)}).Set(float64(phase))

		}
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
	frozen_status := 0
	if c.safetyOptions.MachineControllerFrozen {
		frozen_status = 1
	}
	metric, err := prometheus.NewConstMetric(metrics.MachineControllerFrozenDesc, prometheus.GaugeValue, float64(frozen_status))
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
