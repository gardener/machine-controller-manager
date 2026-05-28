// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"time"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machineutils"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
)

const (
	namespace         = "mcm"
	machineSubsystem  = "machine"
	cloudAPISubsystem = "cloud_api"
	miscSubsystem     = "misc"
)

// variables for subsystem: machine
var (
	// MachineControllerFrozenDesc is a metric about MachineController's frozen status
	MachineControllerFrozenDesc = prometheus.NewDesc("mcm_machine_controller_frozen", "Frozen status of the machine controller manager.", nil, nil)

	// MachineCountDesc is a metric about machine count of the mcm manages
	MachineCountDesc = prometheus.NewDesc("mcm_machine_items_total", "Count of machines currently managed by the mcm.", nil, nil)

	// MachineCSPhase Current status phase of the Machines currently managed by the mcm.
	MachineCSPhase = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, machineSubsystem, "current_status_phase"),
		"Current status phase of the Machines currently managed by the mcm.",
		[]string{"name", "namespace"},
		nil)

	// MachineInfo Information of the Machines currently managed by the mcm.
	MachineInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: machineSubsystem,
		Name:      "info",
		Help:      "Information of the Machines currently managed by the mcm.",
	}, []string{"name", "namespace", "createdAt",
		"spec_provider_id", "spec_class_api_group", "spec_class_kind", "spec_class_name", "node_name"})

	// MachineStatusCondition Information of the mcm managed Machines' status conditions
	MachineStatusCondition = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: machineSubsystem,
		Name:      "status_condition",
		Help:      "Information of the mcm managed Machines' status conditions.",
	}, []string{"name", "namespace", "condition"})

	// MachineCreateDurationSeconds is the Prometheus gauge metric representing the time duration
	// in seconds to create a Machine of a MachineDeployment.
	MachineCreateDurationSeconds = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: machineSubsystem,
		Name:      "machine_create_duration_seconds",
		Help:      "Duration in seconds to create a Machine of a MachineDeployment.",
	}, []string{"name", "namespace", "machine_deployment"})

	// MachineInitializeDurationSeconds is the Prometheus gauge metric representing the time duration
	// in seconds to initialize a Machine of a MachineDeployment.
	MachineInitializeDurationSeconds = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: machineSubsystem,
		Name:      "machine_initialize_duration_seconds",
		Help:      "Duration in seconds to initialize a Machine of a MachineDeployment.",
	}, []string{"name", "namespace", "machine_deployment"})

	// MachineJoinDurationSeconds is the Prometheus gauge metric representing the time duration
	// in seconds for a Machine of a MachineDeployment to join the cluster.
	MachineJoinDurationSeconds = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: machineSubsystem,
		Name:      "machine_join_duration_seconds",
		Help:      "Duration in seconds for a Machine of a MachineDeployment to join the cluster",
	}, []string{"name", "namespace", "machine_deployment"})

	// MachineDrainDurationSeconds is the Prometheus gauge metric representing the time duration
	// in seconds to drain a Machine of a MachineDeployment.
	MachineDrainDurationSeconds = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: machineSubsystem,
		Name:      "machine_drain_duration_seconds",
		Help:      "Duration in seconds to drain a Machine of a MachineDeployment.",
	}, []string{"name", "namespace", "machine_deployment"})

	// MachineDeleteDurationSeconds is the Prometheus gauge metric representing the time duration
	// in seconds to delete a Machine for a MachineDeployment.
	MachineDeleteDurationSeconds = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: machineSubsystem,
		Name:      "machine_delete_duration_seconds",
		Help:      "Duration in seconds to delete a Machine of a MachineDeployment.",
	}, []string{"name", "namespace", "machine_deployment"})
)

// variables for subsystem: cloud_api
var (
	// APIRequestCount Number of Cloud Service API requests, partitioned by provider, and service.
	APIRequestCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: cloudAPISubsystem,
		Name:      "requests_total",
		Help:      "Number of Cloud Service API requests, partitioned by provider, and service.",
	}, []string{"provider", "service"},
	)

	// APIFailedRequestCount Number of Failed Cloud Service API requests, partitioned by provider, and service.
	APIFailedRequestCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: cloudAPISubsystem,
		Name:      "requests_failed_total",
		Help:      "Number of Failed Cloud Service API requests, partitioned by provider, and service.",
	}, []string{"provider", "service"},
	)

	// APIRequestDuration records duration of all successful provider API calls.
	// This metric can be filtered by provider and service.
	APIRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Subsystem: cloudAPISubsystem,
		Name:      "api_request_duration_seconds",
		Help:      "Time(in seconds) it takes for a provider API request to complete",
	}, []string{"provider", "service"})

	// DriverAPIRequestDuration records duration of all successful driver API calls.
	// This metric can be filtered by provider and operation.
	DriverAPIRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Subsystem: cloudAPISubsystem,
		Name:      "driver_request_duration_seconds",
		Help:      "Total time (in seconds) taken for a driver API request to complete",
	}, []string{"provider", "operation"})

	// DriverFailedAPIRequests records number of failed driver API calls.
	// This metric can be filtered by provider, operation and error code.
	DriverFailedAPIRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: cloudAPISubsystem,
		Name:      "driver_requests_failed_total",
		Help:      "Number of failed Driver API requests, partitioned by provider, operation and error code",
	}, []string{"provider", "operation", "error_code"})
)

