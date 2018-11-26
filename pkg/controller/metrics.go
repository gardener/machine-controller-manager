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
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

var (
	machineControllerFrozenDesc = prometheus.NewDesc("mcm_machine_controller_frozen", "Frozen status of the machine controller manager.", nil, nil)
	machineCountDesc            = prometheus.NewDesc("mcm_machine_items_total", "Count of machines currently managed by the mcm.", nil, nil)
	MachineCreated              = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcm_machine_created",
		Help: "Creation time of the machines currently managed by the mcm.",
	}, []string{"name", "namespace", "uid"})

	MachineCSPhase = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcm_machine_current_status_phase",
		Help: "Current status phase of the machines currently managed by the mcm.",
	}, []string{"name", "namespace", "uid", "phase"})

	MachineInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcm_machine_info",
		Help: "Information of the machines currently managed by the mcm.",
	}, []string{"name", "namespace", "uid", "generation", "kind", "api_version",
		"spec_provider_id", "spec_class_api_group", "spec_class_kind", "spec_class_name"})

	MachineStatusCondition = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcm_machine_status_condition",
		Help: "Information of the mcm managed machines' status conditions.",
	}, []string{"name", "namespace", "uid", "condition", "status"})

	machineSetCountDesc = prometheus.NewDesc("mcm_machineset_items_total", "Count of machinesets currently managed by the mcm.", nil, nil)
	MachineSetCreated   = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcm_machine_set_created",
		Help: "Creation time of the machinesets currently managed by the mcm.",
	}, []string{"name", "namespace", "uid"})

	MachineSetInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcm_machine_set_info",
		Help: "Information of the machinesets currently managed by the mcm.",
	}, []string{"name", "namespace", "uid", "generation", "kind", "api_version", "spec_replicas",
		"spec_min_ready_reconds", "spec_machine_class_api_group", "spec_machine_class_kind", "spec_machine_class_name"})

	MachineSetStatusCondition = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcm_machine_set_status_condition",
		Help: "Information of the mcm managed machinesets' status conditions.",
	}, []string{"name", "namespace", "uid", "condition", "status"})

	MachineSetStatusFailedMachines = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcm_machine_set_failed_machines",
		Help: "Information of the mcm managed machinesets' failed machines.",
	}, []string{"name", "namespace", "uid", "failed_machine_name", "failed_machine_provider_id", "failed_machine_owner_ref",
		"failed_machine_last_operation_description", "failed_machine_last_operation_last_update_time", "failed_machine_last_operation_state",
		"failed_machine_last_operation_machine_operation_type"})

	MachineSetStatus = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcm_machine_set_status",
		Help: "Information of the mcm managed machinesets' status conditions.",
	}, []string{"name", "namespace", "uid", "available_replicas",
		"fully_labeled_replicas", "ready_replicas", "replicas"})

	machineDeploymentCountDesc = prometheus.NewDesc("mcm_machinedeployment_items_total", "Count of machinedeployments currently managed by the mcm.", nil, nil)
	MachineDeploymentCreated   = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcm_machine_deployment_created",
		Help: "Creation time of the machinedeployments currently managed by the mcm.",
	}, []string{"name", "namespace", "uid"})
	MachineDeploymentInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcm_machine_deployment_info",
		Help: "Information of the machinedeployments currently managed by the mcm.",
	}, []string{"name", "namespace", "uid", "generation", "kind", "api_version", "spec_replicas", "spec_strategy_type",
		"spec_paused", "spec_revision_history_limit", "spec_progress_deadline_seconds", "spec_min_ready_seconds", "spec_rollbackto_revision",
		"spec_strategy_rolling_update_max_surge", "spec_strategy_rolling_update_max_unavailable"})

	MachineDeploymentStatusCondition = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcm_machine_deployment_status_condition",
		Help: "Information of the mcm managed machinedeployments' status conditions.",
	}, []string{"name", "namespace", "uid", "condition", "status"})

	MachineDeploymentStatus = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcm_machine_deployment_status",
		Help: "Information of the mcm managed machinedeployments' status conditions.",
	}, []string{"name", "namespace", "uid", "available_replicas", "unavailable_replicas", "ready_replicas",
		"updated_replicas", "collision_count", "replicas"})
	MachineDeploymentStatusFailedMachines = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcm_machine_deployment_failed_machines",
		Help: "Information of the mcm managed machinedeployments' failed machines.",
	}, []string{"name", "namespace", "uid", "failed_machine_name", "failed_machine_provider_id", "failed_machine_owner_ref",
		"failed_machine_last_operation_description", "failed_machine_last_operation_last_update_time", "failed_machine_last_operation_state",
		"failed_machine_last_operation_machine_operation_type"})

	// ScrapeFailedCounter is a Prometheus metric, which counts errors during metrics collection.
	ScrapeFailedCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "mcm_scrape_failure_total",
		Help: "Total count of scrape failures.",
	}, []string{"kind"})
)

func init() {
	prometheus.MustRegister(ScrapeFailedCounter)
	prometheus.MustRegister(MachineCreated)
	prometheus.MustRegister(MachineInfo)
	prometheus.MustRegister(MachineStatusCondition)
	prometheus.MustRegister(MachineCSPhase)
	prometheus.MustRegister(MachineSetCreated)
	prometheus.MustRegister(MachineSetInfo)
	prometheus.MustRegister(MachineSetStatus)
	prometheus.MustRegister(MachineSetStatusCondition)
	prometheus.MustRegister(MachineSetStatusFailedMachines)
	prometheus.MustRegister(MachineDeploymentCreated)
	prometheus.MustRegister(MachineDeploymentInfo)
	prometheus.MustRegister(MachineDeploymentStatus)
	prometheus.MustRegister(MachineDeploymentStatusCondition)
	prometheus.MustRegister(MachineDeploymentStatusFailedMachines)
}

