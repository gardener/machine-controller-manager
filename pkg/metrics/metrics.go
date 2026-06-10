// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

const (
	namespace                  = "mcm"
	machineSubsystem           = "machine"
	machinesetSubsystem        = "machine_set"
	machinedeploymentSubsystem = "machine_deployment"
	miscSubsystem              = "misc"
)

// variables for subsystem: machine
var (
	// StaleMachineCount Number of stale (failed) machines that get flagged for termination
	StaleMachineCount = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: machineSubsystem,
		Name:      "stale_machines_total",
		Help:      "Total count of stale machines flagged for termination that turned stale due to long unhealthiness",
	})
)

// variables for subsystem: machine_set
var (
	// MachineSetCountDesc Count of machinesets currently managed by the mcm
	MachineSetCountDesc = prometheus.NewDesc("mcm_machine_set_items_total", "Count of machinesets currently managed by the mcm.", nil, nil)

	// MachineSetInfo Information of the Machinesets currently managed by the mcm.
	MachineSetInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: machinesetSubsystem,
		Name:      "info",
		Help:      "Information of the Machinesets currently managed by the mcm.",
	}, []string{"name", "namespace", "createdAt",
		"spec_machine_class_api_group", "spec_machine_class_kind", "spec_machine_class_name"})

	// MachineSetInfoSpecReplicas Count of the Machinesets Spec Replicas.
	MachineSetInfoSpecReplicas = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: machinesetSubsystem,
		Name:      "info_spec_replicas",
		Help:      "Count of the Machinesets Spec Replicas.",
	}, []string{"name", "namespace"})

	// MachineSetInfoSpecMinReadySeconds Information of the Machinesets currently managed by the mcm.
	MachineSetInfoSpecMinReadySeconds = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: machinesetSubsystem,
		Name:      "info_spec_min_ready_seconds",
		Help:      "Information of the Machinesets currently managed by the mcm.",
	}, []string{"name", "namespace"})

	// MachineSetStatusCondition Information of the mcm managed Machinesets' status conditions.
	MachineSetStatusCondition = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: machinesetSubsystem,
		Name:      "status_condition",
		Help:      "Information of the mcm managed Machinesets' status conditions.",
	}, []string{"name", "namespace", "condition"})

	// MachineSetStatusFailedMachines Information of the mcm managed Machinesets' failed machines.
	MachineSetStatusFailedMachines = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: machinesetSubsystem,
		Name:      "failed_machines",
		Help:      "Information of the mcm managed Machinesets' failed machines.",
	}, []string{"name", "namespace", "failed_machine_name", "failed_machine_provider_id", "failed_machine_owner_ref",
		"failed_machine_last_operation_state",
		"failed_machine_last_operation_machine_operation_type"})

	// MachineSetStatusAvailableReplicas Information of the mcm managed Machinesets' status for available replicas.
	MachineSetStatusAvailableReplicas = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: machinesetSubsystem,
		Name:      "status_available_replicas",
		Help:      "Information of the mcm managed Machinesets' status for available replicas.",
	}, []string{"name", "namespace"})

	// MachineSetStatusFullyLabelledReplicas Information of the mcm managed Machinesets' status for fully labelled replicas.
	MachineSetStatusFullyLabelledReplicas = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: machinesetSubsystem,
		Name:      "status_fully_labelled_replicas",
		Help:      "Information of the mcm managed Machinesets' status for fully labelled replicas.",
	}, []string{"name", "namespace"})

	// MachineSetStatusReadyReplicas Information of the mcm managed Machinesets' status for ready replicas
	MachineSetStatusReadyReplicas = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: machinesetSubsystem,
		Name:      "status_ready_replicas",
		Help:      "Information of the mcm managed Machinesets' status for ready replicas.",
	}, []string{"name", "namespace"})

	// MachineSetStatusReplicas Information of the mcm managed Machinesets' status for replicas.
	MachineSetStatusReplicas = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: machinesetSubsystem,
		Name:      "status_replicas",
		Help:      "Information of the mcm managed Machinesets' status for replicas.",
	}, []string{"name", "namespace"})
)