// variables for subsystem: misc
var (
	// ScrapeFailedCounter is a Prometheus metric, which counts errors during metrics collection.
	ScrapeFailedCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace:   namespace,
		Subsystem:   miscSubsystem,
		Name:        "scrape_failure_total",
		Help:        "Total count of scrape failures.",
		ConstLabels: map[string]string{"binary": "machine-controller-manager-provider"},
	}, []string{"kind"})
)

// MachineDurations encapsulates duration data for a machine and used as input to update prometheus metrics.
type MachineDurations struct {
	// Create is the duration to create a Machine from the time of its creation timestamp.
	Create time.Duration
	// Create is the duration to initialize a Machine after creation.
	Initialize time.Duration
	// Join is the duration for a Machine to join the cluster ie associated Node becomes Ready
	Join time.Duration
	// Drain is the duration for the machine-controller to drain the Machine or rather the Node associated with a Machine
	Drain time.Duration
	// Delete is the duration for the machine-controller to delete the Machine from the time its deletion timestamp was set.
	Delete time.Duration
}

// UpdateMetricsForMachineDurations updates the prometheus metrics relevant for machine activity durations such as
// create/initialize/join/drain/delete using the values specified in the given [MachineDurations] object.
func UpdateMetricsForMachineDurations(machine *v1alpha1.Machine, newDurations MachineDurations) {
	mcdName := machineutils.GetMachineDeploymentName(machine)
	if mcdName == "" {
		klog.Warningf("Machine %q does not possess 'name' label which is its MachineDeployment name", machine.Name)
		return
	}
	metricLabels := prometheus.Labels{
		"name":               mcdName,
		"namespace":          machine.GetNamespace(),
		"machine_deployment": mcdName,
	}
	metricLabelsStr := labels.FormatLabels(metricLabels)
	if newDurations.Create != 0 {
		numSecs := newDurations.Create.Round(time.Second).Seconds()
		MachineCreateDurationSeconds.With(metricLabels).Set(numSecs)
		klog.V(3).Infof("updated machine_create_duration_seconds metric to %f with labels %s", numSecs, metricLabelsStr)
	}
	if newDurations.Initialize != 0 {
		numSecs := newDurations.Initialize.Round(time.Second).Seconds()
		MachineInitializeDurationSeconds.With(metricLabels).Set(numSecs)
		klog.V(3).Infof("updated machine_initialize_duration_seconds metric to %f with labels %s", numSecs, metricLabelsStr)
	}
	if newDurations.Join != 0 {
		numSecs := newDurations.Join.Round(time.Second).Seconds()
		MachineJoinDurationSeconds.With(metricLabels).Set(numSecs)
		klog.V(3).Infof("updated machine_join_duration_seconds metric to %f with labels %s", numSecs, metricLabelsStr)
	}
	if newDurations.Drain != 0 {
		numSecs := newDurations.Drain.Round(time.Second).Seconds()
		MachineDrainDurationSeconds.With(metricLabels).Set(numSecs)
		klog.V(3).Infof("updated machine_drain_duration_seconds metric to %f with labels %s", numSecs, metricLabelsStr)
	}
	if newDurations.Delete != 0 {
		numSecs := newDurations.Delete.Round(time.Second).Seconds()
		MachineDeleteDurationSeconds.With(metricLabels).Set(numSecs)
		klog.V(3).Infof("updated machine_delete_duration_seconds metric to %f with labels %s", numSecs, metricLabelsStr)
	}
}

func registerMachineSubsystemMetrics() {
	prometheus.MustRegister(MachineInfo)
	prometheus.MustRegister(MachineStatusCondition)
	prometheus.MustRegister(MachineCreateDurationSeconds)
	prometheus.MustRegister(MachineInitializeDurationSeconds)
	prometheus.MustRegister(MachineJoinDurationSeconds)
}

func registerCloudAPISubsystemMetrics() {
	prometheus.MustRegister(APIRequestCount)
	prometheus.MustRegister(APIFailedRequestCount)
	prometheus.MustRegister(APIRequestDuration)
	prometheus.MustRegister(DriverAPIRequestDuration)
	prometheus.MustRegister(DriverFailedAPIRequests)
}

func registerMiscellaneousMetrics() {
	prometheus.MustRegister(ScrapeFailedCounter)
}

func init() {
	registerMachineSubsystemMetrics()
	registerCloudAPISubsystemMetrics()
	registerMiscellaneousMetrics()
}