// Describe is method required to implement the prometheus.Collect interface.
func (c *controller) Describe(ch chan<- *prometheus.Desc) {
	ch <- machineCountDesc
}

// CollectMachineDeploymentMetrics is method to collect MachineSet related metrics.
func (c *controller) CollectMachineDeploymentMetrics(ch chan<- prometheus.Metric) {
	machineDeploymentList, err := c.machineDeploymentLister.MachineDeployments(c.namespace).List(labels.Everything())
	if err != nil {
		ScrapeFailedCounter.With(prometheus.Labels{"kind": "machinedeployment-count"}).Inc()
		return
	}
	metric, err := prometheus.NewConstMetric(machineDeploymentCountDesc, prometheus.GaugeValue, float64(len(machineDeploymentList)))
	if err != nil {
		ScrapeFailedCounter.With(prometheus.Labels{"kind": "machinedeployment-count"}).Inc()
		return
	}
	ch <- metric

	for _, machineDeployment := range machineDeploymentList {

		mdMeta := machineDeployment.ObjectMeta
		mdType := machineDeployment.TypeMeta
		mdSpec := machineDeployment.Spec

		MachineDeploymentCreated.With(prometheus.Labels{
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
		MachineDeploymentInfo.With(infoLabels).Set(float64(1))

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

			MachineDeploymentStatusCondition.With(prometheus.Labels{
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
		MachineDeploymentStatus.With(statusLabels).Set(float64(machineDeployment.Status.ReadyReplicas - machineDeployment.Status.Replicas))

		if machineDeployment.Status.FailedMachines != nil {
			for _, failed_machine := range machineDeployment.Status.FailedMachines {
				MachineDeploymentStatusFailedMachines.With(prometheus.Labels{
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

// CollectMachineSetMetrics is method to collect MachineSet related metrics.
func (c *controller) CollectMachineSetMetrics(ch chan<- prometheus.Metric) {
	machineSetList, err := c.machineSetLister.MachineSets(c.namespace).List(labels.Everything())
	if err != nil {
		ScrapeFailedCounter.With(prometheus.Labels{"kind": "machineset-count"}).Inc()
		return
	}
	metric, err := prometheus.NewConstMetric(machineSetCountDesc, prometheus.GaugeValue, float64(len(machineSetList)))
	if err != nil {
		ScrapeFailedCounter.With(prometheus.Labels{"kind": "machineset-count"}).Inc()
		return
	}
	ch <- metric

	for _, machineSet := range machineSetList {

		msMeta := machineSet.ObjectMeta
		msType := machineSet.TypeMeta
		msSpec := machineSet.Spec

		MachineSetCreated.With(prometheus.Labels{
			"name":      msMeta.Name,
			"uid":       string(msMeta.UID),
			"namespace": msMeta.Namespace}).Set(
			float64(msMeta.GetCreationTimestamp().Time.Unix()))

		MachineSetInfo.With(prometheus.Labels{
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

			MachineSetStatusCondition.With(prometheus.Labels{
				"name":      msMeta.Name,
				"namespace": msMeta.Namespace,
				"uid":       string(msMeta.UID),
				"condition": string(condition.Type),
				"status":    string(condition.Status)}).Set(float64(status))
		}

		MachineSetStatus.With(prometheus.Labels{
			"name":                   msMeta.Name,
			"namespace":              msMeta.Namespace,
			"uid":                    string(msMeta.UID),
			"available_replicas":     string(machineSet.Status.AvailableReplicas),
			"fully_labeled_replicas": string(machineSet.Status.FullyLabeledReplicas),
			"ready_replicas":         string(machineSet.Status.ReadyReplicas),
			"replicas":               string(machineSet.Status.Replicas)}).Set(float64(machineSet.Status.ReadyReplicas - machineSet.Status.Replicas))

		if machineSet.Status.FailedMachines != nil {

			for _, failed_machine := range *machineSet.Status.FailedMachines {
				MachineSetStatusFailedMachines.With(prometheus.Labels{
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
		ScrapeFailedCounter.With(prometheus.Labels{"kind": "machine-count"}).Inc()
		return
	}

	for _, machine := range machineList {
		mMeta := machine.ObjectMeta
		mType := machine.TypeMeta
		mSpec := machine.Spec

		MachineCreated.With(prometheus.Labels{
			"name":      mMeta.Name,
			"uid":       string(mMeta.UID),
			"namespace": mMeta.Namespace}).Set(
			float64(mMeta.GetCreationTimestamp().Time.Unix()))

		MachineInfo.With(prometheus.Labels{
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

			MachineStatusCondition.With(prometheus.Labels{
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
			MachineCSPhase.With(prometheus.Labels{
				"name":      mMeta.Name,
				"namespace": mMeta.Namespace,
				"uid":       string(mMeta.UID),
				"phase":     string(machine.Status.CurrentStatus.Phase)}).Set(float64(phase))

		}
	}

	metric, err := prometheus.NewConstMetric(machineCountDesc, prometheus.GaugeValue, float64(len(machineList)))
	if err != nil {
		ScrapeFailedCounter.With(prometheus.Labels{"kind": "machine-count"}).Inc()
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
	metric, err := prometheus.NewConstMetric(machineControllerFrozenDesc, prometheus.GaugeValue, float64(frozen_status))
	if err != nil {
		ScrapeFailedCounter.With(prometheus.Labels{"kind": "machine-count"}).Inc()
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
