/*
Copyright (c) 2017 SAP SE or an SAP affiliate company. All rights reserved.

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

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// MachineControllerFrozenDesc is a metric about MachineController's frozen status
	MachineControllerFrozenDesc = prometheus.NewDesc("mcm_machine_controller_frozen", "Frozen status of the machine controller manager.", nil, nil)
	// MachineCountDesc is a metric about machine count of the mcm manages
	MachineCountDesc = prometheus.NewDesc("mcm_machine_items_total", "Count of machines currently managed by the mcm.", nil, nil)

	//MachineCSPhase Current status phase of the Machines currently managed by the mcm.
	MachineCSPhase = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcm_machine_current_status_phase",
		Help: "Current status phase of the Machines currently managed by the mcm.",
	}, []string{"name", "namespace"})

	//MachineInfo Information of the Machines currently managed by the mcm.
	MachineInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcm_machine_info",
		Help: "Information of the Machines currently managed by the mcm.",
	}, []string{"name", "namespace", "createdAt",
		"spec_provider_id", "spec_class_api_group", "spec_class_kind", "spec_class_name"})

	// MachineStatusCondition Information of the mcm managed Machines' status conditions
	MachineStatusCondition = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcm_machine_status_condition",
		Help: "Information of the mcm managed Machines' status conditions.",
	}, []string{"name", "namespace", "condition"})

	// MachineSetCountDesc Count of machinesets currently managed by the mcm
	MachineSetCountDesc = prometheus.NewDesc("mcm_machineset_items_total", "Count of machinesets currently managed by the mcm.", nil, nil)

	// MachineSetInfo Information of the Machinesets currently managed by the mcm.
	MachineSetInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcm_machine_set_info",
		Help: "Information of the Machinesets currently managed by the mcm.",
	}, []string{"name", "namespace", "createdAt",
		"spec_machine_class_api_group", "spec_machine_class_kind", "spec_machine_class_name"})

	// MachineSetInfoSpecReplicas Count of the Machinesets Spec Replicas.
	MachineSetInfoSpecReplicas = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcm_machine_set_info_spec_replicas",
		Help: "Count of the Machinesets Spec Replicas.",
	}, []string{"name", "namespace"})

	// MachineSetInfoSpecMinReadySeconds Information of the Machinesets currently managed by the mcm.
	MachineSetInfoSpecMinReadySeconds = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcm_machine_set_info_spec_min_ready_seconds",
		Help: "Information of the Machinesets currently managed by the mcm.",
	}, []string{"name", "namespace"})

	// MachineSetStatusCondition Information of the mcm managed Machinesets' status conditions.
	MachineSetStatusCondition = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcm_machine_set_status_condition",
		Help: "Information of the mcm managed Machinesets' status conditions.",
	}, []string{"name", "namespace", "condition"})

	// MachineSetStatusFailedMachines Information of the mcm managed Machinesets' failed machines.
	MachineSetStatusFailedMachines = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcm_machine_set_failed_machines",
		Help: "Information of the mcm managed Machinesets' failed machines.",
	}, []string{"name", "namespace", "failed_machine_name", "failed_machine_provider_id", "failed_machine_owner_ref",
		"failed_machine_last_operation_state",
		"failed_machine_last_operation_machine_operation_type"})

	// MachineSetStatusAvailableReplicas Information of the mcm managed Machinesets' status for available replicas.
	MachineSetStatusAvailableReplicas = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcm_machine_set_status_available_replicas",
		Help: "Information of the mcm managed Machinesets' status for available replicas.",
	}, []string{"name", "namespace"})

	// MachineSetStatusFullyLabelledReplicas Information of the mcm managed Machinesets' status for fully labelled replicas.
	MachineSetStatusFullyLabelledReplicas = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcm_machine_set_status_fully_labelled_replicas",
		Help: "Information of the mcm managed Machinesets' status for fully labelled replicas.",
	}, []string{"name", "namespace"})

	// MachineSetStatusReadyReplicas Information of the mcm managed Machinesets' status for ready replicas
	MachineSetStatusReadyReplicas = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcm_machine_set_status_ready_replicas",
		Help: "Information of the mcm managed Machinesets' status for ready replicas.",
	}, []string{"name", "namespace"})

	// MachineSetStatusReplicas Information of the mcm managed Machinesets' status for replicas.
	MachineSetStatusReplicas = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcm_machine_set_status_replicas",
		Help: "Information of the mcm managed Machinesets' status for replicas.",
	}, []string{"name", "namespace"})

	// MachineDeploymentCountDesc Count of machinedeployments currently managed by the mcm.
	MachineDeploymentCountDesc = prometheus.NewDesc("mcm_machinedeployment_items_total", "Count of machinedeployments currently managed by the mcm.", nil, nil)

	// MachineDeploymentInfo Information of the Machinedeployments currently managed by the mcm.
	MachineDeploymentInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcm_machine_deployment_info",
		Help: "Information of the Machinedeployments currently managed by the mcm.",
	}, []string{"name", "namespace", "createdAt", "spec_strategy_type"})

	// MachineDeploymentInfoSpecPaused Information of the Machinedeployments paused status.
	MachineDeploymentInfoSpecPaused = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcm_machine_deployment_info_spec_paused",
		Help: "Information of the Machinedeployments paused status.",
	}, []string{"name", "namespace"})

	// MachineDeploymentInfoSpecReplicas Information of the Machinedeployments spec replicas.
	MachineDeploymentInfoSpecReplicas = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcm_machine_deployment_info_spec_replicas",
		Help: "Information of the Machinedeployments spec replicas.",
	}, []string{"name", "namespace"})

	// MachineDeploymentInfoSpecMinReadySeconds Information of the Machinedeployments spec min ready seconds.
	MachineDeploymentInfoSpecMinReadySeconds = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcm_machine_deployment_info_spec_min_ready_seconds",
		Help: "Information of the Machinedeployments spec min ready seconds.",
	}, []string{"name", "namespace"})

	// MachineDeploymentInfoSpecRollingUpdateMaxSurge Information of the Machinedeployments spec rolling update max surge.
	MachineDeploymentInfoSpecRollingUpdateMaxSurge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcm_machine_deployment_info_spec_rolling_update_max_surge",
		Help: "Information of the Machinedeployments spec rolling update max surge.",
	}, []string{"name", "namespace"})

	// MachineDeploymentInfoSpecRollingUpdateMaxUnavailable Information of the Machinedeployments spec rolling update max unavailable.
	MachineDeploymentInfoSpecRollingUpdateMaxUnavailable = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcm_machine_deployment_info_spec_rolling_update_max_unavailable",
		Help: "Information of the Machinedeployments spec rolling update max unavailable.",
	}, []string{"name", "namespace"})

	// MachineDeploymentInfoSpecRevisionHistoryLimit Information of the Machinedeployments spec revision history limit.
	MachineDeploymentInfoSpecRevisionHistoryLimit = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcm_machine_deployment_info_spec_revision_history_limit",
		Help: "Information of the Machinedeployments spec revision history limit.",
	}, []string{"name", "namespace"})

	// MachineDeploymentInfoSpecProgressDeadlineSeconds Information of the Machinedeployments spec deadline seconds.
	MachineDeploymentInfoSpecProgressDeadlineSeconds = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcm_machine_deployment_info_spec_progress_deadline_seconds",
		Help: "Information of the Machinedeployments spec deadline seconds.",
	}, []string{"name", "namespace"})

	// MachineDeploymentInfoSpecRollbackToRevision Information of the Machinedeployments spec rollback to revision.
	MachineDeploymentInfoSpecRollbackToRevision = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcm_machine_deployment_info_spec_rollback_to_revision",
		Help: "Information of the Machinedeployments spec rollback to revision.",
	}, []string{"name", "namespace"})

	// MachineDeploymentStatusCondition Information of the mcm managed Machinedeployments' status conditions.
	MachineDeploymentStatusCondition = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcm_machine_deployment_status_condition",
		Help: "Information of the mcm managed Machinedeployments' status conditions.",
	}, []string{"name", "namespace", "condition"})

	// MachineDeploymentStatusAvailableReplicas Count of the mcm managed Machinedeployments available replicas.
	MachineDeploymentStatusAvailableReplicas = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcm_machine_deployment_status_available_replicas",
		Help: "Count of the mcm managed Machinedeployments available replicas.",
	}, []string{"name", "namespace"})

	// MachineDeploymentStatusUnavailableReplicas Count of the mcm managed Machinedeployments unavailable replicas.
	MachineDeploymentStatusUnavailableReplicas = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcm_machine_deployment_status_unavailable_replicas",
		Help: "Count of the mcm managed Machinedeployments unavailable replicas.",
	}, []string{"name", "namespace"})

	// MachineDeploymentStatusReadyReplicas Count of the mcm managed Machinedeployments ready replicas.
	MachineDeploymentStatusReadyReplicas = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcm_machine_deployment_status_ready_replicas",
		Help: "Count of the mcm managed Machinedeployments ready replicas.",
	}, []string{"name", "namespace"})

	// MachineDeploymentStatusUpdatedReplicas Count of the mcm managed Machinedeployments updated replicas.
	MachineDeploymentStatusUpdatedReplicas = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcm_machine_deployment_status_updated_replicas",
		Help: "Count of the mcm managed Machinedeployments updated replicas.",
	}, []string{"name", "namespace"})

	// MachineDeploymentStatusCollisionCount Mcm managed Machinedeployments collision count.
	MachineDeploymentStatusCollisionCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcm_machine_deployment_status_collision_count",
		Help: "Mcm managed Machinedeployments collision count.",
	}, []string{"name", "namespace"})

	// MachineDeploymentStatusReplicas Count of the mcm managed Machinedeployments replicas.
	MachineDeploymentStatusReplicas = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcm_machine_deployment_status_replicas",
		Help: "Count of the mcm managed Machinedeployments replicas.",
	}, []string{"name", "namespace"})

	// MachineDeploymentStatusFailedMachines Information of the mcm managed Machinedeployments' failed machines.
	MachineDeploymentStatusFailedMachines = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mcm_machine_deployment_failed_machines",
		Help: "Information of the mcm managed Machinedeployments' failed machines.",
	}, []string{"name", "namespace", "failed_machine_name", "failed_machine_provider_id", "failed_machine_owner_ref",
		"failed_machine_last_operation_state",
		"failed_machine_last_operation_machine_operation_type"})

	// APIRequestCount Number of Cloud Service API requests, partitioned by provider, and service.
	APIRequestCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "mcm_cloud_api_requests_total",
		Help: "Number of Cloud Service API requests, partitioned by provider, and service.",
	}, []string{"provider", "service"},
	)

	// APIFailedRequestCount Number of Failed Cloud Service API requests, partitioned by provider, and service.
	APIFailedRequestCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "mcm_cloud_api_requests_failed_total",
		Help: "Number of Failed Cloud Service API requests, partitioned by provider, and service.",
	}, []string{"provider", "service"},
	)

	// ScrapeFailedCounter is a Prometheus metric, which counts errors during metrics collection.
	ScrapeFailedCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "mcm_scrape_failure_total",
		Help: "Total count of scrape failures.",
	}, []string{"kind"})
)

func init() {
	prometheus.MustRegister(ScrapeFailedCounter)
	prometheus.MustRegister(MachineInfo)
	prometheus.MustRegister(MachineStatusCondition)
	prometheus.MustRegister(MachineCSPhase)
	prometheus.MustRegister(MachineSetInfo)
	prometheus.MustRegister(MachineSetInfoSpecReplicas)
	prometheus.MustRegister(MachineSetInfoSpecMinReadySeconds)
	prometheus.MustRegister(MachineSetStatusAvailableReplicas)
	prometheus.MustRegister(MachineSetStatusFullyLabelledReplicas)
	prometheus.MustRegister(MachineSetStatusReadyReplicas)
	prometheus.MustRegister(MachineSetStatusReplicas)
	prometheus.MustRegister(MachineSetStatusCondition)
	prometheus.MustRegister(MachineSetStatusFailedMachines)
	prometheus.MustRegister(MachineDeploymentInfo)
	prometheus.MustRegister(MachineDeploymentInfoSpecPaused)
	prometheus.MustRegister(MachineDeploymentInfoSpecReplicas)
	prometheus.MustRegister(MachineDeploymentInfoSpecRevisionHistoryLimit)
	prometheus.MustRegister(MachineDeploymentInfoSpecMinReadySeconds)
	prometheus.MustRegister(MachineDeploymentInfoSpecRollingUpdateMaxSurge)
	prometheus.MustRegister(MachineDeploymentInfoSpecRollingUpdateMaxUnavailable)
	prometheus.MustRegister(MachineDeploymentInfoSpecProgressDeadlineSeconds)
	prometheus.MustRegister(MachineDeploymentInfoSpecRollbackToRevision)
	prometheus.MustRegister(MachineDeploymentStatusCondition)
	prometheus.MustRegister(MachineDeploymentStatusAvailableReplicas)
	prometheus.MustRegister(MachineDeploymentStatusUnavailableReplicas)
	prometheus.MustRegister(MachineDeploymentStatusReadyReplicas)
	prometheus.MustRegister(MachineDeploymentStatusUpdatedReplicas)
	prometheus.MustRegister(MachineDeploymentStatusCollisionCount)
	prometheus.MustRegister(MachineDeploymentStatusReplicas)
	prometheus.MustRegister(MachineDeploymentStatusFailedMachines)
	prometheus.MustRegister(APIRequestCount)
	prometheus.MustRegister(APIFailedRequestCount)
}