// variables for subsystem: machine_deployment
var (
	// MachineDeploymentCountDesc Count of machinedeployments currently managed by the mcm.
	MachineDeploymentCountDesc = prometheus.NewDesc("mcm_machine_deployment_items_total", "Count of machinedeployments currently managed by the mcm.", nil, nil)

	// MachineDeploymentInfo Information of the Machinedeployments currently managed by the mcm.
	MachineDeploymentInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: machinedeploymentSubsystem,
		Name:      "info",
		Help:      "Information of the Machinedeployments currently managed by the mcm.",
	}, []string{"name", "namespace", "createdAt", "spec_strategy_type"})

	// MachineDeploymentInfoSpecPaused Information of the Machinedeployments paused status.
	MachineDeploymentInfoSpecPaused = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: machinedeploymentSubsystem,
		Name:      "info_spec_paused",
		Help:      "Information of the Machinedeployments paused status.",
	}, []string{"name", "namespace"})

	// MachineDeploymentInfoSpecReplicas Information of the Machinedeployments spec replicas.
	MachineDeploymentInfoSpecReplicas = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: machinedeploymentSubsystem,
		Name:      "info_spec_replicas",
		Help:      "Information of the Machinedeployments spec replicas.",
	}, []string{"name", "namespace"})

	// MachineDeploymentInfoSpecMinReadySeconds Information of the Machinedeployments spec min ready seconds.
	MachineDeploymentInfoSpecMinReadySeconds = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: machinedeploymentSubsystem,
		Name:      "info_spec_min_ready_seconds",
		Help:      "Information of the Machinedeployments spec min ready seconds.",
	}, []string{"name", "namespace"})

	// MachineDeploymentInfoSpecRollingUpdateMaxSurge Information of the Machinedeployments spec rolling update max surge.
	MachineDeploymentInfoSpecRollingUpdateMaxSurge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: machinedeploymentSubsystem,
		Name:      "info_spec_rolling_update_max_surge",
		Help:      "Information of the Machinedeployments spec rolling update max surge.",
	}, []string{"name", "namespace"})

	// MachineDeploymentInfoSpecRollingUpdateMaxUnavailable Information of the Machinedeployments spec rolling update max unavailable.
	MachineDeploymentInfoSpecRollingUpdateMaxUnavailable = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: machinedeploymentSubsystem,
		Name:      "info_spec_rolling_update_max_unavailable",
		Help:      "Information of the Machinedeployments spec rolling update max unavailable.",
	}, []string{"name", "namespace"})

	// MachineDeploymentInfoSpecRevisionHistoryLimit Information of the Machinedeployments spec revision history limit.
	MachineDeploymentInfoSpecRevisionHistoryLimit = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: machinedeploymentSubsystem,
		Name:      "info_spec_revision_history_limit",
		Help:      "Information of the Machinedeployments spec revision history limit.",
	}, []string{"name", "namespace"})

	// MachineDeploymentInfoSpecProgressDeadlineSeconds Information of the Machinedeployments spec deadline seconds.
	MachineDeploymentInfoSpecProgressDeadlineSeconds = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: machinedeploymentSubsystem,
		Name:      "info_spec_progress_deadline_seconds",
		Help:      "Information of the Machinedeployments spec deadline seconds.",
	}, []string{"name", "namespace"})

	// MachineDeploymentInfoSpecRollbackToRevision Information of the Machinedeployments spec rollback to revision.
	MachineDeploymentInfoSpecRollbackToRevision = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: machinedeploymentSubsystem,
		Name:      "info_spec_rollback_to_revision",
		Help:      "Information of the Machinedeployments spec rollback to revision.",
	}, []string{"name", "namespace"})

	// MachineDeploymentStatusCondition Information of the mcm managed Machinedeployments' status conditions.
	MachineDeploymentStatusCondition = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: machinedeploymentSubsystem,
		Name:      "status_condition",
		Help:      "Information of the mcm managed Machinedeployments' status conditions.",
	}, []string{"name", "namespace", "condition"})

	// MachineDeploymentStatusAvailableReplicas Count of the mcm managed Machinedeployments available replicas.
	MachineDeploymentStatusAvailableReplicas = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: machinedeploymentSubsystem,
		Name:      "status_available_replicas",
		Help:      "Count of the mcm managed Machinedeployments available replicas.",
	}, []string{"name", "namespace"})

	// MachineDeploymentStatusUnavailableReplicas Count of the mcm managed Machinedeployments unavailable replicas.
	MachineDeploymentStatusUnavailableReplicas = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: machinedeploymentSubsystem,
		Name:      "status_unavailable_replicas",
		Help:      "Count of the mcm managed Machinedeployments unavailable replicas.",
	}, []string{"name", "namespace"})

	// MachineDeploymentStatusReadyReplicas Count of the mcm managed Machinedeployments ready replicas.
	MachineDeploymentStatusReadyReplicas = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: machinedeploymentSubsystem,
		Name:      "status_ready_replicas",
		Help:      "Count of the mcm managed Machinedeployments ready replicas.",
	}, []string{"name", "namespace"})

	// MachineDeploymentStatusUpdatedReplicas Count of the mcm managed Machinedeployments updated replicas.
	MachineDeploymentStatusUpdatedReplicas = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: machinedeploymentSubsystem,
		Name:      "status_updated_replicas",
		Help:      "Count of the mcm managed Machinedeployments updated replicas.",
	}, []string{"name", "namespace"})

	// MachineDeploymentStatusCollisionCount Mcm managed Machinedeployments collision count.
	MachineDeploymentStatusCollisionCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: machinedeploymentSubsystem,
		Name:      "status_collision_count",
		Help:      "Mcm managed Machinedeployments collision count.",
	}, []string{"name", "namespace"})

	// MachineDeploymentStatusReplicas Count of the mcm managed Machinedeployments replicas.
	MachineDeploymentStatusReplicas = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: machinedeploymentSubsystem,
		Name:      "status_replicas",
		Help:      "Count of the mcm managed Machinedeployments replicas.",
	}, []string{"name", "namespace"})

	// MachineDeploymentStatusFailedMachines Information of the mcm managed Machinedeployments' failed machines.
	MachineDeploymentStatusFailedMachines = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: machinedeploymentSubsystem,
		Name:      "failed_machines",
		Help:      "Information of the mcm managed Machinedeployments' failed machines.",
	}, []string{"name", "namespace", "failed_machine_name", "failed_machine_provider_id", "failed_machine_owner_ref",
		"failed_machine_last_operation_state",
		"failed_machine_last_operation_machine_operation_type"})
)

// variables for subsystem: misc
var (
	// ScrapeFailedCounter is a Prometheus metric, which counts errors during metrics collection.
	ScrapeFailedCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   namespace,
		Subsystem:   miscSubsystem,
		Name:        "scrape_failure_total",
		Help:        "Total count of scrape failures.",
		ConstLabels: map[string]string{"binary": "machine-controller-manager"},
	}, []string{"kind"})
)

func registerMachineSubsystemMetrics() {
	prometheus.MustRegister(StaleMachineCount)
}

func registerMachineSetSubsystemMetrics() {
	prometheus.MustRegister(MachineSetInfo)
	prometheus.MustRegister(MachineSetInfoSpecReplicas)
	prometheus.MustRegister(MachineSetInfoSpecMinReadySeconds)
	prometheus.MustRegister(MachineSetStatusAvailableReplicas)
	prometheus.MustRegister(MachineSetStatusFullyLabelledReplicas)
	prometheus.MustRegister(MachineSetStatusReadyReplicas)
	prometheus.MustRegister(MachineSetStatusReplicas)
	prometheus.MustRegister(MachineSetStatusCondition)
	prometheus.MustRegister(MachineSetStatusFailedMachines)
}

func registerMachineDeploymentSubsystemMetrics() {
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
}

func registerMiscellaneousMetrics() {
	prometheus.MustRegister(ScrapeFailedCounter)
}

func init() {
	registerMachineSubsystemMetrics()
	registerMachineSetSubsystemMetrics()
	registerMachineDeploymentSubsystemMetrics()
	registerMiscellaneousMetrics()
}
